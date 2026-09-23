package cve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// canned NVD 2.0 payload: one FortiOS CVE with a versionEndExcluding range and
// one whose FortiOS cpeMatch pins an exact version (no range bounds).
const nvdPayload = `{
  "vulnerabilities": [
    {
      "cve": {
        "id": "CVE-2024-1111",
        "published": "2024-02-01T10:00:00.000",
        "lastModified": "2024-03-01T10:00:00.000",
        "descriptions": [
          {"lang": "es", "value": "no"},
          {"lang": "en", "value": "A FortiOS flaw fixed in 7.4.4."}
        ],
        "metrics": {
          "cvssMetricV31": [
            {"cvssData": {"baseScore": 9.8, "baseSeverity": "CRITICAL", "vectorString": "CVSS:3.1/AV:N"}}
          ]
        },
        "references": [{"url": "https://example.test/a"}],
        "configurations": [
          {"nodes": [
            {"cpeMatch": [
              {"criteria": "cpe:2.3:o:fortinet:fortios:*:*:*:*:*:*:*:*", "vulnerable": true,
               "versionStartIncluding": "7.4.0", "versionEndExcluding": "7.4.4"}
            ]}
          ]}
        ]
      }
    },
    {
      "cve": {
        "id": "CVE-2023-2222",
        "published": "2023-05-01T10:00:00.000",
        "lastModified": "2023-06-01T10:00:00.000",
        "descriptions": [{"lang": "en", "value": "Pinned to 7.4.3 exactly."}],
        "metrics": {
          "cvssMetricV2": [
            {"cvssData": {"baseScore": 5.0, "vectorString": "AV:N/AC:L"}}
          ]
        },
        "configurations": [
          {"nodes": [
            {"cpeMatch": [
              {"criteria": "cpe:2.3:o:fortinet:fortios:7.4.3:*:*:*:*:*:*:*", "vulnerable": true}
            ]}
          ]}
        ]
      }
    }
  ]
}`

func nvdTestServer(t *testing.T, payload string, status int) *NVD {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	n := NewNVD("")
	n.baseURL = srv.URL
	return n
}

func TestNVDForVersion(t *testing.T) {
	n := nvdTestServer(t, nvdPayload, http.StatusOK)
	cves, err := n.ForVersion(context.Background(), "v7.4.3")
	if err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	if len(cves) != 2 {
		t.Fatalf("want 2 CVEs, got %d", len(cves))
	}
	byID := map[string]CVE{}
	for _, c := range cves {
		byID[c.ID] = c
	}
	a := byID["CVE-2024-1111"]
	if a.Severity != "CRITICAL" || a.CVSS != 9.8 {
		t.Errorf("CVE-2024-1111 metrics = %s/%v, want CRITICAL/9.8", a.Severity, a.CVSS)
	}
	if a.FixedIn != "7.4.4" {
		t.Errorf("CVE-2024-1111 FixedIn = %q, want 7.4.4", a.FixedIn)
	}
	if a.Description != "A FortiOS flaw fixed in 7.4.4." {
		t.Errorf("english description not picked: %q", a.Description)
	}
	if a.Vector != "CVSS:3.1/AV:N" {
		t.Errorf("vector = %q", a.Vector)
	}
	// v2-only metrics: score kept, severity derived from score (5.0 -> MEDIUM).
	b := byID["CVE-2023-2222"]
	if b.CVSS != 5.0 || b.Severity != "MEDIUM" {
		t.Errorf("CVE-2023-2222 metrics = %s/%v, want MEDIUM/5.0", b.Severity, b.CVSS)
	}
}

