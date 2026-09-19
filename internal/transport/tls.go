package transport

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// TLSConfig configures how the client trusts the FortiGate's certificate.
// FortiGate management certs are self-signed by default, so pinning the SHA-256
// fingerprint is the recommended middle ground between full CA verification and
// the --insecure footgun.
type TLSConfig struct {
	CAFile      string // optional PEM bundle to trust
	Fingerprint string // pinned server cert SHA-256 (hex, colons optional)
	Insecure    bool   // skip verification entirely (footgun)
}

// build turns a TLSConfig into a *tls.Config.
func (t TLSConfig) build() (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if t.Fingerprint != "" {
		want := normalizeFingerprint(t.Fingerprint)
		// Pinning supersedes chain verification: verify the leaf ourselves.
		cfg.InsecureSkipVerify = true
		cfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("tls: server presented no certificate")
			}
			sum := sha256.Sum256(rawCerts[0])
			got := hex.EncodeToString(sum[:])
			if !strings.EqualFold(got, want) {
				return fmt.Errorf("tls: server fingerprint %s does not match pinned %s", got, want)
			}
			return nil
		}
		return cfg, nil
	}

	if t.Insecure {
		cfg.InsecureSkipVerify = true
		return cfg, nil
	}

	if t.CAFile != "" {
		pem, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tls: read ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("tls: no certificates found in %s", t.CAFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// normalizeFingerprint lowercases and strips colons/whitespace so "AA:BB.." and
// "aabb.." compare equal.
func normalizeFingerprint(fp string) string {
	r := strings.NewReplacer(":", "", " ", "", "\t", "")
	return strings.ToLower(r.Replace(fp))
}
