// Package config models the fortigate-cli configuration file (profiles +
// contexts, kubeconfig-style) and resolves effective settings from the
// documented precedence: explicit flag > env var > context > profile > built-in.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// File is the on-disk configuration.
type File struct {
	CurrentContext string             `yaml:"current_context,omitempty"`
	Contexts       map[string]Context `yaml:"contexts,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
}

// Context binds a name to a profile and optional per-use overrides.
type Context struct {
	Profile string `yaml:"profile"`
	VDOM    string `yaml:"vdom,omitempty"`
}

// Profile is a connection target (one FortiGate, optionally a default VDOM).
type Profile struct {
	Server    string         `yaml:"server"`
	VDOM      string         `yaml:"vdom,omitempty"`
	Auth      AuthConfig     `yaml:"auth"`
	TLS       TLSConfig      `yaml:"tls,omitempty"`
	RateLimit RateLimit      `yaml:"rate_limit,omitempty"`
	Defaults  ProfileDefault `yaml:"defaults,omitempty"`
}

// RateLimit configures client-side request throttling.
type RateLimit struct {
	QPS   float64 `yaml:"qps,omitempty"`
	Burst int     `yaml:"burst,omitempty"`
}

// AuthConfig describes credentials for a profile.
type AuthConfig struct {
	Type string `yaml:"type"` // "token" (M1) | "session" (later)
	// Name is an optional label for a token (not sent to the device).
	Name string `yaml:"name,omitempty"`
	// User is the admin account for session auth (later phase).
	User string `yaml:"user,omitempty"`
	// SecretRef is the preferred way to hold the token/password:
	// keyring://service/key or env:NAME.
	SecretRef string `yaml:"secret_ref,omitempty"`
	// Secret is discouraged in plain text; resolve() warns if present.
	Secret string `yaml:"secret,omitempty"`
}

// TLSConfig mirrors transport.TLSConfig in serializable form.
type TLSConfig struct {
	CAFile      string `yaml:"ca_file,omitempty"`
	Fingerprint string `yaml:"fingerprint,omitempty"`
	Verify      *bool  `yaml:"verify,omitempty"`
}

// ProfileDefault holds per-profile defaults.
type ProfileDefault struct {
	Output string `yaml:"output,omitempty"`
}

// DefaultPath returns the config file path, honoring FGT_CLI_CONFIG and
// XDG_CONFIG_HOME.
func DefaultPath() string {
	if p := os.Getenv("FGT_CLI_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "fortigate-cli-config.yaml"
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "fortigate-cli", "config.yaml")
}

// Load reads the config file at path. A missing file yields an empty File and
// no error, so the CLI works purely from env vars/flags.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &f, nil
}

// Save writes the config file to path, creating parent directories. The
// directory is created 0700 because it holds the credential file; the file
// itself is 0600.
func Save(path string, f *File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
