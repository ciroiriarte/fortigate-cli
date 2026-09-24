// Package transport is the base HTTPS client for the FortiOS REST API. It owns
// auth, TLS, retries and rate limiting; higher layers speak in terms of the
// provider interface and domain types, never raw HTTP.
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/ciroiriarte/fortigate-cli/internal/auth"
	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
)

// apiPrefix is prepended to every request path. Callers pass FortiOS-relative
// paths such as "cmdb/firewall/address" or "monitor/system/interface".
const apiPrefix = "/api/v2/"

// Options configures a Client.
type Options struct {
	BaseURL    string // e.g. https://fw.example.com or https://host:8443
	Auth       auth.Provider
	TLS        TLSConfig
	VDOM       string        // default vdom applied to every request; "" = device default
	Global     bool          // target the global scope (?global=1) instead of a vdom
	Timeout    time.Duration // per-request timeout (default 30s)
	MaxRetries int           // retries for idempotent requests (default 3)
	Debug      bool          // log request/response metadata to stderr
	UserAgent  string
	RateQPS    float64 // client-side rate limit in requests/sec; <=0 disables
	Burst      int     // token-bucket burst; defaults to max(1, ceil(RateQPS))
}

// Client is the base HTTP client. It is safe for concurrent use.
type Client struct {
	base       *url.URL
	hc         *http.Client
	auth       auth.Provider
	vdom       string
	global     bool
	maxRetries int
	debug      bool
	userAgent  string
	limiter    *rate.Limiter
}

// VDOM returns the client's configured default VDOM ("" = device default,
// effectively "root" unless Global is set). It lets a provider replicate the
// request scoping for a global config object that FortiOS does not filter by the
// ?vdom= param (e.g. system/interface), without changing transport behavior.
func (c *Client) VDOM() string { return c.vdom }

// Global reports whether the client targets the global scope instead of a VDOM.
func (c *Client) Global() bool { return c.global }

// Request is a single API call.
type Request struct {
	Method string
	// Path is relative to /api/v2/, e.g. "cmdb/firewall/address".
	Path  string
	Query url.Values
	// Body, when set on a write method, is sent as the JSON request body.
	Body []byte
	// VDOM overrides the client default for this request ("" keeps the default).
	VDOM string
}

// New builds a Client from Options.
func New(opt Options) (*Client, error) {
	if opt.BaseURL == "" {
		return nil, fmt.Errorf("transport: BaseURL is required")
	}
	u, err := url.Parse(strings.TrimRight(opt.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("transport: invalid BaseURL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("transport: BaseURL must be an absolute https URL, got %q", opt.BaseURL)
	}
	if err := EnsureSecureURL(u); err != nil {
		return nil, err
	}
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	tlsConf, err := opt.TLS.build()
	if err != nil {
		return nil, err
	}
	// A cookie jar carries the FortiOS session cookie across requests; harmless
	// for token auth, required for session auth (login + API calls share it).
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	hc := &http.Client{Timeout: timeout, Jar: jar, Transport: &http.Transport{TLSClientConfig: tlsConf}}
	// Session auth needs to make its own /logincheck round-trip through this same
	// client (and jar); hand it the client + base URL.
	if b, ok := opt.Auth.(auth.ClientBinder); ok {
		b.Bind(hc, u)
	}
	retries := opt.MaxRetries
	if retries == 0 {
		retries = 3
	}
	ua := opt.UserAgent
	if ua == "" {
		ua = "fortigate-cli"
	}
	return &Client{
		base:       u,
		hc:         hc,
		auth:       opt.Auth,
		vdom:       opt.VDOM,
		global:     opt.Global,
		maxRetries: retries,
		debug:      opt.Debug,
		userAgent:  ua,
		limiter:    newLimiter(opt.RateQPS, opt.Burst),
	}, nil
}

func newLimiter(qps float64, burst int) *rate.Limiter {
	if qps <= 0 {
		return nil
	}
	if burst < 1 {
		burst = int(math.Ceil(qps))
		if burst < 1 {
			burst = 1
		}
	}
	return rate.NewLimiter(rate.Limit(qps), burst)
}

// Do executes req, decoding the response envelope's data into out (may be nil).
func (c *Client) Do(ctx context.Context, req *Request, out any) error {
	body, err := c.doRaw(ctx, req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return protocol.DecodeData(body, out)
}

// DoRaw executes req and returns the raw response body (used by `fgt api`).
func (c *Client) DoRaw(ctx context.Context, req *Request) ([]byte, error) {
	return c.doRaw(ctx, req)
}

func (c *Client) doRaw(ctx context.Context, req *Request) ([]byte, error) {
	idempotent := isIdempotent(req.Method)
	attempts := 1
	if idempotent {
		attempts = c.maxRetries + 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}

		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, &protocol.APIError{Kind: protocol.KindTransport, Message: err.Error()}
			}
		}

		body, status, transErr := c.attempt(ctx, req)
		if transErr != nil {
			// Authentication failures are terminal: retrying re-runs the auth
			// step (for session auth, another /logincheck), which risks admin
			// lockout. Everything else is a genuine transport hiccup — retry.
			var ae *authApplyError
			if errors.As(transErr, &ae) {
				return nil, &protocol.APIError{Kind: protocol.KindTransport, Message: ae.Error()}
			}
			lastErr = &protocol.APIError{Kind: protocol.KindTransport, Message: transErr.Error()}
			continue // transport errors are always retryable for idempotent reqs
		}
		if status >= 200 && status < 300 {
			return body, nil
		}
		apiErr := protocol.DecodeError(status, body)
		if idempotent && retryableStatus(status) {
			lastErr = apiErr
			continue
		}
		return nil, apiErr
	}
	return nil, lastErr
}

