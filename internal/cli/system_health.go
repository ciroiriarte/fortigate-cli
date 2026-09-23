package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/health"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// newTransceiverCmd exposes the optical DDM inventory (monitor surface). The
// endpoint schema is unverified, so json/yaml render the device's ORIGINAL
// records; the table columns and HEALTH grade are derived tolerantly.
func newTransceiverCmd(a *app) *cobra.Command {
	tc := &cobra.Command{
		Use:     "transceiver",
		Aliases: []string{"transceivers", "sfp", "optic"},
		Short:   "Inspect pluggable optics (SFP/QSFP DDM, monitor surface)",
	}
	tc.AddCommand(transceiverListCmd(a))
	return tc
}

func transceiverListCmd(a *app) *cobra.Command {
	var ifaceFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transceiver DDM inventory (tx/rx power, temperature, voltage)",
		Long: "List pluggable-optic diagnostics from monitor/system/interface/transceivers.\n\n" +
			"HEALTH is graded only from module-supplied alarm/warning flags and limits;\n" +
			"a reading with no device-supplied threshold is reported N/A, never PASS/FAIL.\n" +
			"PASS means the reading is within the limits the module supplied, which may be\n" +
			"one-sided (a reading marked \"(high only)\"/\"(low only)\" is unbounded on the\n" +
			"other side).\n" +
			"json/yaml output shows the device's original records verbatim (the endpoint\n" +
			"schema is unverified, so the raw passthrough is the source of truth).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			optics, err := p.ListTransceivers(cmd.Context())
			if err != nil {
				return err
			}
			optics = filterTransceivers(optics, ifaceFilter)
			t := output.Tabular{
				Columns: []string{"PORT", "VENDOR", "PART", "TX(dBm)", "RX(dBm)", "TEMP", "VOLTAGE", "HEALTH"},
				Raw:     rawRecords(transceiverRaws(optics)),
			}
			for _, o := range optics {
				overall, _ := health.Summarize(health.GradeTransceiver(o))
				t.Rows = append(t.Rows, []string{
					o.Interface, o.Vendor, o.Part,
					ddm(o.TxPower), ddm(o.RxPower), ddm(o.Temperature), ddm(o.Voltage),
					string(overall),
				})
			}
			return a.render(t)
		},
	}
	cmd.Flags().StringVarP(&ifaceFilter, "interface", "i", "", "only these interfaces (comma-separated)")
	return cmd
}

// newSensorCmd exposes hardware sensors (monitor surface), with the same raw
// passthrough contract as transceivers.
func newSensorCmd(a *app) *cobra.Command {
	sc := &cobra.Command{
		Use:     "sensor",
		Aliases: []string{"sensors"},
		Short:   "Inspect hardware sensors (PSU/fan/temperature, monitor surface)",
	}
	sc.AddCommand(sensorListCmd(a))
	return sc
}

func sensorListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List hardware sensor readings (name, type, value, status)",
		Long: "List hardware sensors from monitor/system/sensor-info.\n\n" +
			"STATUS is the health grade (PASS/WARN/CRITICAL/N/A) derived from the device's\n" +
			"own alarm flag and thresholds — never an invented limit; a sensor with no\n" +
			"threshold or alarm is N/A. json/yaml output shows the device's original\n" +
			"records verbatim.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			sensors, err := p.ListSensors(cmd.Context())
			if err != nil {
				return err
			}
			t := output.Tabular{
				Columns: []string{"NAME", "TYPE", "VALUE", "STATUS"},
				Raw:     rawRecords(sensorRaws(sensors)),
			}
			for _, s := range sensors {
				t.Rows = append(t.Rows, []string{s.Name, s.Type, sensorValue(s), sensorStatus(s)})
			}
			return a.render(t)
		},
	}
}

