package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Response header names for CLI version info (must match server)
const (
	HeaderUpgradeWarning  = "X-Cora-CLI-Upgrade-Warning"
	HeaderUpgradeRequired = "X-Cora-CLI-Upgrade-Required"
	HeaderLatestVersion   = "X-Cora-CLI-Latest-Version"
)

// checkVersionHeaders checks response headers for version warnings/errors
// and displays them to the user.
func checkVersionHeaders(resp *http.Response) {
	// Check for upgrade required (should be caught by 426 status, but belt and suspenders)
	if upgradeRequired := resp.Header.Get(HeaderUpgradeRequired); upgradeRequired != "" {
		fmt.Fprintf(os.Stderr, "\n⛔ %s\n\n", upgradeRequired)
		return
	}

	// Check for upgrade warning
	if upgradeWarning := resp.Header.Get(HeaderUpgradeWarning); upgradeWarning != "" {
		fmt.Fprintf(os.Stderr, "\n⚠️  %s\n\n", upgradeWarning)
	}

	// Optionally show latest version info
	// latestVersion := resp.Header.Get(HeaderLatestVersion)
	// Could be used for more detailed version comparison
}

// handleUpgradeRequired handles 426 Upgrade Required responses
func handleUpgradeRequired(respBody []byte, apiBaseURL string) error {
	fmt.Fprintf(os.Stderr, "\n⛔ CLI Upgrade Required\n")
	fmt.Fprintf(os.Stderr, "Your version of the Cora CLI is no longer supported.\n\n")
	fmt.Fprintf(os.Stderr, "To upgrade:\n")
	fmt.Fprintf(os.Stderr, "  brew upgrade cora\n")
	fmt.Fprintf(os.Stderr, "  # or download from https://github.com/clairitydev/cora-cli/releases\n\n")
	return fmt.Errorf("CLI version too old - upgrade required")
}

// checkCLIVersionFromDiscovery checks if the current CLI version meets server requirements
// based on the service discovery document.
func checkCLIVersionFromDiscovery(discovery *CoraServiceDiscovery) {
	if discovery == nil || discovery.CLI.MinimumVersion == "" {
		return
	}

	// Skip version check for dev builds
	if Version == "dev" {
		return
	}

	// Compare versions
	cmp := compareVersions(Version, discovery.CLI.MinimumVersion)
	if cmp < 0 {
		fmt.Fprintf(os.Stderr, "\n⛔ CLI Upgrade Required\n")
		fmt.Fprintf(os.Stderr, "Your CLI version (%s) is below the minimum required version (%s).\n\n", Version, discovery.CLI.MinimumVersion)
		fmt.Fprintf(os.Stderr, "To upgrade:\n")
		fmt.Fprintf(os.Stderr, "  brew upgrade cora\n")
		if discovery.CLI.DownloadURL != "" {
			fmt.Fprintf(os.Stderr, "  # or download from %s\n\n", discovery.CLI.DownloadURL)
		}
		return
	}

	// Check if update is recommended
	cmpRecommended := compareVersions(Version, discovery.CLI.RecommendedVersion)
	if cmpRecommended < 0 {
		fmt.Fprintf(os.Stderr, "\n⚠️  A newer CLI version is available (%s). You are using %s.\n", discovery.CLI.LatestVersion, Version)
		fmt.Fprintf(os.Stderr, "   Run 'brew upgrade cora' to update.\n\n")
	}
}

// compareVersions compares two semver version strings.
// Returns:
//   - negative if a < b
//   - 0 if a == b
//   - positive if a > b
func compareVersions(a, b string) int {
	partsA := parseVersion(a)
	partsB := parseVersion(b)

	if partsA == nil || partsB == nil {
		return 0 // Treat unparseable versions as equal
	}

	if partsA[0] != partsB[0] {
		return partsA[0] - partsB[0]
	}
	if partsA[1] != partsB[1] {
		return partsA[1] - partsB[1]
	}
	return partsA[2] - partsB[2]
}

// parseVersion parses a semver version string into [major, minor, patch].
// Returns nil if the version string is invalid.
func parseVersion(version string) []int {
	if version == "" || version == "dev" {
		return nil
	}

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split on dots and optional pre-release suffix
	parts := strings.SplitN(version, "-", 2) // Remove pre-release suffix
	version = parts[0]

	segments := strings.Split(version, ".")
	if len(segments) < 3 {
		return nil
	}

	result := make([]int, 3)
	for i := 0; i < 3; i++ {
		val, err := strconv.Atoi(segments[i])
		if err != nil {
			return nil
		}
		result[i] = val
	}

	return result
}
