package cli

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// ifaceListFake serves a pre-merged interface set to the list command.
type ifaceListFake struct {
	provider.Provider
	ifaces []domain.Interface
}

func (f *ifaceListFake) ListInterfacesFull(context.Context) ([]domain.Interface, error) {
	return f.ifaces, nil
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what was
// written (a.render writes directly to os.Stdout).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

// TestInterfaceListMergedRender asserts `list` renders the merged inventory,
// including a cmdb-only logical interface (ADMIN populated, LINK blank), proving
// the command no longer goes empty for logical-only VDOMs.
func TestInterfaceListMergedRender(t *testing.T) {
	a := &app{prov: &ifaceListFake{ifaces: []domain.Interface{
		{Name: "vlan10", Type: "vlan", AdminStatus: "up", IP: "10.0.10.1 255.255.255.0", VDOM: "root"},
		{Name: "port1", Type: "physical", AdminStatus: "up", Status: "up", Speed: "1000", Duplex: "full", IP: "192.0.2.1", VDOM: "root"},
	}}}
	cmd := interfaceListCmd(a)

	out := captureStdout(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("list RunE: %v", err)
		}
	})

	if !strings.Contains(out, "ADMIN") || !strings.Contains(out, "LINK") {
		t.Errorf("header missing ADMIN/LINK columns:\n%s", out)
	}
	// The cmdb-only logical interface must render, non-empty.
	if !strings.Contains(out, "vlan10") {
		t.Errorf("cmdb-only logical interface vlan10 not rendered:\n%s", out)
	}
	// Locate the vlan10 row and assert ADMIN is populated while LINK is blank.
	var vlanRow string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "vlan10") {
			vlanRow = line
			break
		}
	}
	if vlanRow == "" {
		t.Fatalf("no vlan10 row in output:\n%s", out)
	}
	fields := strings.Fields(vlanRow)
	// Columns: NAME TYPE ADMIN LINK ...  → with LINK blank, field[2] (ADMIN) is
	// "up" and the token after it is not a link state.
	if len(fields) < 3 || fields[0] != "vlan10" || fields[1] != "vlan" || fields[2] != "up" {
		t.Errorf("vlan10 row ADMIN not populated as expected: %q", vlanRow)
	}
	// LINK blank: the row must not carry a live link token where port1's would be.
	if strings.Contains(vlanRow, "1000") || strings.Contains(vlanRow, "full") {
		t.Errorf("vlan10 row unexpectedly carries live link fields: %q", vlanRow)
	}
}
