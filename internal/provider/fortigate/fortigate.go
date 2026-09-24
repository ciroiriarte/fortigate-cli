// Package fortigate implements the Provider interface against a FortiGate's
// FortiOS REST API. cmdb endpoints back configuration reads/writes; monitor
// endpoints back runtime status.
package fortigate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
	"github.com/ciroiriarte/fortigate-cli/internal/transport"
)

func init() {
	provider.SetFortiGateFactory(func(cl *transport.Client) provider.Provider {
		return &fortiGate{cl: cl}
	})
}

type fortiGate struct {
	cl *transport.Client
}

func (f *fortiGate) Name() string { return "fortigate" }

// ListInterfaces reads the monitor surface, which reports live status/IP that
// the cmdb object alone does not carry.
func (f *fortiGate) ListInterfaces(ctx context.Context) ([]domain.Interface, error) {
	// monitor/system/interface returns an object keyed by interface name.
	var raw map[string]struct {
		Name  string `json:"name"`
		Alias string `json:"alias"`
		IP    string `json:"ip"`
		// mask shape varies by build (dotted string on some, numeric prefix
		// length on others), so decode it tolerantly; it is not rendered.
		Mask any    `json:"mask"`
		Link bool   `json:"link"`
		Type string `json:"type"`
		VDOM string `json:"vdom"`
		// speed/duplex shapes vary by build (speed as a number of Mbit/s or a
		// string; duplex as "full"/"half" or 0/1), so decode tolerantly.
		Speed  any `json:"speed"`
		Duplex any `json:"duplex"`
	}
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: "monitor/system/interface"}, &raw); err != nil {
		return nil, err
	}
	out := make([]domain.Interface, 0, len(raw))
	for name, v := range raw {
		status := "down"
		if v.Link {
			status = "up"
		}
		n := v.Name
		if n == "" {
			n = name
		}
		// Speed/duplex are only meaningful on an UP link. FortiOS reports
		// speed:0, duplex:0 for DOWN ports, which would otherwise render as a
		// bogus "0"/"half"; leave both empty ("n/a") for a down link.
		var speed, duplex string
		if v.Link {
			speed = scalarString(v.Speed)
			duplex = duplexString(v.Duplex)
		}
		out = append(out, domain.Interface{
			Name:   n,
			Type:   v.Type,
			IP:     v.IP,
			Status: status,
			VDOM:   v.VDOM,
			Alias:  v.Alias,
			Speed:  speed,
			Duplex: duplex,
		})
	}
	return out, nil
}

// ListInterfacesFull returns the merged interface inventory for the current VDOM
// scope: the cmdb config object is the authoritative full set (every configured
// interface, including logical VLANs/tunnels/zones the monitor surface omits),
// overlaid with the monitor surface's live link status/speed/duplex/IP where the
// interface name matches. Interfaces present only in the monitor surface are kept
// too. Either surface can 404/error independently without failing the command;
// only when BOTH surfaces yield nothing is an error returned.
//
// cmdb/system/interface is a GLOBAL table (FortiOS returns every VDOM's
// interfaces regardless of ?vdom=), so a vdom-scoped call filters it to the
// target VDOM by each interface's own "vdom" field; global scope keeps them all.
func (f *fortiGate) ListInterfacesFull(ctx context.Context) ([]domain.Interface, error) {
	cmdb, cmdbErr := f.cmdbInterfaces(ctx)
	// cmdb/system/interface is a GLOBAL config table: FortiOS returns every
	// interface across ALL VDOMs regardless of the ?vdom= param, each carrying its
	// own "vdom" field. Filter to the effective target VDOM so a vdom-scoped list
	// shows only that VDOM's interfaces; global scope keeps the whole-box view.
	if !f.cl.Global() {
		cmdb = filterByVDOM(cmdb, effectiveVDOM(f.cl.VDOM()))
	}

	monList, monErr := f.ListInterfaces(ctx)

	// Index the monitor live data by interface name for overlay lookup.
	mon := make(map[string]domain.Interface, len(monList))
	for _, m := range monList {
		mon[m.Name] = m
	}

	seen := make(map[string]bool, len(cmdb))
	out := make([]domain.Interface, 0, len(cmdb)+len(monList))
	for _, c := range cmdb {
		if c.Name == "" {
			continue
		}
		seen[c.Name] = true
		if m, ok := mon[c.Name]; ok {
			c.Status = m.Status // live link state
			c.Speed = m.Speed
			c.Duplex = m.Duplex
			if nonZeroIP(m.IP) { // prefer a real monitor IP over the cmdb config IP
				c.IP = m.IP
			}
		}
		out = append(out, c)
	}
	// Defensive: surface any interface the monitor knows but cmdb did not return.
	for _, m := range monList {
		if m.Name != "" && !seen[m.Name] {
			out = append(out, m)
		}
	}

	// Degrade gracefully: only fail when neither surface produced anything.
	if len(out) == 0 && cmdbErr != nil && monErr != nil {
		return nil, cmdbErr
	}
	return out, nil
}

