package cve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ciroiriarte/fortigate-cli/internal/version"
)

// circlBaseURL is the CIRCL CVE Search API root.
const circlBaseURL = "https://cve.circl.lu"

// CIRCL is a fallback client for the CIRCL CVE-Search API. baseURL is injectable
// for tests.
//
// NOTE: the CIRCL API schema has changed over time and is not versioned as
// rigorously as NVD's. Everything here parses DEFENSIVELY into loose structures
// and degrades gracefully (e.g. leaves FixedIn empty) rather than failing when a
// field is missing or reshaped. The exact response shape SHOULD be verified
// against the live API (https://cve.circl.lu/api/) before relying on it.
type CIRCL struct {
	baseURL string
	client  *http.Client
}

// NewCIRCL builds a CIRCL client.
func NewCIRCL() *CIRCL {
	return &CIRCL{
		baseURL: circlBaseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Name identifies this source.
func (c *CIRCL) Name() string { return "circl" }

// ForVersion fetches all FortiOS CVEs and filters client-side by version range.
func (c *CIRCL) ForVersion(ctx context.Context, fortiosVersion string) ([]CVE, error) {
	body, err := c.get(ctx, "/api/search/fortinet/fortios")
	if err != nil {
		return nil, err
	}
	records, err := decodeCIRCLList(body)
	if err != nil {
		return nil, err
	}
	out := make([]CVE, 0, len(records))
	for _, rec := range records {
		cv := rec.normalize()
		if rng, matched := rec.fortiosRange(); matched {
			cv.applyRange(rng, fortiosVersion)
			if !cv.Affected {
				continue // version not in range — exclude from the list
			}
		} else {
			// No parseable FortiOS range: keep it (a fortios search hit) but we
			// cannot scope it to the running version.
			cv.Affected = true
		}
		out = append(out, cv)
	}
	return out, nil
}

// ByID fetches one CVE and computes affected/fixed for the running version.
func (c *CIRCL) ByID(ctx context.Context, id, fortiosVersion string) (CVE, error) {
	body, err := c.get(ctx, "/api/cve/"+strings.ToUpper(id))
	if err != nil {
		return CVE{}, err
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || trimmed == "null" {
		return CVE{}, fmt.Errorf("cve: %s not found in CIRCL", id)
	}
	var rec circlCVE
	if err := json.Unmarshal(body, &rec); err != nil {
		return CVE{}, fmt.Errorf("circl: decode %s: %w", id, err)
	}
	cv := rec.normalize()
	if rng, matched := rec.fortiosRange(); matched {
		cv.applyRange(rng, fortiosVersion)
	} else {
		cv.Affected = false
	}
	return cv, nil
}

func (c *CIRCL) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("circl: %w (HTTP %d)", ErrRateLimited, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("circl: unexpected HTTP %d", resp.StatusCode)
	}
	if err != nil {
		return nil, fmt.Errorf("circl: read body: %w", err)
	}
	return body, nil
}

// decodeCIRCLList tolerates both a bare JSON array and an object wrapping the
// records under "results" or "data" (the CIRCL surface has used both shapes).
func decodeCIRCLList(body []byte) ([]circlCVE, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []circlCVE
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil, fmt.Errorf("circl: decode list: %w", err)
		}
		return arr, nil
	}
	var wrap struct {
		Results []circlCVE `json:"results"`
		Data    []circlCVE `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("circl: decode wrapped list: %w", err)
	}
	if len(wrap.Results) > 0 {
		return wrap.Results, nil
	}
	return wrap.Data, nil
}

// --- CIRCL schema (loose; extra/renamed fields are ignored) ---

type circlCVE struct {
	ID         string  `json:"id"`
	Summary    string  `json:"summary"`
	CVSS       float64 `json:"cvss"`
	CVSSVector string  `json:"cvss-vector"`
	// CIRCL has exposed severity under various keys across versions.
	Severity  string `json:"severity"`
	Published string `json:"Published"`
	Modified  string `json:"Modified"`
	// Alternate lowercase timestamp keys seen on newer builds.
	PublishedAlt string `json:"published"`
	ModifiedAlt  string `json:"modified"`
	// CPEs the record marks vulnerable; used to derive a FortiOS version range.
	VulnerableConfiguration []circlCPE `json:"vulnerable_configuration"`
	VulnerableConfigCPEs    []string   `json:"vulnerable_configuration_cpe_2_2"`
	VulnerableProductStanza []circlCPE `json:"vulnerable_product"`
	References              []string   `json:"references"`
}

// circlCPE tolerates both a bare CPE string and an object carrying an "id"/
// "title" field (CIRCL has used both encodings).
type circlCPE struct {
	value string
}

func (p *circlCPE) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		p.value = s
		return nil
	}
	var obj struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	if obj.ID != "" {
		p.value = obj.ID
	} else {
		p.value = obj.Title
	}
	return nil
}

func (r circlCVE) normalize() CVE {
	c := CVE{ID: r.ID, Description: r.Summary, CVSS: r.CVSS, Vector: r.CVSSVector}
	c.Severity = r.severity()
	c.Published = parseCIRCLTime(firstNonEmptyStr(r.Published, r.PublishedAlt))
	c.Modified = parseCIRCLTime(firstNonEmptyStr(r.Modified, r.ModifiedAlt))
	c.References = append(c.References, r.References...)
	return c
}

// severity returns a normalized severity label, deriving one from the CVSS score
// when CIRCL does not supply a textual value.
func (r circlCVE) severity() string {
	if s := strings.ToUpper(strings.TrimSpace(r.Severity)); s != "" {
		return s
	}
	return severityFromScore(r.CVSS)
}

// fortiosRange derives a version window from any FortiOS CPE strings on the
// record. CIRCL CPEs are typically pinned to a single version (component 5) with
// no exclusive upper bound, so a single pin becomes an exact match and multiple
// pins collapse to the minimum as IntroducedIn with FixedIn left empty. matched
// reports whether any FortiOS CPE was found.
func (r circlCVE) fortiosRange() (rng versionRange, matched bool) {
	var versions []string
	collect := func(cpes []circlCPE) {
		for _, p := range cpes {
			if !strings.Contains(p.value, ":fortinet:fortios:") {
				continue
			}
			matched = true
			if v := cpeExactVersion(p.value); v != "" {
				versions = append(versions, v)
			}
		}
	}
	collect(r.VulnerableConfiguration)
	collect(r.VulnerableProductStanza)
	for _, s := range r.VulnerableConfigCPEs {
		if strings.Contains(s, ":fortinet:fortios:") {
			matched = true
			if v := cpeExactVersion(s); v != "" {
				versions = append(versions, v)
			}
		}
	}
	if len(versions) == 0 {
		return versionRange{}, matched
	}
	if len(versions) == 1 {
		return versionRange{exact: versions[0]}, true
	}
	min := versions[0]
	for _, v := range versions[1:] {
		if version.Compare(v, min) < 0 {
			min = v
		}
	}
	return versionRange{startIncluding: min}, true
}

func parseCIRCLTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04:05.000"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
