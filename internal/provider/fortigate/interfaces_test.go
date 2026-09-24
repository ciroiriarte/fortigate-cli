package fortigate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/config"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// newScopedProvider stands up a fake FortiOS API wired to a provider whose
// transport targets the given vdom/global scope, so the cmdb VDOM filter in
// ListInterfacesFull can be exercised.
func newScopedProvider(t *testing.T, vdom string, global bool, handler http.HandlerFunc) provider.Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	s := &config.Settings{Server: srv.URL, AuthType: "token", Secret: "testtoken", VDOM: vdom, Global: global}
	p, err := provider.New(s, false)
	if err != nil {
		t.Fatalf("provider.New: %v", err)
	}
	return p
}

// multiVDOMHandler serves a cmdb table spanning several VDOMs (2 root, 1 AMSA)
// plus a monitor surface reporting one physical port. cmdb/system/interface is a
// global table, so it returns every VDOM's interfaces regardless of ?vdom=.
func multiVDOMHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v2/cmdb/system/interface":
		w.Write(envelope([]map[string]any{
			{"name": "port1", "type": "physical", "vdom": "root", "status": "up"},
			{"name": "vlan-root", "type": "vlan", "vdom": "root", "status": "up"},
			{"name": "vlan-amsa", "type": "vlan", "vdom": "AMSA", "status": "up"},
		}))
	case "/api/v2/monitor/system/interface":
		// The monitor surface is vdom-scoped by the transport (?vdom=/?global=),
		// unlike the global cmdb table. Model that: port1 lives in root, so only a
		// root-scoped or global request sees it; an AMSA-scoped request sees none.
		q := r.URL.Query()
		if q.Get("global") == "1" || q.Get("vdom") == "root" || (q.Get("vdom") == "" && q.Get("global") == "") {
			w.Write(envelope(map[string]any{
				"port1": map[string]any{"name": "port1", "link": true, "speed": 1000, "duplex": "full"},
			}))
			return
		}
		w.Write(envelope(map[string]any{})) // other VDOMs: no monitor interfaces
	default:
		http.NotFound(w, r)
	}
}

// TestListInterfacesFullMerges asserts the merged inventory covers ALL configured
// interfaces: logical interfaces present only in cmdb (a VLAN + a tunnel, absent
// from the monitor surface) surface with AdminStatus set and a blank live Status,
// while a physical port present in both gets the monitor link/speed/duplex/IP
// overlaid on top of its cmdb config row.
func TestListInterfacesFullMerges(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/cmdb/system/interface":
			w.Write(envelope([]map[string]any{
				{"name": "port1", "type": "physical", "vdom": "root", "status": "up", "ip": "0.0.0.0 0.0.0.0", "alias": "uplink"},
				{"name": "vlan10", "type": "vlan", "vdom": "root", "status": "up", "ip": "10.0.10.1 255.255.255.0", "interface": "port1", "vlanid": 10},
				{"name": "tun0", "type": "tunnel", "vdom": "root", "status": "down", "ip": "0.0.0.0 0.0.0.0"},
			}))
		case "/api/v2/monitor/system/interface":
			// keyed-by-name object; only the physical port is reported live.
			w.Write(envelope(map[string]any{
				"port1": map[string]any{
					"name": "port1", "type": "physical", "vdom": "root",
					"ip": "192.0.2.1", "link": true, "speed": 1000, "duplex": "full",
				},
			}))
		default:
			http.NotFound(w, r)
		}
	})

	ifaces, err := p.ListInterfacesFull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, iface := range ifaces {
		byName[iface.Name] = i
	}
	for _, n := range []string{"port1", "vlan10", "tun0"} {
		if _, ok := byName[n]; !ok {
			t.Fatalf("merged inventory missing %q; got %+v", n, ifaces)
		}
	}

	// Logical VLAN: cmdb-sourced, live link state blank, admin up, cmdb IP kept.
	vlan := ifaces[byName["vlan10"]]
	if vlan.AdminStatus != "up" {
		t.Errorf("vlan10 AdminStatus = %q, want up", vlan.AdminStatus)
	}
	if vlan.Status != "" {
		t.Errorf("vlan10 live Status = %q, want blank (absent from monitor)", vlan.Status)
	}
	if vlan.Type != "vlan" || vlan.IP != "10.0.10.1 255.255.255.0" {
		t.Errorf("vlan10 config fields wrong: %+v", vlan)
	}

	// Logical tunnel: admin down from cmdb, blank live status, zeroed IP dropped.
	tun := ifaces[byName["tun0"]]
	if tun.AdminStatus != "down" || tun.Status != "" || tun.IP != "" {
		t.Errorf("tun0 merged wrong: %+v", tun)
	}

	// Physical port: monitor overlay wins — live link up, speed/duplex set, and
	// the real monitor IP replaces the cmdb 0.0.0.0 placeholder.
	port := ifaces[byName["port1"]]
	if port.AdminStatus != "up" {
		t.Errorf("port1 AdminStatus = %q, want up", port.AdminStatus)
	}
	if port.Status != "up" {
		t.Errorf("port1 live Status = %q, want up", port.Status)
	}
	if port.Speed != "1000" || port.Duplex != "full" {
		t.Errorf("port1 live speed/duplex = %q/%q, want 1000/full", port.Speed, port.Duplex)
	}
	if port.IP != "192.0.2.1" {
		t.Errorf("port1 IP = %q, want monitor IP 192.0.2.1", port.IP)
	}
	if port.Alias != "uplink" {
		t.Errorf("port1 Alias = %q, want uplink (from cmdb)", port.Alias)
	}
}

