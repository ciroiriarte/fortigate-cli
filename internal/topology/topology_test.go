package topology

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
)

// twoSwitchGraph is the canonical fixture: this FortiGate + two FortiSwitch LLDP
// neighbors (x1->prd-svcs-sw-01 port57 at 10G, x2->prd-svcs-sw-02 port57) and one
// HA peer.
func twoSwitchGraph() Graph {
	dev := domain.DeviceStatus{Hostname: "FGT-Amsa-Master", Model: "FG100F", Serial: "FG100FTK1"}
	neighbors := []domain.LLDPNeighbor{
		{LocalPort: "x2", NeighborName: "prd-svcs-sw-02", NeighborPort: "port57", MgmtIPs: []string{"10.0.3.5"}},
		{LocalPort: "x1", NeighborName: "prd-svcs-sw-01", NeighborPort: "port57", MgmtIPs: []string{"10.0.3.4"}},
	}
	ha := []map[string]any{
		{"serial_no": "FG100FTK1", "hostname": "FGT-Amsa-Master"},
		{"serial_no": "FG100FTK2", "hostname": "FGT-Amsa-Slave"},
	}
	ifaces := []domain.Interface{
		{Name: "x1", Speed: "10000"},
		{Name: "x2", Speed: "1000"},
	}
	return Build(dev, neighbors, ha, ifaces)
}

func TestBuildDeterministicSortAndIDs(t *testing.T) {
	g := twoSwitchGraph()

	if g.Device.ID != "fgt" {
		t.Errorf("device id = %q, want fgt", g.Device.ID)
	}
	// 4 nodes: device + 2 switches + 1 HA peer.
	if len(g.Nodes) != 4 {
		t.Fatalf("nodes = %d, want 4: %+v", len(g.Nodes), g.Nodes)
	}
	// 3 edges: 2 LLDP + 1 HA.
	if len(g.Edges) != 3 {
		t.Fatalf("edges = %d, want 3: %+v", len(g.Edges), g.Edges)
	}
	// Neighbors sorted by LocalPort: x1's neighbor before x2's.
	if g.Nodes[1].Label != "prd-svcs-sw-01\n10.0.3.4" {
		t.Errorf("first neighbor node = %q, want prd-svcs-sw-01 (sorted by local port)", g.Nodes[1].Label)
	}
	if g.Nodes[1].ID != "prd_svcs_sw_01" || g.Nodes[2].ID != "prd_svcs_sw_02" {
		t.Errorf("neighbor ids = %q,%q, want prd_svcs_sw_01,prd_svcs_sw_02", g.Nodes[1].ID, g.Nodes[2].ID)
	}
	// Speed enrichment on the 10G x1 link; none forced on x2 label beyond speed.
	if g.Edges[0].Label != "x1 (10G) - port57" {
		t.Errorf("edge0 label = %q, want \"x1 (10G) - port57\"", g.Edges[0].Label)
	}
	if g.Edges[1].Label != "x2 (1G) - port57" {
		t.Errorf("edge1 label = %q, want \"x2 (1G) - port57\"", g.Edges[1].Label)
	}
	// HA edge present, peer identified as the non-self member.
	peer := g.Nodes[3]
	if peer.Type != NodePeer || peer.Label != "FGT-Amsa-Slave" {
		t.Errorf("peer node = %+v, want type peer / label FGT-Amsa-Slave", peer)
	}
	if g.Edges[2].Label != "HA" || g.Edges[2].To != peer.ID {
		t.Errorf("HA edge = %+v, want label HA to %s", g.Edges[2], peer.ID)
	}
}

func TestRenderMermaid(t *testing.T) {
	out := RenderMermaid(twoSwitchGraph())
	for _, want := range []string{
		"graph LR",
		`fgt["FGT-Amsa-Master<br/>FG100F"]`,
		`prd_svcs_sw_01["prd-svcs-sw-01<br/>10.0.3.4"]`,
		`prd_svcs_sw_02["prd-svcs-sw-02<br/>10.0.3.5"]`,
		`fgt -->|"x1 (10G) - port57"| prd_svcs_sw_01`,
		`fgt -->|"x2 (1G) - port57"| prd_svcs_sw_02`,
		`fgt -->|"HA"|`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mermaid missing %q:\n%s", want, out)
		}
	}
}

func TestRenderDOT(t *testing.T) {
	out := RenderDOT(twoSwitchGraph())
	if !strings.HasPrefix(out, "digraph topology {") {
		t.Errorf("dot does not open a digraph:\n%s", out)
	}
	for _, want := range []string{
		`fgt [label="FGT-Amsa-Master\nFG100F"];`,
		`prd_svcs_sw_01 [label="prd-svcs-sw-01\n10.0.3.4"];`,
		`fgt -> prd_svcs_sw_01 [label="x1 (10G) - port57"];`,
		`fgt -> prd_svcs_sw_02 [label="x2 (1G) - port57"];`,
		`[label="HA"];`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dot missing %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "}") {
		t.Errorf("dot does not close the digraph:\n%s", out)
	}
}

func TestRenderJSONDeterministic(t *testing.T) {
	g := twoSwitchGraph()
	out, err := RenderJSON(g)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var decoded Graph
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("json round-trip: %v", err)
	}
	if len(decoded.Nodes) != 4 || len(decoded.Edges) != 3 {
		t.Fatalf("json nodes=%d edges=%d, want 4/3", len(decoded.Nodes), len(decoded.Edges))
	}
	if decoded.Device.Hostname != "FGT-Amsa-Master" || decoded.Device.Model != "FG100F" {
		t.Errorf("json device = %+v", decoded.Device)
	}
	// Deterministic: two builds render byte-identical JSON.
	out2, _ := RenderJSON(twoSwitchGraph())
	if out != out2 {
		t.Errorf("json output not deterministic:\n%s\n---\n%s", out, out2)
	}
}

