package cli

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// haCheckFake serves HA facts to the `system ha check` command.
type haCheckFake struct {
	provider.Provider
	members   []domain.HAMember
	config    domain.HAConfig
	checksums []domain.HAChecksum
	ifaces    []domain.Interface
}

func (h *haCheckFake) HAMembers(context.Context) ([]domain.HAMember, error) { return h.members, nil }
func (h *haCheckFake) HAConfig(context.Context) (domain.HAConfig, error)    { return h.config, nil }
func (h *haCheckFake) HAChecksums(context.Context) ([]domain.HAChecksum, error) {
	return h.checksums, nil
}
func (h *haCheckFake) ListInterfaces(context.Context) ([]domain.Interface, error) {
	return h.ifaces, nil
}

func cleanCluster() *haCheckFake {
	return &haCheckFake{
		members: []domain.HAMember{
			{Hostname: "FGT-Amsa-Master", Serial: "FG100FTK23021921", Priority: 200, Primary: true},
			{Hostname: "FGT-Amsa-Backup", Serial: "FG100FTK23021317", Priority: 150},
		},
		config: domain.HAConfig{Mode: "a-p", GroupName: "AMSA", HeartbeatDevs: []string{"ha1", "ha2"}},
		checksums: []domain.HAChecksum{
			{Serial: "FG100FTK23021921", Primary: true, All: "deadbeef"},
			{Serial: "FG100FTK23021317", All: "deadbeef"},
		},
		ifaces: []domain.Interface{
			{Name: "ha1", Status: "up"},
			{Name: "ha2", Status: "up"},
		},
	}
}

// TestHACheckCommandTree asserts `system ha check` resolves and carries the CI
// exit-code contract in its long help.
func TestHACheckCommandTree(t *testing.T) {
	root := newSystemCmd(&app{})
	check := findCmd(root, "ha", "check")
	if check == nil {
		t.Fatal("system ha check did not resolve")
	}
	for _, sub := range []string{"--fail-on", "Exit codes", "heartbeat", "standalone", "N/A"} {
		if !strings.Contains(strings.ToLower(check.Long), strings.ToLower(sub)) {
			t.Errorf("ha check long help missing %q", sub)
		}
	}
}

// TestHACheckCleanRenders asserts a clean cluster renders the summary + findings
// table and exits 0 even with --fail-on error.
func TestHACheckCleanRenders(t *testing.T) {
	a := &app{prov: cleanCluster()}
	cmd := haCheckCmd(a)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureStdout(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("clean cluster RunE: %v", err)
		}
	})
	for _, want := range []string{"SEVERITY", "SUBJECT", "CODE", "MESSAGE", "ha.heartbeat", "ha.sync", "PASS"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestHACheckExitMapping asserts the exit-code contract: a down heartbeat +
// --fail-on error exits non-zero (2); the same cluster clean exits 0.
func TestHACheckExitMapping(t *testing.T) {
	// Clean cluster, --fail-on error → exit 0.
	a := &app{prov: cleanCluster()}
	cmd := haCheckCmd(a)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.Flags().Set("fail-on", "error")
	var cleanErr error
	captureStdout(t, func() { cleanErr = cmd.RunE(cmd, nil) })
	if got := ExitCodeFor(cleanErr); got != 0 {
		t.Errorf("clean cluster --fail-on error exit = %d, want 0", got)
	}

	// Down heartbeat, --fail-on error → exit 2.
	down := cleanCluster()
	down.ifaces = []domain.Interface{{Name: "ha1", Status: "up"}, {Name: "ha2", Status: "down"}}
	a2 := &app{prov: down}
	cmd2 := haCheckCmd(a2)
	cmd2.SetOut(io.Discard)
	cmd2.SetErr(io.Discard)
	cmd2.Flags().Set("fail-on", "error")
	var downErr error
	captureStdout(t, func() { downErr = cmd2.RunE(cmd2, nil) })
	if got := ExitCodeFor(downErr); got != 2 {
		t.Errorf("down heartbeat --fail-on error exit = %d, want 2", got)
	}
}
