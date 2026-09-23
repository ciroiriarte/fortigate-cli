package health

import (
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
)

func fptr(f float64) *float64 { return &f }
func bptr(b bool) *bool       { return &b }

// findingFor returns the finding whose Code ends with suffix.
func findingFor(t *testing.T, fs []Finding, suffix string) Finding {
	t.Helper()
	for _, f := range fs {
		if len(f.Code) >= len(suffix) && f.Code[len(f.Code)-len(suffix):] == suffix {
			return f
		}
	}
	t.Fatalf("no finding with code suffix %q in %v", suffix, fs)
	return Finding{}
}

// TestTransceiverGradingBoundaries exercises PASS/WARN/CRITICAL from
// device-supplied limits and flags.
func TestTransceiverGradingBoundaries(t *testing.T) {
	optic := domain.Transceiver{
		Interface: "port49",
		// within limits → PASS
		TxPower: domain.DDMReading{Value: fptr(-2.0), HighAlarm: fptr(3), LowAlarm: fptr(-8)},
		// below low-alarm limit → CRITICAL
		RxPower: domain.DDMReading{Value: fptr(-12.0), HighAlarm: fptr(2), LowAlarm: fptr(-9)},
		// above high-warning limit → WARN
		Temperature: domain.DDMReading{Value: fptr(70.0), HighWarn: fptr(65), HighAlarm: fptr(80)},
		// module alarm flag asserted → CRITICAL
		Voltage: domain.DDMReading{Value: fptr(3.3), Alarm: bptr(true)},
	}
	fs := GradeTransceiver(optic)
	if got := findingFor(t, fs, "tx-power").Severity; got != Pass {
		t.Errorf("tx-power = %s, want PASS", got)
	}
	if got := findingFor(t, fs, "rx-power").Severity; got != Critical {
		t.Errorf("rx-power = %s, want CRITICAL", got)
	}
	if got := findingFor(t, fs, "temperature").Severity; got != Warn {
		t.Errorf("temperature = %s, want WARN", got)
	}
	if got := findingFor(t, fs, "voltage").Severity; got != Critical {
		t.Errorf("voltage = %s, want CRITICAL", got)
	}
}

// TestHardRuleNoThresholdStaysNA is the core invariant: a reading present with NO
// device-supplied threshold or flag must be N/A — never PASS (no invented "good")
// and never FAIL (no invented limit).
func TestHardRuleNoThresholdStaysNA(t *testing.T) {
	optic := domain.Transceiver{
		Interface:   "port1",
		TxPower:     domain.DDMReading{Value: fptr(-40.0)}, // extreme value, but no limits supplied
		Temperature: domain.DDMReading{Value: fptr(999.0)}, // absurd, still no basis
	}
	fs := GradeTransceiver(optic)
	for _, f := range fs {
		if f.Severity != NA {
			t.Errorf("%s graded %s, want N/A (no device threshold => no PASS/FAIL): %s", f.Code, f.Severity, f.Message)
		}
	}
	// A reading the module does not report at all yields no finding.
	if len(fs) != 2 {
		t.Errorf("want 2 findings (only the present readings), got %d", len(fs))
	}
}

// TestOneSidedPassIsHonest asserts a PASS graded against a one-sided limit set
// says so in the message, rather than implying a full in-range verdict.
func TestOneSidedPassIsHonest(t *testing.T) {
	// Only a high-alarm limit supplied; value is fine on the bounded side.
	f, ok := gradeReading("port1", "rx-power", "dBm", domain.DDMReading{Value: fptr(-3.0), HighAlarm: fptr(2.0)})
	if !ok || f.Severity != Pass {
		t.Fatalf("want PASS, got %s ok=%v", f.Severity, ok)
	}
	if !strings.Contains(f.Message, "(high only)") {
		t.Errorf("one-sided PASS message should flag the missing bound, got %q", f.Message)
	}
	// Two-sided limits get no bound note.
	f2, _ := gradeReading("port1", "tx-power", "dBm", domain.DDMReading{Value: fptr(-2.0), HighAlarm: fptr(3), LowAlarm: fptr(-8)})
	if strings.Contains(f2.Message, "only)") {
		t.Errorf("two-sided PASS should not carry a one-sided note, got %q", f2.Message)
	}
}

// TestPresentStatusNotOK asserts a bare "present" sensor status does not grade
// PASS (presence != health).
func TestPresentStatusNotOK(t *testing.T) {
	if got := GradeSensor(domain.Sensor{Name: "SFP", Status: "present"}).Severity; got == Pass {
		t.Errorf("status \"present\" graded PASS, want non-PASS (presence is not health)")
	}
}

