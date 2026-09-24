package fortigate

import (
	"context"
	"net/http"
	"testing"
)

// TestHAMembersTwoNodeCluster covers the live ha-peer shape: two members where
// the primary reports master/primary true and the secondary omits them.
func TestHAMembersTwoNodeCluster(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{"serial_no": "FG100FTK23021921", "vcluster_id": 0, "priority": 200, "hostname": "FGT-Amsa-Master", "master": true, "primary": true},
			{"serial_no": "FG100FTK23021317", "vcluster_id": 0, "priority": 150, "hostname": "FGT-Amsa-Backup"},
		}))
	})
	members, err := p.HAMembers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/monitor/system/ha-peer" {
		t.Errorf("ha-peer hit %s %s", got.method, got.path)
	}
	if len(members) != 2 {
		t.Fatalf("want 2 members, got %d", len(members))
	}
	pri := members[0]
	if pri.Hostname != "FGT-Amsa-Master" || pri.Serial != "FG100FTK23021921" || pri.Priority != 200 {
		t.Errorf("primary identity wrong: %+v", pri)
	}
	if !pri.Primary {
		t.Errorf("first member should be primary (master/primary true): %+v", pri)
	}
	sec := members[1]
	if sec.Primary {
		t.Errorf("second member should be secondary (master/primary absent): %+v", sec)
	}
	if sec.Priority != 150 {
		t.Errorf("secondary priority = %d, want 150", sec.Priority)
	}
	// Raw carries the original record verbatim.
	if members[0].Raw["serial_no"] != "FG100FTK23021921" {
		t.Errorf("raw record not passed through: %v", members[0].Raw)
	}
}

// TestHAMembersStandalone asserts a single-member roster decodes without error
// (the standalone case the grader treats as not-clustered).
func TestHAMembersStandalone(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(envelope([]map[string]any{
			{"serial_no": "FG100FTK00000001", "hostname": "solo", "priority": 128, "master": true, "primary": true},
		}))
	})
	members, err := p.HAMembers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
}

// TestHAConfigHBDevParse covers the cmdb/system/ha singleton and the quoted-weight
// hbdev string, which must parse to just the interface names.
func TestHAConfigHBDevParse(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope(map[string]any{
			"mode":       "a-p",
			"group-name": "AMSA-CLUSTER",
			"hbdev":      `"ha1" 50 "ha2" 50 `,
		}))
	})
	cfg, err := p.HAConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/v2/cmdb/system/ha" {
		t.Errorf("ha config hit %s", got.path)
	}
	if cfg.Mode != "a-p" || cfg.GroupName != "AMSA-CLUSTER" {
		t.Errorf("config fields wrong: %+v", cfg)
	}
	if len(cfg.HeartbeatDevs) != 2 || cfg.HeartbeatDevs[0] != "ha1" || cfg.HeartbeatDevs[1] != "ha2" {
		t.Errorf("hbdev parsed to %v, want [ha1 ha2]", cfg.HeartbeatDevs)
	}
}

// TestHAConfigNotFoundGraceful asserts a 404 (older/standalone build) yields a
// zero config and nil error, never a failure.
func TestHAConfigNotFoundGraceful(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"error","http_status":404}`))
	})
	cfg, err := p.HAConfig(context.Background())
	if err != nil {
		t.Fatalf("404 must degrade to zero config: %v", err)
	}
	if cfg.Mode != "" || len(cfg.HeartbeatDevs) != 0 {
		t.Errorf("want zero config on 404, got %+v", cfg)
	}
}

// TestHAChecksumsParse covers the ha-checksums shape, including the nested
// per-VDOM checksum map.
func TestHAChecksumsParse(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{
				"serial_no": "FG100FTK23021921", "is_manage_primary": true, "is_root_primary": true,
				"checksum": map[string]any{
					"global": "aa", "root": "bb", "all": "deadbeef",
					"vdoms": map[string]any{"root": "bb", "PROMED": "cc", "IaaS": "dd", "AMSA": "ee"},
				},
			},
			{
				"serial_no": "FG100FTK23021317",
				"checksum": map[string]any{
					"all":   "deadbeef",
					"vdoms": map[string]any{"root": "bb", "PROMED": "cc", "IaaS": "dd", "AMSA": "ee"},
				},
			},
		}))
	})
	sums, err := p.HAChecksums(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/v2/monitor/system/ha-checksums" {
		t.Errorf("ha-checksums hit %s", got.path)
	}
	if len(sums) != 2 {
		t.Fatalf("want 2 checksums, got %d", len(sums))
	}
	if sums[0].All != "deadbeef" || sums[1].All != "deadbeef" {
		t.Errorf("all checksum wrong: %q %q", sums[0].All, sums[1].All)
	}
	if !sums[0].Primary {
		t.Errorf("first member should be primary (is_manage_primary true): %+v", sums[0])
	}
	if sums[0].VDOMs["AMSA"] != "ee" || sums[0].VDOMs["root"] != "bb" {
		t.Errorf("per-vdom map wrong: %v", sums[0].VDOMs)
	}
	if sums[0].Raw["serial_no"] != "FG100FTK23021921" {
		t.Errorf("raw not passed through: %v", sums[0].Raw)
	}
}

// TestHAChecksumsEmptyGraceful asserts an empty result decodes to an empty slice.
func TestHAChecksumsEmptyGraceful(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(envelope([]map[string]any{}))
	})
	sums, err := p.HAChecksums(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 0 {
		t.Errorf("want 0 checksums, got %d", len(sums))
	}
}
