// Package provider abstracts the backend. Today the only backend is a FortiGate
// spoken to directly over its REST API; FortiManager (which fronts a fleet of
// FortiGates and FortiSwitches via JSON-RPC) is a natural second provider, so
// the CLI layer depends only on this interface and on domain types, never on
// raw HTTP.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/ciroiriarte/fortigate-cli/internal/auth"
	"github.com/ciroiriarte/fortigate-cli/internal/config"
	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/transport"
)

// ErrUnsupported is returned for operations a backend does not support.
var ErrUnsupported = errors.New("operation not supported by this provider")

// Provider is the backend contract consumed by the CLI.
type Provider interface {
	Name() string

	// ListInterfaces returns system interfaces (monitor surface).
	ListInterfaces(ctx context.Context) ([]domain.Interface, error)
	// ListFirewallAddresses returns firewall/address objects (cmdb surface).
	ListFirewallAddresses(ctx context.Context) ([]domain.FirewallAddress, error)
	// ListManagedSwitches returns FortiLink-managed FortiSwitch units.
	ListManagedSwitches(ctx context.Context) ([]domain.ManagedSwitch, error)

	// Raw issues an arbitrary API call (backs `fgt api`). path is relative to
	// /api/v2/, e.g. "cmdb/firewall/address".
	Raw(ctx context.Context, method, path string, params url.Values, body []byte) ([]byte, error)
}

// NewClient builds the raw transport client (auth + TLS + rate limit) from
// resolved settings.
func NewClient(s *config.Settings, debug bool) (*transport.Client, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	tlsCfg := transport.TLSConfig{
		CAFile:      s.TLSCAFile,
		Fingerprint: s.TLSFinger,
		Insecure:    s.TLSInsecure,
	}

	var ap auth.Provider
	switch s.AuthType {
	case "token":
		t, err := auth.NewToken(s.TokenName, s.Secret)
		if err != nil {
			return nil, err
		}
		ap = t
	default:
		return nil, fmt.Errorf("unsupported auth type %q", s.AuthType)
	}

	return transport.New(transport.Options{
		BaseURL:   s.Server,
		Auth:      ap,
		TLS:       tlsCfg,
		VDOM:      s.VDOM,
		Debug:     debug,
		UserAgent: "fortigate-cli",
		RateQPS:   s.RateQPS,
		Burst:     s.RateBurst,
	})
}

// New builds a Provider from resolved settings.
func New(s *config.Settings, debug bool) (Provider, error) {
	cl, err := NewClient(s, debug)
	if err != nil {
		return nil, err
	}
	if fortigateConstructor == nil {
		return nil, fmt.Errorf("fortigate provider not registered (missing import)")
	}
	return fortigateConstructor(cl), nil
}

// The backend constructor is registered by the fortigate subpackage via init()
// to avoid an import cycle while keeping New as the single entry point.
var fortigateConstructor func(*transport.Client) Provider

// SetFortiGateFactory registers the FortiGate provider constructor.
func SetFortiGateFactory(f func(*transport.Client) Provider) { fortigateConstructor = f }
