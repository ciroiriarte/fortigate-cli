// Package topology builds and renders an OBSERVED, one-hop topology graph of a
// FortiGate from live monitor data (device identity + LLDP neighbors + HA
// members), enriched with the cmdb/monitor interface inventory.
//
// This is intentionally backend-neutral: it consumes domain value types and
// untyped HA records (map[string]any) and emits strings, so the renderers are
// pure functions (model in -> string out) and unit-testable without any HTTP.
//
// The graph is NOT physical ground truth. LLDP is optional and can be stale, and
// unmanaged devices (no LLDP) are invisible; the graph shows only what this
// FortiGate currently observes in the queried VDOM scope.
package topology

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
)

// NodeType classifies a graph node for downstream styling/consumers.
type NodeType string

const (
	// NodeDevice is the center node: this FortiGate.
	NodeDevice NodeType = "device"
	// NodeNeighbor is a device discovered via LLDP.
	NodeNeighbor NodeType = "neighbor"
	// NodePeer is an HA cluster peer of this FortiGate.
	NodePeer NodeType = "peer"
)

// deviceID is the stable id of the center node.
const deviceID = "fgt"

// Node is a graph vertex. Label may contain "\n" to request a second display
// line; each renderer translates that separator to its own line-break syntax.
type Node struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Type  NodeType `json:"type"`
}

// Edge is a directed link from the device to a neighbor or HA peer.
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// Device carries the center node's identity for the json rendering.
type Device struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Model    string `json:"model"`
	Serial   string `json:"serial,omitempty"`
}

// Graph is the complete topology model. Nodes always includes the device node.
type Graph struct {
	Device Device `json:"device"`
	Nodes  []Node `json:"nodes"`
	Edges  []Edge `json:"edges"`
}

// Build assembles the topology model from live data. Neighbor NODES are keyed by
// physical device identity (chassis_id, then system_name, then mac), so a switch
// with several links to this FortiGate is one node with several edges — not one
// node per link. It is deterministic: LLDP entries are sorted by LocalPort then
// NeighborName (edge order), neighbor nodes are emitted sorted by id, and node
// ids are stable (same chassis -> same id; distinct chassis colliding on a
// sanitized name disambiguated _2/_3). Output ordering is fixed (device,
// neighbors, HA peers). Any absent/empty input degrades gracefully — an empty
// neighbor set still yields the center node.
func Build(dev domain.DeviceStatus, neighbors []domain.LLDPNeighbor, ha []map[string]any, ifaces []domain.Interface) Graph {
	used := map[string]bool{deviceID: true}

	g := Graph{
		Device: Device{ID: deviceID, Hostname: dev.Hostname, Model: dev.Model, Serial: dev.Serial},
	}
	g.Nodes = append(g.Nodes, Node{ID: deviceID, Label: deviceLabel(dev), Type: NodeDevice})

	speedByPort := make(map[string]string, len(ifaces))
	for _, i := range ifaces {
		speedByPort[i.Name] = i.Speed
	}

	sorted := append([]domain.LLDPNeighbor(nil), neighbors...)
	sort.SliceStable(sorted, func(a, b int) bool {
		if sorted[a].LocalPort != sorted[b].LocalPort {
			return sorted[a].LocalPort < sorted[b].LocalPort
		}
		return sorted[a].NeighborName < sorted[b].NeighborName
	})

	// Group neighbor NODES by physical device identity (chassis_id, falling back
	// to system_name then mac), so one switch triple-homed to this FortiGate is a
	// single node — not one phantom node per link. Each LLDP entry still yields
	// its own EDGE (local-port -> that node), preserving the per-link port label.
	type neighborNode struct {
		id, name, mgmt string
	}
	byChassis := make(map[string]*neighborNode)
	var order []*neighborNode
	for _, n := range sorted {
		key := firstNonEmpty(n.ChassisID, n.NeighborName, n.MAC)
		acc, ok := byChassis[key]
		if !ok {
			base := sanitizeID(n.NeighborName)
			if base == "" {
				base = sanitizeID(key)
			}
			// Same chassis always maps to the same id; only DISTINCT chassis that
			// sanitize to the same base get disambiguated (_2, _3) via uniqueID.
			acc = &neighborNode{id: uniqueID(base, used), name: firstNonEmpty(n.NeighborName, n.ChassisID, n.MAC)}
			byChassis[key] = acc
			order = append(order, acc)
		}
		if acc.mgmt == "" && len(n.MgmtIPs) > 0 && strings.TrimSpace(n.MgmtIPs[0]) != "" {
			acc.mgmt = strings.TrimSpace(n.MgmtIPs[0])
		}
		g.Edges = append(g.Edges, Edge{From: deviceID, To: acc.id, Label: edgeLabel(n, speedByPort[n.LocalPort])})
	}

	neighborNodes := make([]Node, 0, len(order))
	for _, acc := range order {
		label := acc.name
		if acc.mgmt != "" {
			label += "\n" + acc.mgmt
		}
		neighborNodes = append(neighborNodes, Node{ID: acc.id, Label: label, Type: NodeNeighbor})
	}
	sort.Slice(neighborNodes, func(i, j int) bool { return neighborNodes[i].ID < neighborNodes[j].ID })
	g.Nodes = append(g.Nodes, neighborNodes...)

	// HA peer(s): only meaningful when the cluster has more than one member.
	if len(ha) > 1 {
		for _, m := range ha {
			if isSelf(m, dev) {
				continue
			}
			label := firstNonEmpty(haString(m, "hostname"), haString(m, "serial_no"), haString(m, "serial"))
			if label == "" {
				continue
			}
			id := uniqueID(sanitizeID(label), used)
			g.Nodes = append(g.Nodes, Node{ID: id, Label: label, Type: NodePeer})
			g.Edges = append(g.Edges, Edge{From: deviceID, To: id, Label: "HA"})
		}
	}

	return g
}

