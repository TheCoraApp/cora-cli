package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	initForce    bool
	initComments bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .cora.yaml configuration file",
	Long: `Initialize a .cora.yaml configuration file in the current directory.

This creates a configuration file with default settings and helpful comments
explaining each option. Use this as a starting point for customizing how
sensitive data is filtered from your Terraform state.

Examples:
  # Create a new config file with comments
  cora init

  # Overwrite an existing config file
  cora init --force

  # Create a minimal config without comments
  cora init --minimal`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing .cora.yaml file")
	initCmd.Flags().BoolVar(&initComments, "minimal", false, "Generate minimal config without comments")
}

func runInit(cmd *cobra.Command, args []string) error {
	configPath := filepath.Join(".", ".cora.yaml")

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf("config file already exists at %s\nUse --force to overwrite", configPath)
	}

	var content string
	if initComments {
		content = generateMinimalConfig()
	} else {
		content = generateFullConfig()
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Created %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and customize the filtering rules")
	fmt.Println("  2. Commit the file to your repository")
	fmt.Println("  3. Run `cora upload --filter-dry-run` to preview filtering")
	fmt.Println()
	fmt.Println("📚 Documentation: https://thecora.app/docs/configuration")

	return nil
}

func generateMinimalConfig() string {
	return `version: 1

filtering:
  omit_resource_types: []
  omit_attributes: []
  preserve_attributes: []
  honor_terraform_sensitive: true
  omit_data_sources: true
`
}

func generateFullConfig() string {
	return `# Cora CLI Configuration
# https://thecora.app/docs/configuration
#
# This file customizes how sensitive data is filtered from your Terraform
# state before uploading to Cora. Place this file in your Terraform project
# root or any parent directory.

version: 1

filtering:
  # ─────────────────────────────────────────────────────────────────────────
  # Additional resource types to omit entirely (merged with built-in defaults)
  # ─────────────────────────────────────────────────────────────────────────
  # These resources will be completely removed from uploads. Use this for
  # resource types that inherently contain sensitive data.
  #
  # Built-in defaults include:
  #   - aws_secretsmanager_secret_version
  #   - aws_ssm_parameter
  #   - random_password, random_string
  #   - tls_private_key, acme_certificate
  #   - vault_generic_secret, vault_kv_secret, vault_kv_secret_v2
  #   - azurerm_key_vault_secret, azurerm_key_vault_key
  #   - google_secret_manager_secret_version
  #
  omit_resource_types: []
    # - custom_secret_resource
    # - my_internal_credential_store

  # ─────────────────────────────────────────────────────────────────────────
  # Additional attribute patterns to omit (merged with built-in defaults)
  # ─────────────────────────────────────────────────────────────────────────
  # Attributes matching these patterns will be removed from all resources.
  # Patterns are matched as substrings (case-insensitive).
  #
  # Built-in defaults include:
  #   - password, master_password, admin_password
  #   - secret, secret_string, api_key, api_secret
  #   - token, auth_token, access_token
  #   - private_key, private_key_pem
  #   - access_key, secret_key, secret_access_key
  #   - credential, credentials
  #   - connection_string, connection_url
  #
  omit_attributes: []
    # - internal_api_key
    # - my_custom_secret_field

  # ─────────────────────────────────────────────────────────────────────────
  # Attributes to preserve (overrides defaults)
  # ─────────────────────────────────────────────────────────────────────────
  # Use this to keep attributes that would otherwise be filtered. For example,
  # if you have a non-sensitive field that happens to match a pattern.
  #
  preserve_attributes: []
    # - public_connection_string
    # - password_policy_name

  # ─────────────────────────────────────────────────────────────────────────
  # Honor Terraform's sensitive markers
  # ─────────────────────────────────────────────────────────────────────────
  # When true (default), the CLI also filters attributes that Terraform has
  # marked as sensitive via the sensitive_attributes field in state.
  #
  honor_terraform_sensitive: true

  # ─────────────────────────────────────────────────────────────────────────
  # Omit data sources
  # ─────────────────────────────────────────────────────────────────────────
  # When true (default), data source lookups are omitted from uploads.
  # Data sources are read-only references and typically not needed for
  # visualization. Set to false if you want to include them.
  #
  omit_data_sources: true
`
}
