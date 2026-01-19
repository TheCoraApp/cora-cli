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
	apiURL  string
	token   string
	Verbose bool
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
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose output")
}

// LogVerbose prints a message to stderr if verbose mode is enabled
func LogVerbose(format string, args ...interface{}) {
	if Verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// getToken returns the API token from flag, env var, or config file (in that order)
func getToken() (string, error) {
	// 1. Check flag
	if token != "" {
		LogVerbose("🔑 Using token from --token flag")
		return token, nil
	}

	// 2. Check environment variable
	if envToken := os.Getenv("CORA_TOKEN"); envToken != "" {
		LogVerbose("🔑 Using token from CORA_TOKEN environment variable")
		return envToken, nil
	}

	// 3. Check config file
	cfg, err := LoadConfig()
	if err == nil && cfg.Token != "" {
		LogVerbose("🔑 Using token from config file")
		return cfg.Token, nil
	}

	return "", fmt.Errorf("no API token provided. Use --token flag, CORA_TOKEN env var, or run 'cora configure'")
}

// getAPIURL returns the API URL from flag, env var, or config file (in that order)
func getAPIURL() string {
	// 1. Check flag
	if apiURL != "" {
		LogVerbose("🌐 Using API URL from --api-url flag: %s", apiURL)
		return apiURL
	}

	// 2. Check environment variable
	if envURL := os.Getenv("CORA_API_URL"); envURL != "" {
		LogVerbose("🌐 Using API URL from CORA_API_URL environment variable: %s", envURL)
		return envURL
	}

	// 3. Check config file
	cfg, err := LoadConfig()
	if err == nil && cfg.APIURL != "" {
		LogVerbose("🌐 Using API URL from config file: %s", cfg.APIURL)
		return cfg.APIURL
	}

	// 4. Default
	defaultURL := "https://thecora.app"
	LogVerbose("🌐 Using default API URL: %s", defaultURL)
	return defaultURL
}
