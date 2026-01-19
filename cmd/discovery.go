package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CoraServiceDiscovery represents the service discovery document from .well-known/cora.json
type CoraServiceDiscovery struct {
	Version   string           `json:"version"`
	CLI       CLIVersionInfo   `json:"cli"`
	Endpoints ServiceEndpoints `json:"endpoints"`
	Features  FeatureFlags     `json:"features"`
}

// CLIVersionInfo contains CLI version requirements
type CLIVersionInfo struct {
	MinimumVersion     string `json:"minimumVersion"`
	RecommendedVersion string `json:"recommendedVersion"`
	LatestVersion      string `json:"latestVersion"`
	DownloadURL        string `json:"downloadUrl"`
}

// ServiceEndpoints contains API endpoint paths
type ServiceEndpoints struct {
	StateUpload string `json:"stateUpload"`
	PlanUpload  string `json:"planUpload"`
	TokenVerify string `json:"tokenVerify"`
	Workspaces  string `json:"workspaces"`
	Health      string `json:"health"`
}

// FeatureFlags indicates which features are available
type FeatureFlags struct {
	PRRiskAssessment   bool                     `json:"prRiskAssessment"`
	StateEncryption    bool                     `json:"stateEncryption"`
	SensitiveFiltering SensitiveFilteringConfig `json:"sensitiveFiltering"`
}

// SensitiveFilteringConfig contains platform-level filtering settings
type SensitiveFilteringConfig struct {
	Available                bool     `json:"available"`
	Enforced                 bool     `json:"enforced"`
	AdditionalOmitTypes      []string `json:"additionalOmitTypes"`
	AdditionalOmitAttributes []string `json:"additionalOmitAttributes"`
}

// Default endpoints (fallback if discovery fails)
var defaultEndpoints = ServiceEndpoints{
	StateUpload: "/api/terraform-state",
	PlanUpload:  "/api/plans/upload",
	TokenVerify: "/api/tokens/verify",
	Workspaces:  "/api/workspaces",
	Health:      "/api/health",
}

var defaultDiscovery = CoraServiceDiscovery{
	Version: "1.0",
	CLI: CLIVersionInfo{
		MinimumVersion:     "0.1.0",
		RecommendedVersion: "0.2.0",
		LatestVersion:      "0.2.0",
		DownloadURL:        "https://github.com/clairitydev/cora-cli/releases/latest",
	},
	Endpoints: defaultEndpoints,
	Features: FeatureFlags{
		PRRiskAssessment: true,
		StateEncryption:  true,
		SensitiveFiltering: SensitiveFilteringConfig{
			Available:                true,
			Enforced:                 false,
			AdditionalOmitTypes:      []string{},
			AdditionalOmitAttributes: []string{},
		},
	},
}

// cachedDiscovery holds the cached service discovery document
var (
	cachedDiscovery     *CoraServiceDiscovery
	cachedDiscoveryBase string
	discoveryMutex      sync.RWMutex
	discoveryCacheTime  time.Time
	discoveryCacheTTL   = 1 * time.Hour
)

// FetchServiceDiscovery retrieves the service discovery document from the API.
// Results are cached for 1 hour to avoid repeated network calls.
// If a token is provided, it's sent for account-specific settings (e.g., filtering rules).
func FetchServiceDiscovery(baseURL, token string) (*CoraServiceDiscovery, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Check cache first
	discoveryMutex.RLock()
	if cachedDiscovery != nil && cachedDiscoveryBase == baseURL && time.Since(discoveryCacheTime) < discoveryCacheTTL {
		discovery := cachedDiscovery
		discoveryMutex.RUnlock()
		return discovery, nil
	}
	discoveryMutex.RUnlock()

	// Fetch from server
	discoveryURL := fmt.Sprintf("%s/.well-known/cora.json", baseURL)
	LogVerbose("📡 Fetching service discovery from %s", discoveryURL)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("cora-cli/%s", Version))
	req.Header.Set("X-Cora-CLI-Version", Version)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		// On network error, return defaults
		LogVerbose("⚠️  Discovery request failed: %v, using defaults", err)
		return useDefaultDiscovery(baseURL, nil)
	}
	defer resp.Body.Close()

	LogVerbose("📥 Discovery response: %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		// On non-200, return defaults (server might not support discovery yet)
		LogVerbose("⚠️  Discovery returned non-200, using defaults")
		return useDefaultDiscovery(baseURL, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		LogVerbose("⚠️  Failed to read discovery response: %v", err)
		return useDefaultDiscovery(baseURL, err)
	}

	var discovery CoraServiceDiscovery
	if err := json.Unmarshal(body, &discovery); err != nil {
		LogVerbose("⚠️  Failed to parse discovery JSON: %v", err)
		return useDefaultDiscovery(baseURL, err)
	}

	// Log filtering settings
	LogVerbose("🔒 Sensitive filtering available: %v, enforced: %v",
		discovery.Features.SensitiveFiltering.Available,
		discovery.Features.SensitiveFiltering.Enforced)
	if len(discovery.Features.SensitiveFiltering.AdditionalOmitTypes) > 0 {
		LogVerbose("   Organization omit types: %v", discovery.Features.SensitiveFiltering.AdditionalOmitTypes)
	}
	if len(discovery.Features.SensitiveFiltering.AdditionalOmitAttributes) > 0 {
		LogVerbose("   Organization omit attributes: %v", discovery.Features.SensitiveFiltering.AdditionalOmitAttributes)
	}

	// Cache the result
	discoveryMutex.Lock()
	cachedDiscovery = &discovery
	cachedDiscoveryBase = baseURL
	discoveryCacheTime = time.Now()
	discoveryMutex.Unlock()

	return &discovery, nil
}

// useDefaultDiscovery returns the default discovery document and caches it
func useDefaultDiscovery(baseURL string, originalErr error) (*CoraServiceDiscovery, error) {
	discoveryMutex.Lock()
	cachedDiscovery = &defaultDiscovery
	cachedDiscoveryBase = baseURL
	discoveryCacheTime = time.Now()
	discoveryMutex.Unlock()

	// Return defaults without error - this is expected for older servers
	return &defaultDiscovery, nil
}

// GetEndpointURL constructs the full URL for a given endpoint path
func GetEndpointURL(baseURL, path string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

// ClearDiscoveryCache clears the cached service discovery document
// Useful for testing or when switching API URLs
func ClearDiscoveryCache() {
	discoveryMutex.Lock()
	cachedDiscovery = nil
	cachedDiscoveryBase = ""
	discoveryCacheTime = time.Time{}
	discoveryMutex.Unlock()
}
