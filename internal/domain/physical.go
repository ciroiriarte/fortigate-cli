package domain

// Physical-layer value types for the "system transceiver/sensor/health" surface
// (monitor/system/interface/transceivers and monitor/system/sensor-info).
//
// The exact JSON of these two monitor endpoints is UNVERIFIED against a live
// build, so the fortigate provider decodes them defensively into these structs
// and ALSO carries the untouched device record in Raw. The typed fields drive
// only table columns and health grading; json/yaml output renders Raw so the
// device's original records remain the source of truth.

// DDMReading is one Digital Diagnostics Monitoring reading from an optical
// transceiver (tx/rx power in dBm, temperature, supply voltage). Every field is
// optional: a build may expose the value with no thresholds, thresholds with no
// value, an explicit alarm/warning flag, or nothing at all. Health grading uses
// ONLY what the device supplies here — it never invents a limit.
type DDMReading struct {
	Value     *float64 `json:"value,omitempty"`
	HighAlarm *float64 `json:"high_alarm,omitempty"`
	LowAlarm  *float64 `json:"low_alarm,omitempty"`
	HighWarn  *float64 `json:"high_warn,omitempty"`
	LowWarn   *float64 `json:"low_warn,omitempty"`
	// Alarm/Warning are the module-asserted flags when the endpoint reports them
	// (an explicit false is itself a device basis meaning "not in alarm").
	Alarm   *bool `json:"alarm,omitempty"`
	Warning *bool `json:"warning,omitempty"`
}

// Present reports whether the module gave a numeric reading to display/grade.
func (r DDMReading) Present() bool { return r.Value != nil }

// HasBasis reports whether the device supplied anything to grade the reading
// against — a threshold or an explicit alarm/warning flag. Without a basis a
// reading can only be reported at severity N/A (never invented into PASS/FAIL).
func (r DDMReading) HasBasis() bool {
	return r.HighAlarm != nil || r.LowAlarm != nil || r.HighWarn != nil ||
		r.LowWarn != nil || r.Alarm != nil || r.Warning != nil
}

// Transceiver is one pluggable optic (SFP/SFP+/QSFP...) and its DDM readings, as
// reported by monitor/system/interface/transceivers. Raw holds the original
// device record for faithful json/yaml passthrough.
type Transceiver struct {
	Interface   string         `json:"interface"`
	Vendor      string         `json:"vendor,omitempty"`
	Part        string         `json:"part,omitempty"`
	Serial      string         `json:"serial,omitempty"`
	Type        string         `json:"type,omitempty"`
	TxPower     DDMReading     `json:"tx_power"`
	RxPower     DDMReading     `json:"rx_power"`
	Temperature DDMReading     `json:"temperature"`
	Voltage     DDMReading     `json:"voltage"`
	Raw         map[string]any `json:"-"`
}

// SensorThresholds are the six IPMI-style bounds a sensor may carry in the nested
// "thresholds" object of monitor/system/sensor-info. Every bound is optional (nil
// when absent): a build may supply both sides, only one (e.g. temperature reports
// only upper_*), or an empty object with no bound at all. Grading uses ONLY the
// bounds the device supplies here — it never invents a limit.
type SensorThresholds struct {
	LowerNonRecoverable *float64 `json:"lower_non_recoverable,omitempty"`
	LowerCritical       *float64 `json:"lower_critical,omitempty"`
	LowerNonCritical    *float64 `json:"lower_non_critical,omitempty"`
	UpperNonCritical    *float64 `json:"upper_non_critical,omitempty"`
	UpperCritical       *float64 `json:"upper_critical,omitempty"`
	UpperNonRecoverable *float64 `json:"upper_non_recoverable,omitempty"`
}

// Any reports whether at least one bound is present.
func (th SensorThresholds) Any() bool {
	return th.HasLower() || th.HasUpper()
}

// HasLower reports whether any lower (floor) bound is present.
func (th SensorThresholds) HasLower() bool {
	return th.LowerNonRecoverable != nil || th.LowerCritical != nil || th.LowerNonCritical != nil
}

// HasUpper reports whether any upper (ceiling) bound is present.
func (th SensorThresholds) HasUpper() bool {
	return th.UpperNonCritical != nil || th.UpperCritical != nil || th.UpperNonRecoverable != nil
}

// Sensor is one hardware sensor (PSU/power, fan, temperature, voltage) from
// monitor/system/sensor-info. Value/Alarm and every Thresholds bound are optional;
// grading uses only the device-supplied thresholds and alarm flag (Status is a
// tolerant fallback for odd builds that report a flat status text). Raw holds the
// original device record for faithful json/yaml passthrough.
type Sensor struct {
	ID         string           `json:"id,omitempty"`
	Name       string           `json:"name"`
	Type       string           `json:"type,omitempty"`
	Value      *float64         `json:"value,omitempty"`
	Unit       string           `json:"unit,omitempty"`
	Status     string           `json:"status,omitempty"`
	Alarm      *bool            `json:"alarm,omitempty"`
	Thresholds SensorThresholds `json:"thresholds"`
	Raw        map[string]any   `json:"-"`
}

// HasBasis reports whether the device supplied anything to grade the value
// against — any threshold bound or an explicit alarm flag. Without a basis a
// sensor reading can only be reported N/A (never invented into PASS/FAIL).
func (s Sensor) HasBasis() bool {
	return s.Thresholds.Any() || s.Alarm != nil
}
