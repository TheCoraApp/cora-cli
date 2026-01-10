package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via ldflags
	Version = "dev"

	// Global flags
	apiURL string
	token  string
)

var rootCmd = &cobra.Command{
	Use:   "cora",
	Short: "Cora CLI - Upload Terraform state to Cora",
	Long: `Cora CLI is a command-line tool for uploading Terraform state to Cora.

It integrates seamlessly with Atlantis and other CI/CD workflows, allowing you
to keep your infrastructure visualizations up-to-date automatically.

Example usage:
  terraform show -json | cora upload --workspace my-app-prod

For more information, visit https://thecora.app/docs/atlantis`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Cora API URL (default: https://thecora.app)")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "API token (or set CORA_TOKEN env var)")
}

// getToken returns the API token from flag, env var, or config file (in that order)
func getToken() (string, error) {
	// 1. Check flag
	if token != "" {
		return token, nil
	}

	// 2. Check environment variable
	if envToken := os.Getenv("CORA_TOKEN"); envToken != "" {
		return envToken, nil
	}

	// 3. Check config file
	cfg, err := LoadConfig()
	if err == nil && cfg.Token != "" {
		return cfg.Token, nil
	}

	return "", fmt.Errorf("no API token provided. Use --token flag, CORA_TOKEN env var, or run 'cora configure'")
}

// getAPIURL returns the API URL from flag, env var, or config file (in that order)
func getAPIURL() string {
	// 1. Check flag
	if apiURL != "" {
		return apiURL
	}

	// 2. Check environment variable
	if envURL := os.Getenv("CORA_API_URL"); envURL != "" {
		return envURL
	}

	// 3. Check config file
	cfg, err := LoadConfig()
	if err == nil && cfg.APIURL != "" {
		return cfg.APIURL
	}

	// 4. Default
	return "https://thecora.app"
}
