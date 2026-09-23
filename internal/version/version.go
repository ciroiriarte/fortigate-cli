// Package version holds build metadata and the supported FortiOS version matrix.
package version

import (
	"fmt"
	"strings"
)

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

// SupportedMajors is the set of FortiOS <major>.<minor> series fgt targets.
var SupportedMajors = []string{"7.2", "7.4", "7.6", "8.0"}

// SupportsVersion reports whether a FortiOS version string (e.g. "v7.4.12" or
// "7.4.12") falls in the supported matrix, along with the detected
// <major>.<minor> ("" if unparseable).
func SupportsVersion(v string) (supported bool, majorMinor string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return false, ""
	}
	mm := parts[0] + "." + parts[1]
	for _, s := range SupportedMajors {
		if s == mm {
			return true, mm
		}
	}
	return false, mm
}

// Disclaimer is shown in version/help output. fortigate-cli is unofficial.
const Disclaimer = "fortigate-cli is an unofficial, community tool and is not affiliated with, authorized, or endorsed by Fortinet, Inc. FortiGate, FortiOS, and FortiSwitch are trademarks of Fortinet, Inc."

// String returns a human-readable one-line version string.
func String() string {
	return fmt.Sprintf("fgt %s (commit %s, built %s)", Version, Commit, Date)
}
