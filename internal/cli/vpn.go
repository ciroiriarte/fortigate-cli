package cli

// vpn.go assembles the `vpn` command group: the IPsec subtree (dotted
// vpn.ipsec cmdb paths) and the SSL-VPN group (curated config in vpn_ssl.go
// plus a `sessions` monitor read).

import "github.com/spf13/cobra"

// newVpnCmd builds the `vpn` command group.
func newVpnCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vpn",
		Short: "Manage VPN configuration (cmdb surface)",
	}

	phase1 := resource{
		use: "phase1-interface", short: "Manage IPsec phase1 interface tunnels",
		path: "vpn.ipsec/phase1-interface", mkey: "name",
		columns: []column{{field: "name"}, {field: "interface"},
			{field: "ike-version", header: "IKE"}, {field: "remote-gw", header: "REMOTE-GW"}, {field: "proposal"}},
		fields: []fieldSpec{
			{name: "type", usage: "static|dynamic|ddns"},
			{name: "interface", usage: "local egress interface"},
			{name: "ike-version", usage: "1|2"},
			{name: "mode", usage: "aggressive|main (IKEv1)"},
			{name: "peertype", usage: "any|one|dialup|peer|peergrp"},
			{name: "authmethod", usage: "psk|signature"},
			{name: "psksecret", usage: "pre-shared key"},
			{name: "proposal", usage: "phase1 proposals, e.g. aes256gcm-prfsha384"},
			{name: "dhgrp", usage: "DH groups, e.g. 14 21"},
			{name: "remote-gw", usage: "remote gateway IPv4 (static type)"},
			{name: "nattraversal", usage: "enable|disable|forced"},
			{name: "net-device", usage: "enable|disable"},
			{name: "dpd", usage: "disable|on-idle|on-demand"},
			{name: "comments", usage: "free-text comments"},
		},
	}

	phase2 := resource{
		use: "phase2-interface", short: "Manage IPsec phase2 interface selectors",
		path: "vpn.ipsec/phase2-interface", mkey: "name",
		columns: []column{{field: "name"}, {field: "phase1name", header: "PHASE1"},
			{field: "src-subnet", header: "SRC"}, {field: "dst-subnet", header: "DST"}, {field: "pfs"}},
		fields: []fieldSpec{
			{name: "phase1name", usage: "parent phase1-interface tunnel"},
			{name: "proposal", usage: "phase2 proposals"},
			{name: "src-subnet", usage: "proxy-ID local subnet <ip>/<mask>"},
			{name: "dst-subnet", usage: "proxy-ID remote subnet <ip>/<mask>"},
			{name: "pfs", usage: "enable|disable"},
			{name: "dhgrp", usage: "PFS DH groups"},
			{name: "keylifeseconds", usage: "120-172800"},
			{name: "auto-negotiate", usage: "enable|disable"},
			{name: "add-route", usage: "phase1|enable|disable"},
			{name: "comments", usage: "free-text comments"},
		},
	}

	ipsec := &cobra.Command{Use: "ipsec", Short: "Manage IPsec VPN configuration"}
	ipsec.AddCommand(a.newResourceCmd(phase1), a.newResourceCmd(phase2))

	// SSL-VPN: curated config (vpn_ssl.go) plus a monitor read of active sessions.
	ssl := newVpnSSLCmd(a)
	ssl.AddCommand(sslSessionsCmd(a))

	cmd.AddCommand(ipsec, ssl)
	return cmd
}

// sslSessionsCmd lists active SSL-VPN sessions (monitor surface).
func sslSessionsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List active SSL-VPN sessions (monitor surface)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			sessions, err := p.SSLSessions(cmd.Context())
			if err != nil {
				return err
			}
			return a.render(objectsTable(sessions))
		},
	}
}
