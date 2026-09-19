// Package version holds build metadata and the supported FortiOS version matrix.
package version

import "fmt"

// Build info, injected via -ldflags at build time (see Makefile).
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Supported upstream version matrix, surfaced by `fgt version`. The REST API
// under /api/v2 has been stable across these releases; older minors are
// best-effort since cmdb object schemas shift between major versions.
const (
	SupportedFortiOS = "FortiOS 7.2.x / 7.4.x / 7.6.x (best-effort 8.0.x)"
)

// Disclaimer is shown in version/help output. fortigate-cli is unofficial.
const Disclaimer = "fortigate-cli is an unofficial, community tool and is not affiliated with, authorized, or endorsed by Fortinet, Inc. FortiGate, FortiOS, and FortiSwitch are trademarks of Fortinet, Inc."

// String returns a human-readable one-line version string.
func String() string {
	return fmt.Sprintf("fgt %s (commit %s, built %s)", Version, Commit, Date)
}
