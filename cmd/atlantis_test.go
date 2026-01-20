package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFindAtlantisConfig(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Test: no config found
	_, err := findAtlantisConfig()
	if err == nil {
		t.Error("Expected error when no atlantis.yaml exists")
	}

	// Test: atlantis.yaml exists
	os.WriteFile("atlantis.yaml", []byte("version: 3"), 0644)
	path, err := findAtlantisConfig()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if path != "atlantis.yaml" {
		t.Errorf("Expected 'atlantis.yaml', got %s", path)
	}
}

func TestCreateCoraWorkflow(t *testing.T) {
	workflow := createCoraWorkflow()

	// Check plan stage
	if workflow.Plan == nil {
		t.Fatal("Expected Plan stage to be created")
	}
	if len(workflow.Plan.Steps) != 3 {
		t.Errorf("Expected 3 plan steps, got %d", len(workflow.Plan.Steps))
	}

	// Check apply stage
	if workflow.Apply == nil {
		t.Fatal("Expected Apply stage to be created")
	}
	if len(workflow.Apply.Steps) != 2 {
		t.Errorf("Expected 2 apply steps, got %d", len(workflow.Apply.Steps))
	}

	// Verify cora review step exists in plan
	hasReview := false
	for _, step := range workflow.Plan.Steps {
		if m, ok := step.(map[string]interface{}); ok {
			if run, ok := m["run"].(string); ok && strings.Contains(run, "cora review") {
				hasReview = true
			}
		}
	}
	if !hasReview {
		t.Error("Expected 'cora review' step in plan")
	}

	// Verify cora upload step exists in apply
	hasUpload := false
	for _, step := range workflow.Apply.Steps {
		if m, ok := step.(map[string]interface{}); ok {
			if run, ok := m["run"].(string); ok && strings.Contains(run, "cora upload") {
				hasUpload = true
			}
		}
	}
	if !hasUpload {
		t.Error("Expected 'cora upload' step in apply")
	}
}

