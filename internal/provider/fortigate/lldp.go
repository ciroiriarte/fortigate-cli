package fortigate

import (
	"context"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
	"github.com/ciroiriarte/fortigate-cli/internal/transport"
)

// ListLLDPNeighbors reads monitor/network/lldp/neighbors (the observed LLDP
// neighbor table), VDOM-scoped by the transport.
//
// Field mapping is grounded in the live FG100F/7.4.12 record shape (mac,
// chassis_id, port_name=local interface, port_id/port_desc=neighbor port,
// system_name=neighbor device, system_desc, ttl, addresses[]=neighbor mgmt IPs).
// It decodes tolerantly — the same array/keyed/nested normalization used for
// transceivers/sensors, and defensive field lookups so missing/renamed/retyped
// fields never panic. Each returned LLDPNeighbor carries the untouched device
// record in Raw for faithful json/yaml passthrough. An empty result (no
// neighbors, or LLDP reception disabled) or a 404/unsupported endpoint yields an
// empty slice and nil error.
func (f *fortiGate) ListLLDPNeighbors(ctx context.Context) ([]domain.LLDPNeighbor, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/network/lldp/neighbors"})
	if err != nil {
		if protocol.IsNotFound(err) {
			return []domain.LLDPNeighbor{}, nil // unavailable/unsupported → empty
		}
		return nil, err
	}
	var payload any
	if err := protocol.DecodeData(body, &payload); err != nil {
		return nil, err
	}
	recs := toRecords(payload)
	out := make([]domain.LLDPNeighbor, 0, len(recs))
	for _, nr := range recs {
		out = append(out, decodeLLDPNeighbor(nr))
	}
	return out, nil
}

// decodeLLDPNeighbor maps one neighbor record onto domain.LLDPNeighbor. The
// neighbor port falls back from port_id to port_desc; addresses[] is flattened
// into MgmtIPs. Raw is the verbatim device record.
func decodeLLDPNeighbor(nr namedRecord) domain.LLDPNeighbor {
	rec := nr.rec
	local := getString(rec, "port_name", "local_port", "interface", "port")
	if local == "" {
		local = nr.name // keyed-collection key, kept out of Raw
	}
	n := domain.LLDPNeighbor{
		LocalPort:    local,
		NeighborName: getString(rec, "system_name", "neighbor_name", "sys_name"),
		NeighborPort: getString(rec, "port_id", "port_desc", "neighbor_port", "remote_port"),
		ChassisID:    getString(rec, "chassis_id", "chassisid"),
		MAC:          getString(rec, "mac", "port_mac"),
		MgmtIPs:      mgmtIPs(rec),
		SystemDesc:   getString(rec, "system_desc", "sys_desc"),
		Raw:          rec,
	}
	if ttl := floatPtr(firstVal(rec, "ttl", "time_to_live")); ttl != nil {
		n.TTL = int(*ttl)
	}
	return n
}

// mgmtIPs flattens the neighbor's advertised management addresses. The live
// shape is addresses:[{"type":"ipv4","address":"10.0.3.4"}]; each entry's
// address string is collected (tolerating a bare-string entry too).
func mgmtIPs(rec map[string]any) []string {
	arr, ok := firstVal(rec, "addresses", "mgmt_addresses", "management_addresses").([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		switch t := e.(type) {
		case map[string]any:
			if ip := getString(t, "address", "ip", "mgmt_ip"); ip != "" {
				out = append(out, ip)
			}
		case string:
			if t != "" {
				out = append(out, t)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
