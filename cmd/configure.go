package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure Cora CLI credentials",
	Long: `Configure stores your Cora API token locally for future use.

The token is stored in ~/.config/cora/credentials.json with secure permissions (0600).

You can create an API token at https://thecora.app/settings/tokens

Example:
  cora configure --token YOUR_API_TOKEN

Or interactively:
  cora configure`,
	RunE: runConfigure,
}

var (
	configToken  string
	configAPIURL string
)

func init() {
	rootCmd.AddCommand(configureCmd)
	configureCmd.Flags().StringVar(&configToken, "token", "", "API token to store")
	configureCmd.Flags().StringVar(&configAPIURL, "api-url", "", "API URL to store (default: https://thecora.app)")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	// Load existing config
	cfg, err := LoadConfig()
	if err != nil {
		cfg = &Config{}
	}

	// Get token from flag or prompt
	tokenToStore := configToken
	if tokenToStore == "" {
		fmt.Print("Enter your Cora API token: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		tokenToStore = strings.TrimSpace(input)
	}

	if tokenToStore == "" {
		return fmt.Errorf("token cannot be empty")
	}

	cfg.Token = tokenToStore

	// Set API URL if provided
	if configAPIURL != "" {
		cfg.APIURL = configAPIURL
	}

	// Save config
	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	path, _ := configPath()
	fmt.Printf("Configuration saved to %s\n", path)
	fmt.Println("You can now use 'cora upload' without the --token flag.")

	return nil
}
