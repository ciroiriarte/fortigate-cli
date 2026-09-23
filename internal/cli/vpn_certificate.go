package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// newVpnCertificateCmd exposes the certificate store: read (list/show) is
// generic cmdb with secret material (private-key, passwords) redacted before
// rendering (table AND json/yaml); import is a separate privileged monitor
// upload (POST monitor/vpn-certificate/<store>/import), not a generic cmdb write.
func newVpnCertificateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "certificate",
		Aliases: []string{"cert"},
		Short:   "Manage the certificate store (read + import)",
		Long: "List/show installed certificates (local device certs/CSRs, CA, remote,\n" +
			"CRL) and settings — secret fields (private keys, passwords) are redacted —\n" +
			"and import certificates via the privileged monitor upload endpoint.",
	}

	// local device certs carry the private key + enrollment passwords — redact.
	secrets := []string{"private-key", "password", "scep-password", "est-http-password", "est-srp-password"}
	local := resource{
		use: "local", readOnly: true, short: "Local device certificates and CSRs",
		path: "vpn.certificate/local", mkey: "name", redact: secrets,
		columns: []column{{field: "name"}, {field: "source"}, {field: "range"}, {field: "state"}, {field: "comments"}},
	}
	ca := resource{
		use: "ca", readOnly: true, short: "Trusted CA certificates",
		path: "vpn.certificate/ca", mkey: "name",
		columns: []column{{field: "name"}, {field: "source"}, {field: "range"}, {field: "comments"}},
	}
	remote := resource{
		use: "remote", readOnly: true, short: "Remote (OCSP) certificates",
		path: "vpn.certificate/remote", mkey: "name",
		columns: []column{{field: "name"}, {field: "range"}, {field: "comments"}},
	}
	crl := resource{
		use: "crl", readOnly: true, short: "Certificate revocation lists",
		path: "vpn.certificate/crl", mkey: "name",
		columns: []column{{field: "name"}, {field: "range"}, {field: "comments"}},
	}
	setting := resource{
		use: "setting", single: true, short: "Certificate verification settings",
		path: "vpn.certificate/setting",
		fields: []fieldSpec{
			{name: "ocsp-status", usage: "enable|disable"},
			{name: "ocsp-default-server", usage: "default OCSP server ref"},
			{name: "strict-crl-check", usage: "enable|disable"},
			{name: "ssl-min-proto-version", usage: "default|SSLv3|TLSv1|TLSv1-1|TLSv1-2"},
			{name: "cert-expire-warning", usage: "days before expiry to warn"},
		},
	}

	// Attach the privileged monitor import commands to each store.
	localCmd := a.newResourceCmd(local)
	localCmd.AddCommand(certLocalImportCmd(a))
	caCmd := a.newResourceCmd(ca)
	caCmd.AddCommand(certFileImportCmd(a, "ca", "CA certificate", "global"))
	remoteCmd := a.newResourceCmd(remote)
	remoteCmd.AddCommand(certFileImportCmd(a, "remote", "remote (OCSP) certificate", "global"))
	crlCmd := a.newResourceCmd(crl)
	crlCmd.AddCommand(certFileImportCmd(a, "crl", "certificate revocation list", "global"))

	cmd.AddCommand(localCmd, caCmd, remoteCmd, crlCmd, a.newResourceCmd(setting))
	return cmd
}

// certLocalImportCmd imports a local (device) certificate — a PEM cert+key pair
// or a PKCS12/PFX bundle. Secret material comes from files/stdin/env, never a
// flag value. This is a privileged, mutating upload (confirmation-gated).
func certLocalImportCmd(a *app) *cobra.Command {
	var certFile, keyFile, p12File, pwFile, pwEnv, scope string
	cmd := &cobra.Command{
		Use:   "import <name>",
		Short: "Import a local certificate (PEM cert+key or PKCS12) — privileged upload",
		Long: "Import a device certificate. Provide either --cert (PEM, or - for stdin)\n" +
			"plus --key (PEM), or --pkcs12 (a .p12/.pfx bundle) with its passphrase via\n" +
			"--password-file/--password-env. Secrets are never passed as flag values.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			req := provider.CertImport{Store: "local", Name: args[0], Scope: scope}
			switch {
			case p12File != "":
				if req.Cert, err = os.ReadFile(p12File); err != nil {
					return err
				}
				req.Type = "pkcs12"
				if req.Password, err = resolvePassphrase(pwFile, pwEnv); err != nil {
					return err
				}
			case certFile != "":
				if req.Cert, err = readCertInput(cmd, certFile); err != nil {
					return err
				}
				req.Type = "regular"
				if keyFile != "" {
					if req.Key, err = readCertInput(cmd, keyFile); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("provide --cert (with optional --key) or --pkcs12")
			}
			if err := confirmWrite(a, "IMPORT", "vpn.certificate/local/"+req.Name); err != nil {
				return err
			}
			if err := p.ImportCertificate(cmd.Context(), req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported local certificate %s\n", req.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&certFile, "cert", "", "PEM certificate file (or - for stdin)")
	cmd.Flags().StringVar(&keyFile, "key", "", "PEM private-key file")
	cmd.Flags().StringVar(&p12File, "pkcs12", "", "PKCS12/PFX bundle file")
	cmd.Flags().StringVar(&pwFile, "password-file", "", "file holding the PKCS12/key passphrase")
	cmd.Flags().StringVar(&pwEnv, "password-env", "", "env var holding the PKCS12/key passphrase")
	cmd.Flags().StringVar(&scope, "scope", "vdom", "import scope: vdom|global")
	cmd.MarkFlagsMutuallyExclusive("cert", "pkcs12")
	cmd.MarkFlagsMutuallyExclusive("key", "pkcs12")
	return cmd
}

// certFileImportCmd imports a single-file certificate (CA / remote / CRL).
func certFileImportCmd(a *app, store, label, defScope string) *cobra.Command {
	var certFile, scope string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a " + label + " (PEM) — privileged upload",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			cert, err := readCertInput(cmd, certFile)
			if err != nil {
				return err
			}
			if len(cert) == 0 {
				return fmt.Errorf("empty certificate; pass --cert <file>")
			}
			if err := confirmWrite(a, "IMPORT", "vpn.certificate/"+store); err != nil {
				return err
			}
			if err := p.ImportCertificate(cmd.Context(), provider.CertImport{Store: store, Scope: scope, Cert: cert}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %s\n", label)
			return nil
		},
	}
	cmd.Flags().StringVar(&certFile, "cert", "", "PEM certificate file (or - for stdin)")
	cmd.Flags().StringVar(&scope, "scope", defScope, "import scope: vdom|global")
	return cmd
}

// readCertInput reads a cert/key from a file, or from stdin when path is "-".
func readCertInput(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("no input file given (use --cert; - for stdin)")
	}
	if path == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	return os.ReadFile(path)
}

// resolvePassphrase reads a passphrase from a file or env var (never a flag).
func resolvePassphrase(file, env string) (string, error) {
	switch {
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	case env != "":
		return os.Getenv(env), nil
	default:
		return "", nil
	}
}
