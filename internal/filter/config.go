package filter

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FilterConfig represents the filtering configuration from .cora.yaml
type FilterConfig struct {
	Version   int                    `yaml:"version"`
	Filtering FilteringConfigSection `yaml:"filtering"`
}

// FilteringConfigSection contains the filtering-specific settings
type FilteringConfigSection struct {
	// OmitResourceTypes are additional resource types to omit entirely (merged with defaults)
	OmitResourceTypes []string `yaml:"omit_resource_types"`

	// OmitAttributes are additional attribute patterns to omit (merged with defaults)
	OmitAttributes []string `yaml:"omit_attributes"`

	// PreserveAttributes are attribute patterns to never omit (overrides defaults)
	PreserveAttributes []string `yaml:"preserve_attributes"`

	// HonorTerraformSensitive controls whether to use Terraform's sensitive_attributes
	// Defaults to true if not specified
	HonorTerraformSensitive *bool `yaml:"honor_terraform_sensitive"`

	// OmitDataSources controls whether to omit data source lookups entirely
	// Defaults to true if not specified
	OmitDataSources *bool `yaml:"omit_data_sources"`
}

// MergedConfig represents the final merged configuration with defaults
type MergedConfig struct {
	OmitResourceTypes       []string
	OmitAttributes          []string
	PreserveAttributes      []string
	HonorTerraformSensitive bool
	OmitDataSources         bool

	// Platform-specific settings (tracked separately for reporting)
	PlatformOmitResourceTypes []string
	PlatformOmitAttributes    []string
}

// LoadConfig searches for .cora.yaml in the current directory and parent directories,
// then merges with defaults. Returns nil if no config file found.
func LoadConfig() (*FilterConfig, error) {
	configPath, err := findConfigFile()
	if err != nil {
		return nil, err
	}
	if configPath == "" {
		return nil, nil // No config file found, not an error
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg FilterConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// findConfigFile searches for .cora.yaml starting from cwd and walking up.
func findConfigFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		configPath := filepath.Join(dir, ".cora.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Also check .cora.yml
		configPath = filepath.Join(dir, ".cora.yml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, no config found
			return "", nil
		}
		dir = parent
	}
}

// GetMergedConfig loads the config file (if exists) and merges with defaults.
func GetMergedConfig() (*MergedConfig, string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, "", err
	}

	merged := &MergedConfig{
		OmitResourceTypes:       append([]string{}, DefaultOmitResourceTypes...),
		OmitAttributes:          append([]string{}, DefaultOmitAttributes...),
		PreserveAttributes:      []string{},
		HonorTerraformSensitive: true,
		OmitDataSources:         true,
	}

	configSource := "defaults"

	if cfg != nil {
		configSource = ".cora.yaml"

		// Merge additional resource types
		if len(cfg.Filtering.OmitResourceTypes) > 0 {
			merged.OmitResourceTypes = append(merged.OmitResourceTypes, cfg.Filtering.OmitResourceTypes...)
		}

		// Merge additional attributes
		if len(cfg.Filtering.OmitAttributes) > 0 {
			merged.OmitAttributes = append(merged.OmitAttributes, cfg.Filtering.OmitAttributes...)
		}

		// Set preserve attributes
		if len(cfg.Filtering.PreserveAttributes) > 0 {
			merged.PreserveAttributes = cfg.Filtering.PreserveAttributes
		}

		// Honor Terraform sensitive
		if cfg.Filtering.HonorTerraformSensitive != nil {
			merged.HonorTerraformSensitive = *cfg.Filtering.HonorTerraformSensitive
		}

		// Omit data sources
		if cfg.Filtering.OmitDataSources != nil {
			merged.OmitDataSources = *cfg.Filtering.OmitDataSources
		}
	}

	return merged, configSource, nil
}

// MergeWithPlatformSettings merges the current config with platform-provided settings.
// Platform settings for additional patterns are additive.
func (m *MergedConfig) MergeWithPlatformSettings(additionalTypes, additionalAttributes []string) {
	if len(additionalTypes) > 0 {
		m.PlatformOmitResourceTypes = additionalTypes
		m.OmitResourceTypes = append(m.OmitResourceTypes, additionalTypes...)
	}
	if len(additionalAttributes) > 0 {
		m.PlatformOmitAttributes = additionalAttributes
		m.OmitAttributes = append(m.OmitAttributes, additionalAttributes...)
	}
}
