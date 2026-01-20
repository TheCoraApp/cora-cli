package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/clairitydev/cora/internal/environment"
	"github.com/clairitydev/cora/internal/filter"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Upload Terraform plan for PR risk assessment",
	Long: `Upload a Terraform plan JSON to Cora for PR risk assessment.

This command analyzes your Terraform plan and provides:
  - Risk scoring based on resource changes
  - Blast radius analysis
  - GitHub PR comments (when configured)

The plan can be provided via stdin (pipe) or from a file.

Environment Auto-Detection:
  When running in Atlantis or GitHub Actions, the CLI automatically detects
  the environment and populates GitHub context (owner, repo, PR number, commit)
  from native environment variables. You can override any auto-detected value
  by explicitly passing the corresponding flag.

Examples:
  # Pipe from terraform show
  terraform show -json tfplan | cora review --workspace my-app-prod

  # Read from file
  cora review --workspace my-app-prod --file plan.json

  # In Atlantis: context is auto-detected, just run:
  terraform show -json tfplan | cora review --workspace my-app-prod

  # With explicit GitHub PR context (overrides auto-detection)
  terraform show -json tfplan | cora review \
    --workspace my-app-prod \
    --github-owner myorg \
    --github-repo myrepo \
    --pr-number 123 \
    --commit-sha abc123

Environment Variables:
  CORA_TOKEN     API token (alternative to --token flag)
  CORA_API_URL   API URL (alternative to --api-url flag)`,
	PreRunE: autoDetectEnvironment,
	RunE:    runReview,
}

var (
	reviewWorkspace string
	reviewPlanFile  string
	reviewSource    string

	// GitHub context for PR comments
	githubOwner string
	githubRepo  string
	prNumber    int
	commitSha   string

	// Filtering flags for review command
	reviewNoFilter     bool
	reviewFilterDryRun bool
	reviewOutputFormat string
)

// autoDetectEnvironment detects CI/CD environment and auto-populates flags
func autoDetectEnvironment(cmd *cobra.Command, args []string) error {
	result := environment.Detect()
	if result == nil {
		LogVerbose("🔍 No CI/CD environment detected, using CLI defaults")
		return nil
	}

	env := result.Environment
	LogVerbose("🔍 Auto-detected: %s", env.Description())

	// Print any warnings
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", warning)
	}

	// Auto-populate source if not explicitly set
	if !cmd.Flags().Changed("source") {
		reviewSource = env.Name()
		LogVerbose("   → source=%s (auto-detected)", reviewSource)
	}

	// Auto-populate workspace if not explicitly set and environment provides one
	if !cmd.Flags().Changed("workspace") && env.Workspace() != "" {
		reviewWorkspace = env.Workspace()
		LogVerbose("   → workspace=%s (auto-detected)", reviewWorkspace)
	}

	// Auto-populate GitHub context if available
	gh := env.GitHubContext()
	if gh != nil {
		if !cmd.Flags().Changed("github-owner") {
			githubOwner = gh.Owner
			LogVerbose("   → github-owner=%s (auto-detected)", githubOwner)
		}
		if !cmd.Flags().Changed("github-repo") {
			githubRepo = gh.Repo
			LogVerbose("   → github-repo=%s (auto-detected)", githubRepo)
		}
		if !cmd.Flags().Changed("pr-number") {
			prNumber = gh.PRNumber
			LogVerbose("   → pr-number=%d (auto-detected)", prNumber)
		}
		if !cmd.Flags().Changed("commit-sha") {
			commitSha = gh.CommitSHA
			LogVerbose("   → commit-sha=%s (auto-detected)", commitSha)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(reviewCmd)

	// Workspace flag - optional when auto-detected from CI/CD environment
	reviewCmd.Flags().StringVarP(&reviewWorkspace, "workspace", "w", "", "Target workspace name (auto-detected in Atlantis/GitHub Actions)")

	// Plan input
	reviewCmd.Flags().StringVarP(&reviewPlanFile, "file", "f", "", "Path to Terraform plan JSON file (reads from stdin if not provided)")
	reviewCmd.Flags().StringVar(&reviewSource, "source", "cli", "Source identifier (auto-detected: 'atlantis', 'github-actions', or 'cli')")

	// GitHub context (optional, for PR comments)
	reviewCmd.Flags().StringVar(&githubOwner, "github-owner", "", "GitHub repository owner (for PR comments)")
	reviewCmd.Flags().StringVar(&githubRepo, "github-repo", "", "GitHub repository name (for PR comments)")
	reviewCmd.Flags().IntVar(&prNumber, "pr-number", 0, "GitHub PR number (for PR comments)")
	reviewCmd.Flags().StringVar(&commitSha, "commit-sha", "", "Git commit SHA (for PR comments)")

	// Filtering flags
	reviewCmd.Flags().BoolVar(&reviewNoFilter, "no-filter", false, "Disable sensitive data filtering")
	reviewCmd.Flags().BoolVar(&reviewFilterDryRun, "filter-dry-run", false, "Show what would be filtered without uploading")
	reviewCmd.Flags().StringVar(&reviewOutputFormat, "output-format", "text", "Output format for dry-run: text or json")
}

// PlanUploadRequest matches the server-side PlanUploadRequest type
type PlanUploadRequest struct {
	Workspace  string                 `json:"workspace"`
	Plan       map[string]interface{} `json:"plan"`
	GitHub     *GitHubContext         `json:"github,omitempty"`
	Source     string                 `json:"source,omitempty"`
	CapturedAt string                 `json:"capturedAt,omitempty"`
}

// GitHubContext contains GitHub PR information for posting comments
type GitHubContext struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	PRNumber  int    `json:"prNumber"`
	CommitSHA string `json:"commitSha"`
}

