package cve

import (
	"context"
	"errors"
	"testing"
)

// fakeSource is a stub Source with programmable behavior.
type fakeSource struct {
	name   string
	err    error
	cves   []CVE
	byID   CVE
	called bool
}

func (f *fakeSource) Name() string { return f.name }

func (f *fakeSource) ForVersion(_ context.Context, _ string) ([]CVE, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.cves, nil
}

func (f *fakeSource) ByID(_ context.Context, _, _ string) (CVE, error) {
	f.called = true
	if f.err != nil {
		return CVE{}, f.err
	}
	return f.byID, nil
}

func TestAutoFallsBackOnRateLimit(t *testing.T) {
	nvd := &fakeSource{name: "nvd", err: ErrRateLimited}
	circl := &fakeSource{name: "circl", cves: []CVE{{ID: "CVE-2024-1111"}}}
	auto := &autoSource{nvd: nvd, circl: circl}

	cves, err := auto.ForVersion(context.Background(), "7.4.3")
	if err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	if !nvd.called || !circl.called {
		t.Errorf("both sources should be tried; nvd=%v circl=%v", nvd.called, circl.called)
	}
	if len(cves) != 1 || cves[0].ID != "CVE-2024-1111" {
		t.Fatalf("want CIRCL result, got %+v", cves)
	}
	if auto.Name() != "circl" || !auto.fellBack {
		t.Errorf("auto should report circl fallback, got name=%s fellBack=%v", auto.Name(), auto.fellBack)
	}
	if SourceNote(auto) != "circl (fallback)" {
		t.Errorf("SourceNote = %q, want 'circl (fallback)'", SourceNote(auto))
	}
}

func TestAutoFallsBackOnUnavailable(t *testing.T) {
	nvd := &fakeSource{name: "nvd", err: ErrUnavailable}
	circl := &fakeSource{name: "circl", byID: CVE{ID: "CVE-2024-1111", Affected: true}}
	auto := &autoSource{nvd: nvd, circl: circl}

	got, err := auto.ByID(context.Background(), "CVE-2024-1111", "7.4.3")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if !got.Affected || !auto.fellBack {
		t.Errorf("expected CIRCL fallback answer, got %+v fellBack=%v", got, auto.fellBack)
	}
}

func TestAutoNoFallbackOnNormalError(t *testing.T) {
	// A non-retryable error (e.g. not-found) from NVD must NOT trigger fallback.
	nvd := &fakeSource{name: "nvd", err: errors.New("cve: CVE-9999-0000 not found in NVD")}
	circl := &fakeSource{name: "circl"}
	auto := &autoSource{nvd: nvd, circl: circl}

	if _, err := auto.ByID(context.Background(), "CVE-9999-0000", "7.4.3"); err == nil {
		t.Fatal("expected the NVD not-found error to propagate")
	}
	if circl.called {
		t.Error("CIRCL must not be queried on a non-retryable NVD error")
	}
}

func TestAutoNVDSuccessNoFallback(t *testing.T) {
	nvd := &fakeSource{name: "nvd", cves: []CVE{{ID: "CVE-2024-1111"}}}
	circl := &fakeSource{name: "circl"}
	auto := &autoSource{nvd: nvd, circl: circl}

	if _, err := auto.ForVersion(context.Background(), "7.4.3"); err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	if circl.called {
		t.Error("CIRCL must not be queried when NVD succeeds")
	}
	if auto.fellBack || SourceNote(auto) != "nvd" {
		t.Errorf("SourceNote = %q, want 'nvd'", SourceNote(auto))
	}
}

func TestNewSourceModes(t *testing.T) {
	if _, err := New("bogus", ""); err == nil {
		t.Error("unknown mode should error")
	}
	for _, m := range []string{"", "auto", "nvd", "circl"} {
		if _, err := New(m, ""); err != nil {
			t.Errorf("New(%q): %v", m, err)
		}
	}
}