func TestNVDByIDRangeBoundaries(t *testing.T) {
	n := nvdTestServer(t, nvdPayload, http.StatusOK)
	// CVE-2024-1111 range is [7.4.0, 7.4.4).
	cases := []struct {
		version   string
		want      bool
		wantFixed string
	}{
		{"7.4.0", true, "7.4.4"},  // versionStartIncluding is inclusive
		{"7.4.3", true, "7.4.4"},  // inside range
		{"7.4.4", false, "7.4.4"}, // versionEndExcluding is exclusive (same branch => bounds reported)
		{"7.4.5", false, "7.4.4"}, // above range but same 7.4 branch => bounds reported
		{"7.3.9", false, ""},      // different branch (7.3) => never affected, no bounds reported
	}
	for _, c := range cases {
		got, err := n.ByID(context.Background(), "CVE-2024-1111", c.version)
		if err != nil {
			t.Fatalf("ByID(%s): %v", c.version, err)
		}
		if got.Affected != c.want {
			t.Errorf("CVE-2024-1111 @ %s Affected = %v, want %v", c.version, got.Affected, c.want)
		}
		if got.FixedIn != c.wantFixed {
			t.Errorf("CVE-2024-1111 @ %s FixedIn = %q, want %q", c.version, got.FixedIn, c.wantFixed)
		}
	}
}

func TestNVDByIDExactVersion(t *testing.T) {
	n := nvdTestServer(t, nvdPayload, http.StatusOK)
	// CVE-2023-2222 pins fortios:7.4.3 exactly (no range bounds).
	aff, err := n.ByID(context.Background(), "CVE-2023-2222", "7.4.3")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if !aff.Affected {
		t.Error("7.4.3 should be affected by an exact-7.4.3 CPE")
	}
	notAff, err := n.ByID(context.Background(), "CVE-2023-2222", "7.4.4")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if notAff.Affected {
		t.Error("7.4.4 should NOT be affected by an exact-7.4.3 CPE")
	}
}

// nvdEndIncludingPayload has a FortiOS cpeMatch bounded by versionEndIncluding
// (inclusive upper bound), which names no clean single fixed version.
const nvdEndIncludingPayload = `{
  "vulnerabilities": [
    {
      "cve": {
        "id": "CVE-2022-3333",
        "published": "2022-08-01T10:00:00.000",
        "lastModified": "2022-09-01T10:00:00.000",
        "descriptions": [{"lang": "en", "value": "Affects up to and including 7.2.5."}],
        "metrics": {
          "cvssMetricV31": [
            {"cvssData": {"baseScore": 7.5, "baseSeverity": "HIGH", "vectorString": "CVSS:3.1/AV:N"}}
          ]
        },
        "configurations": [
          {"nodes": [
            {"cpeMatch": [
              {"criteria": "cpe:2.3:o:fortinet:fortios:*:*:*:*:*:*:*:*", "vulnerable": true,
               "versionStartIncluding": "7.2.0", "versionEndIncluding": "7.2.5"}
            ]}
          ]}
        ]
      }
    }
  ]
}`

func TestNVDByIDEndIncludingBoundary(t *testing.T) {
	n := nvdTestServer(t, nvdEndIncludingPayload, http.StatusOK)
	// Range is [7.2.0, 7.2.5] inclusive on both ends.
	cases := []struct {
		version        string
		want           bool
		wantIntroduced string
	}{
		{"7.2.0", true, "7.2.0"},  // startIncluding inclusive
		{"7.2.5", true, "7.2.0"},  // endIncluding is inclusive
		{"7.2.6", false, "7.2.0"}, // just above the inclusive end, same 7.2 branch
		{"7.3.0", false, ""},      // different branch (7.3) => never affected, no bounds
	}
	for _, c := range cases {
		got, err := n.ByID(context.Background(), "CVE-2022-3333", c.version)
		if err != nil {
			t.Fatalf("ByID(%s): %v", c.version, err)
		}
		if got.Affected != c.want {
			t.Errorf("CVE-2022-3333 @ %s Affected = %v, want %v", c.version, got.Affected, c.want)
		}
		// An inclusive upper bound is not a clean fixed version, so FixedIn stays
		// empty (never a wrong value) while IntroducedIn is reported for a
		// same-branch box.
		if got.FixedIn != "" {
			t.Errorf("versionEndIncluding must not set FixedIn, got %q", got.FixedIn)
		}
		if got.IntroducedIn != c.wantIntroduced {
			t.Errorf("CVE-2022-3333 @ %s IntroducedIn = %q, want %q", c.version, got.IntroducedIn, c.wantIntroduced)
		}
	}
}

