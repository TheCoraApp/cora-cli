// Package environment provides auto-detection for CI/CD environments
// like Atlantis and GitHub Actions, extracting context for Cora uploads.
package environment

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Environment represents a detected CI/CD environment
type Environment interface {
	// Name returns the environment identifier (e.g., "atlantis", "github-actions")
	Name() string

	// GitHubContext returns GitHub PR context if available, or nil
	GitHubContext() *GitHubContext

	// Workspace returns the auto-constructed workspace name
	Workspace() string

	// Description returns a human-readable description for logging
	Description() string
}

// GitHubContext contains GitHub PR information
type GitHubContext struct {
	Owner     string
	Repo      string
	PRNumber  int
	CommitSHA string
}

// DetectionResult contains the detected environment and any warnings
type DetectionResult struct {
	Environment Environment
	Warnings    []string
}

// Detect checks for known CI/CD environments and returns the first match.
// Returns nil if no known environment is detected.
func Detect() *DetectionResult {
	// Check Atlantis first (more specific)
	if env := detectAtlantis(); env != nil {
		return env
	}

	// Check GitHub Actions
	if env := detectGitHubActions(); env != nil {
		return env
	}

	return nil
}

// AtlantisEnv represents the Atlantis CI environment
type AtlantisEnv struct {
	workspace     string
	projectName   string
	repoOwner     string
	repoName      string
	prNumber      int
	commitSHA     string
	headBranch    string
	baseBranch    string
	relativeDir   string
	tfVersion     string
}

func detectAtlantis() *DetectionResult {
	// ATLANTIS_TERRAFORM_VERSION is the most reliable detection signal
	tfVersion := os.Getenv("ATLANTIS_TERRAFORM_VERSION")
	if tfVersion == "" {
		return nil
	}

	prNum, _ := strconv.Atoi(os.Getenv("PULL_NUM"))

	env := &AtlantisEnv{
		workspace:   os.Getenv("WORKSPACE"),
		projectName: os.Getenv("PROJECT_NAME"),
		repoOwner:   os.Getenv("BASE_REPO_OWNER"),
		repoName:    os.Getenv("BASE_REPO_NAME"),
		prNumber:    prNum,
		commitSHA:   os.Getenv("HEAD_COMMIT"),
		headBranch:  os.Getenv("HEAD_BRANCH_NAME"),
		baseBranch:  os.Getenv("BASE_BRANCH_NAME"),
		relativeDir: os.Getenv("REPO_REL_DIR"),
		tfVersion:   tfVersion,
	}

	result := &DetectionResult{
		Environment: env,
		Warnings:    []string{},
	}

	// Warn if PR context is incomplete
	if env.prNumber == 0 {
		result.Warnings = append(result.Warnings,
			"Atlantis environment detected but PULL_NUM is not set. GitHub PR comments will be disabled.")
	}

	return result
}

func (e *AtlantisEnv) Name() string {
	return "atlantis"
}

func (e *AtlantisEnv) GitHubContext() *GitHubContext {
	// All fields required for a valid context
	if e.repoOwner == "" || e.repoName == "" || e.prNumber == 0 || e.commitSHA == "" {
		return nil
	}

	return &GitHubContext{
		Owner:     e.repoOwner,
		Repo:      e.repoName,
		PRNumber:  e.prNumber,
		CommitSHA: e.commitSHA,
	}
}

func (e *AtlantisEnv) Workspace() string {
	// If PROJECT_NAME is set, use PROJECT_NAME-WORKSPACE
	// Otherwise just use WORKSPACE
	if e.projectName != "" {
		return e.projectName + "-" + e.workspace
	}
	return e.workspace
}

func (e *AtlantisEnv) Description() string {
	parts := []string{"Atlantis"}

	if e.repoOwner != "" && e.repoName != "" {
		parts = append(parts, "repo="+e.repoOwner+"/"+e.repoName)
	}
	if e.prNumber > 0 {
		parts = append(parts, "PR=#"+strconv.Itoa(e.prNumber))
	}
	if e.workspace != "" {
		parts = append(parts, "workspace="+e.Workspace())
	}

	return strings.Join(parts, ", ")
}

