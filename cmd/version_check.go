package cmd

import (
	"fmt"
	"net/http"
	"os"
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
