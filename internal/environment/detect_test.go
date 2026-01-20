package environment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setEnv is a helper that sets environment variables and returns a cleanup function
func setEnv(t *testing.T, vars map[string]string) func() {
	t.Helper()

	// Store original values
	originals := make(map[string]string)
	for k := range vars {
		originals[k] = os.Getenv(k)
	}

	// Set new values
	for k, v := range vars {
		os.Setenv(k, v)
	}

	// Return cleanup function
	return func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}
}

// clearAllCIEnvVars removes all known CI environment variables
func clearAllCIEnvVars(t *testing.T) func() {
	t.Helper()

	ciVars := []string{
		// Atlantis
		"ATLANTIS_TERRAFORM_VERSION",
		"WORKSPACE",
		"PROJECT_NAME",
		"BASE_REPO_OWNER",
		"BASE_REPO_NAME",
		"PULL_NUM",
		"HEAD_COMMIT",
		"HEAD_BRANCH_NAME",
		"BASE_BRANCH_NAME",
		"REPO_REL_DIR",
		// GitHub Actions
		"GITHUB_ACTIONS",
		"GITHUB_REPOSITORY",
		"GITHUB_REPOSITORY_OWNER",
		"GITHUB_REF",
		"GITHUB_REF_NAME",
		"GITHUB_SHA",
		"GITHUB_HEAD_REF",
		"GITHUB_BASE_REF",
		"GITHUB_EVENT_NAME",
		"GITHUB_EVENT_PATH",
	}

	originals := make(map[string]string)
	for _, k := range ciVars {
		originals[k] = os.Getenv(k)
		os.Unsetenv(k)
	}

	return func() {
		for k, v := range originals {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}
}

func TestDetect_NoEnvironment(t *testing.T) {
	cleanup := clearAllCIEnvVars(t)
	defer cleanup()

	result := Detect()
	if result != nil {
		t.Errorf("Expected nil when no CI environment detected, got %+v", result)
	}
}

func TestDetect_Atlantis(t *testing.T) {
	tests := []struct {
		name            string
		envVars         map[string]string
		wantName        string
		wantWorkspace   string
		wantGitHub      bool
		wantPRNumber    int
		wantWarnings    int
	}{
		{
			name: "full atlantis context",
			envVars: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.5.0",
				"WORKSPACE":                  "default",
				"PROJECT_NAME":               "my-app",
				"BASE_REPO_OWNER":            "myorg",
				"BASE_REPO_NAME":             "infra",
				"PULL_NUM":                   "123",
				"HEAD_COMMIT":                "abc123",
			},
			wantName:      "atlantis",
			wantWorkspace: "my-app-default",
			wantGitHub:    true,
			wantPRNumber:  123,
			wantWarnings:  0,
		},
		{
			name: "atlantis without project name",
			envVars: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.5.0",
				"WORKSPACE":                  "production",
				"BASE_REPO_OWNER":            "myorg",
				"BASE_REPO_NAME":             "infra",
				"PULL_NUM":                   "456",
				"HEAD_COMMIT":                "def456",
			},
			wantName:      "atlantis",
			wantWorkspace: "production",
			wantGitHub:    true,
			wantPRNumber:  456,
			wantWarnings:  0,
		},
		{
			name: "atlantis missing PR number",
			envVars: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.5.0",
				"WORKSPACE":                  "staging",
				"BASE_REPO_OWNER":            "myorg",
				"BASE_REPO_NAME":             "infra",
				"HEAD_COMMIT":                "ghi789",
			},
			wantName:      "atlantis",
			wantWorkspace: "staging",
			wantGitHub:    false, // Missing PR number
			wantPRNumber:  0,
			wantWarnings:  1,
		},
		{
			name: "atlantis minimal - only tf version",
			envVars: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.5.0",
			},
			wantName:      "atlantis",
			wantWorkspace: "",
			wantGitHub:    false,
			wantPRNumber:  0,
			wantWarnings:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := clearAllCIEnvVars(t)
			defer cleanup()

			envCleanup := setEnv(t, tt.envVars)
			defer envCleanup()

			result := Detect()
			if result == nil {
				t.Fatal("Expected detection result, got nil")
			}

			env := result.Environment
			if env.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", env.Name(), tt.wantName)
			}

			if env.Workspace() != tt.wantWorkspace {
				t.Errorf("Workspace() = %q, want %q", env.Workspace(), tt.wantWorkspace)
			}

			gh := env.GitHubContext()
			if tt.wantGitHub && gh == nil {
				t.Error("Expected GitHubContext, got nil")
			}
			if !tt.wantGitHub && gh != nil {
				t.Errorf("Expected nil GitHubContext, got %+v", gh)
			}
			if gh != nil && gh.PRNumber != tt.wantPRNumber {
				t.Errorf("GitHubContext.PRNumber = %d, want %d", gh.PRNumber, tt.wantPRNumber)
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("Warnings count = %d, want %d. Warnings: %v",
					len(result.Warnings), tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestDetect_GitHubActions(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		wantName      string
		wantWorkspace string
		wantGitHub    bool
		wantPRNumber  int
		wantWarnings  int
	}{
		{
			name: "full github actions PR context",
			envVars: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "myorg/myrepo",
				"GITHUB_REF":        "refs/pull/42/merge",
				"GITHUB_SHA":        "abc123def",
				"GITHUB_HEAD_REF":   "feature-branch",
				"GITHUB_BASE_REF":   "main",
				"GITHUB_EVENT_NAME": "pull_request",
			},
			wantName:      "github-actions",
			wantWorkspace: "feature-branch",
			wantGitHub:    true,
			wantPRNumber:  42,
			wantWarnings:  0,
		},
		{
			name: "github actions push event (no PR)",
			envVars: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "myorg/myrepo",
				"GITHUB_REF":        "refs/heads/main",
				"GITHUB_REF_NAME":   "main",
				"GITHUB_SHA":        "abc123def",
				"GITHUB_EVENT_NAME": "push",
			},
			wantName:      "github-actions",
			wantWorkspace: "main", // Falls back to ref_name
			wantGitHub:    false,  // No PR context
			wantPRNumber:  0,
			wantWarnings:  1, // Warning about no PR context
		},
		{
			name: "github actions with repository owner",
			envVars: map[string]string{
				"GITHUB_ACTIONS":          "true",
				"GITHUB_REPOSITORY_OWNER": "myorg",
				"GITHUB_REPOSITORY":       "myorg/myrepo",
				"GITHUB_REF":              "refs/pull/99/merge",
				"GITHUB_SHA":              "xyz789",
				"GITHUB_HEAD_REF":         "fix-bug",
				"GITHUB_EVENT_NAME":       "pull_request",
			},
			wantName:      "github-actions",
			wantWorkspace: "fix-bug",
			wantGitHub:    true,
			wantPRNumber:  99,
			wantWarnings:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := clearAllCIEnvVars(t)
			defer cleanup()

			envCleanup := setEnv(t, tt.envVars)
			defer envCleanup()

			result := Detect()
			if result == nil {
				t.Fatal("Expected detection result, got nil")
			}

			env := result.Environment
			if env.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", env.Name(), tt.wantName)
			}

			if env.Workspace() != tt.wantWorkspace {
				t.Errorf("Workspace() = %q, want %q", env.Workspace(), tt.wantWorkspace)
			}

			gh := env.GitHubContext()
			if tt.wantGitHub && gh == nil {
				t.Error("Expected GitHubContext, got nil")
			}
			if !tt.wantGitHub && gh != nil {
				t.Errorf("Expected nil GitHubContext, got %+v", gh)
			}
			if gh != nil && gh.PRNumber != tt.wantPRNumber {
				t.Errorf("GitHubContext.PRNumber = %d, want %d", gh.PRNumber, tt.wantPRNumber)
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("Warnings count = %d, want %d. Warnings: %v",
					len(result.Warnings), tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestDetect_AtlantisTakesPrecedence(t *testing.T) {
	cleanup := clearAllCIEnvVars(t)
	defer cleanup()

	// Set both Atlantis and GitHub Actions env vars
	envCleanup := setEnv(t, map[string]string{
		"ATLANTIS_TERRAFORM_VERSION": "1.5.0",
		"WORKSPACE":                  "prod",
		"GITHUB_ACTIONS":             "true",
		"GITHUB_REPOSITORY":          "other/repo",
	})
	defer envCleanup()

	result := Detect()
	if result == nil {
		t.Fatal("Expected detection result, got nil")
	}

	if result.Environment.Name() != "atlantis" {
		t.Errorf("Expected atlantis to take precedence, got %q", result.Environment.Name())
	}
}

func TestExtractPRNumberFromRef(t *testing.T) {
	tests := []struct {
		ref  string
		want int
	}{
		{"refs/pull/123/merge", 123},
		{"refs/pull/1/merge", 1},
		{"refs/pull/99999/merge", 99999},
		{"refs/heads/main", 0},
		{"refs/tags/v1.0.0", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := extractPRNumberFromRef(tt.ref)
			if got != tt.want {
				t.Errorf("extractPRNumberFromRef(%q) = %d, want %d", tt.ref, got, tt.want)
			}
		})
	}
}

func TestExtractPRNumberFromEvent(t *testing.T) {
	t.Run("pull_request event", func(t *testing.T) {
		// Create temp event file
		eventData := map[string]interface{}{
			"pull_request": map[string]interface{}{
				"number": 456,
			},
		}
		eventJSON, _ := json.Marshal(eventData)
		tmpFile := filepath.Join(t.TempDir(), "event.json")
		os.WriteFile(tmpFile, eventJSON, 0644)

		got := extractPRNumberFromEvent(tmpFile)
		if got != 456 {
			t.Errorf("extractPRNumberFromEvent() = %d, want 456", got)
		}
	})

	t.Run("issue_comment event", func(t *testing.T) {
		eventData := map[string]interface{}{
			"number": 789,
		}
		eventJSON, _ := json.Marshal(eventData)
		tmpFile := filepath.Join(t.TempDir(), "event.json")
		os.WriteFile(tmpFile, eventJSON, 0644)

		got := extractPRNumberFromEvent(tmpFile)
		if got != 789 {
			t.Errorf("extractPRNumberFromEvent() = %d, want 789", got)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		got := extractPRNumberFromEvent("/nonexistent/path")
		if got != 0 {
			t.Errorf("extractPRNumberFromEvent() = %d, want 0", got)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		got := extractPRNumberFromEvent("")
		if got != 0 {
			t.Errorf("extractPRNumberFromEvent() = %d, want 0", got)
		}
	})
}

func TestAtlantisEnv_Description(t *testing.T) {
	env := &AtlantisEnv{
		workspace:   "default",
		projectName: "my-app",
		repoOwner:   "myorg",
		repoName:    "infra",
		prNumber:    123,
	}

	desc := env.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}

	// Should contain key info
	expected := []string{"Atlantis", "myorg/infra", "PR=#123", "my-app-default"}
	for _, want := range expected {
		if !contains(desc, want) {
			t.Errorf("Description %q should contain %q", desc, want)
		}
	}
}

func TestGitHubActionsEnv_Description(t *testing.T) {
	env := &GitHubActionsEnv{
		repoOwner: "myorg",
		repoName:  "myrepo",
		prNumber:  42,
		eventName: "pull_request",
	}

	desc := env.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}

	expected := []string{"GitHub Actions", "myorg/myrepo", "PR=#42", "pull_request"}
	for _, want := range expected {
		if !contains(desc, want) {
			t.Errorf("Description %q should contain %q", desc, want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
