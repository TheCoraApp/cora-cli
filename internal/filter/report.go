package filter

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// OutputFormat specifies the format for dry-run output
type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
)

// DryRunReport is the JSON-serializable report for machine-readable output
type DryRunReport struct {
	Omissions []OmittedField `json:"omissions"`
	Summary   FilterSummary  `json:"summary"`
	Config    ConfigReport   `json:"config"`
}

// ConfigReport describes the configuration used for filtering
type ConfigReport struct {
	Source             string   `json:"source"`
	OmitResourceTypes  []string `json:"omit_resource_types"`
	OmitAttributeCount int      `json:"omit_attribute_pattern_count"`
	PreserveAttributes []string `json:"preserve_attributes,omitempty"`
}

// PrintDryRunReport outputs the filtering results without uploading
func PrintDryRunReport(result *FilterResult, config *MergedConfig, configSource string, format OutputFormat) error {
	switch format {
	case OutputFormatJSON:
		return printJSONReport(result, config, configSource)
	case OutputFormatText:
		return printTextReport(result, config, configSource)
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
}

func printJSONReport(result *FilterResult, config *MergedConfig, configSource string) error {
	report := DryRunReport{
		Omissions: result.Omissions,
		Summary:   result.Summary,
		Config: ConfigReport{
			Source:             configSource,
			OmitResourceTypes:  config.OmitResourceTypes,
			OmitAttributeCount: len(config.OmitAttributes),
			PreserveAttributes: config.PreserveAttributes,
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func printTextReport(result *FilterResult, config *MergedConfig, configSource string) error {
	fmt.Println()
	fmt.Println("🔒 Sensitive Data Filter - Dry Run Report")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()

	// Summary
	fmt.Println("📊 Summary")
	fmt.Printf("   Resources: %d total, %d omitted\n",
		result.Summary.TotalResources, result.Summary.OmittedResources)
	fmt.Printf("   Attributes: %d total, %d omitted\n",
		result.Summary.TotalAttributes, result.Summary.OmittedAttributes)
	fmt.Printf("   Config source: %s\n", configSource)

	// Show if platform settings are active
	hasPlatformSettings := len(config.PlatformOmitResourceTypes) > 0 || len(config.PlatformOmitAttributes) > 0
	if hasPlatformSettings {
		fmt.Printf("   Organization settings: active\n")
	}
	fmt.Println()

	if len(result.Omissions) == 0 {
		fmt.Println("✅ No sensitive data detected")
		fmt.Println()
		return nil
	}

	// Separate omissions into categories
	platformResourceOmissions := []OmittedField{}
	platformAttributeOmissions := []OmittedField{}
	dataSourceOmissions := []OmittedField{}
	resourceOmissions := []OmittedField{}
	attributeOmissions := []OmittedField{}

	for _, o := range result.Omissions {
		if o.FromPlatform {
			if o.Type == "resource" {
				platformResourceOmissions = append(platformResourceOmissions, o)
			} else {
				platformAttributeOmissions = append(platformAttributeOmissions, o)
			}
		} else if o.Type == "resource" && o.Reason == "data source lookup omitted" {
			dataSourceOmissions = append(dataSourceOmissions, o)
		} else {
			if o.Type == "resource" {
				resourceOmissions = append(resourceOmissions, o)
			} else {
				attributeOmissions = append(attributeOmissions, o)
			}
		}
	}

	// Show platform settings first (if any)
	if len(platformResourceOmissions) > 0 || len(platformAttributeOmissions) > 0 {
		fmt.Println("🏢 Omitted by Organization Settings")
		fmt.Println("   These filters are configured in your Cora account settings.")
		fmt.Println()

		if len(platformResourceOmissions) > 0 {
			for _, o := range platformResourceOmissions {
				fmt.Printf("   ⛔ %s\n", o.Path)
				fmt.Printf("      %s\n", o.Reason)
			}
		}

		if len(platformAttributeOmissions) > 0 {
			grouped := groupAttributeOmissions(platformAttributeOmissions)
			printGroupedAttributes(grouped, 10)
		}
		fmt.Println()
	}

	// Data source omissions - show as a simple summary
	if len(dataSourceOmissions) > 0 {
		fmt.Printf("📂 Omitted %d data source lookups (read-only queries, not infrastructure)\n", len(dataSourceOmissions))
		fmt.Println()
	}

	// Omitted resources (non-platform, non-data-source)
	if len(resourceOmissions) > 0 {
		fmt.Println("🗑️  Omitted Resources")
		for _, o := range resourceOmissions {
			fmt.Printf("   ⛔ %s\n", o.Path)
			fmt.Printf("      %s\n", o.Reason)
		}
		fmt.Println()
	}

	// Omitted attributes - grouped by base path (without array indices)
	if len(attributeOmissions) > 0 {
		fmt.Println("🔐 Omitted Attributes")
		grouped := groupAttributeOmissions(attributeOmissions)
		printGroupedAttributes(grouped, 20)
		fmt.Println()
	}

	fmt.Println("ℹ️  Use --no-filter to upload without filtering (if allowed by your organization)")
	fmt.Println()

	return nil
}

// printGroupedAttributes prints grouped attribute omissions with a limit
func printGroupedAttributes(grouped map[string]groupedOmission, maxShow int) {
	// Sort by count descending, then by path
	sortedPaths := make([]string, 0, len(grouped))
	for path := range grouped {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Slice(sortedPaths, func(i, j int) bool {
		ci, cj := grouped[sortedPaths[i]], grouped[sortedPaths[j]]
		if ci.count != cj.count {
			return ci.count > cj.count
		}
		return sortedPaths[i] < sortedPaths[j]
	})

	shown := 0
	for _, path := range sortedPaths {
		if shown >= maxShow {
			remaining := len(sortedPaths) - maxShow
			fmt.Printf("   ... and %d more attribute groups\n", remaining)
			break
		}

		info := grouped[path]
		if info.count > 1 {
			fmt.Printf("   🚫 %s (%d occurrences)\n", path, info.count)
		} else {
			// Show the original path for single occurrences
			fmt.Printf("   🚫 %s\n", info.originalPath)
		}
		fmt.Printf("      %s\n", info.reason)
		shown++
	}
}

// groupedOmission holds information about grouped omissions
type groupedOmission struct {
	count        int
	reason       string
	originalPath string // For single occurrences, keep the original
}

// groupAttributeOmissions groups attribute omissions by their base path,
// collapsing array indices like [0], [1], etc. into [*]
func groupAttributeOmissions(omissions []OmittedField) map[string]groupedOmission {
	// Regex to match array indices like [0], [1], ["key"], etc.
	indexPattern := regexp.MustCompile(`\[\d+\]|\["[^"]+"\]`)

	grouped := make(map[string]groupedOmission)

	for _, o := range omissions {
		// Normalize path by replacing indices with [*]
		normalizedPath := indexPattern.ReplaceAllString(o.Path, "[*]")

		if existing, ok := grouped[normalizedPath]; ok {
			existing.count++
			grouped[normalizedPath] = existing
		} else {
			grouped[normalizedPath] = groupedOmission{
				count:        1,
				reason:       o.Reason,
				originalPath: o.Path,
			}
		}
	}

	return grouped
}

// PrintVerboseOmissions prints omission details to stderr for verbose mode
func PrintVerboseOmissions(result *FilterResult, logFunc func(string, ...interface{})) {
	if len(result.Omissions) == 0 {
		logFunc("🔒 No sensitive data detected")
		return
	}

	logFunc("🔒 Filtering sensitive data: %d resources omitted, %d attributes omitted",
		result.Summary.OmittedResources, result.Summary.OmittedAttributes)

	// Only show first few in verbose mode
	maxShow := 5
	for i, o := range result.Omissions {
		if i >= maxShow {
			remaining := len(result.Omissions) - maxShow
			logFunc("   ... and %d more omissions", remaining)
			break
		}
		emoji := "🗑️"
		if o.Type == "attribute" {
			emoji = "🚫"
		}
		logFunc("   %s %s", emoji, o.Path)
	}
}
