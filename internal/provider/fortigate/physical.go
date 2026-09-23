package fortigate

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
	"github.com/ciroiriarte/fortigate-cli/internal/transport"
)

// ListTransceivers reads monitor/system/interface/transceivers (optical DDM
// inventory).
//
// Field mapping assumed from documented shape; verify against a live build via
// `fgt api GET 'monitor/system/interface/transceivers?action=schema'`.
//
// The endpoint schema is UNVERIFIED, so this decodes tolerantly: results may be
// an array of records, an object keyed by interface name, or a single nested
// container; DDM readings may be nested (optical_data/diagnostics/ddm) or flat,
// and each reading may be a bare number or an object with value + thresholds +
// alarm/warning flags. Each returned Transceiver carries the untouched device
// record in Raw for faithful json/yaml passthrough. A 404/unsupported endpoint
// (e.g. a FortiGate-VM with no optics) yields an empty slice and nil error.
func (f *fortiGate) ListTransceivers(ctx context.Context) ([]domain.Transceiver, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/system/interface/transceivers"})
	if err != nil {
		if protocol.IsNotFound(err) {
			return []domain.Transceiver{}, nil // unavailable on this platform → N/A
		}
		return nil, err
	}
	var payload any
	if err := protocol.DecodeData(body, &payload); err != nil {
		return nil, err
	}
	recs := toRecords(payload)
	out := make([]domain.Transceiver, 0, len(recs))
	for _, nr := range recs {
		out = append(out, decodeTransceiver(nr))
	}
	return out, nil
}

// ListSensors reads monitor/system/sensor-info (power/fan/temperature/voltage).
//
// Field mapping is aligned to the documented monitor/system/sensor-info shape
// (id/name/type/value/alarm + a nested "thresholds" object); still verify against
// a specific live build via `fgt api GET 'monitor/system/sensor-info?action=schema'`,
// since hardware sensors over REST exist only on some models.
//
// Same tolerance and absent-hardware contract as ListTransceivers.
func (f *fortiGate) ListSensors(ctx context.Context) ([]domain.Sensor, error) {
	body, err := f.cl.DoRaw(ctx, &transport.Request{Method: "GET", Path: "monitor/system/sensor-info"})
	if err != nil {
		if protocol.IsNotFound(err) {
			return []domain.Sensor{}, nil // unavailable on this platform → N/A
		}
		return nil, err
	}
	var payload any
	if err := protocol.DecodeData(body, &payload); err != nil {
		return nil, err
	}
	recs := toRecords(payload)
	out := make([]domain.Sensor, 0, len(recs))
	for _, nr := range recs {
		out = append(out, decodeSensor(nr))
	}
	return out, nil
}

// --- tolerant shape normalization ---------------------------------------

// namedRecord pairs a verbatim device record with the collection key it was
// found under (empty for array/single shapes). Keeping the key separate lets the
// decoder use it for identity WITHOUT mutating the record that becomes .Raw, so
// json/yaml passthrough stays byte-faithful to the device.
type namedRecord struct {
	rec  map[string]any
	name string
}

// toRecords normalizes an unwrapped monitor payload into a flat list of records,
// coping with the common FortiOS shapes: a plain array, an object keyed by name
// (values are objects), or a single object nesting the collection under one key.
func toRecords(v any) []namedRecord {
	switch t := v.(type) {
	case []any:
		return mapsFrom(t)
	case map[string]any:
		// A container that nests the collection under one key (e.g.
		// {"transceivers":[...]}): descend deterministically.
		if recs := descendArray(t); recs != nil {
			return recs
		}
		// Object keyed by name: every value is itself an object.
		if keyed := keyedRecords(t); keyed != nil {
			return keyed
		}
		// Otherwise treat the object itself as a single record.
		if len(t) > 0 {
			return []namedRecord{{rec: t}}
		}
	}
	return nil
}

// mapsFrom keeps only the object elements of a list.
func mapsFrom(arr []any) []namedRecord {
	out := make([]namedRecord, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, namedRecord{rec: m})
		}
	}
	return out
}

// descendArray finds the collection an object nests under a single key, choosing
// deterministically (Go map iteration is randomized): a known container key
// first, then the longest array value, tie-broken by the lexicographically-first
// key name.
func descendArray(m map[string]any) []namedRecord {
	for _, k := range []string{"results", "transceivers", "sensors", "sensor", "data"} {
		if arr, ok := m[k].([]any); ok {
			if recs := mapsFrom(arr); len(recs) > 0 {
				return recs
			}
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var best []any
	for _, k := range keys {
		if arr, ok := m[k].([]any); ok && len(arr) > len(best) {
			best = arr
		}
	}
	if recs := mapsFrom(best); len(recs) > 0 {
		return recs
	}
	return nil
}

// keyedRecords treats m as a name->object map when every value is an object,
// returning records in a deterministic (sorted-by-key) order. The map key is
// carried as namedRecord.name, never injected into the record.
func keyedRecords(m map[string]any) []namedRecord {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]namedRecord, 0, len(m))
	for _, k := range keys {
		mv, ok := m[k].(map[string]any)
		if !ok {
			return nil // not a uniform keyed collection
		}
		out = append(out, namedRecord{rec: mv, name: k})
	}
	return out
}

