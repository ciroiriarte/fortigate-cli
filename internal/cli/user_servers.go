package cli

import "github.com/spf13/cobra"

// userServerCommands returns the curated remote-auth server resources.
func userServerCommands(a *app) []*cobra.Command {
	ldap := resource{
		use: "ldap", short: "Manage LDAP authentication servers",
		path: "user/ldap", mkey: "name",
		columns: []column{{field: "name"}, {field: "server"}, {field: "dn"}, {field: "type"}, {field: "secure"}},
		fields: []fieldSpec{
			{name: "server", usage: "primary LDAP server"},
			{name: "secondary-server", usage: "failover LDAP server"},
			{name: "cnid", usage: "common-name identifier (e.g. cn, sAMAccountName)"},
			{name: "dn", usage: "base distinguished name"},
			{name: "type", usage: "simple|anonymous|regular"},
			{name: "username", usage: "bind DN (type regular)"},
			{name: "password", usage: "bind password"},
			{name: "port", usage: "LDAP port (default 389)"},
			{name: "secure", usage: "disable|starttls|ldaps"},
			{name: "ca-cert", usage: "CA cert for ldaps/starttls"},
			{name: "group-search-base", usage: "base DN for group lookups"},
		},
	}

	radius := resource{
		use: "radius", short: "Manage RADIUS authentication servers",
		path: "user/radius", mkey: "name",
		columns: []column{{field: "name"}, {field: "server"}, {field: "auth-type"}},
		fields: []fieldSpec{
			{name: "server", usage: "primary RADIUS server"},
			{name: "secondary-server", usage: "failover RADIUS server"},
			{name: "secret", usage: "shared secret"},
			{name: "auth-type", usage: "auto|ms_chap_v2|ms_chap|chap|pap"},
			{name: "nas-ip", usage: "NAS IP / called-station-id"},
			{name: "timeout", usage: "seconds"},
		},
	}

	tacacs := resource{
		use: "tacacs", aliases: []string{"tacacs+"}, short: "Manage TACACS+ authentication servers",
		path: "user/tacacs+", mkey: "name",
		columns: []column{{field: "name"}, {field: "server"}, {field: "authen-type"}},
		fields: []fieldSpec{
			{name: "server", usage: "primary TACACS+ server"},
			{name: "secondary-server", usage: "failover TACACS+ server"},
			{name: "key", usage: "shared key"},
			{name: "port", usage: "TACACS+ port (default 49)"},
			{name: "authorization", usage: "enable|disable"},
			{name: "authen-type", usage: "auto|ascii|pap|chap|mschap"},
		},
	}

	return []*cobra.Command{a.newResourceCmd(ldap), a.newResourceCmd(radius), a.newResourceCmd(tacacs)}
}
