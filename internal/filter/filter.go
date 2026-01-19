package filter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OmittedField represents a field that was omitted from the state
type OmittedField struct {
	Path         string `json:"path"`                    // Full path to the field (e.g., "aws_db_instance.main.password")
	Reason       string `json:"reason"`                  // Why it was omitted
	Type         string `json:"type"`                    // "resource" or "attribute"
	FromPlatform bool   `json:"from_platform,omitempty"` // True if this came from platform/org settings
}

// FilterResult contains the filtered state and metadata about omissions
type FilterResult struct {
	FilteredJSON []byte         `json:"-"`         // The filtered state JSON
	Omissions    []OmittedField `json:"omissions"` // List of omitted fields
	Summary      FilterSummary  `json:"summary"`   // Summary statistics
}

// FilterSummary contains aggregate statistics about the filtering
type FilterSummary struct {
	TotalResources    int `json:"total_resources"`
	OmittedResources  int `json:"omitted_resources"`
	TotalAttributes   int `json:"total_attributes"`
	OmittedAttributes int `json:"omitted_attributes"`
}

// TerraformState represents the structure of a Terraform state file
type TerraformState struct {
	Version          int                    `json:"version"`
	TerraformVersion string                 `json:"terraform_version"`
	Serial           int                    `json:"serial"`
	Lineage          string                 `json:"lineage"`
	Outputs          map[string]interface{} `json:"outputs"`
	Resources        []Resource             `json:"resources"`
}

// Resource represents a Terraform resource in state
type Resource struct {
	Module    string     `json:"module,omitempty"`
	Mode      string     `json:"mode"`
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Provider  string     `json:"provider"`
	Instances []Instance `json:"instances"`
}

// Instance represents a resource instance
type Instance struct {
	IndexKey            interface{}            `json:"index_key,omitempty"`
	Status              string                 `json:"status,omitempty"`
	SchemaVersion       int                    `json:"schema_version"`
	Attributes          map[string]interface{} `json:"attributes"`
	SensitiveAttributes []interface{}          `json:"sensitive_attributes"`
	Private             string                 `json:"private,omitempty"`
	Dependencies        []string               `json:"dependencies,omitempty"`
	CreateBeforeDestroy bool                   `json:"create_before_destroy,omitempty"`
}

// Filter applies sensitive data filtering to a Terraform state JSON
func Filter(stateJSON []byte, config *MergedConfig) (*FilterResult, error) {
	var state TerraformState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	result := &FilterResult{
		Omissions: []OmittedField{},
		Summary: FilterSummary{
			TotalResources: len(state.Resources),
		},
	}

	// Filter resources
	filteredResources := []Resource{}
	for _, resource := range state.Resources {
		resourcePath := formatResourcePath(resource)

		// Check if data sources should be omitted
		if config.OmitDataSources && resource.Mode == "data" {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   resourcePath,
				Reason: "data source lookup omitted",
				Type:   "resource",
			})
			result.Summary.OmittedResources++
			continue
		}

		// Check if entire resource type should be omitted (check platform first)
		if ResourceTypeMatches(resource.Type, config.PlatformOmitResourceTypes) {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:         resourcePath,
				Reason:       fmt.Sprintf("resource type '%s' is in omit list", resource.Type),
				Type:         "resource",
				FromPlatform: true,
			})
			result.Summary.OmittedResources++
			continue
		}
		if ResourceTypeMatches(resource.Type, config.OmitResourceTypes) {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   resourcePath,
				Reason: fmt.Sprintf("resource type '%s' is in omit list", resource.Type),
				Type:   "resource",
			})
			result.Summary.OmittedResources++
			continue
		}

		// Filter instances
		filteredInstances := []Instance{}
		for i, instance := range resource.Instances {
			instancePath := resourcePath
			if instance.IndexKey != nil {
				instancePath = fmt.Sprintf("%s[%v]", resourcePath, instance.IndexKey)
			} else if len(resource.Instances) > 1 {
				instancePath = fmt.Sprintf("%s[%d]", resourcePath, i)
			}

			// Get sensitive attributes from Terraform's markers
			sensitiveAttrs := parseSensitiveAttributes(instance.SensitiveAttributes)

			// Filter attributes
			filteredAttrs, attrOmissions := filterAttributes(
				instance.Attributes,
				instancePath,
				config,
				sensitiveAttrs,
			)
			result.Omissions = append(result.Omissions, attrOmissions...)
			result.Summary.OmittedAttributes += len(attrOmissions)
			result.Summary.TotalAttributes += countAttributes(instance.Attributes)

			instance.Attributes = filteredAttrs
			// Also clear sensitive_attributes since we've processed them
			instance.SensitiveAttributes = []interface{}{}
			filteredInstances = append(filteredInstances, instance)
		}

		resource.Instances = filteredInstances
		filteredResources = append(filteredResources, resource)
	}

	state.Resources = filteredResources

	// Filter outputs (they can also contain sensitive values)
	if state.Outputs != nil {
		state.Outputs = filterOutputs(state.Outputs, config, result)
	}

	// Re-serialize
	filteredJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize filtered state: %w", err)
	}
	result.FilteredJSON = filteredJSON

	return result, nil
}

