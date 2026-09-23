package cli

import "github.com/spf13/cobra"

// userAccountCommands returns the curated user account resources (local, group).
// newUserCmd wires these into the `user` group.
func userAccountCommands(a *app) []*cobra.Command {
	local := resource{
		use: "local", short: "Manage local user accounts",
		path: "user/local", mkey: "name",
		columns: []column{{field: "name"}, {field: "type"}, {field: "status"},
			{field: "two-factor", header: "2FA"}},
		fields: []fieldSpec{
			{name: "type", usage: "password|radius|tacacs+|ldap"},
			{name: "passwd", usage: "local password (type password)"},
			{name: "ldap-server", usage: "LDAP server reference (type ldap)"},
			{name: "radius-server", usage: "RADIUS server reference (type radius)"},
			{name: "tacacs+-server", flag: "tacacs-server", usage: "TACACS+ server reference (type tacacs+)"},
			{name: "status", usage: "enable|disable"},
			{name: "two-factor", usage: "disable|fortitoken|email|sms"},
			{name: "email-to", usage: "email address for 2FA/notifications"},
		},
	}

	group := resource{
		use: "group", short: "Manage user groups",
		path: "user/group", mkey: "name",
		columns: []column{{field: "name"}, {field: "group-type", header: "TYPE"}, {field: "member"}},
		fields: []fieldSpec{
			{name: "group-type", usage: "firewall|fsso-service|rsso|guest"},
			{name: "member", kind: kindRefList, usage: "comma-separated local users and/or server references"},
			{name: "authtimeout", usage: "per-group authentication timeout (minutes)"},
		},
	}

	return []*cobra.Command{a.newResourceCmd(local), a.newResourceCmd(group)}
}