// GitHubActionsEnv represents the GitHub Actions CI environment
type GitHubActionsEnv struct {
	repoOwner  string
	repoName   string
	prNumber   int
	commitSHA  string
	headBranch string
	baseBranch string
	eventName  string
	refName    string
}

func detectGitHubActions() *DetectionResult {
	// GITHUB_ACTIONS is always "true" in GitHub Actions
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return nil
	}

	// Parse GITHUB_REPOSITORY (format: owner/repo)
	var repoOwner, repoName string
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			repoOwner = parts[0]
			repoName = parts[1]
		}
	}

	// GITHUB_REPOSITORY_OWNER is also available directly
	if repoOwner == "" {
		repoOwner = os.Getenv("GITHUB_REPOSITORY_OWNER")
	}

	// Extract PR number from GITHUB_REF (format: refs/pull/123/merge)
	prNumber := extractPRNumberFromRef(os.Getenv("GITHUB_REF"))

	// Fallback: try to get PR number from event payload
	if prNumber == 0 {
		prNumber = extractPRNumberFromEvent(os.Getenv("GITHUB_EVENT_PATH"))
	}

	env := &GitHubActionsEnv{
		repoOwner:  repoOwner,
		repoName:   repoName,
		prNumber:   prNumber,
		commitSHA:  os.Getenv("GITHUB_SHA"),
		headBranch: os.Getenv("GITHUB_HEAD_REF"),
		baseBranch: os.Getenv("GITHUB_BASE_REF"),
		eventName:  os.Getenv("GITHUB_EVENT_NAME"),
		refName:    os.Getenv("GITHUB_REF_NAME"),
	}

	result := &DetectionResult{
		Environment: env,
		Warnings:    []string{},
	}

	// Warn if this doesn't appear to be a PR context
	if prNumber == 0 {
		result.Warnings = append(result.Warnings,
			"GitHub Actions detected but no PR context found (event: "+env.eventName+"). GitHub PR comments will be disabled.")
	}

	return result
}

// extractPRNumberFromRef parses refs/pull/123/merge format
func extractPRNumberFromRef(ref string) int {
	re := regexp.MustCompile(`refs/pull/(\d+)/`)
	matches := re.FindStringSubmatch(ref)
	if len(matches) >= 2 {
		num, _ := strconv.Atoi(matches[1])
		return num
	}
	return 0
}

// extractPRNumberFromEvent reads the GitHub event payload JSON
func extractPRNumberFromEvent(eventPath string) int {
	if eventPath == "" {
		return 0
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return 0
	}

	var event struct {
		PullRequest *struct {
			Number int `json:"number"`
		} `json:"pull_request"`
		Number int `json:"number"` // For issue_comment events
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return 0
	}

	if event.PullRequest != nil {
		return event.PullRequest.Number
	}
	return event.Number
}

func (e *GitHubActionsEnv) Name() string {
	return "github-actions"
}

func (e *GitHubActionsEnv) GitHubContext() *GitHubContext {
	// All fields required for a valid context
	if e.repoOwner == "" || e.repoName == "" || e.prNumber == 0 || e.commitSHA == "" {
		return nil
	}

	return &GitHubContext{
		Owner:     e.repoOwner,
		Repo:      e.repoName,
		PRNumber:  e.prNumber,
		CommitSHA: e.commitSHA,
	}
}

func (e *GitHubActionsEnv) Workspace() string {
	// For GitHub Actions, use head branch or ref name as workspace
	if e.headBranch != "" {
		return e.headBranch
	}
	if e.refName != "" {
		return e.refName
	}
	return ""
}

func (e *GitHubActionsEnv) Description() string {
	parts := []string{"GitHub Actions"}

	if e.repoOwner != "" && e.repoName != "" {
		parts = append(parts, "repo="+e.repoOwner+"/"+e.repoName)
	}
	if e.prNumber > 0 {
		parts = append(parts, "PR=#"+strconv.Itoa(e.prNumber))
	}
	if e.eventName != "" {
		parts = append(parts, "event="+e.eventName)
	}

	return strings.Join(parts, ", ")
}
