package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zalando/go-keyring"
)

// Settings is the fully-resolved connection configuration the rest of the CLI
// consumes. It is produced by Resolve from file + env + flag inputs.
type Settings struct {
	Server      string
	VDOM        string
	Global      bool // target the global scope instead of a VDOM
	AuthType    string
	TokenName   string
	User        string
	Secret      string
	Output      string
	TLSCAFile   string
	TLSFinger   string
	TLSInsecure bool
	RateQPS     float64
	RateBurst   int
	ProfileName string
	ContextName string
	// NVDAPIKey is an optional NIST NVD API key used by `fgt cve` for higher
	// rate limits. It targets the external NVD service, not the FortiGate, so it
	// is read from env only (NVD_API_KEY or FGT_CLI_NVD_API_KEY).
	NVDAPIKey string
}

// Overrides carries explicit flag values (highest precedence). Empty string /
// nil means "not set", so the next precedence tier applies.
type Overrides struct {
	Profile     string
	Context     string
	Server      string
	VDOM        string
	TokenSecret string
	User        string
	Password    string
	Global      *bool
	Output      string
	Insecure    *bool
	Fingerprint string
}

// Resolve computes effective Settings using precedence:
// flag > env > context-selected profile > profile defaults > built-in.
func Resolve(f *File, ov Overrides) (*Settings, error) {
	s := &Settings{Output: "table", AuthType: "token"}

	// 1. Pick context (flag > env > file.current_context).
	ctxName := firstNonEmpty(ov.Context, os.Getenv("FGT_CLI_CONTEXT"), f.CurrentContext)
	s.ContextName = ctxName

	// 2. Determine profile (flag > env > context's profile).
	profName := firstNonEmpty(ov.Profile, os.Getenv("FGT_CLI_PROFILE"))
	if profName == "" && ctxName != "" {
		if c, ok := f.Contexts[ctxName]; ok {
			profName = c.Profile
			s.VDOM = c.VDOM
		}
	}
	s.ProfileName = profName

	// 3. Layer in the profile from file, if any.
	if profName != "" {
		prof, ok := f.Profiles[profName]
		if !ok {
			return nil, fmt.Errorf("profile %q not found in config", profName)
		}
		s.Server = prof.Server
		if prof.VDOM != "" {
			s.VDOM = prof.VDOM
		}
		s.AuthType = orDefault(prof.Auth.Type, "token")
		s.TokenName = prof.Auth.Name
		s.User = prof.Auth.User
		s.TLSCAFile = prof.TLS.CAFile
		s.TLSFinger = prof.TLS.Fingerprint
		if prof.TLS.Verify != nil {
			s.TLSInsecure = !*prof.TLS.Verify
		}
		if prof.Defaults.Output != "" {
			s.Output = prof.Defaults.Output
		}
		s.RateQPS = prof.RateLimit.QPS
		s.RateBurst = prof.RateLimit.Burst
		sec, err := resolveSecret(prof.Auth)
		if err != nil {
			return nil, err
		}
		s.Secret = sec
	}

	// 4. Env vars override profile values.
	s.Server = firstNonEmpty(os.Getenv("FGT_CLI_SERVER"), s.Server)
	s.VDOM = firstNonEmpty(os.Getenv("FGT_CLI_VDOM"), s.VDOM)
	if v := os.Getenv("FGT_CLI_GLOBAL"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			s.Global = b
		}
	}
	s.User = firstNonEmpty(os.Getenv("FGT_CLI_USER"), s.User)
	// Each secret source is gated on the auth type so a stray FGT_CLI_PASSWORD
	// can't silently overwrite a token (or vice-versa).
	if v := os.Getenv("FGT_CLI_TOKEN"); v != "" && s.AuthType != "session" {
		s.Secret = v
	}
	if v := os.Getenv("FGT_CLI_PASSWORD"); v != "" && s.AuthType == "session" {
		s.Secret = v
	}
	s.TLSFinger = firstNonEmpty(os.Getenv("FGT_CLI_TLS_FINGERPRINT"), s.TLSFinger)
	s.Output = firstNonEmpty(os.Getenv("FGT_CLI_OUTPUT"), s.Output)
	// NVD API key for `fgt cve`: the issue asks for NVD_API_KEY; also honor the
	// FGT_CLI_ prefix for consistency. The unprefixed name wins when both are set.
	s.NVDAPIKey = firstNonEmpty(os.Getenv("NVD_API_KEY"), os.Getenv("FGT_CLI_NVD_API_KEY"), s.NVDAPIKey)
	if v := os.Getenv("FGT_CLI_INSECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			s.TLSInsecure = b
		}
	}

	// 5. Explicit flags override everything.
	s.Server = firstNonEmpty(ov.Server, s.Server)
	s.VDOM = firstNonEmpty(ov.VDOM, s.VDOM)
	if ov.Global != nil {
		s.Global = *ov.Global
	}
	// An explicit --vdom flag wins over an env/profile-implied global scope
	// (honors the documented flag > env precedence).
	if ov.VDOM != "" {
		s.Global = false
	}
	// Global scope and a VDOM are mutually exclusive; when global wins, clear any
	// inherited VDOM so requests carry ?global=1, not ?vdom=.
	if s.Global {
		s.VDOM = ""
	}
	s.User = firstNonEmpty(ov.User, s.User)
	if ov.TokenSecret != "" && s.AuthType != "session" {
		s.Secret = ov.TokenSecret
		fmt.Fprintln(os.Stderr, "[fgt] warning: --token exposes the secret in ps/shell history; prefer a secret_ref or FGT_CLI_TOKEN")
	}
	if ov.Password != "" && s.AuthType == "session" {
		s.Secret = ov.Password
		fmt.Fprintln(os.Stderr, "[fgt] warning: --password exposes the secret in ps/shell history; prefer a secret_ref or FGT_CLI_PASSWORD")
	}
	s.TLSFinger = firstNonEmpty(ov.Fingerprint, s.TLSFinger)
	s.Output = firstNonEmpty(ov.Output, s.Output)
	if ov.Insecure != nil {
		s.TLSInsecure = *ov.Insecure
	}

	if s.AuthType == "" {
		s.AuthType = "token"
	}
	return s, nil
}

