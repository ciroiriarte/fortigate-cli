package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/cve"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

// cveIDPattern validates a CVE identifier (case-insensitive).
var cveIDPattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// cveDisclaimer is appended to the command help: this data is best-effort.
const cveDisclaimer = "CVE data comes from public NVD (NIST) and CIRCL feeds and is best-effort:\n" +
	"it may be incomplete or lag real advisories. Cross-check the Fortinet PSIRT\n" +
	"advisories (https://www.fortiguard.com/psirt) for authoritative guidance.\n" +
	"This tool is unofficial and not affiliated with Fortinet.\n\n" +
	"The CIRCL fallback provides exact-version CPE pins only and cannot compute\n" +
	"version ranges, so its affected/fixed answers are best-effort; supply an NVD\n" +
	"API key (NVD_API_KEY) for authoritative range matching.\n\n" +
	"Exit codes: a successful lookup exits 0 by default. Pass --exit-code N to\n" +
	"exit N when a vulnerability is found (check: running version affected; list:\n" +
	"at least one CVE at/above --min-severity). Lookup/render errors keep their\n" +
	"own non-zero exit codes regardless of --exit-code."

// newCVECmd builds the `cve` command group. External CVE feeds are queried from
// internal/cve, which owns its own HTTP client — deliberately outside the
// FortiGate transport boundary (that client only speaks to the pinned device).
func newCVECmd(a *app) *cobra.Command {
	var source, minSeverity, nvdAPIKey string
	var exitCode int

	cmd := &cobra.Command{
		Use:   "cve",
		Short: "Check FortiOS CVEs affecting the connected device (NVD/CIRCL)",
		Long: "Report CVEs affecting the FortiOS version running on the connected FortiGate.\n" +
			"The running version is read from the device (monitor surface); CVE data is\n" +
			"fetched from external feeds (NVD primary, CIRCL fallback), NOT from the\n" +
			"FortiGate.\n\n" + cveDisclaimer,
	}
	pf := cmd.PersistentFlags()
	pf.StringVar(&source, "source", "auto", "CVE data source: auto|nvd|circl")
	pf.StringVar(&minSeverity, "min-severity", "none", "minimum severity to report: none|low|medium|high|critical")
	pf.StringVar(&nvdAPIKey, "nvd-api-key", "", "NVD API key (overrides NVD_API_KEY / FGT_CLI_NVD_API_KEY)")
	pf.IntVar(&exitCode, "exit-code", 0, "exit with this code when a vulnerability is found (0 = always exit 0 on a successful lookup)")

	cmd.AddCommand(
		newCVEListCmd(a, &source, &minSeverity, &nvdAPIKey, &exitCode),
		newCVECheckCmd(a, &source, &minSeverity, &nvdAPIKey, &exitCode),
	)
	return cmd
}

// deviceVersion reads the running FortiOS version and refuses to proceed when
// the device does not report one — assessing CVEs against an empty version would
// silently return "not affected" for everything.
func deviceVersion(a *app, cmd *cobra.Command) (string, error) {
	p, err := a.Provider()
	if err != nil {
		return "", err
	}
	st, err := p.DeviceStatus(cmd.Context())
	if err != nil {
		return "", err
	}
	ver := strings.TrimSpace(st.Version)
	if ver == "" {
		return "", fmt.Errorf("cannot determine FortiOS version from device; refusing to assess CVEs")
	}
	return ver, nil
}

// vulnExitError returns the requested exit-code signal when a vulnerability was
// found and a non-zero --exit-code was configured; otherwise nil.
func vulnExitError(found bool, exitCode int) error {
	if found && exitCode != 0 {
		return exitCodeError{code: exitCode}
	}
	return nil
}

// resolveSource builds the CVE source, resolving the NVD key from the flag then
// the resolved settings (NVD_API_KEY / FGT_CLI_NVD_API_KEY).
func (a *app) resolveSource(source, nvdAPIKey string) (cve.Source, error) {
	key := nvdAPIKey
	if key == "" {
		if s, err := a.resolvedSettings(); err == nil {
			key = s.NVDAPIKey
		}
	}
	return cve.New(source, key)
}