// --- per-record decoders ------------------------------------------------

// ddmContainers are the nested keys under which a build may group DDM readings.
var ddmContainers = []string{"optical_data", "diagnostics", "ddm", "dom", "diag"}

func decodeTransceiver(nr namedRecord) domain.Transceiver {
	rec := nr.rec
	iface := getString(rec, "interface", "name", "port", "intf")
	if iface == "" {
		iface = nr.name // keyed-collection key, kept out of Raw
	}
	t := domain.Transceiver{
		Interface: iface,
		Vendor:    getString(rec, "vendor", "vendor_name"),
		Part:      getString(rec, "part", "part_number", "vendor_part_number", "vendor_pn", "model"),
		Serial:    getString(rec, "serial", "serial_number", "vendor_sn"),
		Type:      getString(rec, "type", "form_factor", "sfp_type"),
		Raw:       rec,
	}
	// Build the scope readings are looked up in: the record itself, overlaid with
	// any nested DDM container.
	scope := rec
	for _, c := range ddmContainers {
		if nested, ok := rec[c].(map[string]any); ok {
			scope = merge(rec, nested)
			break
		}
	}
	t.TxPower = extractReading(scope, "tx_power", "txpower", "tx-power", "tx")
	t.RxPower = extractReading(scope, "rx_power", "rxpower", "rx-power", "rx")
	t.Temperature = extractReading(scope, "temperature", "temp")
	t.Voltage = extractReading(scope, "voltage", "vcc", "volt")
	return t
}

// decodeSensor maps one sensor-info record onto domain.Sensor. The PRIMARY schema
// is the documented monitor/system/sensor-info shape — id/name/type/value/alarm
// plus a nested "thresholds" object that is now the main source of limits. The
// flat/alternative field spellings are KEPT as tolerant fallbacks so the /select
// variant and odd builds still degrade gracefully; missing/renamed/retyped fields
// never panic. Raw is the verbatim device record for json/yaml passthrough.
func decodeSensor(nr namedRecord) domain.Sensor {
	rec := nr.rec
	name := getString(rec, "name", "sensor", "label")
	if name == "" {
		name = nr.name // keyed-collection key, kept out of Raw
	}
	return domain.Sensor{
		ID:         getString(rec, "id", "sensor_id"),
		Name:       name,
		Type:       getString(rec, "type", "category", "sensor_type"),
		Value:      floatPtr(firstVal(rec, "value", "reading", "curr", "current")),
		Unit:       getString(rec, "unit", "units"),
		Status:     getString(rec, "status", "state", "alarm_status", "health"),
		Alarm:      boolPtr(firstVal(rec, "alarm", "alarm_flag", "in_alarm", "fault")),
		Thresholds: decodeSensorThresholds(rec),
		Raw:        rec,
	}
}

// decodeSensorThresholds reads the six sensor bounds. The primary source is the
// nested "thresholds" object; when a build omits it, the same keys are read flat
// off the record as a fallback. Each bound is optional — an absent or empty
// "thresholds":{} yields all-nil bounds (no basis to grade against).
func decodeSensorThresholds(rec map[string]any) domain.SensorThresholds {
	src := rec
	if nested, ok := rec["thresholds"].(map[string]any); ok {
		src = nested // documented primary path (may be empty {} → all-nil)
	}
	return domain.SensorThresholds{
		LowerNonRecoverable: floatPtr(firstVal(src, "lower_non_recoverable")),
		LowerCritical:       floatPtr(firstVal(src, "lower_critical")),
		LowerNonCritical:    floatPtr(firstVal(src, "lower_non_critical")),
		UpperNonCritical:    floatPtr(firstVal(src, "upper_non_critical")),
		UpperCritical:       floatPtr(firstVal(src, "upper_critical")),
		UpperNonRecoverable: floatPtr(firstVal(src, "upper_non_recoverable")),
	}
}

