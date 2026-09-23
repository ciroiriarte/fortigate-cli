package cve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// canned CIRCL payloads. The list endpoint returns a bare array; each record
// pins one or more FortiOS CPEs. Fields intentionally use mixed casing to
// exercise the defensive parsing.
const circlListPayload = `[
  {
    "id": "CVE-2024-1111",
    "summary": "FortiOS flaw affecting 7.4.3.",
    "cvss": 9.8,
    "cvss-vector": "CVSS:3.1/AV:N",
    "Published": "2024-02-01T10:00:00",
    "vulnerable_configuration": ["cpe:2.3:o:fortinet:fortios:7.4.3:*:*:*:*:*:*:*"],
    "references": ["https://example.test/a"]
  },
  {
    "id": "CVE-2020-9999",
    "summary": "Old FortiOS flaw affecting 6.2.1.",
    "cvss": 7.5,
    "Published": "2020-01-01T10:00:00",
    "vulnerable_configuration": [{"id": "cpe:2.3:o:fortinet:fortios:6.2.1:*:*:*:*:*:*:*", "title": "FortiOS 6.2.1"}]
  }
]`

const circlSinglePayload = `{
  "id": "CVE-2024-1111",
  "summary": "FortiOS flaw affecting 7.4.3.",
  "cvss": 9.8,
  "severity": "critical",
  "Published": "2024-02-01T10:00:00",
  "vulnerable_configuration": ["cpe:2.3:o:fortinet:fortios:7.4.3:*:*:*:*:*:*:*"]
}`

func circlTestServer(t *testing.T) *CIRCL {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/search/"):
			_, _ = w.Write([]byte(circlListPayload))
		case strings.HasPrefix(r.URL.Path, "/api/cve/"):
			_, _ = w.Write([]byte(circlSinglePayload))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewCIRCL()
	c.baseURL = srv.URL
	return c
}

func TestCIRCLForVersionFilters(t *testing.T) {
	c := circlTestServer(t)
	cves, err := c.ForVersion(context.Background(), "7.4.3")
	if err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	// Only the 7.4.3-pinned CVE matches the running version; the 6.2.1 one is
	// filtered out client-side.
	if len(cves) != 1 {
		t.Fatalf("want 1 CVE for 7.4.3, got %d: %+v", len(cves), cves)
	}
	got := cves[0]
	if got.ID != "CVE-2024-1111" {
		t.Errorf("wrong CVE: %s", got.ID)
	}
	if got.Severity != "CRITICAL" { // derived from CVSS 9.8 (no textual severity)
		t.Errorf("severity = %q, want CRITICAL", got.Severity)
	}
	if got.CVSS != 9.8 {
		t.Errorf("cvss = %v", got.CVSS)
	}
	if got.Published.IsZero() {
		t.Error("published time should parse")
	}
}

func TestCIRCLForVersionOlderVersion(t *testing.T) {
	c := circlTestServer(t)
	// A device on 6.2.1 matches the old CVE, not the 7.4.3 one.
	cves, err := c.ForVersion(context.Background(), "6.2.1")
	if err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	if len(cves) != 1 || cves[0].ID != "CVE-2020-9999" {
		t.Fatalf("want CVE-2020-9999 for 6.2.1, got %+v", cves)
	}
}

func TestCIRCLByID(t *testing.T) {
	c := circlTestServer(t)
	got, err := c.ByID(context.Background(), "CVE-2024-1111", "7.4.3")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if !got.Affected {
		t.Error("7.4.3 should be affected by the 7.4.3-pinned CVE")
	}
	if got.Severity != "CRITICAL" {
		t.Errorf("severity = %q, want CRITICAL", got.Severity)
	}
	notAff, err := c.ByID(context.Background(), "CVE-2024-1111", "7.4.4")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if notAff.Affected {
		t.Error("7.4.4 should NOT be affected by an exact-7.4.3 CPE")
	}
}