// newHealthCmd rolls up interface, transceiver, and sensor checks into a graded
// report with a CI-friendly exit code.
func newHealthCmd(a *app) *cobra.Command {
	var failOn, ifaceFilter string
	var minSpeed float64
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Grade physical health: interfaces, optics, sensors (monitor surface)",
		Long: "Roll up physical-layer checks (interface link/duplex/speed, transceiver DDM,\n" +
			"hardware sensors) into a single PASS/WARN/CRITICAL verdict.\n\n" +
			"Grading is conservative: it uses ONLY device-supplied thresholds and\n" +
			"alarm/status flags — it never invents dBm/temperature/voltage limits. A\n" +
			"reading with no device-supplied basis, or a subsystem absent on this\n" +
			"platform (e.g. optics/sensors on a VM), is reported N/A and never fails.\n" +
			"PASS means \"within the limits the module supplied,\" which may be one-sided.\n\n" +
			"Exit codes: exits 0 by default. With --fail-on warn it exits non-zero on any\n" +
			"WARN or worse (1 for WARN, 2 for CRITICAL); with --fail-on error it exits\n" +
			"non-zero (2) only on CRITICAL. Transport/lookup errors keep their own\n" +
			"non-zero exit regardless of --fail-on.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch failOn {
			case "", "warn", "error":
			default:
				return fmt.Errorf("invalid --fail-on %q (want warn|error)", failOn)
			}
			p, err := a.Provider()
			if err != nil {
				return err
			}
			report, err := buildHealthReport(cmd.Context(), p, ifaceFilter, minSpeed)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(),
				"[fgt] health: %s %s (%s) — overall %s; CRITICAL=%d WARN=%d PASS=%d N/A=%d\n",
				emptyDash(report.Model), emptyDash(report.Serial), emptyDash(report.Hostname),
				report.Overall,
				report.Counts[string(health.Critical)], report.Counts[string(health.Warn)],
				report.Counts[string(health.Pass)], report.Counts[string(health.NA)])

			t := output.Tabular{Columns: []string{"SEVERITY", "SUBJECT", "CODE", "MESSAGE"}, Raw: report}
			for _, f := range report.Findings {
				t.Rows = append(t.Rows, []string{string(f.Severity), f.Subject, f.Code, f.Message})
			}
			if err := a.render(t); err != nil {
				return err
			}
			return healthExit(report.Overall, failOn)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "exit non-zero when findings reach this level: warn|error")
	cmd.Flags().StringVarP(&ifaceFilter, "interface", "i", "", "limit interface/transceiver checks to these interfaces (comma-separated)")
	cmd.Flags().Float64Var(&minSpeed, "min-speed", 0, "warn when an up link negotiates below this speed in Mbps (0 = no check)")
	return cmd
}

// buildHealthReport gathers facts from the provider and grades them. Interface
// checks always run; transceiver/sensor sections degrade to N/A when the
// platform does not expose them (empty result), never to a failure.
func buildHealthReport(ctx context.Context, p provider.Provider, ifaceFilter string, minSpeed float64) (health.HealthReport, error) {
	st, err := p.DeviceStatus(ctx)
	if err != nil {
		return health.HealthReport{}, err
	}

	ifaces, err := p.ListInterfaces(ctx)
	if err != nil {
		return health.HealthReport{}, err
	}
	optics, err := p.ListTransceivers(ctx)
	if err != nil {
		return health.HealthReport{}, err
	}
	sensors, err := p.ListSensors(ctx)
	if err != nil {
		return health.HealthReport{}, err
	}

	// The monitor interface object does not carry admin (configured) status, so
	// read it from the cmdb config object to power the admin-up + link-down check.
	// A cmdb read failure degrades gracefully (nil map => AdminUp left unset).
	adminUp := interfaceAdminStatus(ctx, p)

	want := parseFilter(ifaceFilter)
	var findings []health.Finding

	for _, i := range ifaces {
		if !want(i.Name) {
			continue
		}
		findings = append(findings, health.GradeInterface(interfaceFact(i, adminUp), minSpeed)...)
	}

	optics = filterTransceivers(optics, ifaceFilter)
	if len(optics) == 0 {
		findings = append(findings, health.Finding{
			Severity: health.NA, Code: "transceiver.absent", Subject: "transceivers",
			Message: "no transceiver data (unavailable on this platform or no optics present)",
		})
	}
	for _, o := range optics {
		findings = append(findings, health.GradeTransceiver(o)...)
	}

	if len(sensors) == 0 {
		findings = append(findings, health.Finding{
			Severity: health.NA, Code: "sensor.absent", Subject: "sensors",
			Message: "no sensor data (unavailable on this platform)",
		})
	}
	for _, s := range sensors {
		findings = append(findings, health.GradeSensor(s))
	}

	return health.NewReport(st, findings), nil
}

