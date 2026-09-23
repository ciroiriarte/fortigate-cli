package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/health"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// healthFake is a provider stub for the physical-health commands. Unset slices
// model an absent subsystem (VM), exercising the N/A degrade path.
type healthFake struct {
	provider.Provider
	status     domain.DeviceStatus
	ifaces     []domain.Interface
	optics     []domain.Transceiver
	sensors    []domain.Sensor
	cmdbIfaces []provider.Object // cmdb system/interface (admin status source)
	cmdbErr    error             // returned by CmdbList when set
}

func (h *healthFake) DeviceStatus(context.Context) (domain.DeviceStatus, error) { return h.status, nil }
func (h *healthFake) ListInterfaces(context.Context) ([]domain.Interface, error) {
	return h.ifaces, nil
}
func (h *healthFake) ListTransceivers(context.Context) ([]domain.Transceiver, error) {
	return h.optics, nil
}
func (h *healthFake) ListSensors(context.Context) ([]domain.Sensor, error) { return h.sensors, nil }
func (h *healthFake) CmdbList(_ context.Context, _ string) ([]provider.Object, error) {
	return h.cmdbIfaces, h.cmdbErr
}

func fptr(f float64) *float64 { return &f }
func bptr(b bool) *bool       { return &b }

// TestPhysicalHealthCommandTree asserts the three new commands resolve under
// `system`.
func TestPhysicalHealthCommandTree(t *testing.T) {
	root := newSystemCmd(&app{})
	for _, path := range [][]string{
		{"transceiver", "list"},
		{"sensor", "list"},
		{"health"},
	} {
		if findCmd(root, path...) == nil {
			t.Errorf("system %s did not resolve", strings.Join(path, " "))
		}
	}
	// health carries the CI exit-code contract in its long help.
	h := findCmd(root, "health")
	for _, sub := range []string{"--fail-on", "Exit codes", "device-supplied", "N/A"} {
		if !strings.Contains(h.Long, sub) {
			t.Errorf("health long help missing %q", sub)
		}
	}
}

// TestHealthExitMapping covers the --fail-on → exitCodeError mapping for clean,
// warn, and critical verdicts.
func TestHealthExitMapping(t *testing.T) {
	cases := []struct {
		overall health.Severity
		failOn  string
		want    int // expected process exit code
	}{
		{health.Pass, "", 0},
		{health.Warn, "", 0},     // default never fails
		{health.Critical, "", 0}, // default never fails
		{health.Warn, "warn", 1}, // warn threshold, warn verdict
		{health.Critical, "warn", 2},
		{health.Pass, "warn", 0},
		{health.Warn, "error", 0},     // error threshold ignores warn
		{health.Critical, "error", 2}, // error threshold fires on critical
	}
	for _, c := range cases {
		err := healthExit(c.overall, c.failOn)
		if got := ExitCodeFor(err); got != c.want {
			t.Errorf("healthExit(%s, %q) exit = %d, want %d", c.overall, c.failOn, got, c.want)
		}
	}
}