// nvdMultiBranchPayload has one CVE with two FortiOS branch ranges: 6.0.0-6.0.18
// and 7.4.0-7.4.3. It exercises the branch-aware fixed-in selection.
const nvdMultiBranchPayload = `{
  "vulnerabilities": [
    {
      "cve": {
        "id": "CVE-2024-21762",
        "published": "2024-02-08T10:00:00.000",
        "lastModified": "2024-03-01T10:00:00.000",
        "descriptions": [{"lang": "en", "value": "Multi-branch FortiOS flaw."}],
        "metrics": {
          "cvssMetricV31": [
            {"cvssData": {"baseScore": 9.8, "baseSeverity": "CRITICAL", "vectorString": "CVSS:3.1/AV:N"}}
          ]
        },
        "configurations": [
          {"nodes": [
            {"cpeMatch": [
              {"criteria": "cpe:2.3:o:fortinet:fortios:*:*:*:*:*:*:*:*", "vulnerable": true,
               "versionStartIncluding": "6.0.0", "versionEndExcluding": "6.0.18"},
              {"criteria": "cpe:2.3:o:fortinet:fortios:*:*:*:*:*:*:*:*", "vulnerable": true,
               "versionStartIncluding": "7.4.0", "versionEndExcluding": "7.4.3"}
            ]}
          ]}
        ]
      }
    }
  ]
}`

// TestNVDByIDMultiBranchFixedIn asserts the branch-aware fallback: a box reports
// the fixed-in of ITS OWN branch (not the lowest across all branches), and a box
// on a branch the CVE never covered reports affected=no with empty bounds.
func TestNVDByIDMultiBranchFixedIn(t *testing.T) {
	n := nvdTestServer(t, nvdMultiBranchPayload, http.StatusOK)
	cases := []struct {
		version   string
		want      bool
		wantFixed string
	}{
		{"7.4.12", false, "7.4.3"}, // above 7.4 range but same branch => 7.4-branch fixed-in
		{"7.4.1", true, "7.4.3"},   // inside 7.4 range => affected, 7.4-branch fixed-in
		{"8.0.0", false, ""},       // branch never covered => not affected, no bounds
	}
	for _, c := range cases {
		got, err := n.ByID(context.Background(), "CVE-2024-21762", c.version)
		if err != nil {
			t.Fatalf("ByID(%s): %v", c.version, err)
		}
		if got.Affected != c.want {
			t.Errorf("CVE-2024-21762 @ %s Affected = %v, want %v", c.version, got.Affected, c.want)
		}
		if got.FixedIn != c.wantFixed {
			t.Errorf("CVE-2024-21762 @ %s FixedIn = %q, want %q", c.version, got.FixedIn, c.wantFixed)
		}
	}
}

func TestMetricFieldsDerivesSeverity(t *testing.T) {
	// A v3.x metric with an empty baseSeverity must derive one from the score, so
	// the CVE does not falsely rank NONE (and get dropped by --min-severity).
	var m nvdCVSSMetric
	m.CVSSData.BaseScore = 9.1
	m.CVSSData.BaseSeverity = "" // missing textual severity
	score, sev, _ := metricFields(m)
	if score != 9.1 || sev != "CRITICAL" {
		t.Errorf("metricFields = %v/%q, want 9.1/CRITICAL", score, sev)
	}
}

func TestNVDRateLimited(t *testing.T) {
	n := nvdTestServer(t, "", http.StatusTooManyRequests)
	// Shorten retries implicitly: the handler always 429s, so we just assert the
	// sentinel is returned after retries.
	_, err := n.ForVersion(context.Background(), "7.4.3")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("429 should map to ErrRateLimited, got %v", err)
	}
}

func TestNVDServerErrorUnavailable(t *testing.T) {
	n := nvdTestServer(t, "", http.StatusBadGateway)
	_, err := n.ForVersion(context.Background(), "7.4.3")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("5xx should map to ErrUnavailable, got %v", err)
	}
}
