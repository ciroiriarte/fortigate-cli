package fortigate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/config"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// newTestProvider stands up a fake FortiOS API and returns a provider wired to
// it plus a pointer to the last request the server saw. httptest serves on
// loopback, where the transport permits plaintext http.
func newTestProvider(t *testing.T, handler http.HandlerFunc) provider.Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	s := &config.Settings{Server: srv.URL, AuthType: "token", Secret: "testtoken", VDOM: "root"}
	p, err := provider.New(s, false)
	if err != nil {
		t.Fatalf("provider.New: %v", err)
	}
	return p
}

type capture struct {
	method, path, query, auth string
	body                      map[string]any
}

func envelope(results any) []byte {
	b, _ := json.Marshal(map[string]any{
		"results": results, "status": "success", "http_status": 200, "vdom": "root",
	})
	return b
}

// writeResp writes a FortiOS write (create/update) response, where mkey is a
// top-level envelope field (sibling of results), not nested under results.
func writeResp(mkey string) []byte {
	b, _ := json.Marshal(map[string]any{
		"mkey": mkey, "status": "success", "http_status": 200, "revision": "3.0",
	})
	return b
}

func TestCmdbList(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{{"name": "web"}, {"name": "db"}}))
	})
	objs, err := p.CmdbList(context.Background(), "firewall/address")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/cmdb/firewall/address" {
		t.Errorf("list hit %s %s", got.method, got.path)
	}
	if len(objs) != 2 || objs[0]["name"] != "web" {
		t.Errorf("list returned %v", objs)
	}
}

func TestCmdbGetSingleObject(t *testing.T) {
	// FortiOS returns a single-object GET's results as a bare object.
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope(map[string]any{"name": "web", "type": "ipmask"}))
	})
	obj, err := p.CmdbGet(context.Background(), "firewall/address", "web")
	if err != nil {
		t.Fatal(err)
	}
	if obj["type"] != "ipmask" {
		t.Errorf("get returned %v", obj)
	}
}

func TestCmdbCreateSendsBodyAndReturnsMkey(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.query = r.Method, r.URL.Path, r.URL.RawQuery
		got.auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got.body)
		w.Write(writeResp("web"))
	})
	obj := provider.Object{
		"name": "web", "type": "ipmask", "subnet": "10.0.0.0 255.255.255.0",
		"srcaddr": []map[string]string{{"name": "all"}},
	}
	mkey, err := p.CmdbCreate(context.Background(), "firewall/address", obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "POST" || got.path != "/api/v2/cmdb/firewall/address" {
		t.Errorf("create hit %s %s", got.method, got.path)
	}
	if got.auth != "Bearer testtoken" {
		t.Errorf("auth header = %q", got.auth)
	}
	if !strings.Contains(got.query, "vdom=root") {
		t.Errorf("vdom not scoped: query=%q", got.query)
	}
	if got.body["name"] != "web" || got.body["type"] != "ipmask" {
		t.Errorf("create body = %v", got.body)
	}
	// child-table shape survives JSON round-trip as [{"name":"all"}]
	sa, ok := got.body["srcaddr"].([]any)
	if !ok || len(sa) != 1 {
		t.Fatalf("srcaddr wrong shape: %v", got.body["srcaddr"])
	}
	if m, _ := sa[0].(map[string]any); m["name"] != "all" {
		t.Errorf("srcaddr[0] = %v", sa[0])
	}
	if mkey != "web" {
		t.Errorf("returned mkey = %q, want web", mkey)
	}
}

func TestCmdbUpdateAndDeletePaths(t *testing.T) {
	var got capture
	handler := func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope(map[string]any{"mkey": "web"}))
	}
	p := newTestProvider(t, handler)

	if err := p.CmdbUpdate(context.Background(), "firewall/address", "web",
		provider.Object{"comment": "updated"}); err != nil {
		t.Fatal(err)
	}
	if got.method != "PUT" || got.path != "/api/v2/cmdb/firewall/address/web" {
		t.Errorf("update hit %s %s", got.method, got.path)
	}

	if err := p.CmdbDelete(context.Background(), "firewall/address", "web"); err != nil {
		t.Fatal(err)
	}
	if got.method != "DELETE" || got.path != "/api/v2/cmdb/firewall/address/web" {
		t.Errorf("delete hit %s %s", got.method, got.path)
	}
}