// PlanUploadResponse matches the server-side PlanUploadResponse type
type PlanUploadResponse struct {
	Success        bool            `json:"success"`
	PlanID         string          `json:"planId"`
	RiskAssessment *RiskAssessment `json:"riskAssessment,omitempty"`
	ViewURL        string          `json:"viewUrl,omitempty"`
	GitHub         *GitHubResult   `json:"github,omitempty"`
	Error          string          `json:"error,omitempty"`
	Message        string          `json:"message,omitempty"`
}

// RiskAssessment contains the risk analysis results
type RiskAssessment struct {
	Score       float64 `json:"score"`
	Level       string  `json:"level"`       // "low", "medium", "high", "critical"
	RuleMatches int     `json:"ruleMatches"` // Number of risk rules triggered
}

// GitHubResult contains the result of GitHub PR comment posting
type GitHubResult struct {
	CommentPosted bool   `json:"commentPosted"`
	CommentURL    string `json:"commentUrl,omitempty"`
}

func runReview(cmd *cobra.Command, args []string) error {
	// Validate workspace is set (either from flag or auto-detection)
	if reviewWorkspace == "" {
		return fmt.Errorf("workspace is required. Use --workspace flag or run in a CI/CD environment (Atlantis/GitHub Actions) for auto-detection")
	}

	// Get authentication token
	authToken, err := getToken()
	if err != nil {
		return err
	}

	// Get API URL
	apiBaseURL := getAPIURL()

	// Fetch service discovery to get endpoints (pass token for account-specific settings)
	discovery, err := FetchServiceDiscovery(apiBaseURL, authToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not fetch service discovery: %v\n", err)
	}

	// Check CLI version against server requirements
	if discovery != nil {
		checkCLIVersionFromDiscovery(discovery)

		// Check if PR risk assessment feature is available
		if !discovery.Features.PRRiskAssessment {
			return fmt.Errorf("PR Risk Assessment feature is not available.\nContact support to enable this feature for your account.")
		}
	}

	// Read plan from file or stdin
	var planData []byte
	if reviewPlanFile != "" {
		planData, err = os.ReadFile(reviewPlanFile)
		if err != nil {
			return fmt.Errorf("failed to read plan file: %w", err)
		}
	} else {
		// Check if stdin has data
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return fmt.Errorf("no input provided. Pipe terraform plan or use --file flag.\n\nExample: terraform show -json tfplan | cora review --workspace my-app")
		}

		planData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	}

	if len(planData) == 0 {
		return fmt.Errorf("empty plan data provided")
	}

	// Parse plan JSON
	var planJSON map[string]interface{}
	if err := json.Unmarshal(planData, &planJSON); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate this looks like a Terraform plan (not state)
	if _, hasResourceChanges := planJSON["resource_changes"]; !hasResourceChanges {
		// Check if this is state instead of plan
		if _, hasResources := planJSON["resources"]; hasResources {
			return fmt.Errorf("this appears to be Terraform state, not a plan.\n\nUse 'terraform show -json tfplan' to output plan JSON, not 'terraform show -json'")
		}
		return fmt.Errorf("invalid Terraform plan: missing 'resource_changes' field.\n\nMake sure you're using 'terraform show -json <planfile>'")
	}

	// Load filter configuration
	filterConfig, configSource, err := filter.GetMergedConfig()
	if err != nil {
		LogVerbose("⚠️  Failed to load filter config: %v", err)
		filterConfig = &filter.MergedConfig{
			OmitResourceTypes:       filter.DefaultOmitResourceTypes,
			OmitAttributes:          filter.DefaultOmitAttributes,
			PreserveAttributes:      []string{},
			HonorTerraformSensitive: true,
		}
		configSource = "defaults"
	}
	LogVerbose("🔒 Filter config source: %s", configSource)

	// Merge with platform settings if available
	if discovery != nil && discovery.Features.SensitiveFiltering.Available {
		filterConfig.MergeWithPlatformSettings(
			discovery.Features.SensitiveFiltering.AdditionalOmitTypes,
			discovery.Features.SensitiveFiltering.AdditionalOmitAttributes,
		)
		LogVerbose("🔒 Merged platform filtering settings")

		// Check if filtering is enforced by the platform
		if reviewNoFilter && discovery.Features.SensitiveFiltering.Enforced {
			return fmt.Errorf("⛔ Filtering is required by your organization's settings. Cannot use --no-filter")
		}
	}

	// Apply filtering to the plan JSON unless disabled
	if !reviewNoFilter {
		LogVerbose("🔒 Applying sensitive data filter to plan...")
		filterResult, err := filter.FilterPlan(planData, filterConfig)
		if err != nil {
			return fmt.Errorf("failed to filter plan: %w", err)
		}

		// Log omissions in verbose mode
		if Verbose {
			filter.PrintVerboseOmissions(filterResult, LogVerbose)
		}

		// Handle dry-run mode
		if reviewFilterDryRun {
			format := filter.OutputFormatText
			if reviewOutputFormat == "json" {
				format = filter.OutputFormatJSON
			}
			return filter.PrintDryRunReport(filterResult, filterConfig, configSource, format)
		}

		// Re-parse the filtered plan
		if err := json.Unmarshal(filterResult.FilteredJSON, &planJSON); err != nil {
			return fmt.Errorf("failed to parse filtered plan: %w", err)
		}
		LogVerbose("📊 Filtered plan size: %d bytes (original: %d bytes)",
			len(filterResult.FilteredJSON), len(planData))
	} else {
		LogVerbose("⚠️  Sensitive data filtering disabled")
		if reviewFilterDryRun {
			fmt.Println("ℹ️  Dry-run has no effect when --no-filter is used")
			return nil
		}
	}

	// Build request payload
	request := PlanUploadRequest{
		Workspace:  reviewWorkspace,
		Plan:       planJSON,
		Source:     reviewSource,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Add GitHub context if all required fields are provided
	if githubOwner != "" && githubRepo != "" && prNumber > 0 && commitSha != "" {
		request.GitHub = &GitHubContext{
			Owner:     githubOwner,
			Repo:      githubRepo,
			PRNumber:  prNumber,
			CommitSHA: commitSha,
		}
	} else if githubOwner != "" || githubRepo != "" || prNumber > 0 || commitSha != "" {
		// Some but not all GitHub fields provided
		fmt.Fprintf(os.Stderr, "Warning: Incomplete GitHub context. All of --github-owner, --github-repo, --pr-number, and --commit-sha are required for PR comments.\n")
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	// Build upload URL using discovered endpoint
	planEndpoint := discovery.Endpoints.PlanUpload
	if planEndpoint == "" {
		planEndpoint = "/api/plans/upload"
	}
	uploadURL := GetEndpointURL(apiBaseURL, planEndpoint)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	req.Header.Set("User-Agent", fmt.Sprintf("cora-cli/%s", Version))
	req.Header.Set("X-Cora-CLI-Version", Version)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload plan: %w", err)
	}
	defer resp.Body.Close()

	// Check for CLI version warnings/errors in response headers
	checkVersionHeaders(resp)

	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var result PlanUploadResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Display results
		fmt.Println("✅ Plan analyzed successfully")
		fmt.Printf("   Plan ID: %s\n", result.PlanID)

		if result.RiskAssessment != nil {
			fmt.Printf("\n📊 Risk Assessment\n")
			fmt.Printf("   Level: %s\n", formatRiskLevel(result.RiskAssessment.Level))
			fmt.Printf("   Score: %.1f\n", result.RiskAssessment.Score)
			if result.RiskAssessment.RuleMatches > 0 {
				fmt.Printf("   Rules triggered: %d\n", result.RiskAssessment.RuleMatches)
			}
		}

		if result.ViewURL != "" {
			fmt.Printf("\n🔗 View details: %s\n", result.ViewURL)
		}

		if result.GitHub != nil && result.GitHub.CommentPosted {
			fmt.Printf("\n💬 GitHub comment posted: %s\n", result.GitHub.CommentURL)
		}

		return nil

	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed. Check your API token.\n\nGet a token at: %s/settings/tokens", apiBaseURL)

	case http.StatusForbidden:
		var errResp PlanUploadResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("access denied: %s", errResp.Message)
		}
		return fmt.Errorf("access denied. PR Risk Assessment may not be enabled for your account.")

	case http.StatusBadRequest:
		var errResp PlanUploadResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			if errResp.Error != "" {
				return fmt.Errorf("plan analysis failed: %s", errResp.Error)
			}
		}
		return fmt.Errorf("plan analysis failed: invalid request")

	case 426: // Upgrade Required
		return handleUpgradeRequired(respBody, apiBaseURL)

	default:
		return fmt.Errorf("plan analysis failed with status %d: %s", resp.StatusCode, string(respBody))
	}
}

// formatRiskLevel returns a formatted risk level with emoji
func formatRiskLevel(level string) string {
	switch level {
	case "critical":
		return "🔴 Critical"
	case "high":
		return "🟠 High"
	case "medium":
		return "🟡 Medium"
	case "low":
		return "🟢 Low"
	default:
		return level
	}
}