// filterAttributes recursively filters sensitive attributes from a map
func filterAttributes(
	attrs map[string]interface{},
	basePath string,
	config *MergedConfig,
	terraformSensitive map[string]bool,
) (map[string]interface{}, []OmittedField) {
	if attrs == nil {
		return nil, nil
	}

	filtered := make(map[string]interface{})
	var omissions []OmittedField

	for key, value := range attrs {
		attrPath := basePath + "." + key

		// Check if preserved
		if isPreserved(key, config.PreserveAttributes) {
			filtered[key] = value
			continue
		}

		// Check if should be omitted by platform pattern (check first)
		if matchedPattern, found := AttributeMatchingPattern(key, config.PlatformOmitAttributes); found {
			omissions = append(omissions, OmittedField{
				Path:         attrPath,
				Reason:       fmt.Sprintf("matches pattern '%s'", matchedPattern),
				Type:         "attribute",
				FromPlatform: true,
			})
			continue
		}

		// Check if should be omitted by pattern
		if matchedPattern, found := AttributeMatchingPattern(key, config.OmitAttributes); found {
			omissions = append(omissions, OmittedField{
				Path:   attrPath,
				Reason: fmt.Sprintf("matches pattern '%s'", matchedPattern),
				Type:   "attribute",
			})
			continue
		}

		// Check if Terraform marked it sensitive
		if config.HonorTerraformSensitive && terraformSensitive[key] {
			omissions = append(omissions, OmittedField{
				Path:   attrPath,
				Reason: "marked as sensitive by Terraform",
				Type:   "attribute",
			})
			continue
		}

		// Handle nested objects
		switch v := value.(type) {
		case map[string]interface{}:
			nestedFiltered, nestedOmissions := filterAttributes(v, attrPath, config, terraformSensitive)
			filtered[key] = nestedFiltered
			omissions = append(omissions, nestedOmissions...)
		case []interface{}:
			filteredArray, arrayOmissions := filterArray(v, attrPath, config, terraformSensitive)
			filtered[key] = filteredArray
			omissions = append(omissions, arrayOmissions...)
		default:
			filtered[key] = value
		}
	}

	return filtered, omissions
}

// filterArray filters sensitive values from an array
func filterArray(
	arr []interface{},
	basePath string,
	config *MergedConfig,
	terraformSensitive map[string]bool,
) ([]interface{}, []OmittedField) {
	filtered := make([]interface{}, 0, len(arr))
	var omissions []OmittedField

	for i, item := range arr {
		itemPath := fmt.Sprintf("%s[%d]", basePath, i)

		switch v := item.(type) {
		case map[string]interface{}:
			nestedFiltered, nestedOmissions := filterAttributes(v, itemPath, config, terraformSensitive)
			filtered = append(filtered, nestedFiltered)
			omissions = append(omissions, nestedOmissions...)
		default:
			filtered = append(filtered, item)
		}
	}

	return filtered, omissions
}

