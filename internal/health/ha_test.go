package health

import (
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
)

// twoMembers is the live A-P roster: one elected primary, one secondary.
func twoMembers() []domain.HAMember {
	return []domain.HAMember{
		{Hostname: "FGT-Amsa-Master", Serial: "FG100FTK23021921", Priority: 200, Primary: true},
		{Hostname: "FGT-Amsa-Backup", Serial: "FG100FTK23021317", Priority: 150},
	}
}

func syncedChecksums() []domain.HAChecksum {
	return []domain.HAChecksum{
		{Serial: "FG100FTK23021921", Primary: true, All: "deadbeef",
			VDOMs: map[string]string{"root": "bb", "AMSA": "ee"}},
		{Serial: "FG100FTK23021317", All: "deadbeef",
			VDOMs: map[string]string{"root": "bb", "AMSA": "ee"}},
	}
}

// sevByCode returns the severity of the first finding whose code equals code.
func sevByCode(t *testing.T, fs []Finding, code string) Severity {
	t.Helper()
	for _, f := range fs {
		if f.Code == code {
			return f.Severity
		}
	}
	t.Fatalf("no finding with code %q in %v", code, fs)
	return NA
}

// TestGradeHAAllGood is the live target: 2 members, both hbdev up, checksums in
// sync → all PASS, no WARN/CRITICAL.
func TestGradeHAAllGood(t *testing.T) {
	cfg := domain.HAConfig{Mode: "a-p", GroupName: "AMSA", HeartbeatDevs: []string{"ha1", "ha2"}}
	hb := map[string]bool{"ha1": true, "ha2": true}
	fs := GradeHA(twoMembers(), cfg, hb, syncedChecksums())

	overall, counts := Summarize(fs)
	if overall != Pass {
		t.Fatalf("overall = %s, want PASS; findings=%v", overall, fs)
	}
	if counts[string(Warn)] != 0 || counts[string(Critical)] != 0 {
		t.Errorf("expected no WARN/CRITICAL, got %v", counts)
	}
	if sevByCode(t, fs, "ha.primary") != Pass {
		t.Error("ha.primary should PASS with exactly one primary")
	}
	if sevByCode(t, fs, "ha.sync") != Pass {
		t.Error("ha.sync should PASS with matching checksums")
	}
	// One ha.member per unit.
	members := 0
	for _, f := range fs {
		if f.Code == "ha.member" {
			members++
		}
	}
	if members != 2 {
		t.Errorf("want 2 ha.member findings, got %d", members)
	}
}

// TestGradeHAHeartbeatDown asserts a down heartbeat link is CRITICAL.
func TestGradeHAHeartbeatDown(t *testing.T) {
	cfg := domain.HAConfig{Mode: "a-p", HeartbeatDevs: []string{"ha1", "ha2"}}
	hb := map[string]bool{"ha1": true, "ha2": false} // ha2 down
	fs := GradeHA(twoMembers(), cfg, hb, syncedChecksums())

	var ha2 Severity = NA
	for _, f := range fs {
		if f.Code == "ha.heartbeat" && f.Subject == "ha2" {
			ha2 = f.Severity
		}
	}
	if ha2 != Critical {
		t.Errorf("ha2 heartbeat down = %s, want CRITICAL", ha2)
	}
	if overall, _ := Summarize(fs); overall != Critical {
		t.Errorf("overall = %s, want CRITICAL", overall)
	}
}

// TestGradeHASingleHeartbeatRedundancy asserts a single configured hbdev raises a
// WARN redundancy finding (link itself still PASSes when up).
func TestGradeHASingleHeartbeatRedundancy(t *testing.T) {
	cfg := domain.HAConfig{Mode: "a-p", HeartbeatDevs: []string{"ha1"}}
	hb := map[string]bool{"ha1": true}
	fs := GradeHA(twoMembers(), cfg, hb, syncedChecksums())

	if sevByCode(t, fs, "ha.heartbeat.redundancy") != Warn {
		t.Error("single heartbeat link should WARN on redundancy")
	}
	if overall, _ := Summarize(fs); overall != Warn {
		t.Errorf("overall = %s, want WARN (redundancy only)", overall)
	}
}