// attempt performs a single HTTP round-trip.
func (c *Client) attempt(ctx context.Context, req *Request) (body []byte, status int, err error) {
	u := *c.base
	u.Path = apiPrefix + strings.TrimLeft(req.Path, "/")

	query := url.Values{}
	for k, vs := range req.Query {
		query[k] = append(query[k], vs...)
	}
	// Scope the request: an explicit per-request vdom/global in Query wins;
	// otherwise the client default global scope (?global=1) or vdom (?vdom=).
	if query.Get("vdom") == "" && query.Get("global") == "" {
		switch {
		case req.VDOM != "":
			// A per-request VDOM overrides the client default, global included.
			query.Set("vdom", req.VDOM)
		case c.global:
			query.Set("global", "1")
		case c.vdom != "":
			query.Set("vdom", c.vdom)
		}
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var reqBody io.Reader
	if len(req.Body) > 0 {
		reqBody = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), reqBody)
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.userAgent)
	if len(req.Body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if c.auth != nil {
		if err := c.auth.Apply(httpReq, !isIdempotent(req.Method)); err != nil {
			return nil, 0, &authApplyError{err: err}
		}
	}

	if c.debug {
		fmt.Fprintf(stderr, "[fgt] %s %s\n", httpReq.Method, u.String())
	}

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if c.debug {
		fmt.Fprintf(stderr, "[fgt] -> %d (%d bytes)\n", resp.StatusCode, len(b))
	}
	return b, resp.StatusCode, nil
}

// EnsureSecureURL rejects sending credentials over plaintext http to a
// non-loopback host — the API token would travel in the clear (and FortiOS
// enforces HTTPS for token auth anyway). Loopback http stays allowed for SSH
// tunnels and the test suite.
func EnsureSecureURL(u *url.URL) error {
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf("transport: refusing to send credentials over %q to %q — use https (loopback http is allowed for tunnels/tests)", u.Scheme, u.Hostname())
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// authApplyError wraps a failure from auth.Provider.Apply so the retry loop can
// treat it as terminal rather than a retryable transport error.
type authApplyError struct{ err error }

func (e *authApplyError) Error() string { return "apply auth: " + e.err.Error() }
func (e *authApplyError) Unwrap() error { return e.err }

func isIdempotent(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	default:
		return false
	}
}

// backoff returns an exponential delay with jitter for the given attempt (>=1).
func backoff(attempt int) time.Duration {
	base := time.Duration(math.Pow(2, float64(attempt-1))) * 200 * time.Millisecond
	if base > 5*time.Second {
		base = 5 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(base/2 + 1)))
	return base + jitter
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