// TestHealthReportVMDegradesToNA asserts a VM (no optics, no sensors) still runs
// interface checks and reports both hardware sections N/A — never a failure.
func TestHealthReportVMDegradesToNA(t *testing.T) {
	f := &healthFake{
		status: domain.DeviceStatus{Model: "FGVM64", Serial: "FGVMTEST"},
		ifaces: []domain.Interface{{Name: "port1", Status: "up", Speed: "1000", Duplex: "full"}},
		// optics + sensors nil → absent
	}
	report, err := buildHealthReport(context.Background(), f, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Overall == health.Critical || report.Overall == health.Warn {
		t.Errorf("VM overall = %s, want a non-failing verdict", report.Overall)
	}
	if healthExit(report.Overall, "warn") != nil {
		t.Error("a healthy VM must not fail even with --fail-on warn")
	}
	var haveTxNA, haveSensorNA bool
	for _, fd := range report.Findings {
		if fd.Code == "transceiver.absent" && fd.Severity == health.NA {
			haveTxNA = true
		}
		if fd.Code == "sensor.absent" && fd.Severity == health.NA {
			haveSensorNA = true
		}
	}
	if !haveTxNA || !haveSensorNA {
		t.Errorf("VM should report optics+sensors N/A: %+v", report.Findings)
	}
}

// TestHealthReportCriticalSensor asserts a device alarm rolls up to CRITICAL and
// fires the exit code.
func TestHealthReportCriticalSensor(t *testing.T) {
	f := &healthFake{
		status:  domain.DeviceStatus{Model: "FG100F"},
		ifaces:  []domain.Interface{{Name: "port1", Status: "up"}},
		sensors: []domain.Sensor{{Name: "PSU1", Type: "power", Alarm: bptr(true)}},
	}
	report, err := buildHealthReport(context.Background(), f, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Overall != health.Critical {
		t.Fatalf("overall = %s, want CRITICAL", report.Overall)
	}
	if code := ExitCodeFor(healthExit(report.Overall, "error")); code != 2 {
		t.Errorf("--fail-on error exit = %d, want 2", code)
	}
}

// TestHealthAdminUpLinkDown asserts the flagship check fires: a cmdb admin-up
// interface whose monitor link is down grades CRITICAL, while an admin-down
// interface with link down does not fail (stays N/A). Admin status is sourced
// from the cmdb system/interface object.
func TestHealthAdminUpLinkDown(t *testing.T) {
	f := &healthFake{
		status: domain.DeviceStatus{Model: "FG100F"},
		ifaces: []domain.Interface{
			{Name: "port1", Status: "down"}, // link down
			{Name: "port2", Status: "down"}, // link down
		},
		cmdbIfaces: []provider.Object{
			{"name": "port1", "status": "up"},   // admin up  => admin-up + link-down => CRITICAL
			{"name": "port2", "status": "down"}, // admin down => not a fault
		},
	}
	report, err := buildHealthReport(context.Background(), f, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]health.Severity{}
	for _, fd := range report.Findings {
		if fd.Code == "interface.link" {
			got[fd.Subject] = fd.Severity
		}
	}
	if got["port1"] != health.Critical {
		t.Errorf("port1 (admin-up, link-down) = %s, want CRITICAL", got["port1"])
	}
	if got["port2"] == health.Critical || got["port2"] == health.Warn {
		t.Errorf("port2 (admin-down, link-down) = %s, want a non-failing verdict", got["port2"])
	}
	if report.Overall != health.Critical {
		t.Errorf("overall = %s, want CRITICAL", report.Overall)
	}
}

// TestHealthCmdbReadDegrades asserts a cmdb read failure does not fail the whole
// health run — AdminUp is simply left unset (link-down stays N/A).
func TestHealthCmdbReadDegrades(t *testing.T) {
	f := &healthFake{
		ifaces:  []domain.Interface{{Name: "port1", Status: "down"}},
		cmdbErr: errors.New("cmdb unavailable"),
	}
	report, err := buildHealthReport(context.Background(), f, "", 0)
	if err != nil {
		t.Fatalf("cmdb read failure must not fail the health run: %v", err)
	}
	if report.Overall == health.Critical || report.Overall == health.Warn {
		t.Errorf("overall = %s, want a non-failing verdict when admin status is unknown", report.Overall)
	}
}

// TestInterfaceFactParseFloatFailure asserts a non-numeric speed leaves SpeedMbps
// unset (no ParseFloat panic, no bogus 0), so the min-speed check is skipped.
func TestInterfaceFactParseFloatFailure(t *testing.T) {
	f := interfaceFact(domain.Interface{Name: "port1", Status: "up", Speed: "auto", Duplex: "full"}, nil)
	if f.SpeedMbps != nil {
		t.Errorf("non-numeric speed should leave SpeedMbps nil, got %v", *f.SpeedMbps)
	}
	// A numeric speed still parses.
	f2 := interfaceFact(domain.Interface{Name: "port2", Status: "up", Speed: "1000"}, nil)
	if f2.SpeedMbps == nil || *f2.SpeedMbps != 1000 {
		t.Errorf("numeric speed should parse to 1000, got %v", f2.SpeedMbps)
	}
}

// TestTransceiverRawFaithfulness asserts the transceiver list's json/yaml Raw is
// the device's ORIGINAL record, not the typed projection: an unmodeled field on
// the raw record must survive into the json output.
func TestTransceiverRawFaithfulness(t *testing.T) {
	optics := []domain.Transceiver{{
		Interface: "port49",
		TxPower:   domain.DDMReading{Value: fptr(-2.5)},
		Raw: map[string]any{
			"interface":          "port49",
			"vendor_part_number": "SFP-10GSR-85",
			"unmodeled_field":    "verbatim-value",
		},
	}}
	raw := rawRecords(transceiverRaws(optics))

	var buf bytes.Buffer
	if err := output.Render(&buf, output.Tabular{Raw: raw}, output.Options{Format: output.JSON}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"unmodeled_field", "verbatim-value", "vendor_part_number"} {
		if !strings.Contains(out, want) {
			t.Errorf("json output dropped device record field %q; got:\n%s", want, out)
		}
	}
}

// TestSensorRawFaithfulness mirrors the above for sensors.
func TestSensorRawFaithfulness(t *testing.T) {
	sensors := []domain.Sensor{{
		Name: "Fan1",
		Raw:  map[string]any{"name": "Fan1", "vendor_specific": "keepme", "rpm": 9000},
	}}
	raw := rawRecords(sensorRaws(sensors))
	var buf bytes.Buffer
	if err := output.Render(&buf, output.Tabular{Raw: raw}, output.Options{Format: output.JSON}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "vendor_specific") || !strings.Contains(buf.String(), "keepme") {
		t.Errorf("sensor json dropped original record: %s", buf.String())
	}
}