// filterOutputs filters sensitive values from outputs
func filterOutputs(
	outputs map[string]interface{},
	config *MergedConfig,
	result *FilterResult,
) map[string]interface{} {
	filtered := make(map[string]interface{})

	for name, output := range outputs {
		outputPath := "outputs." + name

		// Check if output name matches sensitive patterns
		if matchedPattern, found := AttributeMatchingPattern(name, config.OmitAttributes); found {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   outputPath,
				Reason: fmt.Sprintf("matches pattern '%s'", matchedPattern),
				Type:   "attribute",
			})
			result.Summary.OmittedAttributes++
			continue
		}

		// Check if output is marked sensitive
		if outputMap, ok := output.(map[string]interface{}); ok {
			if sensitive, ok := outputMap["sensitive"].(bool); ok && sensitive {
				result.Omissions = append(result.Omissions, OmittedField{
					Path:   outputPath,
					Reason: "output marked as sensitive",
					Type:   "attribute",
				})
				result.Summary.OmittedAttributes++
				continue
			}
		}

		filtered[name] = output
	}

	return filtered
}

// parseSensitiveAttributes converts Terraform's sensitive_attributes format to a simple map
func parseSensitiveAttributes(sensitive []interface{}) map[string]bool {
	result := make(map[string]bool)

	for _, item := range sensitive {
		// Terraform uses a path format like [{"type":"get_attr","value":"password"}]
		if pathItems, ok := item.([]interface{}); ok {
			for _, pathItem := range pathItems {
				if pathMap, ok := pathItem.(map[string]interface{}); ok {
					if pathMap["type"] == "get_attr" {
						if value, ok := pathMap["value"].(string); ok {
							result[value] = true
						}
					}
				}
			}
		}
	}

	return result
}

// isPreserved checks if an attribute name matches a preserve pattern
func isPreserved(attrName string, preservePatterns []string) bool {
	for _, pattern := range preservePatterns {
		if strings.EqualFold(attrName, pattern) {
			return true
		}
	}
	return false
}

// formatResourcePath creates a human-readable path for a resource
func formatResourcePath(r Resource) string {
	if r.Module != "" {
		return fmt.Sprintf("%s.%s.%s", r.Module, r.Type, r.Name)
	}
	return fmt.Sprintf("%s.%s", r.Type, r.Name)
}

// countAttributes counts the total number of attributes (recursively)
func countAttributes(attrs map[string]interface{}) int {
	count := 0
	for _, v := range attrs {
		count++
		switch nested := v.(type) {
		case map[string]interface{}:
			count += countAttributes(nested)
		case []interface{}:
			for _, item := range nested {
				if m, ok := item.(map[string]interface{}); ok {
					count += countAttributes(m)
				}
			}
		}
	}
	return count
}

// TerraformPlan represents the structure of a Terraform plan JSON file
type TerraformPlan struct {
	FormatVersion      string                 `json:"format_version"`
	TerraformVersion   string                 `json:"terraform_version"`
	Variables          map[string]interface{} `json:"variables,omitempty"`
	PlannedValues      *PlannedValues         `json:"planned_values,omitempty"`
	ResourceChanges    []ResourceChange       `json:"resource_changes"`
	PriorState         *TerraformState        `json:"prior_state,omitempty"`
	Configuration      map[string]interface{} `json:"configuration,omitempty"`
	RelevantAttributes []interface{}          `json:"relevant_attributes,omitempty"`
	Checks             []interface{}          `json:"checks,omitempty"`
	Timestamp          string                 `json:"timestamp,omitempty"`
}

// PlannedValues represents the planned_values section of a plan
type PlannedValues struct {
	Outputs    map[string]interface{} `json:"outputs,omitempty"`
	RootModule *PlannedModule         `json:"root_module,omitempty"`
}

// PlannedModule represents a module in planned_values
type PlannedModule struct {
	Resources    []PlannedResource `json:"resources,omitempty"`
	ChildModules []PlannedModule   `json:"child_modules,omitempty"`
	Address      string            `json:"address,omitempty"`
}

// PlannedResource represents a resource in planned_values
type PlannedResource struct {
	Address         string                 `json:"address"`
	Mode            string                 `json:"mode"`
	Type            string                 `json:"type"`
	Name            string                 `json:"name"`
	ProviderName    string                 `json:"provider_name"`
	SchemaVersion   int                    `json:"schema_version"`
	Values          map[string]interface{} `json:"values"`
	SensitiveValues interface{}            `json:"sensitive_values"`
}

