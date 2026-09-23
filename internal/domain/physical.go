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

// Sensor is one hardware sensor (PSU, fan, temperature, voltage) from
// monitor/system/sensor-info. Value/Status/Alarm are optional; grading uses only
// the device-supplied status text or alarm flag. Raw holds the original record.
type Sensor struct {
	Name   string         `json:"name"`
	Type   string         `json:"type,omitempty"`
	Value  *float64       `json:"value,omitempty"`
	Unit   string         `json:"unit,omitempty"`
	Status string         `json:"status,omitempty"`
	Alarm  *bool          `json:"alarm,omitempty"`
	Raw    map[string]any `json:"-"`
}
