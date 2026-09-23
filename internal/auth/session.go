package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// SessionProvider authenticates with an admin username/password via FortiOS's
// /logincheck endpoint, so password admins work where no REST-API token is
// provisioned. It shares the transport's cookie jar (carrying the session
// cookie) and injects the X-CSRFTOKEN header on mutating requests.
type SessionProvider struct {
	User     string
	Password string

	mu       sync.Mutex
	hc       *http.Client
	base     *url.URL
	csrf     string
	loggedIn bool
}

// NewSession builds a SessionProvider, validating inputs.
func NewSession(user, password string) (*SessionProvider, error) {
	if user == "" {
		return nil, fmt.Errorf("session auth: username is empty")
	}
	if password == "" {
		return nil, fmt.Errorf("session auth: password is empty")
	}
	return &SessionProvider{User: user, Password: password}, nil
}

// ClientBinder lets the transport hand an auth provider the HTTP client (with
// the shared cookie jar) and base URL it needs to perform its own round-trips.
// Token auth does not implement it; session auth does.
type ClientBinder interface {
	Bind(hc *http.Client, base *url.URL)
}

// Bind wires the transport's client + base URL so login and API calls share one
// session (one cookie jar). The transport calls this once after construction.
func (s *SessionProvider) Bind(hc *http.Client, base *url.URL) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hc, s.base = hc, base
}

// Apply ensures a session exists and, on writes, sets the CSRF header. Session
// cookies flow automatically through the shared jar, so reads need nothing else.
func (s *SessionProvider) Apply(req *http.Request, write bool) error {
	if err := s.ensureLogin(req.Context()); err != nil {
		return err
	}
	if write {
		s.mu.Lock()
		csrf := s.csrf
		s.mu.Unlock()
		if csrf != "" {
			req.Header.Set("X-CSRFTOKEN", csrf)
		}
	}
	return nil
}

// Refresh forces a re-login (e.g. after the session expires).
func (s *SessionProvider) Refresh(ctx context.Context) error {
	s.mu.Lock()
	s.loggedIn = false
	s.mu.Unlock()
	return s.ensureLogin(ctx)
}

// Kind names the mechanism for diagnostics.
func (s *SessionProvider) Kind() string { return "session" }

func (s *SessionProvider) ensureLogin(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loggedIn {
		return nil
	}
	if s.hc == nil || s.base == nil {
		return fmt.Errorf("session auth: transport not bound")
	}

	form := url.Values{"username": {s.User}, "secretkey": {s.Password}, "ajax": {"1"}}
	loginURL := *s.base
	loginURL.Path = "/logincheck"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.hc.Do(req)
	if err != nil {
		return fmt.Errorf("session login: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	// The authoritative success signal is the ccsrftoken cookie FortiOS sets on a
	// valid login; a failed login returns the same 200 but no session cookie.
	csrf := s.extractCSRF()
	if csrf == "" {
		return fmt.Errorf("session login failed for user %q (check username/password and admin trusted hosts); server said: %s",
			s.User, strings.TrimSpace(string(body)))
	}
	s.csrf = csrf
	s.loggedIn = true
	return nil
}

// extractCSRF returns the ccsrftoken cookie value the jar stored for base,
// stripped of the quotes FortiOS wraps it in. The cookie name may carry a
// port/vdom suffix, so match by prefix.
func (s *SessionProvider) extractCSRF() string {
	if s.hc.Jar == nil {
		return ""
	}
	for _, c := range s.hc.Jar.Cookies(s.base) {
		if strings.HasPrefix(strings.ToLower(c.Name), "ccsrftoken") && c.Value != "" {
			return strings.Trim(c.Value, "\"")
		}
	}
	return ""
}