func TestCmdbUpdateSingleton(t *testing.T) {
	// An empty mkey (system/dns) must PUT the bare object path, no trailing key.
	var path string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write(envelope(map[string]any{}))
	})
	if err := p.CmdbUpdate(context.Background(), "system/dns", "",
		provider.Object{"primary": "1.1.1.1"}); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v2/cmdb/system/dns" {
		t.Errorf("singleton PUT path = %q", path)
	}
}

func TestHAStatus(t *testing.T) {
	// HAStatus returns raw records: whatever the device reports surfaces verbatim
	// (we do not model a per-build struct), so this asserts path + pass-through.
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{"serial_no": "FGT001", "hostname": "primary", "sessions": 1200},
			{"serial_no": "FGT002", "hostname": "secondary"},
		}))
	})
	members, err := p.HAStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/monitor/system/ha-statistics" {
		t.Errorf("ha-status hit %s %s", got.method, got.path)
	}
	if len(members) != 2 {
		t.Fatalf("ha members = %v", members)
	}
	if members[0]["serial_no"] != "FGT001" || members[1]["hostname"] != "secondary" {
		t.Errorf("ha member fields not passed through verbatim: %v", members)
	}
}

func TestSSLSessions(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{"user_name": "ciro", "remote_host": "203.0.113.9", "duration": 1200},
		}))
	})
	sess, err := p.SSLSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/monitor/vpn/ssl" {
		t.Errorf("ssl-sessions hit %s %s", got.method, got.path)
	}
	if len(sess) != 1 || sess[0]["user_name"] != "ciro" {
		t.Errorf("ssl sessions passthrough = %v", sess)
	}
}

func TestDeviceStatus(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		// serial/version/build are envelope-level; model/hostname in results.
		w.Write([]byte(`{"results":{"model":"FG100F","hostname":"fw1"},"serial":"FG100FTK1","version":"v7.4.12","build":2902,"status":"success"}`))
	})
	st, err := p.DeviceStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/v2/monitor/system/status" {
		t.Errorf("status hit %s", got.path)
	}
	if st.Hostname != "fw1" || st.Model != "FG100F" || st.Serial != "FG100FTK1" || st.Version != "v7.4.12" || st.Build != 2902 {
		t.Errorf("device status = %+v", st)
	}
}

func TestConfigBackup(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.query = r.Method, r.URL.Path, r.URL.RawQuery
		w.Write([]byte("#config-version=FG100F\nconfig system global\nend\n"))
	})
	cfg, err := p.ConfigBackup(context.Background(), "global", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/monitor/system/config/backup" {
		t.Errorf("backup hit %s %s", got.method, got.path)
	}
	if !strings.Contains(got.query, "scope=global") {
		t.Errorf("backup query = %q, want scope=global", got.query)
	}
	// backup body is the raw config text, not a JSON envelope.
	if !strings.HasPrefix(string(cfg), "#config-version=") {
		t.Errorf("backup returned %q", string(cfg)[:20])
	}
}

func TestConfigRestore(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got.body)
		w.Write(envelope(map[string]any{"status": "success"}))
	})
	if err := p.ConfigRestore(context.Background(), "global", "", []byte("config x\nend\n")); err != nil {
		t.Fatal(err)
	}
	if got.method != "POST" || got.path != "/api/v2/monitor/system/config/restore" {
		t.Errorf("restore hit %s %s", got.method, got.path)
	}
	if got.body["source"] != "upload" || got.body["scope"] != "global" {
		t.Errorf("restore body = %v", got.body)
	}
	// config is sent base64-encoded.
	if fc, _ := got.body["file_content"].(string); fc == "" {
		t.Errorf("restore missing base64 file_content: %v", got.body)
	}
}

func TestSchema(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.query = r.Method, r.URL.Path, r.URL.RawQuery
		w.Write(envelope(map[string]any{
			"mkey": "name", "mkey_type": "string",
			"children": map[string]any{"type": map[string]any{"type": "option"}},
		}))
	})
	sch, err := p.Schema(context.Background(), "firewall/address")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/cmdb/firewall/address" {
		t.Errorf("schema hit %s %s", got.method, got.path)
	}
	if !strings.Contains(got.query, "action=schema") {
		t.Errorf("schema query = %q, want action=schema", got.query)
	}
	if sch["mkey"] != "name" {
		t.Errorf("schema decoded wrong: %v", sch)
	}
}

func TestCmdbErrorDecodes(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"error","http_status":404,"error":-3,"cli_error":"entry not found"}`))
	})
	_, err := p.CmdbGet(context.Background(), "firewall/address", "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "entry not found") {
		t.Errorf("error = %v, want cli_error text", err)
	}
}