// healthExit maps the overall verdict to the requested --fail-on exit code:
// warn => WARN exits 1, CRITICAL exits 2; error => CRITICAL exits 2. The signal
// rides on exitCodeError so Execute treats it as a deliberate code, not a crash.
func healthExit(overall health.Severity, failOn string) error {
	switch failOn {
	case "warn":
		if overall == health.Critical {
			return exitCodeError{code: 2}
		}
		if overall == health.Warn {
			return exitCodeError{code: 1}
		}
	case "error":
		if overall == health.Critical {
			return exitCodeError{code: 2}
		}
	}
	return nil
}

// --- helpers ------------------------------------------------------------

// interfaceFact adapts a domain.Interface to a grading fact. LinkUp comes from
// the monitor (physical) status; AdminUp is filled from the cmdb admin-status map
// when the interface is present there, so a configured-up but link-down port
// grades CRITICAL. AdminUp stays nil for an interface absent from cmdb, keeping a
// legitimately disabled/unplugged port from failing.
func interfaceFact(i domain.Interface, adminUp map[string]bool) health.InterfaceFact {
	up := strings.EqualFold(i.Status, "up")
	f := health.InterfaceFact{Name: i.Name, LinkUp: &up, Duplex: i.Duplex}
	if a, ok := adminUp[i.Name]; ok {
		f.AdminUp = &a
	}
	if i.Speed != "" {
		if s, err := strconv.ParseFloat(strings.TrimSpace(i.Speed), 64); err == nil {
			f.SpeedMbps = &s
		}
	}
	return f
}

// interfaceAdminStatus reads the cmdb system/interface admin status into a
// name->up map. FortiOS models admin state as the interface's "status" field
// ("up"/"down"). A read failure or a missing status yields no entry, so AdminUp
// is left unset rather than failing the health run.
func interfaceAdminStatus(ctx context.Context, p provider.Provider) map[string]bool {
	objs, err := p.CmdbList(ctx, "system/interface")
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(objs))
	for _, o := range objs {
		name, _ := o["name"].(string)
		if name == "" {
			continue
		}
		st, ok := o["status"].(string)
		if !ok {
			continue
		}
		out[name] = strings.EqualFold(st, "up")
	}
	return out
}

// parseFilter returns a predicate matching interface names against a comma list
// (empty list matches everything).
func parseFilter(csv string) func(string) bool {
	if strings.TrimSpace(csv) == "" {
		return func(string) bool { return true }
	}
	set := map[string]struct{}{}
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			set[s] = struct{}{}
		}
	}
	return func(name string) bool { _, ok := set[name]; return ok }
}

func filterTransceivers(in []domain.Transceiver, csv string) []domain.Transceiver {
	want := parseFilter(csv)
	if strings.TrimSpace(csv) == "" {
		return in
	}
	out := in[:0:0]
	for _, o := range in {
		if want(o.Interface) {
			out = append(out, o)
		}
	}
	return out
}

// rawRecords wraps original device records for json/yaml. An empty set renders
// as [] rather than null so scripting output stays a stable array.
func rawRecords(recs []map[string]any) any {
	if recs == nil {
		return []map[string]any{}
	}
	return recs
}

func transceiverRaws(in []domain.Transceiver) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, o := range in {
		if o.Raw != nil {
			out = append(out, o.Raw)
		}
	}
	return out
}

func sensorRaws(in []domain.Sensor) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, s := range in {
		if s.Raw != nil {
			out = append(out, s.Raw)
		}
	}
	return out
}

// ddm renders a DDM reading value for a table cell ("-" when the module reports
// none).
func ddm(r domain.DDMReading) string {
	if r.Value == nil {
		return "-"
	}
	return strconv.FormatFloat(*r.Value, 'f', -1, 64)
}

func sensorValue(s domain.Sensor) string {
	if s.Value == nil {
		return ""
	}
	v := strconv.FormatFloat(*s.Value, 'f', -1, 64)
	if s.Unit != "" {
		v += " " + s.Unit
	}
	return v
}

// sensorStatus renders the health grade (PASS/WARN/CRITICAL/N/A) from the same
// device-supplied thresholds and alarm flag the health roll-up uses, so the table
// STATUS reflects the grade rather than an unverified status text.
func sensorStatus(s domain.Sensor) string {
	return string(health.GradeSensor(s).Severity)
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