// TestListInterfacesFullFiltersByVDOM asserts that, because cmdb/system/interface
// is a global table returning every VDOM's interfaces, a vdom-scoped call keeps
// ONLY the target VDOM's interfaces (other VDOMs filtered out).
func TestListInterfacesFullFiltersByVDOM(t *testing.T) {
	p := newScopedProvider(t, "AMSA", false, multiVDOMHandler)

	ifaces, err := p.ListInterfacesFull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("vdom=AMSA want 1 interface, got %d: %+v", len(ifaces), ifaces)
	}
	if ifaces[0].Name != "vlan-amsa" || ifaces[0].VDOM != "AMSA" {
		t.Errorf("vdom=AMSA returned wrong interface: %+v", ifaces[0])
	}
	// root's interfaces (incl. the physical port present in monitor) must be gone.
	for _, iface := range ifaces {
		if iface.VDOM == "root" {
			t.Errorf("root interface %q leaked into AMSA scope", iface.Name)
		}
	}
}

// TestListInterfacesFullDefaultVDOMIsRoot asserts that with neither --vdom nor
// --global set, the effective scope is root: only root's interfaces list.
func TestListInterfacesFullDefaultVDOMIsRoot(t *testing.T) {
	p := newScopedProvider(t, "", false, multiVDOMHandler)

	ifaces, err := p.ListInterfacesFull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, iface := range ifaces {
		if iface.VDOM != "root" {
			t.Errorf("default scope leaked non-root interface: %+v", iface)
		}
		names[iface.Name] = true
	}
	if !names["port1"] || !names["vlan-root"] || names["vlan-amsa"] {
		t.Errorf("default (root) scope wrong set: %+v", ifaces)
	}
	// The physical port still gets its monitor overlay under the default scope.
	for _, iface := range ifaces {
		if iface.Name == "port1" && (iface.Status != "up" || iface.Speed != "1000") {
			t.Errorf("port1 lost its monitor overlay under default scope: %+v", iface)
		}
	}
}

// TestListInterfacesFullGlobalKeepsAll asserts that under global scope the whole
// box view is kept: every VDOM's interfaces are returned unfiltered.
func TestListInterfacesFullGlobalKeepsAll(t *testing.T) {
	p := newScopedProvider(t, "", true, multiVDOMHandler)

	ifaces, err := p.ListInterfacesFull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 3 {
		t.Fatalf("global scope want all 3 interfaces, got %d: %+v", len(ifaces), ifaces)
	}
	vdoms := map[string]bool{}
	for _, iface := range ifaces {
		vdoms[iface.VDOM] = true
	}
	if !vdoms["root"] || !vdoms["AMSA"] {
		t.Errorf("global scope should span all VDOMs, got %+v", ifaces)
	}
}

// TestListInterfacesFullMonitorDegrades asserts a monitor read failure does not
// fail the command: the full cmdb inventory is still returned, with live fields
// left blank.
func TestListInterfacesFullMonitorDegrades(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/cmdb/system/interface":
			w.Write(envelope([]map[string]any{
				{"name": "vlan10", "type": "vlan", "vdom": "root", "status": "up", "ip": "10.0.10.1 255.255.255.0"},
				{"name": "tun0", "type": "tunnel", "vdom": "root", "status": "up"},
			}))
		case "/api/v2/monitor/system/interface":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status":"error","http_status":404}`))
		default:
			http.NotFound(w, r)
		}
	})

	ifaces, err := p.ListInterfacesFull(context.Background())
	if err != nil {
		t.Fatalf("monitor failure must not fail the command: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("want 2 cmdb interfaces despite monitor failure, got %d: %+v", len(ifaces), ifaces)
	}
	for _, iface := range ifaces {
		if iface.AdminStatus != "up" {
			t.Errorf("%s AdminStatus = %q, want up (from cmdb)", iface.Name, iface.AdminStatus)
		}
		if iface.Status != "" || iface.Speed != "" || iface.Duplex != "" {
			t.Errorf("%s should have blank live fields when monitor is down: %+v", iface.Name, iface)
		}
	}
}
