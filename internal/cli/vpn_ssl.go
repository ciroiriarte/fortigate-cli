package cli

import "github.com/spf13/cobra"

// newVpnSSLCmd builds SSL-VPN resources over the dotted vpn.ssl and vpn.ssl.web categories.
func newVpnSSLCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "ssl", Short: "Manage SSL-VPN configuration (cmdb surface)"}

	settings := resource{
		use: "settings", short: "Manage SSL-VPN settings", single: true,
		path: "vpn.ssl/settings",
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "port", usage: "listen port (default 443)"},
			{name: "servercert", usage: "server certificate name"},
			{name: "default-portal", usage: "default portal name"},
			{name: "reqclientcert", usage: "enable|disable"},
			{name: "idle-timeout", usage: "seconds"},
			{name: "auth-timeout", usage: "seconds"},
			{name: "login-attempt-limit", usage: "max failed logins"},
			{name: "login-block-time", usage: "lockout seconds"},
			{name: "dns-server1", usage: "pushed DNS"},
			{name: "dns-server2", usage: "pushed DNS"},
			{name: "ssl-min-proto-ver", usage: "tls1-0|tls1-1|tls1-2|tls1-3"},
		},
	}

	authenticationRule := resource{
		use: "authentication-rule", aliases: []string{"auth-rule"}, short: "Manage SSL-VPN authentication rules",
		path: "vpn.ssl/settings/authentication-rule", mkey: "id", mkeyArg: "id", numeric: true,
		columns: []column{{field: "id"}, {field: "portal"}, {field: "groups"}, {field: "users"}, {field: "source-interface"}},
		fields: []fieldSpec{
			{name: "portal", usage: "assigned portal"},
			{name: "groups", kind: kindRefList, usage: "user group(s)"},
			{name: "users", kind: kindRefList, usage: "local user(s)"},
			{name: "source-interface", kind: kindRefList, usage: "ingress interface(s)"},
			{name: "source-address", kind: kindRefList, usage: "source address object(s)"},
			{name: "realm", usage: "realm"},
			{name: "client-cert", usage: "enable|disable"},
			{name: "cipher", usage: "any|high|medium"},
		},
	}

	portal := resource{
		use: "portal", short: "Manage SSL-VPN web portals",
		path: "vpn.ssl.web/portal", mkey: "name",
		columns: []column{{field: "name"}, {field: "tunnel-mode"}, {field: "web-mode"}, {field: "split-tunneling"}},
		fields: []fieldSpec{
			{name: "tunnel-mode", usage: "enable|disable"},
			{name: "web-mode", usage: "enable|disable"},
			{name: "split-tunneling", usage: "enable|disable"},
			{name: "ip-pools", kind: kindRefList, usage: "tunnel IP pool(s)"},
			{name: "dns-server1", usage: "pushed DNS"},
			{name: "dns-suffix", usage: "pushed search suffix"},
			{name: "save-password", usage: "enable|disable"},
			{name: "limit-user-logins", usage: "enable|disable"},
			{name: "host-check", usage: "none|av|fw|av-fw|custom"},
		},
	}

	cmd.AddCommand(a.newResourceCmd(settings), a.newResourceCmd(authenticationRule), a.newResourceCmd(portal))
	return cmd
}