// TestGradeHAHeartbeatUnknownIsNA asserts an hbdev absent from the link map is
// N/A for that dev, never a failure.
func TestGradeHAHeartbeatUnknownIsNA(t *testing.T) {
	cfg := domain.HAConfig{Mode: "a-p", HeartbeatDevs: []string{"ha1", "ha2"}}
	hb := map[string]bool{"ha1": true} // ha2 missing
	fs := GradeHA(twoMembers(), cfg, hb, syncedChecksums())

	var ha2 Severity = Pass
	for _, f := range fs {
		if f.Code == "ha.heartbeat" && f.Subject == "ha2" {
			ha2 = f.Severity
		}
	}
	if ha2 != NA {
		t.Errorf("ha2 (absent from link map) = %s, want N/A", ha2)
	}
	if overall, _ := Summarize(fs); overall == Critical || overall == Warn {
		t.Errorf("overall = %s, want a non-failing verdict", overall)
	}
}

// TestGradeHASyncMismatchNamesVDOM asserts a whole-config mismatch is CRITICAL
// and names the diverging VDOM when a per-VDOM checksum differs.
func TestGradeHASyncMismatchNamesVDOM(t *testing.T) {
	cfg := domain.HAConfig{Mode: "a-p", HeartbeatDevs: []string{"ha1", "ha2"}}
	hb := map[string]bool{"ha1": true, "ha2": true}
	sums := []domain.HAChecksum{
		{Serial: "A", All: "aaaa", VDOMs: map[string]string{"root": "bb", "AMSA": "ee"}},
		{Serial: "B", All: "bbbb", VDOMs: map[string]string{"root": "bb", "AMSA": "ff"}}, // AMSA differs
	}
	fs := GradeHA(twoMembers(), cfg, hb, sums)

	var sync Finding
	for _, f := range fs {
		if f.Code == "ha.sync" {
			sync = f
		}
	}
	if sync.Severity != Critical {
		t.Fatalf("ha.sync = %s, want CRITICAL", sync.Severity)
	}
	if !strings.Contains(sync.Message, "AMSA") {
		t.Errorf("ha.sync message should name diverging VDOM AMSA, got %q", sync.Message)
	}
}

// TestGradeHAStandalone asserts a standalone unit yields a single N/A ha.mode and
// no failures — never fail a non-clustered box.
func TestGradeHAStandalone(t *testing.T) {
	// Explicit standalone mode.
	fs := GradeHA(nil, domain.HAConfig{Mode: "standalone"}, nil, nil)
	if len(fs) != 1 || fs[0].Code != "ha.mode" || fs[0].Severity != NA {
		t.Fatalf("standalone should be a single N/A ha.mode, got %v", fs)
	}
	// Fewer than two members is also treated as not-clustered.
	fs = GradeHA([]domain.HAMember{{Hostname: "solo", Primary: true}}, domain.HAConfig{Mode: "a-p"}, nil, nil)
	if len(fs) != 1 || fs[0].Severity != NA {
		t.Errorf("single member should be N/A only, got %v", fs)
	}
}

// TestGradeHANoPrimary asserts zero elected primaries is CRITICAL (no active
// member).
func TestGradeHANoPrimary(t *testing.T) {
	members := []domain.HAMember{
		{Hostname: "a", Serial: "A", Priority: 200},
		{Hostname: "b", Serial: "B", Priority: 150},
	}
	cfg := domain.HAConfig{Mode: "a-p", HeartbeatDevs: []string{"ha1", "ha2"}}
	hb := map[string]bool{"ha1": true, "ha2": true}
	fs := GradeHA(members, cfg, hb, syncedChecksums())
	if sevByCode(t, fs, "ha.primary") != Critical {
		t.Error("zero primaries should be CRITICAL")
	}
}

// TestGradeHASplitBrain asserts more than one elected primary is CRITICAL.
func TestGradeHASplitBrain(t *testing.T) {
	members := []domain.HAMember{
		{Hostname: "a", Serial: "A", Priority: 200, Primary: true},
		{Hostname: "b", Serial: "B", Priority: 150, Primary: true},
	}
	cfg := domain.HAConfig{Mode: "a-p", HeartbeatDevs: []string{"ha1", "ha2"}}
	hb := map[string]bool{"ha1": true, "ha2": true}
	fs := GradeHA(members, cfg, hb, syncedChecksums())
	sync := findByCode(fs, "ha.primary")
	if sync.Severity != Critical || !strings.Contains(sync.Message, "split-brain") {
		t.Errorf("two primaries should be CRITICAL split-brain, got %+v", sync)
	}
}

func findByCode(fs []Finding, code string) Finding {
	for _, f := range fs {
		if f.Code == code {
			return f
		}
	}
	return Finding{}
}
