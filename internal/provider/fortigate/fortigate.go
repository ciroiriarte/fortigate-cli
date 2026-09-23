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
