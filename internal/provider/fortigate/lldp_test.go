package fortigate

import (
	"context"
	"net/http"
	"testing"
)

// TestListLLDPNeighbors feeds the real FG100F/7.4.12 neighbor shape (two
// FortiSwitch neighbors on local ports x1/x2) and asserts the identity fields,
// the neighbor port (port_id), the flattened management IPs, and that .Raw
// carries the record verbatim including the nested addresses array.
func TestListLLDPNeighbors(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{
				"mac": "48:3a:02:d7:5e:de", "chassis_id": "48:3A:02:D7:5E:A5", "port": 23.0,
				"port_name": "x1", "port_id": "port57", "port_desc": "port57",
				"system_name": "prd-svcs-sw-01",
				"system_desc": "FortiSwitch-2048F v8.0.0,build0047,260513 (GA)",
				"ttl":         120.0,
				"addresses":   []map[string]any{{"type": "ipv4", "address": "10.0.3.4"}},
			},
			{
				"mac": "48:3a:02:d7:5e:df", "chassis_id": "48:3A:02:D7:5E:A6", "port": 24.0,
				"port_name": "x2", "port_id": "port58", "port_desc": "port58",
				"system_name": "prd-svcs-sw-02",
				"system_desc": "FortiSwitch-2048F v8.0.0,build0047,260513 (GA)",
				"ttl":         120.0,
				"addresses":   []map[string]any{{"type": "ipv4", "address": "10.0.3.5"}},
			},
		}))
	})
	neighbors, err := p.ListLLDPNeighbors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/monitor/network/lldp/neighbors" {
		t.Errorf("lldp neighbors hit %s %s", got.method, got.path)
	}
	if len(neighbors) != 2 {
		t.Fatalf("want 2 neighbors, got %d", len(neighbors))
	}

	n := neighbors[0]
	if n.LocalPort != "x1" {
		t.Errorf("local_port = %q, want x1", n.LocalPort)
	}
	if n.NeighborName != "prd-svcs-sw-01" {
		t.Errorf("neighbor_name = %q, want prd-svcs-sw-01", n.NeighborName)
	}
	if n.NeighborPort != "port57" {
		t.Errorf("neighbor_port = %q, want port57 (from port_id)", n.NeighborPort)
	}
	if n.ChassisID != "48:3A:02:D7:5E:A5" {
		t.Errorf("chassis_id = %q", n.ChassisID)
	}
	if n.MAC != "48:3a:02:d7:5e:de" {
		t.Errorf("mac = %q", n.MAC)
	}
	if len(n.MgmtIPs) != 1 || n.MgmtIPs[0] != "10.0.3.4" {
		t.Errorf("mgmt_ips = %v, want [10.0.3.4]", n.MgmtIPs)
	}
	if n.SystemDesc != "FortiSwitch-2048F v8.0.0,build0047,260513 (GA)" {
		t.Errorf("system_desc = %q", n.SystemDesc)
	}
	if n.TTL != 120 {
		t.Errorf("ttl = %d, want 120", n.TTL)
	}
	// Raw carries the original device record verbatim, including nested addresses.
	if n.Raw["port_name"] != "x1" || n.Raw["system_name"] != "prd-svcs-sw-01" {
		t.Errorf("raw record not passed through: %v", n.Raw)
	}
	addrs, ok := n.Raw["addresses"].([]any)
	if !ok || len(addrs) != 1 {
		t.Fatalf("raw addresses not verbatim: %v", n.Raw["addresses"])
	}
	a0, _ := addrs[0].(map[string]any)
	if a0["type"] != "ipv4" || a0["address"] != "10.0.3.4" {
		t.Errorf("raw nested address not verbatim: %v", addrs[0])
	}

	if neighbors[1].LocalPort != "x2" || neighbors[1].NeighborName != "prd-svcs-sw-02" {
		t.Errorf("second neighbor decoded wrong: %+v", neighbors[1])
	}
}

// TestListLLDPNeighborsPortDescFallback asserts NeighborPort falls back to
// port_desc when port_id is absent.
func TestListLLDPNeighborsPortDescFallback(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope([]map[string]any{
			{"port_name": "x1", "system_name": "sw", "port_desc": "GigabitEthernet0/1"},
		}))
	})
	neighbors, err := p.ListLLDPNeighbors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(neighbors) != 1 {
		t.Fatalf("want 1 neighbor, got %d", len(neighbors))
	}
	if neighbors[0].NeighborPort != "GigabitEthernet0/1" {
		t.Errorf("neighbor_port = %q, want port_desc fallback", neighbors[0].NeighborPort)
	}
}

// TestListLLDPNeighborsEmpty asserts an empty result (no neighbors / LLDP
// reception off) yields no neighbors and no error.
func TestListLLDPNeighborsEmpty(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope([]any{}))
	})
	neighbors, err := p.ListLLDPNeighbors(context.Background())
	if err != nil {
		t.Fatalf("empty result must not error: %v", err)
	}
	if len(neighbors) != 0 {
		t.Errorf("want 0 neighbors, got %d", len(neighbors))
	}
}

// TestListLLDPNeighbors404 asserts a 404/unsupported endpoint degrades to an
// empty slice + nil error, never a panic or failure.
func TestListLLDPNeighbors404(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"error","http_status":404,"error":-3}`))
	})
	neighbors, err := p.ListLLDPNeighbors(context.Background())
	if err != nil {
		t.Fatalf("404 must be swallowed: %v", err)
	}
	if len(neighbors) != 0 {
		t.Errorf("want 0 neighbors on 404, got %d", len(neighbors))
	}
}