// deviceLabel is "<hostname>\n<model>" (either part omitted when unknown).
func deviceLabel(dev domain.DeviceStatus) string {
	name := firstNonEmpty(dev.Hostname, dev.Serial, deviceID)
	if dev.Model != "" {
		return name + "\n" + dev.Model
	}
	return name
}

// edgeLabel is "<LocalPort> - <NeighborPort>", enriched with the local link
// speed when the interface is found and reports one (e.g. "x1 (10G) - port57").
func edgeLabel(n domain.LLDPNeighbor, speed string) string {
	local := n.LocalPort
	if s := humanSpeed(speed); s != "" {
		local = local + " (" + s + ")"
	}
	return local + " - " + n.NeighborPort
}

// humanSpeed converts an interface speed in Mbps (as FortiOS reports it) into a
// compact label ("10000" -> "10G", "2500" -> "2.5G", "100" -> "100M"). It
// returns "" for a blank/zero/unparseable speed so enrichment is skipped.
func humanSpeed(mbps string) string {
	mbps = strings.TrimSpace(mbps)
	if mbps == "" {
		return ""
	}
	v, err := strconv.ParseFloat(mbps, 64)
	if err != nil || v <= 0 {
		return ""
	}
	if v >= 1000 {
		return fmt.Sprintf("%gG", v/1000)
	}
	return fmt.Sprintf("%gM", v)
}

// isSelf reports whether an HA member record identifies this FortiGate.
func isSelf(m map[string]any, dev domain.DeviceStatus) bool {
	if dev.Serial != "" {
		if s := firstNonEmpty(haString(m, "serial_no"), haString(m, "serial")); s == dev.Serial {
			return true
		}
	}
	if dev.Hostname != "" && haString(m, "hostname") == dev.Hostname {
		return true
	}
	return false
}

// haString reads a string field from an untyped HA record, tolerating absence
// and non-string values.
func haString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// sanitizeID reduces a name to letters/digits/underscore, collapsing everything
// else to "_" and prefixing a leading digit so the result is a valid graph id.
func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	id := strings.Trim(b.String(), "_")
	if id == "" {
		return ""
	}
	if id[0] >= '0' && id[0] <= '9' {
		id = "n_" + id
	}
	return id
}

// uniqueID returns base (or "n" when empty) made unique against used, appending
// "_2", "_3", ... on collision, and records the result.
func uniqueID(base string, used map[string]bool) string {
	if base == "" {
		base = "n"
	}
	id := base
	for i := 2; used[id]; i++ {
		id = fmt.Sprintf("%s_%d", base, i)
	}
	used[id] = true
	return id
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