// cmdbInterfaces reads the cmdb config inventory (the authoritative full set,
// including logical interfaces the monitor surface omits) into domain.Interface
// values carrying only the config-sourced fields (AdminStatus from "status", the
// configured IP, Type/VDOM/Alias). Live link fields are left blank for the merge
// to fill. Decoding is tolerant: missing/renamed/retyped fields never panic.
func (f *fortiGate) cmdbInterfaces(ctx context.Context) ([]domain.Interface, error) {
	// cmdb/system/interface returns an array of config objects.
	var raw []struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		VDOM   string `json:"vdom"`
		Status string `json:"status"` // admin up/down
		IP     string `json:"ip"`     // "A.B.C.D M.M.M.M", empty, or 0.0.0.0
		Alias  string `json:"alias"`
	}
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: "cmdb/system/interface"}, &raw); err != nil {
		return nil, err
	}
	out := make([]domain.Interface, 0, len(raw))
	for _, v := range raw {
		out = append(out, domain.Interface{
			Name:        v.Name,
			Type:        v.Type,
			IP:          normalizeCmdbIP(v.IP),
			AdminStatus: v.Status,
			VDOM:        v.VDOM,
			Alias:       v.Alias,
		})
	}
	return out, nil
}

// effectiveVDOM resolves the client's configured default VDOM to the concrete
// name FortiOS applies: an unset default ("") means the root VDOM.
func effectiveVDOM(vdom string) string {
	if vdom == "" {
		return "root"
	}
	return vdom
}

// filterByVDOM keeps only interfaces owned by the target VDOM (case-sensitive
// match on the FortiOS vdom name). An interface with an empty vdom field is kept
// defensively (e.g. a build that omits it on a non-multi-VDOM box), so a genuine
// single-VDOM device never filters itself empty.
func filterByVDOM(in []domain.Interface, target string) []domain.Interface {
	out := in[:0:0]
	for _, iface := range in {
		if iface.VDOM == target || iface.VDOM == "" {
			out = append(out, iface)
		}
	}
	return out
}

// nonZeroIP reports whether an IP string carries a real address rather than the
// unassigned 0.0.0.0 placeholder FortiOS returns for an unnumbered interface. It
// tolerates "addr", "addr mask", and "addr/pfx" shapes.
func nonZeroIP(s string) bool {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return false
	}
	addr := fields[0]
	if i := strings.IndexByte(addr, '/'); i >= 0 {
		addr = addr[:i]
	}
	return addr != "" && addr != "0.0.0.0" && addr != "::"
}

// normalizeCmdbIP blanks the 0.0.0.0 placeholder cmdb reports for an unnumbered
// interface, otherwise returns the configured "ip mask" string trimmed.
func normalizeCmdbIP(s string) string {
	if !nonZeroIP(s) {
		return ""
	}
	return strings.TrimSpace(s)
}

