package cli

import "github.com/spf13/cobra"

// newVpnCertificateCmd exposes the installed certificate store read-only. The
// cmdb objects carry secret material (private-key, passwords), so every read
// resource redacts it before rendering (table AND json/yaml). Writes are NOT
// generic cmdb create/set: importing a cert/key is a monitor upload
// (POST monitor/vpn-certificate/<type>/import) with key material — out of scope
// for the declarative framework and reachable via `fgt api` until curated.
func newVpnCertificateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "certificate",
		Aliases: []string{"cert"},
		Short:   "Inspect the installed certificate store (read-only)",
		Long: "List and show installed certificates (local device certs/CSRs, CA,\n" +
			"remote, CRL) and the certificate settings. Secret fields (private keys,\n" +
			"passwords) are redacted. Import is a privileged monitor upload — use the\n" +
			"FortiOS GUI/CLI or `fgt api POST monitor/vpn-certificate/local/import`.",
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

	cmd.AddCommand(
		a.newResourceCmd(local),
		a.newResourceCmd(ca),
		a.newResourceCmd(remote),
		a.newResourceCmd(crl),
		a.newResourceCmd(setting),
	)
	return cmd
}
