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

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload Terraform state to Cora",
	Long: `Upload reads Terraform state JSON and uploads it to Cora.

The state can be provided via stdin (pipe) or from a file.

Environment Auto-Detection:
  When running in Atlantis or GitHub Actions, the CLI automatically detects
  the environment and can auto-populate the workspace name from native
  environment variables. You can override any auto-detected value by
  explicitly passing the corresponding flag.

Examples:
  # Pipe from terraform show
  terraform show -json | cora upload --workspace my-app-prod

  # Read from file
  cora upload --workspace my-app-prod --file terraform.tfstate.json

  # In Atlantis: workspace is auto-constructed from PROJECT_NAME and WORKSPACE
  terraform show -json | cora upload

  # With explicit token
  terraform show -json | cora upload --workspace my-app-prod --token YOUR_TOKEN

Environment Variables:
  CORA_TOKEN     API token (alternative to --token flag)
  CORA_API_URL   API URL (alternative to --api-url flag)`,
	PreRunE: autoDetectUploadEnvironment,
	RunE:    runUpload,
}

var (
	workspace    string
	stateFile    string
	uploadSource string
	noFilter     bool
	filterDryRun bool
	outputFormat string
)

// autoDetectUploadEnvironment detects CI/CD environment and auto-populates flags for upload
func autoDetectUploadEnvironment(cmd *cobra.Command, args []string) error {
	result := environment.Detect()
	if result == nil {
		LogVerbose("🔍 No CI/CD environment detected, using CLI defaults")
		return nil
	}

	env := result.Environment
	LogVerbose("🔍 Auto-detected: %s", env.Description())

	// Print any warnings (for upload, we don't need PR context warnings)
	// since upload doesn't post PR comments

	// Auto-populate source if not explicitly set
	if !cmd.Flags().Changed("source") {
		uploadSource = env.Name()
		LogVerbose("   → source=%s (auto-detected)", uploadSource)
	}

	// Auto-populate workspace if not explicitly set and environment provides one
	if !cmd.Flags().Changed("workspace") && env.Workspace() != "" {
		workspace = env.Workspace()
		LogVerbose("   → workspace=%s (auto-detected)", workspace)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Target workspace name (auto-detected in Atlantis/GitHub Actions)")
	uploadCmd.Flags().StringVarP(&stateFile, "file", "f", "", "Path to Terraform state file (reads from stdin if not provided)")
	uploadCmd.Flags().StringVar(&uploadSource, "source", "cli", "Source identifier (auto-detected: 'atlantis', 'github-actions', or 'cli')")
	uploadCmd.Flags().BoolVar(&noFilter, "no-filter", false, "Disable sensitive data filtering")
	uploadCmd.Flags().BoolVar(&filterDryRun, "filter-dry-run", false, "Show what would be filtered without uploading")
	uploadCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format for dry-run: text or json")
}

func runUpload(cmd *cobra.Command, args []string) error {
	// Validate workspace is set (either from flag or auto-detection)
	if workspace == "" {
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
		// Non-fatal: continue with defaults
		fmt.Fprintf(os.Stderr, "Warning: Could not fetch service discovery: %v\n", err)
	}

	// Check CLI version against server requirements
	if discovery != nil {
		checkCLIVersionFromDiscovery(discovery)
	}

	// Read state from file or stdin
	var stateData []byte
	if stateFile != "" {
		stateData, err = os.ReadFile(stateFile)
		if err != nil {
			return fmt.Errorf("failed to read state file: %w", err)
		}
	} else {
		// Check if stdin has data
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return fmt.Errorf("no input provided. Pipe terraform state or use --file flag.\n\nExample: terraform show -json | cora upload --workspace my-app")
		}

		stateData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	}

	if len(stateData) == 0 {
		return fmt.Errorf("empty state data provided")
	}

	// Validate JSON
	var stateJSON map[string]interface{}
	if err := json.Unmarshal(stateData, &stateJSON); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check for required Terraform state fields
	if _, hasVersion := stateJSON["version"]; !hasVersion {
		return fmt.Errorf("invalid Terraform state: missing 'version' field")
	}
	if _, hasResources := stateJSON["resources"]; !hasResources {
		return fmt.Errorf("invalid Terraform state: missing 'resources' field")
	}

	// Load filter configuration
	filterConfig, configSource, err := filter.GetMergedConfig()
	if err != nil {
		LogVerbose("⚠️  Failed to load filter config: %v", err)
		// Continue with defaults
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
		if noFilter && discovery.Features.SensitiveFiltering.Enforced {
			return fmt.Errorf("⛔ Filtering is required by your organization's settings. Cannot use --no-filter")
		}
	}

	// Apply filtering unless disabled
	var uploadData []byte
	sensitiveFiltered := false
	if noFilter {
		LogVerbose("⚠️  Sensitive data filtering disabled")
		uploadData = stateData
	} else {
		LogVerbose("🔒 Applying sensitive data filter...")
		filterResult, err := filter.Filter(stateData, filterConfig)
		if err != nil {
			return fmt.Errorf("failed to filter state: %w", err)
		}

		// Log omissions in verbose mode
		if Verbose {
			filter.PrintVerboseOmissions(filterResult, LogVerbose)
		}

		// Handle dry-run mode
		if filterDryRun {
			// Suppress verbose output for JSON format
			format := filter.OutputFormatText
			if outputFormat == "json" {
				format = filter.OutputFormatJSON
			}
			return filter.PrintDryRunReport(filterResult, filterConfig, configSource, format)
		}

		uploadData = filterResult.FilteredJSON
		sensitiveFiltered = true
		LogVerbose("📊 Filtered state size: %d bytes (original: %d bytes)",
			len(uploadData), len(stateData))
	}

	// Build upload URL using discovered endpoint
	stateEndpoint := discovery.Endpoints.StateUpload
	if stateEndpoint == "" {
		stateEndpoint = "/api/terraform-state"
	}
	uploadURL := fmt.Sprintf("%s?workspace=%s", GetEndpointURL(apiBaseURL, stateEndpoint), workspace)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	LogVerbose("📤 POST %s", uploadURL)
	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(uploadData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	req.Header.Set("User-Agent", fmt.Sprintf("cora-cli/%s", Version))
	req.Header.Set("X-Cora-CLI-Version", Version)
	req.Header.Set("X-Cora-Source", uploadSource)
	if sensitiveFiltered {
		req.Header.Set("X-Cora-Sensitive-Filtered", "true")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload state: %w", err)
	}
	defer resp.Body.Close()

	LogVerbose("📥 Response: %s", resp.Status)

	// Check for CLI version warnings/errors in response headers
	checkVersionHeaders(resp)

	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var result map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err == nil {
			if msg, ok := result["message"].(string); ok {
				fmt.Println(msg)
			} else {
				fmt.Printf("State uploaded successfully to workspace '%s'\n", workspace)
			}
			if resourceCount, ok := result["resourceCount"].(float64); ok {
				fmt.Printf("Resources: %.0f\n", resourceCount)
			}
		} else {
			fmt.Printf("State uploaded successfully to workspace '%s'\n", workspace)
		}
		return nil

	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed. Check your API token.\n\nGet a token at: %s/settings/tokens", apiBaseURL)

	case http.StatusForbidden:
		return fmt.Errorf("access denied. Your token may not have permission for this workspace.")

	case http.StatusBadRequest:
		var errResp map[string]interface{}
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			if errMsg, ok := errResp["error"].(string); ok {
				return fmt.Errorf("upload failed: %s", errMsg)
			}
		}
		return fmt.Errorf("upload failed: invalid request")

	case 426: // Upgrade Required
		return handleUpgradeRequired(respBody, apiBaseURL)

	default:
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}
}
