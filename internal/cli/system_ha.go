package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/health"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// haReport is the structured HA cross-check result rendered by `system ha check`.
// It is the Tabular.Raw payload, so json/yaml carry the full picture — members,
// config, checksums, and the graded findings — faithfully.
type haReport struct {
	Mode      string              `json:"mode"`
	GroupName string              `json:"group_name"`
	Members   []domain.HAMember   `json:"members"`
	Config    domain.HAConfig     `json:"config"`
	Checksums []domain.HAChecksum `json:"checksums"`
	Overall   health.Severity     `json:"overall"`
	Counts    map[string]int      `json:"counts"`
	Findings  []health.Finding    `json:"findings"`
}

// haCheckCmd cross-checks the HA link and cluster state into a graded report with
// a CI-friendly exit code, mirroring `system health`.
func haCheckCmd(a *app) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Cross-check HA links + cluster state (monitor + cmdb surfaces)",
		Long: "Cross-check the HA cluster from real device facts: member roster and roles,\n" +
			"heartbeat-link status, and config-sync checksums.\n\n" +
			"Grading is conservative and never invents cluster state: an unknown fact is\n" +
			"reported N/A, never a failure, and a STANDALONE unit is never failed. The\n" +
			"headline check is the heartbeat link — each configured hbdev interface must be\n" +
			"up (CRITICAL when down); a single heartbeat link WARNs on lost redundancy.\n" +
			"Roles: exactly one elected primary PASSes; zero (no primary) or more than one\n" +
			"(split-brain) is CRITICAL. Config sync compares every member's whole-config\n" +
			"checksum, naming the diverging VDOM when a per-VDOM checksum differs.\n\n" +
			"Data sources degrade gracefully: if one endpoint errors, it is noted and the\n" +
			"rest is still graded; the command only hard-fails when nothing is readable.\n" +
			"json/yaml output carries the full report (members, config, checksums,\n" +
			"findings).\n\n" +
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
			report, err := buildHAReport(cmd.Context(), p)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(),
				"[fgt] ha check: mode %s group %s members %d — overall %s; CRITICAL=%d WARN=%d PASS=%d N/A=%d\n",
				emptyDash(report.Mode), emptyDash(report.GroupName), len(report.Members),
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
	return cmd
}

// buildHAReport gathers HA facts from the provider and grades them. Each data
// source degrades independently: an errored source is noted as an N/A finding and
// the rest is still graded; only when NO source is readable does the command
// fail.
func buildHAReport(ctx context.Context, p provider.Provider) (haReport, error) {
	var srcErrs []health.Finding

	members, err := p.HAMembers(ctx)
	membersErr := err
	if err != nil {
		members = nil
		srcErrs = append(srcErrs, health.Finding{
			Severity: health.NA, Code: "ha.data", Subject: "ha-peer",
			Message: "member roster unreadable: " + err.Error(),
		})
	}
	cfg, err := p.HAConfig(ctx)
	cfgErr := err
	if err != nil {
		cfg = domain.HAConfig{}
		srcErrs = append(srcErrs, health.Finding{
			Severity: health.NA, Code: "ha.data", Subject: "system/ha",
			Message: "HA config unreadable: " + err.Error(),
		})
	}
	checksums, err := p.HAChecksums(ctx)
	checksumsErr := err
	if err != nil {
		checksums = nil
		srcErrs = append(srcErrs, health.Finding{
			Severity: health.NA, Code: "ha.data", Subject: "ha-checksums",
			Message: "config-sync checksums unreadable: " + err.Error(),
		})
	}

	// Only hard-fail when nothing at all is readable.
	if membersErr != nil && cfgErr != nil && checksumsErr != nil {
		return haReport{}, membersErr
	}

	hbLink := haHeartbeatLinks(ctx, p, cfg.HeartbeatDevs)

	findings := append(srcErrs, health.GradeHA(members, cfg, hbLink, checksums)...)
	overall, counts := health.Summarize(findings)

	return haReport{
		Mode:      cfg.Mode,
		GroupName: cfg.GroupName,
		Members:   members,
		Config:    cfg,
		Checksums: checksums,
		Overall:   overall,
		Counts:    counts,
		Findings:  findings,
	}, nil
}

// haHeartbeatLinks builds a name->link-up map for the heartbeat interfaces from
// the monitor interface surface. A read failure degrades to an empty map, so the
// heartbeat check reports each dev N/A rather than failing the run. Only the
// configured hbdev interfaces are kept.
func haHeartbeatLinks(ctx context.Context, p provider.Provider, devs []string) map[string]bool {
	if len(devs) == 0 {
		return nil
	}
	ifaces, err := p.ListInterfaces(ctx)
	if err != nil {
		return nil
	}
	want := map[string]struct{}{}
	for _, d := range devs {
		want[d] = struct{}{}
	}
	out := make(map[string]bool, len(devs))
	for _, i := range ifaces {
		if _, ok := want[i.Name]; ok {
			out[i.Name] = strings.EqualFold(i.Status, "up")
		}
	}
	return out
}
