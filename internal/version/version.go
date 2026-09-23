// Package version holds build metadata and the supported FortiOS version matrix.
package version

import (
	"fmt"
	"strconv"
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

// Compare orders two FortiOS/CPE version strings numerically by their
// dot-separated components (x.y.z). It tolerates a leading "v" and ignores any
// trailing build/suffix (anything after the first character that is neither a
// digit nor a dot, e.g. "7.4.3-build1396" or "7.4.3 beta"). Missing components
// are treated as 0, so "7.4" == "7.4.0". It returns -1 if a < b, 0 if equal, +1
// if a > b. Used for CVE version-range matching.
func Compare(a, b string) int {
	pa, pb := parseVersionParts(a), parseVersionParts(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

// parseVersionParts normalizes a version string into its numeric components,
// stripping a leading "v"/whitespace and truncating at the first non-numeric,
// non-dot rune so build suffixes are ignored.
func parseVersionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// Cut off any build/suffix: keep only the leading run of digits and dots.
	end := len(v)
	for i, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			end = i
			break
		}
	}
	v = v[:end]
	if v == "" {
		return nil
	}
	fields := strings.Split(v, ".")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		if f == "" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// Disclaimer is shown in version/help output. fortigate-cli is unofficial.
const Disclaimer = "fortigate-cli is an unofficial, community tool and is not affiliated with, authorized, or endorsed by Fortinet, Inc. FortiGate, FortiOS, and FortiSwitch are trademarks of Fortinet, Inc."

// String returns a human-readable one-line version string.
func String() string {
	return fmt.Sprintf("fgt %s (commit %s, built %s)", Version, Commit, Date)
}
