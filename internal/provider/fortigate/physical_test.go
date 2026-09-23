package fortigate

import (
	"context"
	"net/http"
	"testing"
)

// TestListTransceiversArrayShape covers the array-of-records shape with nested
// DDM readings that carry a value plus module-supplied thresholds and flags.
func TestListTransceiversArrayShape(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{
				"interface": "port49", "vendor": "FS", "vendor_part_number": "SFP-10GSR-85",
				"serial": "A1", "type": "SFP+",
				"optical_data": map[string]any{
					"tx_power":    map[string]any{"value": -2.5, "high_alarm": 3.0, "low_alarm": -8.0},
					"rx_power":    map[string]any{"value": -3.1, "high_alarm": 2.0, "low_alarm": -9.0, "alarm": false},
					"temperature": map[string]any{"value": 41.0},
					"voltage":     map[string]any{"value": 3.31, "high_alarm": 3.6, "low_alarm": 3.0},
				},
			},
		}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "GET" || got.path != "/api/v2/monitor/system/interface/transceivers" {
		t.Errorf("transceivers hit %s %s", got.method, got.path)
	}
	if len(optics) != 1 {
		t.Fatalf("want 1 optic, got %d", len(optics))
	}
	o := optics[0]
	if o.Interface != "port49" || o.Part != "SFP-10GSR-85" || o.Vendor != "FS" {
		t.Errorf("identity fields wrong: %+v", o)
	}
	if o.TxPower.Value == nil || *o.TxPower.Value != -2.5 {
		t.Errorf("tx value = %v, want -2.5", o.TxPower.Value)
	}
	if o.TxPower.HighAlarm == nil || *o.TxPower.HighAlarm != 3.0 {
		t.Errorf("tx high_alarm = %v, want 3.0", o.TxPower.HighAlarm)
	}
	// temperature had a value but NO thresholds → no basis to grade.
	if !o.Temperature.Present() || o.Temperature.HasBasis() {
		t.Errorf("temperature should be present with no basis: %+v", o.Temperature)
	}
	// Raw carries the original device record verbatim for json/yaml passthrough.
	if o.Raw["interface"] != "port49" || o.Raw["vendor_part_number"] != "SFP-10GSR-85" {
		t.Errorf("raw record not passed through: %v", o.Raw)
	}
}

// TestListTransceiversKeyedShape covers the object-keyed-by-interface shape with
// flat reading values (no nested container).
func TestListTransceiversKeyedShape(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope(map[string]any{
			"port1": map[string]any{"vendor": "ACME", "tx_power": -1.0, "rx_power": -2.0},
			"port2": map[string]any{"vendor": "ACME", "tx_power": -1.5},
		}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(optics) != 2 {
		t.Fatalf("want 2 optics, got %d", len(optics))
	}
	byName := map[string]bool{}
	for _, o := range optics {
		byName[o.Interface] = true
		if o.TxPower.Value == nil {
			t.Errorf("%s tx value missing", o.Interface)
		}
	}
	if !byName["port1"] || !byName["port2"] {
		t.Errorf("interface key not injected: %v", byName)
	}
}

// TestListTransceiversNestedContainer covers a payload that nests the collection
// under a container key (results already stripped by the envelope decoder).
func TestListTransceiversNestedContainer(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope(map[string]any{
			"transceivers": []map[string]any{
				{"interface": "port1", "vendor": "ACME", "tx_power": -1.0},
				{"interface": "port2", "vendor": "ACME", "tx_power": -1.5},
			},
		}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(optics) != 2 || optics[0].Interface == "" {
		t.Fatalf("nested container not descended: %+v", optics)
	}
}

// TestListTransceiversSingleObject covers a single flat object treated as one
// record (scalar values, so it is neither a keyed collection nor a container).
func TestListTransceiversSingleObject(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope(map[string]any{
			"interface": "port49", "vendor": "FS", "tx_power": -2.0,
		}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(optics) != 1 {
		t.Fatalf("single object should be one record, got %d", len(optics))
	}
	if optics[0].Interface != "port49" || optics[0].TxPower.Value == nil {
		t.Errorf("single-object record decoded wrong: %+v", optics[0])
	}
}

// TestListTransceiversFlatSiblingThresholds covers the extractReading path where
// a reading is a bare scalar and its thresholds are flat "<base>_..." siblings.
func TestListTransceiversFlatSiblingThresholds(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope([]map[string]any{
			{
				"interface":           "port1",
				"tx_power":            -2.0,
				"tx_power_high_alarm": 3.0,
				"tx_power_low_alarm":  -8.0,
				"rx_power":            -3.0,
				"rx_power_high_warn":  1.0,
				"rx_power_low_warn":   -7.0,
			},
		}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(optics) != 1 {
		t.Fatalf("want 1 optic, got %d", len(optics))
	}
	o := optics[0]
	if o.TxPower.HighAlarm == nil || *o.TxPower.HighAlarm != 3.0 {
		t.Errorf("flat tx high_alarm not picked up: %+v", o.TxPower)
	}
	if o.TxPower.LowAlarm == nil || *o.TxPower.LowAlarm != -8.0 {
		t.Errorf("flat tx low_alarm not picked up: %+v", o.TxPower)
	}
	if o.RxPower.HighWarn == nil || *o.RxPower.HighWarn != 1.0 {
		t.Errorf("flat rx high_warn not picked up: %+v", o.RxPower)
	}
}

// TestListInterfacesNonNumericSpeed asserts a non-numeric speed string is
// surfaced as-is (the ParseFloat-failure path is handled downstream in the CLI).
func TestListInterfacesNonNumericSpeed(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope(map[string]any{
			"port1": map[string]any{
				"name": "port1", "link": true, "type": "physical",
				"speed": "auto", "duplex": "half",
			},
		}))
	})
	ifaces, err := p.ListInterfaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("want 1 interface, got %d", len(ifaces))
	}
	if ifaces[0].Speed != "auto" || ifaces[0].Duplex != "half" {
		t.Errorf("speed/duplex = %q/%q, want auto/half", ifaces[0].Speed, ifaces[0].Duplex)
	}
}

