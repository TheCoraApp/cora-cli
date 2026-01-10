package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload Terraform state to Cora",
	Long: `Upload reads Terraform state JSON and uploads it to Cora.

The state can be provided via stdin (pipe) or from a file.

Examples:
  # Pipe from terraform show
  terraform show -json | cora upload --workspace my-app-prod

  # Read from file
  cora upload --workspace my-app-prod --file terraform.tfstate.json

  # With explicit token
  terraform show -json | cora upload --workspace my-app-prod --token YOUR_TOKEN

Environment Variables:
  CORA_TOKEN     API token (alternative to --token flag)
  CORA_API_URL   API URL (alternative to --api-url flag)`,
	RunE: runUpload,
}

var (
	workspace string
	stateFile string
)

func init() {
	rootCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Target workspace name (required)")
	uploadCmd.Flags().StringVarP(&stateFile, "file", "f", "", "Path to Terraform state file (reads from stdin if not provided)")
	uploadCmd.MarkFlagRequired("workspace")
}

func runUpload(cmd *cobra.Command, args []string) error {
	// Get authentication token
	authToken, err := getToken()
	if err != nil {
		return err
	}

	// Get API URL
	apiBaseURL := getAPIURL()

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

	// Upload to Cora
	uploadURL := fmt.Sprintf("%s/api/terraform-state?workspace=%s", strings.TrimSuffix(apiBaseURL, "/"), workspace)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(stateData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	req.Header.Set("User-Agent", fmt.Sprintf("cora-cli/%s", Version))
	req.Header.Set("X-Cora-CLI-Version", Version)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload state: %w", err)
	}
	defer resp.Body.Close()

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
