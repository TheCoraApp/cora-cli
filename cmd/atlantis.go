package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	atlantisConfigPath string
	atlantisForce      bool
	atlantisDryRun     bool
	atlantisBackup     bool
)

var atlantisCmd = &cobra.Command{
	Use:   "atlantis",
	Short: "Atlantis integration commands",
	Long: `Commands for integrating Cora with Atlantis.

Use these commands to automatically configure your Atlantis workflows
to include Cora state uploads and PR risk assessments.`,
}

var atlantisInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Add Cora steps to your Atlantis configuration",
	Long: `Automatically modify your atlantis.yaml to include Cora integration.

This command parses your existing Atlantis configuration and adds:
  - cora review: After plan steps (for PR risk assessment)
  - cora upload: After apply steps (for state visualization)

The command is idempotent - running it multiple times won't create duplicates.

Examples:
  # Modify atlantis.yaml in the current directory
  cora atlantis init

  # Specify a custom config path
  cora atlantis init --config ./infra/atlantis.yaml

  # Preview changes without modifying the file
  cora atlantis init --dry-run

  # Create a backup before modifying
  cora atlantis init --backup`,
	RunE: runAtlantisInit,
}

func init() {
	rootCmd.AddCommand(atlantisCmd)
	atlantisCmd.AddCommand(atlantisInitCmd)

	atlantisInitCmd.Flags().StringVarP(&atlantisConfigPath, "config", "c", "", "Path to atlantis.yaml (default: searches current directory)")
	atlantisInitCmd.Flags().BoolVar(&atlantisForce, "force", false, "Overwrite existing Cora steps if present")
	atlantisInitCmd.Flags().BoolVar(&atlantisDryRun, "dry-run", false, "Preview changes without modifying the file")
	atlantisInitCmd.Flags().BoolVar(&atlantisBackup, "backup", false, "Create a backup of the original file before modifying")
}

// AtlantisConfig represents the structure of atlantis.yaml
type AtlantisConfig struct {
	Version   int                       `yaml:"version"`
	Projects  []AtlantisProject         `yaml:"projects,omitempty"`
	Workflows map[string]AtlantisWorkflow `yaml:"workflows,omitempty"`
	// Preserve other fields
	Extra map[string]interface{} `yaml:",inline"`
}

// AtlantisProject represents a project in atlantis.yaml
type AtlantisProject struct {
	Name      string `yaml:"name,omitempty"`
	Dir       string `yaml:"dir,omitempty"`
	Workspace string `yaml:"workspace,omitempty"`
	Workflow  string `yaml:"workflow,omitempty"`
	// Preserve other fields
	Extra map[string]interface{} `yaml:",inline"`
}

// AtlantisWorkflow represents a workflow definition
type AtlantisWorkflow struct {
	Plan  *AtlantisStage `yaml:"plan,omitempty"`
	Apply *AtlantisStage `yaml:"apply,omitempty"`
	// Preserve other fields
	Extra map[string]interface{} `yaml:",inline"`
}

// AtlantisStage represents plan or apply stage
type AtlantisStage struct {
	Steps []interface{} `yaml:"steps,omitempty"`
}