// TestNumericStatusZeroIsNoBasis asserts a numeric per-reading status of 0 is NOT
// decoded as an "ok" basis (an unverified convention must not grade PASS), while
// 2 asserts an alarm and 1 a warning.
func TestNumericStatusZeroIsNoBasis(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope([]map[string]any{
			{
				"interface": "port1",
				"optical_data": map[string]any{
					"tx_power": map[string]any{"value": -2.0, "status": 0.0}, // 0 => no basis
					"rx_power": map[string]any{"value": -3.0, "status": 2.0}, // 2 => alarm
					"voltage":  map[string]any{"value": 3.3, "status": 1.0},  // 1 => warning
				},
			},
		}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	o := optics[0]
	if o.TxPower.HasBasis() {
		t.Errorf("numeric status 0 must not create a grading basis: %+v", o.TxPower)
	}
	if o.RxPower.Alarm == nil || !*o.RxPower.Alarm {
		t.Errorf("numeric status 2 should assert alarm: %+v", o.RxPower)
	}
	if o.Voltage.Warning == nil || !*o.Voltage.Warning {
		t.Errorf("numeric status 1 should assert warning: %+v", o.Voltage)
	}
}

// TestListTransceiversEmptyVM asserts an empty result (a VM) yields no optics and
// no error — the section is N/A, not a failure.
func TestListTransceiversEmptyVM(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope([]any{}))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatalf("empty result must not error: %v", err)
	}
	if len(optics) != 0 {
		t.Errorf("want 0 optics, got %d", len(optics))
	}
}

// TestListTransceivers404 asserts a 404/unsupported endpoint degrades to an empty
// slice + nil error (N/A), never a panic or failure.
func TestListTransceivers404(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"error","http_status":404,"error":-3}`))
	})
	optics, err := p.ListTransceivers(context.Background())
	if err != nil {
		t.Fatalf("404 must be swallowed as N/A: %v", err)
	}
	if len(optics) != 0 {
		t.Errorf("want 0 optics on 404, got %d", len(optics))
	}
}

// TestListSensorsArrayShape covers sensor decode plus raw passthrough.
func TestListSensors(t *testing.T) {
	var got capture
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		w.Write(envelope([]map[string]any{
			{"name": "PS1 Fan 1", "type": "fan", "value": 8000.0, "unit": "RPM", "status": "normal"},
			{"name": "CPU Temp", "type": "temperature", "value": 92.0, "unit": "C", "alarm": true},
		}))
	})
	sensors, err := p.ListSensors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/v2/monitor/system/sensor-info" {
		t.Errorf("sensor-info hit %s", got.path)
	}
	if len(sensors) != 2 {
		t.Fatalf("want 2 sensors, got %d", len(sensors))
	}
	if sensors[0].Name != "PS1 Fan 1" || sensors[0].Status != "normal" || sensors[0].Value == nil {
		t.Errorf("sensor 0 decoded wrong: %+v", sensors[0])
	}
	if sensors[1].Alarm == nil || !*sensors[1].Alarm {
		t.Errorf("sensor 1 alarm flag = %v, want true", sensors[1].Alarm)
	}
	if sensors[0].Raw["unit"] != "RPM" {
		t.Errorf("raw record not passed through: %v", sensors[0].Raw)
	}
}

// TestListSensors404 asserts the absent-hardware contract for sensors.
func TestListSensors404(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"error","http_status":404}`))
	})
	sensors, err := p.ListSensors(context.Background())
	if err != nil {
		t.Fatalf("404 must be swallowed as N/A: %v", err)
	}
	if len(sensors) != 0 {
		t.Errorf("want 0 sensors on 404, got %d", len(sensors))
	}
}

// TestListInterfacesSpeedDuplex asserts the enriched decoder surfaces speed and
// duplex tolerantly (numeric duplex 1=full).
func TestListInterfacesSpeedDuplex(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(envelope(map[string]any{
			"port1": map[string]any{
				"name": "port1", "link": true, "type": "physical",
				"speed": 1000.0, "duplex": 1.0,
			},
		}))
	})
	ifaces, err := p.ListInterfaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("want 1 interface, got %d", len(ifaces))
	}
	if ifaces[0].Speed != "1000" || ifaces[0].Duplex != "full" {
		t.Errorf("speed/duplex = %q/%q, want 1000/full", ifaces[0].Speed, ifaces[0].Duplex)
	}
}