// ResourceChange represents a resource_change entry in a plan
type ResourceChange struct {
	Address       string  `json:"address"`
	ModuleAddress string  `json:"module_address,omitempty"`
	Mode          string  `json:"mode"`
	Type          string  `json:"type"`
	Name          string  `json:"name"`
	ProviderName  string  `json:"provider_name"`
	Change        *Change `json:"change"`
}

// Change represents the change details for a resource
type Change struct {
	Actions         []string               `json:"actions"`
	Before          map[string]interface{} `json:"before"`
	After           map[string]interface{} `json:"after"`
	AfterUnknown    map[string]interface{} `json:"after_unknown,omitempty"`
	BeforeSensitive interface{}            `json:"before_sensitive,omitempty"`
	AfterSensitive  interface{}            `json:"after_sensitive,omitempty"`
}

// FilterPlan applies sensitive data filtering to a Terraform plan JSON
func FilterPlan(planJSON []byte, config *MergedConfig) (*FilterResult, error) {
	var plan TerraformPlan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	result := &FilterResult{
		Omissions: []OmittedField{},
		Summary: FilterSummary{
			TotalResources: len(plan.ResourceChanges),
		},
	}

	// Filter resource_changes
	filteredChanges := []ResourceChange{}
	for _, rc := range plan.ResourceChanges {
		// Check if data sources should be omitted
		if config.OmitDataSources && rc.Mode == "data" {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   rc.Address,
				Reason: "data source lookup omitted",
				Type:   "resource",
			})
			result.Summary.OmittedResources++
			continue
		}

		// Check if entire resource type should be omitted (check platform first)
		if ResourceTypeMatches(rc.Type, config.PlatformOmitResourceTypes) {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:         rc.Address,
				Reason:       fmt.Sprintf("resource type '%s' is in omit list", rc.Type),
				Type:         "resource",
				FromPlatform: true,
			})
			result.Summary.OmittedResources++
			continue
		}
		if ResourceTypeMatches(rc.Type, config.OmitResourceTypes) {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   rc.Address,
				Reason: fmt.Sprintf("resource type '%s' is in omit list", rc.Type),
				Type:   "resource",
			})
			result.Summary.OmittedResources++
			continue
		}

		// Filter change.before and change.after
		if rc.Change != nil {
			sensitiveAttrs := parseSensitiveFromPlan(rc.Change.BeforeSensitive, rc.Change.AfterSensitive)

			if rc.Change.Before != nil {
				filtered, omissions := filterAttributes(rc.Change.Before, rc.Address+".before", config, sensitiveAttrs)
				rc.Change.Before = filtered
				result.Omissions = append(result.Omissions, omissions...)
				result.Summary.OmittedAttributes += len(omissions)
				result.Summary.TotalAttributes += countAttributes(rc.Change.Before)
			}

			if rc.Change.After != nil {
				filtered, omissions := filterAttributes(rc.Change.After, rc.Address+".after", config, sensitiveAttrs)
				rc.Change.After = filtered
				result.Omissions = append(result.Omissions, omissions...)
				result.Summary.OmittedAttributes += len(omissions)
				result.Summary.TotalAttributes += countAttributes(rc.Change.After)
			}

			// Clear sensitive markers since we've processed them
			rc.Change.BeforeSensitive = nil
			rc.Change.AfterSensitive = nil
		}

		filteredChanges = append(filteredChanges, rc)
	}
	plan.ResourceChanges = filteredChanges

	// Filter planned_values if present
	if plan.PlannedValues != nil {
		filterPlannedValues(plan.PlannedValues, config, result)
	}

	// Filter prior_state if present
	if plan.PriorState != nil {
		stateJSON, _ := json.Marshal(plan.PriorState)
		stateResult, err := Filter(stateJSON, config)
		if err == nil {
			var filteredState TerraformState
			if json.Unmarshal(stateResult.FilteredJSON, &filteredState) == nil {
				plan.PriorState = &filteredState
				result.Omissions = append(result.Omissions, stateResult.Omissions...)
				result.Summary.OmittedResources += stateResult.Summary.OmittedResources
				result.Summary.OmittedAttributes += stateResult.Summary.OmittedAttributes
			}
		}
	}

	// Filter variables that may be sensitive
	if plan.Variables != nil {
		plan.Variables = filterVariables(plan.Variables, config, result)
	}

	// Re-serialize
	filteredJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize filtered plan: %w", err)
	}
	result.FilteredJSON = filteredJSON

	return result, nil
}

