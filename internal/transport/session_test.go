package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/auth"
)

// TestSessionAuthFlow exercises the end-to-end session flow through the real
// transport: a read triggers one /logincheck, the session cookie rides
// subsequent requests via the shared jar, the session is reused (no re-login),
// and a write carries the unquoted X-CSRFTOKEN header.
func TestSessionAuthFlow(t *testing.T) {
	var loginHits int
	var sawCookieOnAPI bool
	var csrfOnWrite string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logincheck" {
			loginHits++
			http.SetCookie(w, &http.Cookie{Name: "APSCOOKIE_1", Value: "sess", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "ccsrftoken", Value: `"TOK123"`, Path: "/"})
			w.Write([]byte("1"))
			return
		}
		if c, err := r.Cookie("APSCOOKIE_1"); err == nil && c.Value == "sess" {
			sawCookieOnAPI = true
		}
		if r.Method != http.MethodGet {
			csrfOnWrite = r.Header.Get("X-CSRFTOKEN")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":{},"status":"success","http_status":200}`))
	}))
	defer srv.Close()

	sp, err := auth.NewSession("admin", "pw")
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(Options{BaseURL: srv.URL, Auth: sp})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.DoRaw(context.Background(), &Request{Method: "GET", Path: "cmdb/firewall/address"}); err != nil {
		t.Fatalf("GET: %v", err)
	}
	if loginHits != 1 {
		t.Errorf("login hits = %d, want 1", loginHits)
	}
	if !sawCookieOnAPI {
		t.Error("API request did not carry the session cookie")
	}

	// second read reuses the session — no re-login
	if _, err := c.DoRaw(context.Background(), &Request{Method: "GET", Path: "cmdb/firewall/address"}); err != nil {
		t.Fatal(err)
	}
	if loginHits != 1 {
		t.Errorf("re-login happened: hits = %d, want 1", loginHits)
	}

	// a write carries the CSRF token (unquoted)
	if _, err := c.DoRaw(context.Background(), &Request{Method: "POST", Path: "cmdb/firewall/address", Body: []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	if csrfOnWrite != "TOK123" {
		t.Errorf("X-CSRFTOKEN = %q, want TOK123 (unquoted)", csrfOnWrite)
	}
}

// TestGlobalScope asserts a Global client sends ?global=1 (overriding the
// client-default VDOM), and that a per-request VDOM still overrides global.
func TestGlobalScope(t *testing.T) {
	var q string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.RawQuery
		w.Write([]byte(`{"results":{},"status":"success"}`))
	}))
	defer srv.Close()

	c, err := New(Options{BaseURL: srv.URL, VDOM: "root", Global: true})
	if err != nil {
		t.Fatal(err)
	}
	// global client + client-default VDOM => global=1, no vdom.
	if _, err := c.DoRaw(context.Background(), &Request{Method: "GET", Path: "cmdb/system/global"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "global=1") || strings.Contains(q, "vdom=") {
		t.Errorf("global-scope query = %q, want global=1 and no vdom", q)
	}
	// a per-request VDOM overrides global (documented Request.VDOM contract).
	if _, err := c.DoRaw(context.Background(), &Request{Method: "GET", Path: "cmdb/x", VDOM: "prod"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "vdom=prod") || strings.Contains(q, "global=") {
		t.Errorf("per-request vdom query = %q, want vdom=prod and no global", q)
	}
}

// TestSessionLoginFailureHitsLogincheckOnce is the admin-lockout guard: a bad
// password on an idempotent GET (which the transport would otherwise retry up to
// 4×) must POST /logincheck exactly once and fail terminally — never hammer the
// login endpoint into a lockout.
func TestSessionLoginFailureHitsLogincheckOnce(t *testing.T) {
	var loginHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logincheck" {
			loginHits++
		}
		w.Write([]byte("0")) // FortiOS failure code, no session cookie set
	}))
	defer srv.Close()

	sp, _ := auth.NewSession("admin", "wrong")
	c, _ := New(Options{BaseURL: srv.URL, Auth: sp})
	_, err := c.DoRaw(context.Background(), &Request{Method: "GET", Path: "cmdb/firewall/address"})
	if err == nil || !strings.Contains(err.Error(), "session login failed") {
		t.Fatalf("want session-login-failed error, got %v", err)
	}
	if loginHits != 1 {
		t.Errorf("/logincheck hit %d times on a failed GET, want exactly 1 (admin-lockout hazard)", loginHits)
	}
}