func runAtlantisInit(cmd *cobra.Command, args []string) error {
	// Find atlantis.yaml
	configPath := atlantisConfigPath
	if configPath == "" {
		var err error
		configPath, err = findAtlantisConfig()
		if err != nil {
			return err
		}
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	fmt.Printf("📄 Found Atlantis config: %s\n", configPath)

	// Parse the config
	var config AtlantisConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse atlantis.yaml: %w", err)
	}

	// Track changes
	changes := []string{}

	// Ensure workflows map exists
	if config.Workflows == nil {
		config.Workflows = make(map[string]AtlantisWorkflow)
	}

	// Check if there are any projects
	if len(config.Projects) == 0 && len(config.Workflows) == 0 {
		// No projects or workflows - create a default cora workflow
		fmt.Println("⚠️  No projects or workflows found. Creating a 'cora' workflow template.")
		config.Workflows["cora"] = createCoraWorkflow()
		changes = append(changes, "Created new 'cora' workflow with Cora integration")
	} else {
		// Process existing workflows
		workflowsToProcess := getWorkflowsToProcess(config)
		
		if len(workflowsToProcess) == 0 {
			// No custom workflows defined - create cora workflow and update projects
			fmt.Println("ℹ️  No custom workflows defined. Creating a 'cora' workflow.")
			config.Workflows["cora"] = createCoraWorkflow()
			changes = append(changes, "Created new 'cora' workflow with Cora integration")
			
			// Update projects to use the cora workflow
			for i := range config.Projects {
				if config.Projects[i].Workflow == "" {
					config.Projects[i].Workflow = "cora"
					changes = append(changes, fmt.Sprintf("Updated project '%s' to use 'cora' workflow", getProjectName(config.Projects[i])))
				}
			}
		} else {
			// Modify existing workflows
			for name := range workflowsToProcess {
				workflow := config.Workflows[name]
				modified := addCoraSteps(&workflow, name, atlantisForce)
				if modified {
					config.Workflows[name] = workflow
					changes = append(changes, fmt.Sprintf("Added Cora steps to workflow '%s'", name))
				} else {
					fmt.Printf("ℹ️  Workflow '%s' already has Cora steps (use --force to replace)\n", name)
				}
			}
		}
	}

	if len(changes) == 0 {
		fmt.Println("\n✅ No changes needed - Cora integration already configured!")
		return nil
	}

	// Show changes
	fmt.Println("\n📝 Changes to be made:")
	for _, change := range changes {
		fmt.Printf("   • %s\n", change)
	}

	if atlantisDryRun {
		fmt.Println("\n🔍 Dry run - no changes written")
		fmt.Println("\nPreview of modified config:")
		fmt.Println("─────────────────────────────")
		output, _ := yaml.Marshal(&config)
		fmt.Println(string(output))
		return nil
	}

	// Confirm with user (unless force flag)
	if !atlantisForce {
		fmt.Print("\nProceed with these changes? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Create backup if requested
	if atlantisBackup {
		backupPath := configPath + ".backup"
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		fmt.Printf("📦 Created backup: %s\n", backupPath)
	}

	// Write the modified config
	output, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// Add header comment
	header := "# Atlantis Configuration\n# Modified by Cora CLI to include infrastructure visualization and PR risk assessment\n# https://thecora.app/docs/atlantis\n\n"
	output = append([]byte(header), output...)

	if err := os.WriteFile(configPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("\n✅ Successfully updated %s\n", configPath)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Install the Cora CLI on your Atlantis Docker image")
	fmt.Println("     See: https://thecora.app/docs/cli#installation")
	fmt.Println("  2. Set CORA_TOKEN environment variable in your Atlantis server")
	fmt.Println("  3. Commit the updated atlantis.yaml")
	fmt.Println("  4. Open a PR to test the integration")
	fmt.Println()
	fmt.Println("📚 Documentation: https://thecora.app/docs/atlantis")

	return nil
}

// findAtlantisConfig searches for atlantis.yaml in common locations
func findAtlantisConfig() (string, error) {
	candidates := []string{
		"atlantis.yaml",
		"atlantis.yml",
		".atlantis.yaml",
		".atlantis.yml",
	}

	for _, name := range candidates {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}

	// Check common subdirectories
	subdirs := []string{".", "infra", "infrastructure", "terraform", ".github"}
	for _, dir := range subdirs {
		for _, name := range candidates {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("atlantis.yaml not found\n\nSearched for: %s\nUse --config to specify the path", strings.Join(candidates, ", "))
}

// getWorkflowsToProcess returns the set of workflows that need processing
func getWorkflowsToProcess(config AtlantisConfig) map[string]bool {
	workflows := make(map[string]bool)
	
	// Add all explicitly defined workflows
	for name := range config.Workflows {
		workflows[name] = true
	}
	
	return workflows
}

// getProjectName returns a display name for a project
func getProjectName(p AtlantisProject) string {
	if p.Name != "" {
		return p.Name
	}
	if p.Dir != "" {
		return p.Dir
	}
	return "(unnamed)"
}

// createCoraWorkflow creates a new workflow with Cora integration
func createCoraWorkflow() AtlantisWorkflow {
	return AtlantisWorkflow{
		Plan: &AtlantisStage{
			Steps: []interface{}{
				"init",
				"plan",
				map[string]interface{}{
					"run": "terraform show -json $PLANFILE | cora review",
				},
			},
		},
		Apply: &AtlantisStage{
			Steps: []interface{}{
				"apply",
				map[string]interface{}{
					"run": "terraform show -json | cora upload",
				},
			},
		},
	}
}

// addCoraSteps adds Cora steps to an existing workflow
// Returns true if changes were made
func addCoraSteps(workflow *AtlantisWorkflow, workflowName string, force bool) bool {
	modified := false

	// Add to plan stage
	if workflow.Plan != nil {
		if !hasCoraStep(workflow.Plan.Steps, "cora review") || force {
			workflow.Plan.Steps = addCoraReviewStep(workflow.Plan.Steps, force)
			modified = true
		}
	} else {
		// Create plan stage if it doesn't exist
		workflow.Plan = &AtlantisStage{
			Steps: []interface{}{
				"init",
				"plan",
				map[string]interface{}{
					"run": "terraform show -json $PLANFILE | cora review",
				},
			},
		}
		modified = true
	}

	// Add to apply stage
	if workflow.Apply != nil {
		if !hasCoraStep(workflow.Apply.Steps, "cora upload") || force {
			workflow.Apply.Steps = addCoraUploadStep(workflow.Apply.Steps, force)
			modified = true
		}
	} else {
		// Create apply stage if it doesn't exist
		workflow.Apply = &AtlantisStage{
			Steps: []interface{}{
				"apply",
				map[string]interface{}{
					"run": "terraform show -json | cora upload",
				},
			},
		}
		modified = true
	}

	return modified
}

// hasCoraStep checks if a step list already contains a Cora command
func hasCoraStep(steps []interface{}, command string) bool {
	for _, step := range steps {
		switch s := step.(type) {
		case map[string]interface{}:
			if run, ok := s["run"].(string); ok && strings.Contains(run, command) {
				return true
			}
		case string:
			if strings.Contains(s, command) {
				return true
			}
		}
	}
	return false
}

// addCoraReviewStep adds the cora review step after plan
func addCoraReviewStep(steps []interface{}, force bool) []interface{} {
	// Find position after "plan" step
	insertIdx := len(steps) // Default to end
	
	for i, step := range steps {
		// Remove existing cora review step if force
		if force {
			if isCoraStep(step, "cora review") {
				steps = append(steps[:i], steps[i+1:]...)
				break
			}
		}
	}

	for i, step := range steps {
		if s, ok := step.(string); ok && s == "plan" {
			insertIdx = i + 1
			break
		}
	}

	// Insert cora review step
	coraStep := map[string]interface{}{
		"run": "terraform show -json $PLANFILE | cora review",
	}
	
	// Insert at position
	result := make([]interface{}, 0, len(steps)+1)
	result = append(result, steps[:insertIdx]...)
	result = append(result, coraStep)
	result = append(result, steps[insertIdx:]...)
	
	return result
}

// addCoraUploadStep adds the cora upload step after apply
func addCoraUploadStep(steps []interface{}, force bool) []interface{} {
	// Find position after "apply" step
	insertIdx := len(steps) // Default to end
	
	for i, step := range steps {
		// Remove existing cora upload step if force
		if force {
			if isCoraStep(step, "cora upload") {
				steps = append(steps[:i], steps[i+1:]...)
				break
			}
		}
	}

	for i, step := range steps {
		if s, ok := step.(string); ok && s == "apply" {
			insertIdx = i + 1
			break
		}
	}

	// Insert cora upload step
	coraStep := map[string]interface{}{
		"run": "terraform show -json | cora upload",
	}
	
	// Insert at position
	result := make([]interface{}, 0, len(steps)+1)
	result = append(result, steps[:insertIdx]...)
	result = append(result, coraStep)
	result = append(result, steps[insertIdx:]...)
	
	return result
}

// isCoraStep checks if a step is a Cora command
func isCoraStep(step interface{}, command string) bool {
	switch s := step.(type) {
	case map[string]interface{}:
		if run, ok := s["run"].(string); ok && strings.Contains(run, command) {
			return true
		}
	case string:
		if strings.Contains(s, command) {
			return true
		}
	}
	return false
}