func newCVEListCmd(a *app, source, minSeverity, nvdAPIKey *string, exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List CVEs affecting the running FortiOS version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			minRank, err := cve.ParseMinSeverity(*minSeverity)
			if err != nil {
				return err
			}
			ver, err := deviceVersion(a, cmd)
			if err != nil {
				return err
			}
			src, err := a.resolveSource(*source, *nvdAPIKey)
			if err != nil {
				return err
			}
			cves, err := src.ForVersion(cmd.Context(), ver)
			if err != nil {
				return err
			}
			filtered := cves[:0:0]
			for _, c := range cves {
				if cve.MeetsSeverity(c, minRank) {
					filtered = append(filtered, c)
				}
			}
			sortCVEs(filtered)
			fmt.Fprintf(cmd.ErrOrStderr(), "[fgt] FortiOS %s — %d CVE(s); source: %s\n",
				ver, len(filtered), cve.SourceNote(src))

			t := output.Tabular{
				Columns: []string{"CVE", "SEVERITY", "CVSS", "FIXED-IN", "PUBLISHED", "SUMMARY"},
				Raw:     filtered,
			}
			for _, c := range filtered {
				t.Rows = append(t.Rows, []string{
					c.ID,
					c.Severity,
					formatCVSS(c.CVSS),
					c.FixedIn,
					formatDate(c.Published),
					truncate(c.Description, 80),
				})
			}
			if err := a.render(t); err != nil {
				return err
			}
			return vulnExitError(len(filtered) > 0, *exitCode)
		},
	}
}

func newCVECheckCmd(a *app, source, minSeverity, nvdAPIKey *string, exitCode *int) *cobra.Command {
	_ = minSeverity // not meaningful for a single-CVE lookup
	return &cobra.Command{
		Use:   "check <CVE-ID>",
		Short: "Report whether the running FortiOS version is affected by a CVE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			if !cveIDPattern.MatchString(id) {
				return fmt.Errorf("invalid CVE id %q (want CVE-YYYY-NNNN)", args[0])
			}
			ver, err := deviceVersion(a, cmd)
			if err != nil {
				return err
			}
			src, err := a.resolveSource(*source, *nvdAPIKey)
			if err != nil {
				return err
			}
			c, err := src.ByID(cmd.Context(), id, ver)
			if err != nil {
				return err
			}
			note := cve.SourceNote(src)
			fmt.Fprintf(cmd.ErrOrStderr(), "[fgt] FortiOS %s — %s: AFFECTED=%s; source: %s\n",
				ver, c.ID, yesNo(c.Affected), note)
			// CIRCL pins exact versions and can't compute ranges, so a fallback
			// "affected: no" may be an under-report.
			if strings.Contains(note, "fallback") {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"[fgt] warning: answered by CIRCL fallback, which provides exact-version pins only and "+
						"cannot compute version ranges; an \"affected: no\" may be incomplete — retry with an "+
						"NVD API key (NVD_API_KEY) for authoritative range matching")
			}

			t := output.Tabular{Columns: []string{"FIELD", "VALUE"}, Raw: c, Rows: [][]string{
				{"cve", c.ID},
				{"running-version", ver},
				{"affected", yesNo(c.Affected)},
				{"fixed-in", c.FixedIn},
				{"introduced-in", c.IntroducedIn},
				{"severity", c.Severity},
				{"cvss", formatCVSS(c.CVSS)},
				{"published", formatDate(c.Published)},
				{"summary", c.Description},
			}}
			if err := a.render(t); err != nil {
				return err
			}
			return vulnExitError(c.Affected, *exitCode)
		},
	}
}

// sortCVEs orders by severity desc, then CVSS desc, then ID asc.
func sortCVEs(cves []cve.CVE) {
	sort.SliceStable(cves, func(i, j int) bool {
		si, sj := cve.SeverityRank(cves[i].Severity), cve.SeverityRank(cves[j].Severity)
		if si != sj {
			return si > sj
		}
		if cves[i].CVSS != cves[j].CVSS {
			return cves[i].CVSS > cves[j].CVSS
		}
		return cves[i].ID < cves[j].ID
	})
}

func formatCVSS(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if max >= 1 && len([]rune(s)) > max {
		return string([]rune(s)[:max-1]) + "…"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