// DeviceStatus reads monitor/system/status. serial/version/build sit at the
// envelope top level (siblings of results); model/hostname are inside results.
func (f *fortiGate) DeviceStatus(ctx context.Context) (domain.DeviceStatus, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/system/status"})
	if err != nil {
		return domain.DeviceStatus{}, err
	}
	var raw struct {
		Serial  string `json:"serial"`
		Version string `json:"version"`
		Build   int    `json:"build"`
		Results struct {
			Model    string `json:"model"`
			Hostname string `json:"hostname"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.DeviceStatus{}, err
	}
	return domain.DeviceStatus{
		Hostname: raw.Results.Hostname,
		Model:    raw.Results.Model,
		Serial:   raw.Serial,
		Version:  raw.Version,
		Build:    raw.Build,
	}, nil
}

// cmdbPath joins the cmdb prefix with an object path and optional mkey.
func cmdbPath(path, mkey string) string {
	p := "cmdb/" + strings.Trim(path, "/")
	if mkey != "" {
		p += "/" + url.PathEscape(mkey)
	}
	return p
}

// CmdbList returns all objects at a cmdb path.
func (f *fortiGate) CmdbList(ctx context.Context, path string) ([]provider.Object, error) {
	var out []provider.Object
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: cmdbPath(path, "")}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CmdbGet returns one object. FortiOS returns a single-object GET's results as
// either a 1-element array or a bare object, so both are handled.
func (f *fortiGate) CmdbGet(ctx context.Context, path, mkey string) (provider.Object, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: cmdbPath(path, mkey)})
	if err != nil {
		return nil, err
	}
	var arr []provider.Object
	if err := protocol.DecodeData(body, &arr); err == nil && len(arr) > 0 {
		return arr[0], nil
	}
	var obj provider.Object
	if err := protocol.DecodeData(body, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// CmdbCreate POSTs a new object and returns its (possibly auto-assigned) mkey.
func (f *fortiGate) CmdbCreate(ctx context.Context, path string, obj provider.Object) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "POST", Path: cmdbPath(path, ""), Body: b})
	if err != nil {
		return "", err
	}
	env, err := protocol.DecodeEnvelope(body)
	if err != nil {
		return "", nil // created, but mkey unreadable
	}
	return env.MkeyString(), nil
}

// CmdbUpdate PUTs changed fields to path/mkey ("" mkey => singleton object).
func (f *fortiGate) CmdbUpdate(ctx context.Context, path, mkey string, obj provider.Object) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = f.cl.DoRaw(ctx, &transport.Request{Method: "PUT", Path: cmdbPath(path, mkey), Body: b})
	return err
}

// CmdbDelete removes path/mkey.
func (f *fortiGate) CmdbDelete(ctx context.Context, path, mkey string) error {
	_, err := f.cl.DoRaw(ctx, &transport.Request{Method: "DELETE", Path: cmdbPath(path, mkey)})
	return err
}

// ListManagedSwitches reports FortiLink-managed FortiSwitch units. These are
// administered through the FortiGate's switch-controller, never contacted
// directly.
func (f *fortiGate) ListManagedSwitches(ctx context.Context) ([]domain.ManagedSwitch, error) {
	var out []domain.ManagedSwitch
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: "monitor/switch-controller/managed-switch/status"}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// HAStatus reports HA cluster members from the monitor surface, returned as raw
// records so the exact per-build field set surfaces verbatim. On a standalone
// unit FortiOS returns a single-member list.
func (f *fortiGate) HAStatus(ctx context.Context) ([]provider.Object, error) {
	var out []provider.Object
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: "monitor/system/ha-statistics"}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SSLSessions reports active SSL-VPN sessions from the monitor surface, returned
// as raw records so the per-build field set surfaces verbatim.
func (f *fortiGate) SSLSessions(ctx context.Context) ([]provider.Object, error) {
	var out []provider.Object
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: "monitor/vpn/ssl"}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ConfigBackup downloads the device configuration (monitor surface). The
// response body is the config text itself, not a JSON envelope.
func (f *fortiGate) ConfigBackup(ctx context.Context, scope, vdom string) ([]byte, error) {
	q := url.Values{}
	if scope != "" {
		q.Set("scope", scope)
	}
	if vdom != "" {
		q.Set("vdom", vdom)
	}
	return f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/system/config/backup", Query: q})
}

// ConfigRestore uploads a configuration to the device (monitor surface). FortiOS
// expects source=upload plus the base64-encoded file content; this replaces the
// running config and usually reboots the unit.
func (f *fortiGate) ConfigRestore(ctx context.Context, scope, vdom string, config []byte) error {
	if scope == "" {
		scope = "global"
	}
	payload := map[string]any{
		"source":       "upload",
		"scope":        scope,
		"file_content": base64.StdEncoding.EncodeToString(config),
	}
	if vdom != "" {
		payload["vdom"] = vdom
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = f.cl.DoRaw(ctx, &transport.Request{Method: "POST", Path: "monitor/system/config/restore", Body: b})
	return err
}

// ImportCertificate POSTs a certificate to monitor/vpn-certificate/<store>/import
// with base64-encoded content. Key material never appears in a URL/query, and
// the transport's --debug logs only method+URL, so it is not logged.
func (f *fortiGate) ImportCertificate(ctx context.Context, req provider.CertImport) error {
	body := map[string]any{}
	if req.Scope != "" {
		body["scope"] = req.Scope
	}
	if len(req.Cert) > 0 {
		body["file_content"] = base64.StdEncoding.EncodeToString(req.Cert)
	}
	switch req.Store {
	case "local":
		if req.Type != "" {
			body["type"] = req.Type
		}
		if req.Name != "" {
			body["certname"] = req.Name
		}
		if len(req.Key) > 0 {
			body["key_file_content"] = base64.StdEncoding.EncodeToString(req.Key)
		}
		if req.Password != "" {
			body["password"] = req.Password
		}
	case "ca":
		body["import_method"] = "file"
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = f.cl.DoRaw(ctx, &transport.Request{
		Method: "POST",
		Path:   "monitor/vpn-certificate/" + req.Store + "/import",
		Body:   b,
	})
	return err
}

// Schema returns a cmdb object's field schema (GET cmdb/<path>?action=schema).
// FortiOS self-describes each object per build, so this is more authoritative
// than any static doc.
func (f *fortiGate) Schema(ctx context.Context, path string) (provider.Object, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{
		Method: "GET",
		Path:   cmdbPath(path, ""),
		Query:  url.Values{"action": {"schema"}},
	})
	if err != nil {
		return nil, err
	}
	var obj provider.Object
	if err := protocol.DecodeData(body, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// Raw backs `fgt api` / `fgt raw`.
func (f *fortiGate) Raw(ctx context.Context, method, path string, params url.Values, body []byte) ([]byte, error) {
	return f.cl.DoRaw(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Query:  params,
		Body:   body,
	})
}