// parseSensitiveFromPlan extracts sensitive attribute names from plan sensitive markers
func parseSensitiveFromPlan(beforeSensitive, afterSensitive interface{}) map[string]bool {
	result := make(map[string]bool)

	extractSensitive := func(v interface{}) {
		switch s := v.(type) {
		case map[string]interface{}:
			for key, val := range s {
				if b, ok := val.(bool); ok && b {
					result[key] = true
				}
			}
		case bool:
			// If the entire value is marked sensitive, we'll handle it elsewhere
		}
	}

	extractSensitive(beforeSensitive)
	extractSensitive(afterSensitive)

	return result
}

// filterPlannedValues filters sensitive data from planned_values
func filterPlannedValues(pv *PlannedValues, config *MergedConfig, result *FilterResult) {
	if pv.RootModule != nil {
		filterPlannedModule(pv.RootModule, config, result)
	}

	if pv.Outputs != nil {
		for name := range pv.Outputs {
			// Check platform patterns first
			if matchedPattern, found := AttributeMatchingPattern(name, config.PlatformOmitAttributes); found {
				result.Omissions = append(result.Omissions, OmittedField{
					Path:         "planned_values.outputs." + name,
					Reason:       fmt.Sprintf("matches pattern '%s'", matchedPattern),
					Type:         "attribute",
					FromPlatform: true,
				})
				result.Summary.OmittedAttributes++
				delete(pv.Outputs, name)
				continue
			}
			if matchedPattern, found := AttributeMatchingPattern(name, config.OmitAttributes); found {
				result.Omissions = append(result.Omissions, OmittedField{
					Path:   "planned_values.outputs." + name,
					Reason: fmt.Sprintf("matches pattern '%s'", matchedPattern),
					Type:   "attribute",
				})
				result.Summary.OmittedAttributes++
				delete(pv.Outputs, name)
			}
		}
	}
}

// filterPlannedModule recursively filters a planned module and its children
func filterPlannedModule(pm *PlannedModule, config *MergedConfig, result *FilterResult) {
	filteredResources := []PlannedResource{}

	for _, pr := range pm.Resources {
		// Check if data sources should be omitted
		if config.OmitDataSources && pr.Mode == "data" {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   pr.Address,
				Reason: "data source lookup omitted",
				Type:   "resource",
			})
			result.Summary.OmittedResources++
			continue
		}

		// Check platform settings first
		if ResourceTypeMatches(pr.Type, config.PlatformOmitResourceTypes) {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:         pr.Address,
				Reason:       fmt.Sprintf("resource type '%s' is in omit list", pr.Type),
				Type:         "resource",
				FromPlatform: true,
			})
			result.Summary.OmittedResources++
			continue
		}
		if ResourceTypeMatches(pr.Type, config.OmitResourceTypes) {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   pr.Address,
				Reason: fmt.Sprintf("resource type '%s' is in omit list", pr.Type),
				Type:   "resource",
			})
			result.Summary.OmittedResources++
			continue
		}

		sensitiveAttrs := parseSensitiveFromPlan(pr.SensitiveValues, nil)
		filtered, omissions := filterAttributes(pr.Values, pr.Address, config, sensitiveAttrs)
		pr.Values = filtered
		pr.SensitiveValues = nil
		result.Omissions = append(result.Omissions, omissions...)
		result.Summary.OmittedAttributes += len(omissions)

		filteredResources = append(filteredResources, pr)
	}
	pm.Resources = filteredResources

	for i := range pm.ChildModules {
		filterPlannedModule(&pm.ChildModules[i], config, result)
	}
}

// filterVariables filters sensitive variables from the plan
func filterVariables(vars map[string]interface{}, config *MergedConfig, result *FilterResult) map[string]interface{} {
	filtered := make(map[string]interface{})

	for name, value := range vars {
		if matchedPattern, found := AttributeMatchingPattern(name, config.OmitAttributes); found {
			result.Omissions = append(result.Omissions, OmittedField{
				Path:   "variables." + name,
				Reason: fmt.Sprintf("matches pattern '%s'", matchedPattern),
				Type:   "attribute",
			})
			result.Summary.OmittedAttributes++
			continue
		}
		filtered[name] = value
	}

	return filtered
}
