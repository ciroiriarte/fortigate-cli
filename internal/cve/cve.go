// Package cve fetches FortiOS CVE data from public vulnerability feeds (NIST NVD
// primary, CIRCL fallback) and answers "which CVEs affect this running version"
// and "is this CVE fixed yet". It deliberately lives OUTSIDE internal/transport:
// that client is hard-pinned to the FortiGate host and /api/v2 with per-box TLS
// pinning and must never reach out to third-party services. This package owns
// its own *http.Client for the external calls.
//
// The data is best-effort: public feeds can lag or be incomplete. Authoritative
// guidance is the Fortinet PSIRT (https://www.fortiguard.com/psirt).
package cve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ciroiriarte/fortigate-cli/internal/version"
)

// ErrRateLimited is returned by a source that was throttled (HTTP 403/429). The
// auto orchestrator treats it as a signal to fall back to another source.
var ErrRateLimited = errors.New("cve: source rate-limited")

// ErrUnavailable is returned when a source is unreachable (network error) or
// returns a 5xx. Like ErrRateLimited, it signals the auto orchestrator to fall
// back to another source.
var ErrUnavailable = errors.New("cve: source unavailable")

// CVE is a single vulnerability record, normalized across feeds.
type CVE struct {
	ID           string    `json:"id"`
	Severity     string    `json:"severity"` // CRITICAL/HIGH/MEDIUM/LOW/NONE
	CVSS         float64   `json:"cvss"`     // base score, prefer CVSS v3.1 then v3.0 then v2
	Vector       string    `json:"vector,omitempty"`
	Published    time.Time `json:"published"`
	Modified     time.Time `json:"modified"`
	Description  string    `json:"description"`
	Affected     bool      `json:"affected"`               // meaningful for a specific-version query
	IntroducedIn string    `json:"introducedIn,omitempty"` // versionStartIncluding (may be "")
	FixedIn      string    `json:"fixedIn,omitempty"`      // versionEndExcluding — minimum fixed version (may be "")
	References   []string  `json:"references,omitempty"`
}

// Source is a CVE data backend.
type Source interface {
	// Name identifies the backend that answered (e.g. "nvd", "circl").
	Name() string
	// ForVersion returns the CVEs affecting the given running FortiOS version.
	ForVersion(ctx context.Context, fortiosVersion string) ([]CVE, error)
	// ByID returns one CVE and computes Affected/FixedIn for the running version.
	ByID(ctx context.Context, id, fortiosVersion string) (CVE, error)
}

// CPEFor builds the FortiOS CPE 2.3 URI for a version, normalizing a leading
// "v" and any build suffix away (e.g. "v7.4.3" -> the 7.4.3 CPE).
func CPEFor(v string) string {
	return "cpe:2.3:o:fortinet:fortios:" + normalizeVersion(v) + ":*:*:*:*:*:*:*"
}

// normalizeVersion strips a leading "v"/whitespace and any build suffix, so the
// value is a bare dotted version suitable for a CPE segment. It reuses the same
// truncation rule as version.Compare (digits and dots only).
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	for i, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return v[:i]
		}
	}
	return v
}

// severityRank maps a severity label to an ordinal for filtering/sorting.
// Higher is more severe; an unknown label ranks as 0 (NONE).
func severityRank(sev string) int {
	switch strings.ToUpper(strings.TrimSpace(sev)) {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}

// SeverityRank exposes the ordinal ranking of a severity label (CRITICAL=4 …
// NONE/unknown=0) for sorting and threshold filtering.
func SeverityRank(sev string) int { return severityRank(sev) }

// ParseMinSeverity maps a --min-severity flag value to its ordinal threshold.
// "none" (or "") means no filtering. An unknown value is an error.
func ParseMinSeverity(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none":
		return 0, nil
	case "low":
		return 1, nil
	case "medium":
		return 2, nil
	case "high":
		return 3, nil
	case "critical":
		return 4, nil
	default:
		return 0, fmt.Errorf("invalid --min-severity %q (want none|low|medium|high|critical)", s)
	}
}

// MeetsSeverity reports whether a CVE's severity is at or above the threshold.
func MeetsSeverity(c CVE, minRank int) bool {
	return severityRank(c.Severity) >= minRank
}

// severityFromScore derives a severity label from a CVSS base score using the
// standard CVSS v3 banding. Used whenever a feed exposes a score without a
// textual severity (v2 metrics, or a v3.x entry with an empty baseSeverity).
func severityFromScore(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0.0:
		return "LOW"
	default:
		return "NONE"
	}
}

// versionRange is a FortiOS-affecting version window drawn from a CPE match. Any
// bound may be empty (open on that side). exact pins a single vulnerable version
// when the CPE carries no range bounds at all.
type versionRange struct {
	startIncluding string // versionStartIncluding (inclusive lower bound)
	startExcluding string // versionStartExcluding (exclusive lower bound)
	endIncluding   string // versionEndIncluding (inclusive upper bound)
	endExcluding   string // versionEndExcluding (exclusive upper bound; the fix)
	exact          string // exact pinned version (used only when no bounds set)
}

// hasBounds reports whether any range bound is populated.
func (r versionRange) hasBounds() bool {
	return r.startIncluding != "" || r.startExcluding != "" ||
		r.endIncluding != "" || r.endExcluding != ""
}

// applyRange records the version window on a CVE relative to the running
// version. IntroducedIn takes the inclusive lower bound (the field's documented
// meaning); FixedIn takes the exclusive upper bound only — an inclusive upper
// bound (versionEndIncluding) does not name a clean single fixed version, so
// FixedIn is left empty while Affected still honors that bound.
func (c *CVE) applyRange(r versionRange, running string) {
	c.IntroducedIn = r.startIncluding
	c.FixedIn = r.endExcluding
	c.Affected = affectedByRange(running, r)
}

// affectedByRange reports whether running falls inside the range. All four NVD
// bound kinds are honored:
//   - startIncluding: running < start          → not affected
//   - startExcluding: running <= start         → not affected
//   - endExcluding:   running >= end           → not affected
//   - endIncluding:   running > end            → not affected
//
// With no bounds set it matches the exact pinned version; with neither bounds
// nor an exact version it returns true (a bare fortios CPE with no version
// scoping is treated as affecting).
func affectedByRange(running string, r versionRange) bool {
	if !r.hasBounds() {
		if r.exact != "" {
			return version.Compare(running, r.exact) == 0
		}
		return true
	}
	if r.startIncluding != "" && version.Compare(running, r.startIncluding) < 0 {
		return false
	}
	if r.startExcluding != "" && version.Compare(running, r.startExcluding) <= 0 {
		return false
	}
	if r.endExcluding != "" && version.Compare(running, r.endExcluding) >= 0 {
		return false
	}
	if r.endIncluding != "" && version.Compare(running, r.endIncluding) > 0 {
		return false
	}
	return true
}
