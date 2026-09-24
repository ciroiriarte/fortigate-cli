package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// lldpFake serves a fixed LLDP neighbor set to the neighbors command.
type lldpFake struct {
	provider.Provider
	neighbors []domain.LLDPNeighbor
}

func (f *lldpFake) ListLLDPNeighbors(context.Context) ([]domain.LLDPNeighbor, error) {
	return f.neighbors, nil
}

// TestNetworkCommandTree asserts `network lldp neighbors` resolves under the root.
func TestNetworkCommandTree(t *testing.T) {
	root := NewRootCmd()
	if findCmd(root, "network", "lldp", "neighbors") == nil {
		t.Fatal("network lldp neighbors did not resolve")
	}
}

// TestLLDPNeighborsRender asserts the table renders both neighbors with the
// expected headers and sorts by local port.
func TestLLDPNeighborsRender(t *testing.T) {
	a := &app{prov: &lldpFake{neighbors: []domain.LLDPNeighbor{
		{
			LocalPort: "x2", NeighborName: "prd-svcs-sw-02", NeighborPort: "port58",
			ChassisID: "48:3A:02:D7:5E:A6", MgmtIPs: []string{"10.0.3.5"},
			SystemDesc: "FortiSwitch-2048F v8.0.0,build0047,260513 (GA)",
			Raw:        map[string]any{"port_name": "x2", "system_name": "prd-svcs-sw-02"},
		},
		{
			LocalPort: "x1", NeighborName: "prd-svcs-sw-01", NeighborPort: "port57",
			ChassisID: "48:3A:02:D7:5E:A5", MgmtIPs: []string{"10.0.3.4"},
			SystemDesc: "FortiSwitch-2048F v8.0.0,build0047,260513 (GA)",
			Raw:        map[string]any{"port_name": "x1", "system_name": "prd-svcs-sw-01"},
		},
	}}}
	cmd := lldpNeighborsCmd(a)

	out := captureStdout(t, func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("neighbors RunE: %v", err)
		}
	})

	for _, hdr := range []string{"LOCAL-PORT", "NEIGHBOR", "NEIGHBOR-PORT", "MGMT-IP", "CHASSIS-ID", "SYSTEM-DESC"} {
		if !strings.Contains(out, hdr) {
			t.Errorf("header missing %q:\n%s", hdr, out)
		}
	}
	for _, want := range []string{"prd-svcs-sw-01", "prd-svcs-sw-02", "10.0.3.4", "10.0.3.5", "port57", "port58"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Sorted by local port: x1 row must render before x2.
	if i, j := strings.Index(out, "x1"), strings.Index(out, "x2"); i < 0 || j < 0 || i > j {
		t.Errorf("rows not sorted by local port (x1 before x2):\n%s", out)
	}
}
