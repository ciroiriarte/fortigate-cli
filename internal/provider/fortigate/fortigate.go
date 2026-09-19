// Package fortigate implements the Provider interface against a FortiGate's
// FortiOS REST API. cmdb endpoints back configuration reads/writes; monitor
// endpoints back runtime status.
package fortigate

import (
	"context"
	"net/url"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
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
		Mask  string `json:"mask"`
		Link  bool   `json:"link"`
		Type  string `json:"type"`
		VDOM  string `json:"vdom"`
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

// ListFirewallAddresses reads the cmdb surface (configuration objects).
func (f *fortiGate) ListFirewallAddresses(ctx context.Context) ([]domain.FirewallAddress, error) {
	var out []domain.FirewallAddress
	if err := f.cl.Do(ctx, &transport.Request{Method: "GET", Path: "cmdb/firewall/address"}, &out); err != nil {
		return nil, err
	}
	return out, nil
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

// Raw backs `fgt api` / `fgt raw`.
func (f *fortiGate) Raw(ctx context.Context, method, path string, params url.Values, body []byte) ([]byte, error) {
	return f.cl.DoRaw(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Query:  params,
		Body:   body,
	})
}
