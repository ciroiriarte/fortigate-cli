package cve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ciroiriarte/fortigate-cli/internal/version"
)

// nvdBaseURL is the documented NVD 2.0 CVE API endpoint.
const nvdBaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

// nvdTimeLayout is NVD's ISO-8601 timestamp form (no zone, UTC assumed).
const nvdTimeLayout = "2006-01-02T15:04:05.000"

// NVD is a client for the NIST NVD 2.0 REST API. baseURL is a struct field so
// tests can point it at an httptest server.
type NVD struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewNVD builds an NVD client. An empty apiKey is fine (lower rate limits).
func NewNVD(apiKey string) *NVD {
	return &NVD{
		baseURL: nvdBaseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Name identifies this source.
func (n *NVD) Name() string { return "nvd" }

// ForVersion queries NVD for CVEs whose configurations match the FortiOS CPE for
// the running version. NVD performs the version-range matching server-side via
// virtualMatchString.
func (n *NVD) ForVersion(ctx context.Context, fortiosVersion string) ([]CVE, error) {
	q := url.Values{}
	q.Set("virtualMatchString", CPEFor(fortiosVersion))
	resp, err := n.fetch(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]CVE, 0, len(resp.Vulnerabilities))
	for _, v := range resp.Vulnerabilities {
		c := v.CVE.normalize()
		// Compute the range fields relative to the running version so FixedIn is
		// populated for the affecting configuration.
		if rng, matched := v.CVE.fortiosRange(fortiosVersion); matched {
			c.applyRange(rng, fortiosVersion)
		} else {
			// The server matched this CVE to the CPE, so treat it as affecting.
			c.Affected = true
		}
		out = append(out, c)
	}
	return out, nil
}

// ByID fetches a single CVE and decides whether the running version is affected
// by walking its FortiOS cpeMatch entries.
func (n *NVD) ByID(ctx context.Context, id, fortiosVersion string) (CVE, error) {
	q := url.Values{}
	q.Set("cveId", strings.ToUpper(id))
	resp, err := n.fetch(ctx, q)
	if err != nil {
		return CVE{}, err
	}
	if len(resp.Vulnerabilities) == 0 {
		return CVE{}, fmt.Errorf("cve: %s not found in NVD", id)
	}
	raw := resp.Vulnerabilities[0].CVE
	c := raw.normalize()
	if rng, matched := raw.fortiosRange(fortiosVersion); matched {
		c.applyRange(rng, fortiosVersion)
	} else {
		// No FortiOS configuration on this CVE — cannot claim the running FortiOS
		// is affected.
		c.Affected = false
	}
	return c, nil
}

// fetch performs a GET with retries/backoff on 429 and maps 403/429 to
// ErrRateLimited so the auto orchestrator can fall back.
func (n *NVD) fetch(ctx context.Context, q url.Values) (*nvdResponse, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Linear backoff before a retry; respect context cancellation.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 600 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.baseURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if n.apiKey != "" {
			req.Header.Set("apiKey", n.apiKey)
		}
		resp, err := n.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("nvd: %w: %v", ErrUnavailable, err)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusForbidden:
			return nil, fmt.Errorf("nvd: %w (HTTP 403 — check NVD_API_KEY)", ErrRateLimited)
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("nvd: %w (HTTP 429)", ErrRateLimited)
			continue // retry with backoff
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("nvd: %w: server error HTTP %d", ErrUnavailable, resp.StatusCode)
			continue
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("nvd: unexpected HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if readErr != nil {
			return nil, fmt.Errorf("nvd: read body: %w", readErr)
		}
		var out nvdResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("nvd: decode response: %w", err)
		}
		return &out, nil
	}
	return nil, lastErr
}

// --- NVD 2.0 schema (only the fields we consume; defensive about the rest) ---

type nvdResponse struct {
	Vulnerabilities []struct {
		CVE nvdCVE `json:"cve"`
	} `json:"vulnerabilities"`
}

type nvdCVE struct {
	ID           string `json:"id"`
	Published    string `json:"published"`
	LastModified string `json:"lastModified"`
	Descriptions []struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	} `json:"descriptions"`
	Metrics struct {
		V31 []nvdCVSSMetric `json:"cvssMetricV31"`
		V30 []nvdCVSSMetric `json:"cvssMetricV30"`
		V2  []nvdCVSSMetric `json:"cvssMetricV2"`
	} `json:"metrics"`
	References []struct {
		URL string `json:"url"`
	} `json:"references"`
	Configurations []struct {
		Nodes []struct {
			CPEMatch []nvdCPEMatch `json:"cpeMatch"`
		} `json:"nodes"`
	} `json:"configurations"`
}

type nvdCVSSMetric struct {
	CVSSData struct {
		BaseScore    float64 `json:"baseScore"`
		BaseSeverity string  `json:"baseSeverity"`
		VectorString string  `json:"vectorString"`
	} `json:"cvssData"`
	// v2 carries baseSeverity as a sibling of cvssData rather than inside it.
	BaseSeverity string `json:"baseSeverity"`
}

