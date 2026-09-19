// Package auth provides pluggable authentication for the FortiOS API.
//
// M1 ships API-token auth (Authorization: Bearer <token>), which is
// non-interactive and automation-safe. Session auth (POST /logincheck, cookie +
// X-CSRFTOKEN) is a later phase; the Provider interface below already
// accommodates it so the transport never has to change.
package auth

import (
	"context"
	"fmt"
	"net/http"
)

// Provider injects credentials into outgoing requests and can refresh them.
type Provider interface {
	// Apply adds auth to the request. write indicates a mutating method, which
	// matters for session-based CSRF handling (a no-op for tokens).
	Apply(req *http.Request, write bool) error
	// Refresh renews short-lived credentials. No-op for API tokens.
	Refresh(ctx context.Context) error
	// Kind names the auth mechanism for diagnostics.
	Kind() string
}

// TokenProvider authenticates with a FortiOS REST API token. FortiOS tokens are
// opaque secrets tied to a REST-API admin; the token id/name is only a label.
type TokenProvider struct {
	// Name is an optional human label for the token (not sent).
	Name string
	// Secret is the API token value.
	Secret string
}

// NewToken builds a TokenProvider, validating the inputs.
func NewToken(name, secret string) (*TokenProvider, error) {
	if secret == "" {
		return nil, fmt.Errorf("api token is empty")
	}
	return &TokenProvider{Name: name, Secret: secret}, nil
}

// Apply sets the Bearer Authorization header. FortiOS enforces HTTPS for token
// auth; the transport rejects plaintext to non-loopback hosts before we get here.
func (t *TokenProvider) Apply(req *http.Request, _ bool) error {
	req.Header.Set("Authorization", "Bearer "+t.Secret)
	return nil
}

// Refresh is a no-op for tokens.
func (t *TokenProvider) Refresh(context.Context) error { return nil }

// Kind returns the mechanism name.
func (t *TokenProvider) Kind() string { return "token" }