// TestEscaping asserts a neighbor name with a quote and a space is escaped in
// both mermaid and dot renderings.
func TestEscaping(t *testing.T) {
	dev := domain.DeviceStatus{Hostname: "fw", Model: "FG"}
	neighbors := []domain.LLDPNeighbor{
		{LocalPort: "x1", NeighborName: `sw "core" 01`, NeighborPort: "p1"},
	}
	g := Build(dev, neighbors, nil, nil)

	mm := RenderMermaid(g)
	if strings.Contains(mm, `"core"`) || !strings.Contains(mm, "&quot;core&quot;") {
		t.Errorf("mermaid did not escape quotes:\n%s", mm)
	}
	dot := RenderDOT(g)
	if !strings.Contains(dot, `\"core\"`) {
		t.Errorf("dot did not escape quotes:\n%s", dot)
	}
	// Id sanitized from a name with quotes/spaces.
	if g.Nodes[1].ID != "sw__core__01" {
		t.Errorf("sanitized id = %q, want sw__core__01", g.Nodes[1].ID)
	}
}

// TestChassisGroupingTripleHomed asserts one physical switch with three links to
// this FortiGate (same chassis_id + system_name, different local ports) renders
// as ONE neighbor node with THREE edges — the sw-core-promed live case.
func TestChassisGroupingTripleHomed(t *testing.T) {
	dev := domain.DeviceStatus{Hostname: "fw", Model: "FG"}
	neighbors := []domain.LLDPNeighbor{
		{LocalPort: "wan1", NeighborName: "sw-core-promed", NeighborPort: "port1", ChassisID: "CC:E1:94:10:24:8F", MgmtIPs: []string{"10.0.0.2"}},
		{LocalPort: "wan2", NeighborName: "sw-core-promed", NeighborPort: "port2", ChassisID: "CC:E1:94:10:24:8F"},
		{LocalPort: "port1", NeighborName: "sw-core-promed", NeighborPort: "port3", ChassisID: "CC:E1:94:10:24:8F"},
	}
	g := Build(dev, neighbors, nil, nil)

	// device + exactly one neighbor node.
	if len(g.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2 (device + 1 switch): %+v", len(g.Nodes), g.Nodes)
	}
	sw := g.Nodes[1]
	if sw.Type != NodeNeighbor || sw.ID != "sw_core_promed" || sw.Label != "sw-core-promed\n10.0.0.2" {
		t.Errorf("switch node = %+v, want single sw_core_promed node with mgmt IP", sw)
	}
	// Three edges, all to the one node, with the distinct per-link port labels.
	if len(g.Edges) != 3 {
		t.Fatalf("edges = %d, want 3: %+v", len(g.Edges), g.Edges)
	}
	wantLabels := map[string]bool{"port1 - port3": false, "wan1 - port1": false, "wan2 - port2": false}
	for _, e := range g.Edges {
		if e.To != sw.ID {
			t.Errorf("edge %+v does not target the single switch node %s", e, sw.ID)
		}
		if _, ok := wantLabels[e.Label]; !ok {
			t.Errorf("unexpected edge label %q", e.Label)
		}
		wantLabels[e.Label] = true
	}
	for l, seen := range wantLabels {
		if !seen {
			t.Errorf("missing edge label %q", l)
		}
	}
	// Sorted by LocalPort: port1 before wan1 before wan2.
	if g.Edges[0].Label != "port1 - port3" || g.Edges[1].Label != "wan1 - port1" || g.Edges[2].Label != "wan2 - port2" {
		t.Errorf("edges not sorted by local port: %+v", g.Edges)
	}
}

// TestSameNameDistinctChassis asserts two DIFFERENT switches that happen to share
// a system_name still render as two disambiguated nodes.
func TestSameNameDistinctChassis(t *testing.T) {
	dev := domain.DeviceStatus{Hostname: "fw", Model: "FG"}
	neighbors := []domain.LLDPNeighbor{
		{LocalPort: "x1", NeighborName: "access-sw", NeighborPort: "p1", ChassisID: "AA:AA:AA:AA:AA:AA"},
		{LocalPort: "x2", NeighborName: "access-sw", NeighborPort: "p1", ChassisID: "BB:BB:BB:BB:BB:BB"},
	}
	g := Build(dev, neighbors, nil, nil)
	if len(g.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3 (device + 2 distinct switches): %+v", len(g.Nodes), g.Nodes)
	}
	if g.Nodes[1].ID != "access_sw" || g.Nodes[2].ID != "access_sw_2" {
		t.Errorf("ids = %q,%q, want access_sw, access_sw_2", g.Nodes[1].ID, g.Nodes[2].ID)
	}
	if len(g.Edges) != 2 {
		t.Fatalf("edges = %d, want 2: %+v", len(g.Edges), g.Edges)
	}
}

// TestEmptyLLDPStillRendersCenter asserts the center node renders with no
// neighbors and no HA (standalone).
func TestEmptyLLDPStillRendersCenter(t *testing.T) {
	dev := domain.DeviceStatus{Hostname: "solo", Model: "FG40F"}
	g := Build(dev, nil, []map[string]any{{"hostname": "solo"}}, nil)
	if len(g.Nodes) != 1 || g.Nodes[0].Type != NodeDevice {
		t.Fatalf("nodes = %+v, want only the device node", g.Nodes)
	}
	if len(g.Edges) != 0 {
		t.Errorf("edges = %+v, want none", g.Edges)
	}
	out := RenderMermaid(g)
	if !strings.Contains(out, `fgt["solo<br/>FG40F"]`) {
		t.Errorf("mermaid missing center node:\n%s", out)
	}
}