type nvdCPEMatch struct {
	Criteria              string `json:"criteria"`
	Vulnerable            bool   `json:"vulnerable"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
}

// versionRange builds the affected-version window from this cpeMatch.
func (m nvdCPEMatch) versionRange() versionRange {
	return versionRange{
		startIncluding: m.VersionStartIncluding,
		startExcluding: m.VersionStartExcluding,
		endIncluding:   m.VersionEndIncluding,
		endExcluding:   m.VersionEndExcluding,
		exact:          cpeExactVersion(m.Criteria),
	}
}

// normalize converts an NVD CVE record into our CVE type (without range fields,
// which depend on the running version).
func (r nvdCVE) normalize() CVE {
	c := CVE{ID: r.ID}
	c.Published = parseNVDTime(r.Published)
	c.Modified = parseNVDTime(r.LastModified)
	c.Description = pickEnglish(r.Descriptions)
	c.CVSS, c.Severity, c.Vector = r.pickMetrics()
	for _, ref := range r.References {
		if ref.URL != "" {
			c.References = append(c.References, ref.URL)
		}
	}
	return c
}

// pickMetrics returns the preferred CVSS score/severity/vector: v3.1, then v3.0,
// then v2. When a metric lacks a textual baseSeverity, it is derived from the
// score via the standard CVSS banding so the CVE never falsely ranks NONE.
func (r nvdCVE) pickMetrics() (float64, string, string) {
	if len(r.Metrics.V31) > 0 {
		return metricFields(r.Metrics.V31[0])
	}
	if len(r.Metrics.V30) > 0 {
		return metricFields(r.Metrics.V30[0])
	}
	if len(r.Metrics.V2) > 0 {
		return metricFields(r.Metrics.V2[0])
	}
	return 0, "NONE", ""
}

// metricFields extracts score/severity/vector from one CVSS metric, deriving a
// severity label from the score when the feed omits a textual one. v2 metrics
// carry baseSeverity as a sibling of cvssData; v3.x carry it inside.
func metricFields(m nvdCVSSMetric) (float64, string, string) {
	sev := m.CVSSData.BaseSeverity
	if sev == "" {
		sev = m.BaseSeverity
	}
	if sev == "" {
		sev = severityFromScore(m.CVSSData.BaseScore)
	}
	return m.CVSSData.BaseScore, strings.ToUpper(sev), m.CVSSData.VectorString
}

// fortiosRange scans the CVE's configurations for a FortiOS cpeMatch and returns
// the affected-version window for the vulnerable FortiOS entry. It prefers the
// candidate whose range actually contains the running version so the reported
// bounds describe the affecting configuration; otherwise it falls back to the
// one with the lowest fixed version. matched reports whether any FortiOS
// cpeMatch was found at all.
func (r nvdCVE) fortiosRange(running string) (rng versionRange, matched bool) {
	var candidates []nvdCPEMatch
	for _, cfg := range r.Configurations {
		for _, node := range cfg.Nodes {
			for _, m := range node.CPEMatch {
				if !m.Vulnerable {
					continue
				}
				if !strings.Contains(m.Criteria, ":fortinet:fortios:") {
					continue
				}
				candidates = append(candidates, m)
			}
		}
	}
	if len(candidates) == 0 {
		return versionRange{}, false
	}
	for _, m := range candidates {
		r := m.versionRange()
		if affectedByRange(running, r) {
			return r, true
		}
	}
	// No candidate contains the running version: report the one with the lowest
	// fixed version, treating an empty (open-ended) upper bound as the highest so
	// it sorts LAST rather than first.
	sort.SliceStable(candidates, func(i, j int) bool {
		return lessFixed(candidates[i].VersionEndExcluding, candidates[j].VersionEndExcluding)
	})
	return candidates[0].versionRange(), true
}

// lessFixed orders two versionEndExcluding values ascending, with an empty value
// (open-ended, no known fix) sorting after any concrete version.
func lessFixed(a, b string) bool {
	if a == "" {
		return false
	}
	if b == "" {
		return true
	}
	return version.Compare(a, b) < 0
}

// cpeExactVersion extracts the version field (component index 5) of a CPE 2.3
// URI, returning "" for the wildcard/NA markers "*" and "-".
func cpeExactVersion(criteria string) string {
	parts := strings.Split(criteria, ":")
	if len(parts) < 6 {
		return ""
	}
	v := parts[5]
	if v == "*" || v == "-" || v == "" {
		return ""
	}
	return v
}

func parseNVDTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(nvdTimeLayout, s); err == nil {
		return t
	}
	// Some records include a zone; try RFC3339 as a secondary layout.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func pickEnglish(ds []struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}) string {
	for _, d := range ds {
		if strings.EqualFold(d.Lang, "en") {
			return d.Value
		}
	}
	if len(ds) > 0 {
		return ds[0].Value
	}
	return ""
}