// TestSensorGrading covers alarm flag, status text, and no-basis N/A.
func TestSensorGrading(t *testing.T) {
	cases := []struct {
		name string
		in   domain.Sensor
		want Severity
	}{
		{"alarm flag", domain.Sensor{Name: "CPU", Alarm: bptr(true)}, Critical},
		{"status normal", domain.Sensor{Name: "Fan1", Status: "normal"}, Pass},
		{"status warning", domain.Sensor{Name: "PS1", Status: "high warning"}, Warn},
		{"status fail", domain.Sensor{Name: "PS2", Status: "fault"}, Critical},
		{"no basis", domain.Sensor{Name: "Volt", Value: fptr(12.1)}, NA},
	}
	for _, c := range cases {
		if got := GradeSensor(c.in).Severity; got != c.want {
			t.Errorf("%s: GradeSensor = %s, want %s", c.name, got, c.want)
		}
	}
}

// TestSensorNoInventedThreshold: a bare numeric value with no status/alarm must
// stay N/A even though it looks extreme.
func TestSensorNoInventedThreshold(t *testing.T) {
	if got := GradeSensor(domain.Sensor{Name: "CPU Temp", Type: "temperature", Value: fptr(150)}).Severity; got != NA {
		t.Errorf("bare value graded %s, want N/A", got)
	}
}

// TestInterfaceGrading covers duplex, min-speed, link-up PASS, and the honest
// N/A for a down link with unknown admin state.
func TestInterfaceGrading(t *testing.T) {
	// half-duplex → WARN
	if got := findingFor(t, GradeInterface(InterfaceFact{Name: "p1", LinkUp: bptr(true), Duplex: "half"}, 0), "duplex").Severity; got != Warn {
		t.Errorf("half-duplex = %s, want WARN", got)
	}
	// speed below --min-speed → WARN
	fs := GradeInterface(InterfaceFact{Name: "p2", LinkUp: bptr(true), SpeedMbps: fptr(100)}, 1000)
	if got := findingFor(t, fs, "speed").Severity; got != Warn {
		t.Errorf("slow link = %s, want WARN", got)
	}
	// admin up + link down → WARN (FortiOS leaves unused ports admin-up; not critical)
	fs = GradeInterface(InterfaceFact{Name: "p3", AdminUp: bptr(true), LinkUp: bptr(false)}, 0)
	if got := findingFor(t, fs, "link").Severity; got != Warn {
		t.Errorf("admin-up link-down = %s, want WARN", got)
	}
	// down link must never produce a duplex finding, even if duplex reads "half"
	fs = GradeInterface(InterfaceFact{Name: "p3b", AdminUp: bptr(true), LinkUp: bptr(false), Duplex: "half"}, 0)
	for _, f := range fs {
		if f.Code == "interface.duplex" {
			t.Errorf("down link produced a duplex finding: %+v", f)
		}
	}
	// link up, nothing wrong → PASS
	fs = GradeInterface(InterfaceFact{Name: "p4", LinkUp: bptr(true), SpeedMbps: fptr(1000)}, 1000)
	if got := findingFor(t, fs, "link").Severity; got != Pass {
		t.Errorf("clean link = %s, want PASS", got)
	}
	// link down, admin unknown → N/A (not a failure)
	fs = GradeInterface(InterfaceFact{Name: "p5", LinkUp: bptr(false)}, 0)
	if got := findingFor(t, fs, "link").Severity; got != NA {
		t.Errorf("down link w/ unknown admin = %s, want N/A", got)
	}
}

// TestSummarize covers overall roll-up and N/A neutrality.
func TestSummarize(t *testing.T) {
	// All N/A → overall N/A, no failure.
	overall, counts := Summarize([]Finding{{Severity: NA}, {Severity: NA}})
	if overall != NA || counts[string(NA)] != 2 {
		t.Errorf("all-NA summarize = %s %v", overall, counts)
	}
	// Worst wins; N/A does not mask a real WARN.
	overall, _ = Summarize([]Finding{{Severity: Pass}, {Severity: NA}, {Severity: Warn}})
	if overall != Warn {
		t.Errorf("mixed summarize = %s, want WARN", overall)
	}
	overall, _ = Summarize([]Finding{{Severity: Warn}, {Severity: Critical}, {Severity: Pass}})
	if overall != Critical {
		t.Errorf("summarize = %s, want CRITICAL", overall)
	}
}