// Validate checks that the resolved settings are usable for a connection.
func (s *Settings) Validate() error {
	if s.Server == "" {
		return fmt.Errorf("no server configured: set a profile, --server, or FGT_CLI_SERVER")
	}
	switch s.AuthType {
	case "token":
		if s.Secret == "" {
			return fmt.Errorf("token auth requires an API token: set auth.secret_ref, FGT_CLI_TOKEN, or --token")
		}
	case "session":
		if s.User == "" {
			return fmt.Errorf("session auth requires an admin username: set auth.user, FGT_CLI_USER, or --user")
		}
		if s.Secret == "" {
			return fmt.Errorf("session auth requires a password: set auth.secret_ref, FGT_CLI_PASSWORD, or --password")
		}
	default:
		return fmt.Errorf("unknown auth type %q (want token or session)", s.AuthType)
	}
	return nil
}

// ResolveSecretRef dereferences keyring://service/key or env:NAME into plaintext.
func ResolveSecretRef(ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "keyring://"):
		rest := strings.TrimPrefix(ref, "keyring://")
		service, key, ok := strings.Cut(rest, "/")
		if !ok {
			return "", fmt.Errorf("invalid keyring ref %q: want keyring://service/key", ref)
		}
		v, err := keyring.Get(service, key)
		if err != nil {
			return "", fmt.Errorf("read secret from keyring (%s): %w", ref, err)
		}
		return v, nil
	case strings.HasPrefix(ref, "env:"):
		return os.Getenv(strings.TrimPrefix(ref, "env:")), nil
	default:
		return "", fmt.Errorf("unsupported secret ref scheme: %q (want keyring://service/key or env:NAME)", ref)
	}
}

func resolveSecret(a AuthConfig) (string, error) {
	if a.SecretRef != "" {
		return ResolveSecretRef(a.SecretRef)
	}
	if a.Secret != "" {
		fmt.Fprintln(os.Stderr, "[fgt] warning: plaintext secret in config; prefer secret_ref: keyring://… or an env var")
	}
	return a.Secret, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
