// Package health grades physical-layer facts into pass/warn/critical findings.
// It is backend-neutral and pure: facts (domain values) go in, []Finding comes
// out — no HTTP, no I/O. Grading uses ONLY thresholds and status/alarm flags the
// device itself supplies; it never invents a dBm/temperature/voltage limit. When
// the device exposes a reading but no basis to grade it, the finding is N/A, and
// N/A never counts as a failure.
package health

import (
	"fmt"
	"strings"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
)

// Severity is a grading outcome, ordered NA < PASS < WARN < CRITICAL by rank.
type Severity string

const (
	NA       Severity = "N/A"
	Pass     Severity = "PASS"
	Warn     Severity = "WARN"
	Critical Severity = "CRITICAL"
)

func (s Severity) rank() int {
	switch s {
	case Critical:
		return 3
	case Warn:
		return 2
	case Pass:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether s is at least as severe as o.
func (s Severity) AtLeast(o Severity) bool { return s.rank() >= o.rank() }

// worst returns the more severe of a and b.
func worst(a, b Severity) Severity {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// Finding is one graded observation about a subject (a port, an optic reading, a
// sensor, an interface).
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Subject  string   `json:"subject"`
	Message  string   `json:"message"`
}

// HealthReport is the rolled-up result rendered by `system health`.
type HealthReport struct {
	Model    string         `json:"model,omitempty"`
	Serial   string         `json:"serial,omitempty"`
	Hostname string         `json:"hostname,omitempty"`
	Version  string         `json:"version,omitempty"`
	Overall  Severity       `json:"overall"`
	Counts   map[string]int `json:"counts"`
	Findings []Finding      `json:"findings"`
}

// NewReport builds a report from device identity and the graded findings,
// computing the overall severity and per-severity counts.
func NewReport(st domain.DeviceStatus, findings []Finding) HealthReport {
	overall, counts := Summarize(findings)
	return HealthReport{
		Model:    st.Model,
		Serial:   st.Serial,
		Hostname: st.Hostname,
		Version:  st.Version,
		Overall:  overall,
		Counts:   counts,
		Findings: findings,
	}
}

// Summarize returns the overall severity (the worst finding; N/A when there is
// nothing gradeable) and a count per severity level.
func Summarize(findings []Finding) (Severity, map[string]int) {
	counts := map[string]int{
		string(Critical): 0, string(Warn): 0, string(Pass): 0, string(NA): 0,
	}
	overall := NA
	for _, f := range findings {
		counts[string(f.Severity)]++
		overall = worst(overall, f.Severity)
	}
	return overall, counts
}

// InterfaceFact is the interface state the interface check grades. AdminUp and
// LinkUp are optional: link-down is only graded a fault when admin-up is known,
// so a legitimately disabled or unplugged port never fails.
type InterfaceFact struct {
	Name      string
	AdminUp   *bool
	LinkUp    *bool
	Duplex    string
	SpeedMbps *float64
}

// GradeTransceiver grades each DDM reading of one optic. It emits a finding per
// reading the module reports (tx/rx power, temperature, voltage); readings the
// module does not report are skipped.
func GradeTransceiver(t domain.Transceiver) []Finding {
	var out []Finding
	subj := t.Interface
	add := func(name, unit string, r domain.DDMReading) {
		if f, ok := gradeReading(subj, name, unit, r); ok {
			out = append(out, f)
		}
	}
	add("tx-power", "dBm", t.TxPower)
	add("rx-power", "dBm", t.RxPower)
	add("temperature", "C", t.Temperature)
	add("voltage", "V", t.Voltage)
	return out
}

// gradeReading grades one DDM reading against device-supplied bases only. ok is
// false when the module reports no value at all (nothing to say).
func gradeReading(subject, name, unit string, r domain.DDMReading) (Finding, bool) {
	if !r.Present() {
		return Finding{}, false
	}
	val := *r.Value
	sev := NA
	reason := "no device-supplied threshold to grade against"

	switch {
	case r.Alarm != nil && *r.Alarm:
		sev, reason = Critical, "module alarm asserted"
	case r.HighAlarm != nil && val > *r.HighAlarm:
		sev, reason = Critical, fmt.Sprintf("above module high-alarm limit %s", num(*r.HighAlarm))
	case r.LowAlarm != nil && val < *r.LowAlarm:
		sev, reason = Critical, fmt.Sprintf("below module low-alarm limit %s", num(*r.LowAlarm))
	case r.Warning != nil && *r.Warning:
		sev, reason = Warn, "module warning asserted"
	case r.HighWarn != nil && val > *r.HighWarn:
		sev, reason = Warn, fmt.Sprintf("above module high-warning limit %s", num(*r.HighWarn))
	case r.LowWarn != nil && val < *r.LowWarn:
		sev, reason = Warn, fmt.Sprintf("below module low-warning limit %s", num(*r.LowWarn))
	case r.HasBasis():
		sev, reason = Pass, "within module-supplied limits"+boundNote(r)
	}

	return Finding{
		Severity: sev,
		Code:     "transceiver." + name,
		Subject:  subject,
		Message:  fmt.Sprintf("%s %s%s — %s", name, num(val), unit, reason),
	}, true
}

// GradeSensor grades one hardware sensor from its device-supplied alarm flag or
// status text. With neither, the reading is reported N/A.
func GradeSensor(s domain.Sensor) Finding {
	sev := NA
	reason := "no device-supplied status to grade against"
	switch {
	case s.Alarm != nil && *s.Alarm:
		sev, reason = Critical, "alarm asserted"
	case s.Status != "" && statusIsCritical(s.Status):
		sev, reason = Critical, "status "+s.Status
	case s.Status != "" && statusIsWarn(s.Status):
		sev, reason = Warn, "status "+s.Status
	case s.Status != "" && statusIsOK(s.Status):
		sev, reason = Pass, "status "+s.Status
	case s.Alarm != nil && !*s.Alarm:
		sev, reason = Pass, "no alarm asserted"
	}
	msg := reason
	if s.Value != nil {
		msg = fmt.Sprintf("%s%s — %s", num(*s.Value), unitSuffix(s.Unit), reason)
	}
	return Finding{
		Severity: sev,
		Code:     "sensor." + sensorCode(s.Type),
		Subject:  s.Name,
		Message:  msg,
	}
}

// GradeInterface grades one interface: admin-up + link-down is a warning (only
// when admin state is known — FortiOS leaves unused ports admin-enabled, so this
// is common and not critical), half-duplex is a warning, and speed below
// minSpeedMbps (0 = no check) is a warning. Otherwise a status finding is
// emitted: PASS when the link is up, N/A when link state cannot be judged.
func GradeInterface(f InterfaceFact, minSpeedMbps float64) []Finding {
	var out []Finding
	issue := false

	if f.AdminUp != nil && *f.AdminUp && f.LinkUp != nil && !*f.LinkUp {
		issue = true
		out = append(out, Finding{Warn, "interface.link", f.Name, "admin up but link down"})
	}
	// Only grade duplex on an UP link: a down link reports no real duplex (and
	// enrichment leaves it empty), so half-duplex must never fire on it.
	if f.LinkUp != nil && *f.LinkUp && strings.EqualFold(f.Duplex, "half") {
		issue = true
		out = append(out, Finding{Warn, "interface.duplex", f.Name, "half-duplex link"})
	}
	if minSpeedMbps > 0 && f.SpeedMbps != nil && *f.SpeedMbps < minSpeedMbps {
		issue = true
		out = append(out, Finding{
			Warn, "interface.speed", f.Name,
			fmt.Sprintf("link speed %s Mbps below expected %s Mbps", num(*f.SpeedMbps), num(minSpeedMbps)),
		})
	}
	if issue {
		return out
	}
	// No issue detected: summarize link state.
	switch {
	case f.LinkUp != nil && *f.LinkUp:
		out = append(out, Finding{Pass, "interface.link", f.Name, "link up"})
	default:
		out = append(out, Finding{NA, "interface.link", f.Name, "link down (admin state not reported; not graded)"})
	}
	return out
}

// --- status-text classification (device wording, not invented thresholds) ---

func statusIsCritical(s string) bool {
	return containsAny(s, "alarm", "critical", "fail", "fault", "shutdown", "error")
}
func statusIsWarn(s string) bool { return containsAny(s, "warn", "high", "low", "degrad") }

// statusIsOK matches device wording that positively asserts health. "present" is
// deliberately excluded — a module being present says nothing about its health.
func statusIsOK(s string) bool {
	return containsAny(s, "normal", "ok", "good", "nominal", "healthy")
}

// boundNote flags a one-sided PASS: a reading graded PASS against limits that
// bound only one direction is honestly reported so a value out of range on the
// UNBOUNDED side is not silently read as fully healthy. The missing bound is
// never invented.
func boundNote(r domain.DDMReading) string {
	hasHigh := r.HighAlarm != nil || r.HighWarn != nil
	hasLow := r.LowAlarm != nil || r.LowWarn != nil
	switch {
	case hasHigh && !hasLow:
		return " (high only)"
	case hasLow && !hasHigh:
		return " (low only)"
	default:
		return ""
	}
}

func containsAny(s string, subs ...string) bool {
	l := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func sensorCode(t string) string {
	if t == "" {
		return "reading"
	}
	return strings.ToLower(strings.ReplaceAll(t, " ", "-"))
}

func unitSuffix(u string) string {
	if u == "" {
		return ""
	}
	return " " + u
}

// num formats a float without a trailing ".0" for whole values.
func num(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}
