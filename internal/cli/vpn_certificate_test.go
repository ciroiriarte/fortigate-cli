package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCertLocalImportReadsFiles(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "c.pem")
	keyFile := filepath.Join(dir, "k.pem")
	if err := os.WriteFile(certFile, []byte("CERTDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("KEYDATA"), 0o600); err != nil {
		t.Fatal(err)
	}

	tp := &testProvider{}
	cmd := certLocalImportCmd(&app{prov: tp, assumeYes: true})
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("cert", certFile)
	_ = cmd.Flags().Set("key", keyFile)
	_ = cmd.Flags().Set("scope", "vdom")
	if err := cmd.RunE(cmd, []string{"web"}); err != nil {
		t.Fatalf("local import: %v", err)
	}
	req := tp.lastCertImport
	if req == nil {
		t.Fatal("ImportCertificate not called")
	}
	if req.Store != "local" || req.Type != "regular" || req.Name != "web" || req.Scope != "vdom" {
		t.Errorf("import req = %+v", req)
	}
	// The command reads raw file bytes; the provider base64-encodes them.
	if string(req.Cert) != "CERTDATA" || string(req.Key) != "KEYDATA" {
		t.Errorf("cert/key bytes = %q / %q", req.Cert, req.Key)
	}

	// No --cert and no --pkcs12 => a clear error, no import attempted.
	tp2 := &testProvider{}
	c2 := certLocalImportCmd(&app{prov: tp2, assumeYes: true})
	c2.SetOut(io.Discard)
	if err := c2.RunE(c2, []string{"web"}); err == nil {
		t.Error("expected error when neither --cert nor --pkcs12 given")
	}
	if tp2.lastCertImport != nil {
		t.Error("no import should be attempted without input")
	}
}

func TestResolvePassphrase(t *testing.T) {
	dir := t.TempDir()
	pf := filepath.Join(dir, "pw")
	if err := os.WriteFile(pf, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := resolvePassphrase(pf, ""); got != "s3cret" {
		t.Errorf("password-file = %q, want s3cret (trailing newline trimmed)", got)
	}
	t.Setenv("CERT_PW", "envpw")
	if got, _ := resolvePassphrase("", "CERT_PW"); got != "envpw" {
		t.Errorf("password-env = %q, want envpw", got)
	}
	if got, _ := resolvePassphrase("", ""); got != "" {
		t.Errorf("no source => empty, got %q", got)
	}
}
