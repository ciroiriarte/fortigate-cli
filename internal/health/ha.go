package health

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
)

// GradeHA cross-checks the HA cluster from real device facts only: the member
// roster + roles, the heartbeat-link status, and the config-sync checksums. It
// never invents cluster state — an unknown fact is reported N/A, never a failure,
// and a standalone unit is never failed.
//
//   - Standalone / not clustered (mode "standalone" or fewer than two members):
//     one N/A ha.mode finding; the cluster checks are skipped.
//   - Members/roles: one PASS ha.member per unit, plus ha.primary — PASS when
//     exactly one primary is elected, CRITICAL on zero (no primary) or more than
//     one (split-brain).
//   - Heartbeat links (the headline): per configured hbdev interface, PASS when
//     its link is up, CRITICAL when down; a single configured hbdev also raises a
//     WARN ha.heartbeat.redundancy. An interface absent from the link map is N/A.
//   - Config sync: every member's checksum.All equal → PASS ha.sync; a mismatch →
//     CRITICAL ha.sync, naming the diverging per-VDOM checksum(s) when possible.
func GradeHA(members []domain.HAMember, cfg domain.HAConfig, hbLink map[string]bool, checksums []domain.HAChecksum) []Finding {
	if strings.EqualFold(cfg.Mode, "standalone") || len(members) < 2 {
		return []Finding{{
			Severity: NA, Code: "ha.mode", Subject: "cluster",
			Message: "standalone — HA not configured",
		}}
	}

	var out []Finding

	// Members / roles.
	primaries := 0
	for _, m := range members {
		role := "secondary"
		if m.Primary {
			role = "primary"
			primaries++
		}
		out = append(out, Finding{
			Severity: Pass, Code: "ha.member", Subject: haSubject(m),
			Message: fmt.Sprintf("serial %s, priority %d, role %s", emptyDash(m.Serial), m.Priority, role),
		})
	}
	switch {
	case primaries == 1:
		out = append(out, Finding{Pass, "ha.primary", "cluster", "exactly one primary elected"})
	case primaries == 0:
		out = append(out, Finding{Critical, "ha.primary", "cluster", "no primary elected — cluster has no active member"})
	default:
		out = append(out, Finding{Critical, "ha.primary", "cluster",
			fmt.Sprintf("split-brain: %d members claim primary", primaries)})
	}

	// Heartbeat links (headline).
	out = append(out, gradeHeartbeat(cfg.HeartbeatDevs, hbLink)...)

	// Config sync.
	out = append(out, gradeHASync(checksums))

	return out
}

// gradeHeartbeat grades each configured heartbeat interface against the observed
// link map. An interface missing from the map is N/A (not a failure). A single
// configured heartbeat interface adds a redundancy WARNING.
func gradeHeartbeat(devs []string, hbLink map[string]bool) []Finding {
	if len(devs) == 0 {
		return []Finding{{NA, "ha.heartbeat", "heartbeat", "no heartbeat interfaces configured"}}
	}
	var out []Finding
	for _, dev := range devs {
		up, ok := hbLink[dev]
		switch {
		case !ok:
			out = append(out, Finding{NA, "ha.heartbeat", dev,
				fmt.Sprintf("heartbeat link %s not found in interface status", dev)})
		case up:
			out = append(out, Finding{Pass, "ha.heartbeat", dev, fmt.Sprintf("heartbeat link %s up", dev)})
		default:
			out = append(out, Finding{Critical, "ha.heartbeat", dev, fmt.Sprintf("heartbeat link %s down", dev)})
		}
	}
	if len(devs) == 1 {
		out = append(out, Finding{Warn, "ha.heartbeat.redundancy", "heartbeat",
			"single heartbeat link — no redundancy"})
	}
	return out
}

// gradeHASync compares the whole-config checksum across members. All equal →
// PASS; a mismatch → CRITICAL, naming which per-VDOM checksum diverges when the
// VDOM maps make it identifiable. Fewer than two members, or no checksum
// reported, is N/A (no basis).
func gradeHASync(checksums []domain.HAChecksum) Finding {
	if len(checksums) < 2 {
		return Finding{NA, "ha.sync", "config-sync", "config-sync checksums not reported"}
	}
	base := checksums[0].All
	if base == "" {
		return Finding{NA, "ha.sync", "config-sync", "config-sync checksums not reported"}
	}
	inSync := true
	for _, c := range checksums[1:] {
		if c.All != base {
			inSync = false
			break
		}
	}
	if inSync {
		return Finding{Pass, "ha.sync", "config-sync", "config in sync across all members"}
	}
	if vdoms := divergingVDOMs(checksums); len(vdoms) > 0 {
		return Finding{Critical, "ha.sync", "config-sync",
			fmt.Sprintf("config out of sync: VDOM %s differs", strings.Join(vdoms, ", "))}
	}
	return Finding{Critical, "ha.sync", "config-sync", "config out of sync: whole-config checksum mismatch"}
}

// divergingVDOMs returns the VDOM names whose per-VDOM checksum is not identical
// across all members (sorted for stable messaging). A VDOM only some members
// report is treated as diverging.
func divergingVDOMs(checksums []domain.HAChecksum) []string {
	names := map[string]struct{}{}
	for _, c := range checksums {
		for name := range c.VDOMs {
			names[name] = struct{}{}
		}
	}
	var diverged []string
	for name := range names {
		var ref string
		set := false
		mismatch := false
		for _, c := range checksums {
			v, ok := c.VDOMs[name]
			if !ok {
				mismatch = true
				break
			}
			if !set {
				ref, set = v, true
				continue
			}
			if v != ref {
				mismatch = true
				break
			}
		}
		if mismatch {
			diverged = append(diverged, name)
		}
	}
	sort.Strings(diverged)
	return diverged
}

// haSubject names a member by hostname, falling back to serial then a placeholder.
func haSubject(m domain.HAMember) string {
	if m.Hostname != "" {
		return m.Hostname
	}
	if m.Serial != "" {
		return m.Serial
	}
	return "member"
}

// emptyDash renders "-" for an empty string in a finding message.
func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
