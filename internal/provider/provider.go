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

// Object is a single cmdb object as returned by the API (untyped, so any object
// on any FortiOS version round-trips without a per-object Go struct).
type Object = map[string]any

// CertImport describes a certificate upload to a vpn-certificate store.
type CertImport struct {
	Store    string // "local" | "ca" | "remote" | "crl"
	Type     string // "regular" (PEM cert+key) | "pkcs12" — local only
	Name     string // certname — local only
	Scope    string // "vdom" | "global"
	Cert     []byte // the PEM certificate, or PKCS12 bytes
	Key      []byte // the PEM private key (local regular)
	Password string // PKCS12 / encrypted-key passphrase
}

// Provider is the backend contract consumed by the CLI.
type Provider interface {
	Name() string

	// ListInterfaces returns system interfaces (monitor surface).
	ListInterfaces(ctx context.Context) ([]domain.Interface, error)
	// ListManagedSwitches returns FortiLink-managed FortiSwitch units.
	ListManagedSwitches(ctx context.Context) ([]domain.ManagedSwitch, error)
	// DeviceStatus returns device identity + FortiOS version (monitor surface).
	DeviceStatus(ctx context.Context) (domain.DeviceStatus, error)
	// HAStatus returns the raw HA cluster member records (monitor surface). The
	// shape is left untyped so it renders faithfully across FortiOS builds
	// without baking in per-version field names.
	HAStatus(ctx context.Context) ([]Object, error)
	// SSLSessions returns the raw active SSL-VPN session records (monitor
	// surface), left untyped for the same reason as HAStatus.
	SSLSessions(ctx context.Context) ([]Object, error)

	// Generic cmdb CRUD. path is the cmdb-relative object path, e.g.
	// "firewall/address" or "firewall.service/custom". These back the curated
	// resource commands and work for any object on any FortiOS version.
	CmdbList(ctx context.Context, path string) ([]Object, error)
	CmdbGet(ctx context.Context, path, mkey string) (Object, error)
	// CmdbCreate POSTs obj and returns the created object's mkey (which FortiOS
	// may have auto-assigned).
	CmdbCreate(ctx context.Context, path string, obj Object) (string, error)
	// CmdbUpdate PUTs obj to path/mkey; a "" mkey targets a singleton object.
	CmdbUpdate(ctx context.Context, path, mkey string, obj Object) error
	CmdbDelete(ctx context.Context, path, mkey string) error

	// ConfigBackup returns the device configuration as a text file (monitor
	// surface). scope is "global" or "vdom"; vdom names the VDOM for vdom scope.
	ConfigBackup(ctx context.Context, scope, vdom string) ([]byte, error)
	// ConfigRestore uploads a configuration to the device. This REPLACES the
	// running config and typically reboots the unit. scope/vdom as above.
	ConfigRestore(ctx context.Context, scope, vdom string, config []byte) error
	// ImportCertificate uploads a certificate to the vpn-certificate store via the
	// monitor import endpoint. This is a privileged, mutating upload of key
	// material (POST monitor/vpn-certificate/<store>/import).
	ImportCertificate(ctx context.Context, req CertImport) error
	// Schema returns a cmdb object's field schema for the target build
	// (GET cmdb/<path>?action=schema) — the authoritative, per-version field set
	// FortiOS describes about itself. Backs `fgt schema`.
	Schema(ctx context.Context, path string) (Object, error)
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
	case "session":
		sp, err := auth.NewSession(s.User, s.Secret)
		if err != nil {
			return nil, err
		}
		ap = sp
	default:
		return nil, fmt.Errorf("unsupported auth type %q", s.AuthType)
	}

	return transport.New(transport.Options{
		BaseURL:   s.Server,
		Auth:      ap,
		TLS:       tlsCfg,
		VDOM:      s.VDOM,
		Global:    s.Global,
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
