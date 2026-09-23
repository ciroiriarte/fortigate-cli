// Package fortigate implements the Provider interface against a FortiGate's
// FortiOS REST API. cmdb endpoints back configuration reads/writes; monitor
// endpoints back runtime status.
package fortigate

import (
	"context"
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
		out = append(out, domain.Interface{
			Name:   n,
			Type:   v.Type,
			IP:     v.IP,
			Status: status,
			VDOM:   v.VDOM,
			Alias:  v.Alias,
		})
	}
	return out, nil
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