// extractReading pulls one DDM reading from scope. The reading node may be a
// bare number or an object carrying value + thresholds + alarm/warning flags;
// thresholds may also appear as flat "<base>_high_alarm"-style siblings. A
// numeric/text status field (0 ok / 1 warn / 2 alarm) is mapped onto the flags.
func extractReading(scope map[string]any, keys ...string) domain.DDMReading {
	var r domain.DDMReading
	var node any
	var base string
	for _, k := range keys {
		if v, ok := scope[k]; ok {
			node, base = v, k
			break
		}
	}
	switch t := node.(type) {
	case map[string]any:
		r.Value = floatPtr(firstVal(t, "value", "reading", "curr", "current"))
		r.HighAlarm = floatPtr(firstVal(t, "high_alarm", "high_alarm_threshold", "alarm_high", "high_alarm_level"))
		r.LowAlarm = floatPtr(firstVal(t, "low_alarm", "low_alarm_threshold", "alarm_low", "low_alarm_level"))
		r.HighWarn = floatPtr(firstVal(t, "high_warn", "high_warning", "warn_high", "high_warning_threshold"))
		r.LowWarn = floatPtr(firstVal(t, "low_warn", "low_warning", "warn_low", "low_warning_threshold"))
		r.Alarm = boolPtr(firstVal(t, "alarm", "alarm_flag", "in_alarm"))
		r.Warning = boolPtr(firstVal(t, "warning", "warn", "warn_flag", "in_warning"))
		applyStatus(&r, firstVal(t, "status", "state"))
	case nil:
		// no node located
	default:
		r.Value = floatPtr(node)
	}
	// Fill thresholds/flags from flat siblings when the object form did not carry
	// them (e.g. "tx_power" scalar with "tx_power_high_alarm" alongside).
	if base != "" {
		if r.HighAlarm == nil {
			r.HighAlarm = floatPtr(firstVal(scope, base+"_high_alarm", base+"_alarm_high"))
		}
		if r.LowAlarm == nil {
			r.LowAlarm = floatPtr(firstVal(scope, base+"_low_alarm", base+"_alarm_low"))
		}
		if r.HighWarn == nil {
			r.HighWarn = floatPtr(firstVal(scope, base+"_high_warn", base+"_warn_high", base+"_high_warning"))
		}
		if r.LowWarn == nil {
			r.LowWarn = floatPtr(firstVal(scope, base+"_low_warn", base+"_warn_low", base+"_low_warning"))
		}
	}
	return r
}

// applyStatus maps a per-reading status code/text onto the alarm/warning flags,
// without overwriting an explicit flag. Numeric convention assumed 1=warn,
// 2+=alarm (over-reporting is the safe direction); a 0/unknown numeric status is
// deliberately NOT treated as an "ok" basis, so an unverified convention cannot
// grade a reading PASS — it falls to N/A instead. Text is keyword-matched. This
// is a decode assumption — verify per build. Only sets flags still nil so an
// explicit alarm/warning wins.
func applyStatus(r *domain.DDMReading, v any) {
	switch t := v.(type) {
	case float64:
		switch {
		case t >= 2:
			setBoolIfNil(&r.Alarm, true)
		case t == 1:
			setBoolIfNil(&r.Warning, true)
		}
	case string:
		s := strings.ToLower(t)
		switch {
		case strings.Contains(s, "alarm"), strings.Contains(s, "critical"), strings.Contains(s, "fail"):
			setBoolIfNil(&r.Alarm, true)
		case strings.Contains(s, "warn"):
			setBoolIfNil(&r.Warning, true)
		case strings.Contains(s, "normal"), strings.Contains(s, "ok"), s == "good":
			setBoolIfNil(&r.Alarm, false)
		}
	}
}

func setBoolIfNil(p **bool, val bool) {
	if *p == nil {
		v := val
		*p = &v
	}
}

// --- scalar helpers -----------------------------------------------------

// merge returns a shallow copy of base overlaid with over (over wins).
func merge(base, over map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// firstVal returns the first present value among keys.
func firstVal(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// getString returns the first key resolvable to a non-empty string.
func getString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s := scalarString(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// scalarString renders a decoded JSON scalar as a display string ("" for
// absent/complex values); trailing ".0" is trimmed from whole floats.
func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

// duplexString normalizes a duplex value to "full"/"half"/"" across the shapes
// FortiOS uses (a "full"/"half" string, or 1/0).
func duplexString(v any) string {
	switch t := v.(type) {
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		if strings.HasPrefix(s, "full") {
			return "full"
		}
		if strings.HasPrefix(s, "half") {
			return "half"
		}
		return s
	case float64:
		if t == 1 {
			return "full"
		}
		if t == 0 {
			return "half"
		}
	case bool:
		if t {
			return "full"
		}
		return "half"
	}
	return ""
}

// floatPtr coerces a decoded JSON scalar to *float64 (nil when absent/unparseable).
func floatPtr(v any) *float64 {
	switch t := v.(type) {
	case float64:
		f := t
		return &f
	case int:
		f := float64(t)
		return &f
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return &f
		}
	}
	return nil
}

// boolPtr coerces a decoded JSON scalar to *bool (nil when absent). Numbers are
// truthy when non-zero; strings match true/1/enable/yes/on and their negatives.
func boolPtr(v any) *bool {
	switch t := v.(type) {
	case bool:
		b := t
		return &b
	case float64:
		b := t != 0
		return &b
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "enable", "enabled", "yes", "on":
			b := true
			return &b
		case "false", "0", "disable", "disabled", "no", "off":
			b := false
			return &b
		}
	}
	return nil
}
