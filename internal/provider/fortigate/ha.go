package fortigate

import (
	"context"
	"strconv"
	"strings"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
	"github.com/ciroiriarte/fortigate-cli/internal/transport"
)

// HAMembers reads monitor/system/ha-peer (the cluster member roster).
//
// Grounded in the live FG100F/7.4.12 A-P shape: each record carries serial_no,
// hostname, priority, vcluster_id, and — on the primary only — master/primary
// true (absent on secondaries). Decoding reuses the tolerant array/keyed/nested
// normalization (toRecords) and defensive field lookups, so a missing/renamed/
// retyped field never panics. Each member carries its untouched record in Raw. A
// standalone unit returns a single member; a 404/unsupported endpoint yields an
// empty slice and nil error.
func (f *fortiGate) HAMembers(ctx context.Context) ([]domain.HAMember, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/system/ha-peer"})
	if err != nil {
		if protocol.IsNotFound(err) {
			return []domain.HAMember{}, nil
		}
		return nil, err
	}
	var payload any
	if err := protocol.DecodeData(body, &payload); err != nil {
		return nil, err
	}
	recs := toRecords(payload)
	out := make([]domain.HAMember, 0, len(recs))
	for _, nr := range recs {
		out = append(out, decodeHAMember(nr))
	}
	return out, nil
}

// HAConfig reads the cmdb/system/ha singleton (mode, group name, heartbeat
// interfaces). FortiOS returns the singleton's results as either a bare object
// or a 1-element array, so both are handled. The hbdev field is a single string
// of quoted interface names each followed by a weight (e.g. `"ha1" 50 "ha2" 50 `);
// parseHBDev extracts just the interface names. A 404/absent object (older or
// standalone build) yields a zero HAConfig and nil error.
func (f *fortiGate) HAConfig(ctx context.Context) (domain.HAConfig, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "cmdb/system/ha"})
	if err != nil {
		if protocol.IsNotFound(err) {
			return domain.HAConfig{}, nil
		}
		return domain.HAConfig{}, err
	}
	obj := decodeSingleton(body)
	return domain.HAConfig{
		Mode:          getString(obj, "mode"),
		GroupName:     getString(obj, "group-name", "group_name", "groupname"),
		HeartbeatDevs: parseHBDev(firstVal(obj, "hbdev", "heartbeat-device", "hbdev-list")),
	}, nil
}

// HAChecksums reads monitor/system/ha-checksums (per-member config-sync state).
//
// Grounded in the live shape: each record carries serial_no, is_manage_primary/
// is_root_primary, and a nested checksum object with all/global/root and a vdoms
// map. Config is in sync when every member's checksum.all matches. Decoding is
// tolerant with the original record in Raw. A 404/unsupported endpoint yields an
// empty slice and nil error.
func (f *fortiGate) HAChecksums(ctx context.Context) ([]domain.HAChecksum, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/system/ha-checksums"})
	if err != nil {
		if protocol.IsNotFound(err) {
			return []domain.HAChecksum{}, nil
		}
		return nil, err
	}
	var payload any
	if err := protocol.DecodeData(body, &payload); err != nil {
		return nil, err
	}
	recs := toRecords(payload)
	out := make([]domain.HAChecksum, 0, len(recs))
	for _, nr := range recs {
		out = append(out, decodeHAChecksum(nr))
	}
	return out, nil
}

// --- per-record decoders ------------------------------------------------

// decodeHAMember maps one ha-peer record onto domain.HAMember. Primary is true
// when EITHER master or primary is reported true (both appear on the elected
// unit); on a secondary both keys are absent, leaving Primary false. Raw is the
// verbatim device record.
func decodeHAMember(nr namedRecord) domain.HAMember {
	rec := nr.rec
	m := domain.HAMember{
		Hostname: getString(rec, "hostname", "name"),
		Serial:   getString(rec, "serial_no", "serial", "serial_number"),
		Raw:      rec,
	}
	if p := floatPtr(firstVal(rec, "priority")); p != nil {
		m.Priority = int(*p)
	}
	if v := floatPtr(firstVal(rec, "vcluster_id", "vcluster")); v != nil {
		m.VclusterID = int(*v)
	}
	if b := boolPtr(firstVal(rec, "master")); b != nil && *b {
		m.Primary = true
	}
	if b := boolPtr(firstVal(rec, "primary")); b != nil && *b {
		m.Primary = true
	}
	return m
}

// decodeHAChecksum maps one ha-checksums record onto domain.HAChecksum. The
// whole-config checksum and per-VDOM map come from the nested "checksum" object
// (with a flat fallback for odd builds). Primary reflects the device's
// is_manage_primary/is_root_primary flag. Raw is the verbatim device record.
func decodeHAChecksum(nr namedRecord) domain.HAChecksum {
	rec := nr.rec
	c := domain.HAChecksum{
		Serial: getString(rec, "serial_no", "serial", "serial_number"),
		Raw:    rec,
	}
	if b := boolPtr(firstVal(rec, "is_manage_primary", "is_root_primary", "primary", "master")); b != nil && *b {
		c.Primary = true
	}
	src := rec
	if nested, ok := rec["checksum"].(map[string]any); ok {
		src = nested
	}
	c.All = getString(src, "all", "checksum")
	if vd, ok := src["vdoms"].(map[string]any); ok {
		c.VDOMs = make(map[string]string, len(vd))
		for k, v := range vd {
			c.VDOMs[k] = scalarString(v)
		}
	}
	return c
}

// --- helpers ------------------------------------------------------------

// decodeSingleton unwraps a cmdb singleton's response body into one object,
// coping with FortiOS returning results as a bare object or a 1-element array.
func decodeSingleton(body []byte) map[string]any {
	var arr []map[string]any
	if err := protocol.DecodeData(body, &arr); err == nil && len(arr) > 0 {
		return arr[0]
	}
	var obj map[string]any
	if err := protocol.DecodeData(body, &obj); err == nil {
		return obj
	}
	return nil
}

// parseHBDev extracts the heartbeat INTERFACE names from the hbdev field, which
// FortiOS reports as a single string of quoted names each followed by a numeric
// weight (`"ha1" 50 "ha2" 50 `). It also tolerates an array shape (of names or
// {interface-name} objects) some builds/JSON-RPC fronts use.
func parseHBDev(v any) []string {
	switch t := v.(type) {
	case string:
		return parseHBDevString(t)
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			switch ev := e.(type) {
			case string:
				if s := strings.TrimSpace(ev); s != "" && !isNumericToken(s) {
					out = append(out, s)
				}
			case map[string]any:
				if s := getString(ev, "interface-name", "name", "interface"); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	return nil
}

// parseHBDevString pulls interface names out of the quoted-weight hbdev string.
// Quoted tokens are the interface names; the bare numbers between them are
// weights and dropped. If the string carries no quotes at all (an odd build), it
// falls back to whitespace splitting, keeping the non-numeric tokens.
func parseHBDevString(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote, sawQuote := false, false
	for _, r := range s {
		if r == '"' {
			if inQuote && cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			inQuote = !inQuote
			sawQuote = true
			continue
		}
		if inQuote {
			cur.WriteRune(r)
		}
	}
	if sawQuote {
		return out
	}
	for _, tok := range strings.Fields(s) {
		if !isNumericToken(tok) {
			out = append(out, tok)
		}
	}
	return out
}

// isNumericToken reports whether tok parses as a number (an hbdev weight).
func isNumericToken(tok string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(tok), 64)
	return err == nil
}