func TestHasCoraStep(t *testing.T) {
	tests := []struct {
		name    string
		steps   []interface{}
		command string
		want    bool
	}{
		{
			name:    "empty steps",
			steps:   []interface{}{},
			command: "cora review",
			want:    false,
		},
		{
			name:    "string steps without cora",
			steps:   []interface{}{"init", "plan"},
			command: "cora review",
			want:    false,
		},
		{
			name: "map step with cora review",
			steps: []interface{}{
				"init",
				"plan",
				map[string]interface{}{"run": "terraform show -json $PLANFILE | cora review"},
			},
			command: "cora review",
			want:    true,
		},
		{
			name: "map step with cora upload",
			steps: []interface{}{
				"apply",
				map[string]interface{}{"run": "terraform show -json | cora upload"},
			},
			command: "cora upload",
			want:    true,
		},
		{
			name: "different cora command",
			steps: []interface{}{
				map[string]interface{}{"run": "cora upload --workspace test"},
			},
			command: "cora review",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasCoraStep(tt.steps, tt.command)
			if got != tt.want {
				t.Errorf("hasCoraStep() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddCoraReviewStep(t *testing.T) {
	tests := []struct {
		name     string
		steps    []interface{}
		wantLen  int
		checkIdx int // Index where cora step should be
	}{
		{
			name:     "after plan step",
			steps:    []interface{}{"init", "plan"},
			wantLen:  3,
			checkIdx: 2,
		},
		{
			name:     "no plan step - adds to end",
			steps:    []interface{}{"init"},
			wantLen:  2,
			checkIdx: 1,
		},
		{
			name:     "empty steps",
			steps:    []interface{}{},
			wantLen:  1,
			checkIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addCoraReviewStep(tt.steps, false)

			if len(result) != tt.wantLen {
				t.Errorf("Expected %d steps, got %d", tt.wantLen, len(result))
			}

			// Check the cora step exists at expected position
			if tt.checkIdx < len(result) {
				step := result[tt.checkIdx]
				if m, ok := step.(map[string]interface{}); ok {
					if run, ok := m["run"].(string); ok {
						if !strings.Contains(run, "cora review") {
							t.Errorf("Expected 'cora review' at index %d, got %s", tt.checkIdx, run)
						}
					}
				}
			}
		})
	}
}

func TestAddCoraUploadStep(t *testing.T) {
	tests := []struct {
		name     string
		steps    []interface{}
		wantLen  int
		checkIdx int
	}{
		{
			name:     "after apply step",
			steps:    []interface{}{"apply"},
			wantLen:  2,
			checkIdx: 1,
		},
		{
			name:     "no apply step - adds to end",
			steps:    []interface{}{"init"},
			wantLen:  2,
			checkIdx: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addCoraUploadStep(tt.steps, false)

			if len(result) != tt.wantLen {
				t.Errorf("Expected %d steps, got %d", tt.wantLen, len(result))
			}

			// Check the cora step exists at expected position
			if tt.checkIdx < len(result) {
				step := result[tt.checkIdx]
				if m, ok := step.(map[string]interface{}); ok {
					if run, ok := m["run"].(string); ok {
						if !strings.Contains(run, "cora upload") {
							t.Errorf("Expected 'cora upload' at index %d, got %s", tt.checkIdx, run)
						}
					}
				}
			}
		})
	}
}

func TestAddCoraSteps(t *testing.T) {
	t.Run("adds to existing workflow", func(t *testing.T) {
		workflow := &AtlantisWorkflow{
			Plan: &AtlantisStage{
				Steps: []interface{}{"init", "plan"},
			},
			Apply: &AtlantisStage{
				Steps: []interface{}{"apply"},
			},
		}

		modified := addCoraSteps(workflow, "test", false)
		if !modified {
			t.Error("Expected workflow to be modified")
		}

		// Should have 3 plan steps now
		if len(workflow.Plan.Steps) != 3 {
			t.Errorf("Expected 3 plan steps, got %d", len(workflow.Plan.Steps))
		}

		// Should have 2 apply steps now
		if len(workflow.Apply.Steps) != 2 {
			t.Errorf("Expected 2 apply steps, got %d", len(workflow.Apply.Steps))
		}
	})

	t.Run("creates stages if missing", func(t *testing.T) {
		workflow := &AtlantisWorkflow{}

		modified := addCoraSteps(workflow, "test", false)
		if !modified {
			t.Error("Expected workflow to be modified")
		}

		if workflow.Plan == nil {
			t.Error("Expected Plan stage to be created")
		}
		if workflow.Apply == nil {
			t.Error("Expected Apply stage to be created")
		}
	})

	t.Run("idempotent - doesn't duplicate", func(t *testing.T) {
		workflow := &AtlantisWorkflow{
			Plan: &AtlantisStage{
				Steps: []interface{}{
					"init",
					"plan",
					map[string]interface{}{"run": "terraform show -json $PLANFILE | cora review"},
				},
			},
			Apply: &AtlantisStage{
				Steps: []interface{}{
					"apply",
					map[string]interface{}{"run": "terraform show -json | cora upload"},
				},
			},
		}

		modified := addCoraSteps(workflow, "test", false)
		if modified {
			t.Error("Expected workflow to not be modified (already has cora steps)")
		}
	})
}

func TestAtlantisConfigParsing(t *testing.T) {
	yamlContent := `
version: 3
projects:
  - name: networking
    dir: terraform/networking
    workflow: custom
  - name: app
    dir: terraform/app

workflows:
  custom:
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - apply
`

	var config AtlantisConfig
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if config.Version != 3 {
		t.Errorf("Expected version 3, got %d", config.Version)
	}

	if len(config.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(config.Projects))
	}

	if len(config.Workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(config.Workflows))
	}

	workflow, ok := config.Workflows["custom"]
	if !ok {
		t.Fatal("Expected 'custom' workflow to exist")
	}

	if len(workflow.Plan.Steps) != 2 {
		t.Errorf("Expected 2 plan steps, got %d", len(workflow.Plan.Steps))
	}
}

func TestFullIntegration(t *testing.T) {
	// Create a temp directory with an atlantis.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "atlantis.yaml")

	initialConfig := `version: 3
projects:
  - name: infra
    dir: .
    workflow: default

workflows:
  default:
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - apply
`

	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Read and parse
	data, _ := os.ReadFile(configPath)
	var config AtlantisConfig
	yaml.Unmarshal(data, &config)

	// Add cora steps
	for name := range config.Workflows {
		workflow := config.Workflows[name]
		addCoraSteps(&workflow, name, false)
		config.Workflows[name] = workflow
	}

	// Verify the result
	workflow := config.Workflows["default"]

	// Should have cora review after plan
	if !hasCoraStep(workflow.Plan.Steps, "cora review") {
		t.Error("Expected 'cora review' step to be added to plan")
	}

	// Should have cora upload after apply
	if !hasCoraStep(workflow.Apply.Steps, "cora upload") {
		t.Error("Expected 'cora upload' step to be added to apply")
	}

	// Serialize and verify it's valid YAML
	output, err := yaml.Marshal(&config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Parse it again to make sure it's valid
	var reparsed AtlantisConfig
	err = yaml.Unmarshal(output, &reparsed)
	if err != nil {
		t.Fatalf("Failed to reparse marshaled config: %v", err)
	}
}
