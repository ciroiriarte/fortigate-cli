package cli

import "github.com/spf13/cobra"

// systemServiceCommands returns the curated system service subcommands
// (ntp, dhcp server, snmp sysinfo/community/user). newSystemCmd wires these in.
func systemServiceCommands(a *app) []*cobra.Command {
	ntp := resource{
		use: "ntp", short: "Manage NTP settings", single: true,
		path: "system/ntp",
		fields: []fieldSpec{
			{name: "type", usage: "fortiguard|custom"},
			{name: "ntpsync", usage: "enable|disable"},
			{name: "syncinterval", usage: "minutes between syncs"},
			{name: "server-mode", usage: "enable|disable (act as NTP server)"},
			{name: "source-ip", usage: "source IP"},
		},
	}

	server := resource{
		use: "server", short: "Manage DHCP servers",
		path: "system.dhcp/server", mkey: "id", mkeyArg: "id", numeric: true,
		columns: []column{
			{field: "id"},
			{field: "interface"},
			{field: "default-gateway", header: "GATEWAY"},
			{field: "netmask"},
			{field: "status"},
		},
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "interface", usage: "interface this scope serves"},
			{name: "default-gateway", usage: "gateway handed to clients"},
			{name: "netmask", usage: "subnet mask"},
			{name: "dns-service", usage: "local|default|specify"},
			{name: "dns-server1", usage: "DNS handed to clients"},
			{name: "dns-server2", usage: "secondary DNS handed to clients"},
			{name: "domain", usage: "DHCP domain"},
			{name: "lease-time", usage: "seconds"},
		},
	}
	dhcp := &cobra.Command{Use: "dhcp", Short: "Manage DHCP configuration"}
	dhcp.AddCommand(a.newResourceCmd(server))

	sysinfo := resource{
		use: "sysinfo", short: "Manage SNMP system information", single: true,
		path: "system.snmp/sysinfo",
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "description", usage: "system description"},
			{name: "contact-info", usage: "contact"},
			{name: "location", usage: "physical location"},
			{name: "trap-high-cpu-threshold", usage: "percent"},
		},
	}

	community := resource{
		use: "community", short: "Manage SNMP communities",
		path: "system.snmp/community", mkey: "id", mkeyArg: "id", numeric: true,
		columns: []column{
			{field: "id"},
			{field: "name"},
			{field: "status"},
		},
		fields: []fieldSpec{
			{name: "name", usage: "community name"},
			{name: "status", usage: "enable|disable"},
			{name: "query-v1-status", usage: "enable|disable"},
			{name: "query-v2c-status", usage: "enable|disable"},
			{name: "trap-v1-status", usage: "enable|disable"},
			{name: "trap-v2c-status", usage: "enable|disable"},
			{name: "events", usage: "SNMP trap events"},
		},
	}

	user := resource{
		use: "user", short: "Manage SNMP users",
		path: "system.snmp/user", mkey: "name",
		columns: []column{
			{field: "name"},
			{field: "security-level", header: "SEC-LEVEL"},
			{field: "status"},
		},
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "security-level", usage: "no-auth-no-priv|auth-no-priv|auth-priv"},
			{name: "auth-proto", usage: "md5|sha|sha224|sha256|sha384|sha512"},
			{name: "auth-pwd", usage: "auth password"},
			{name: "priv-proto", usage: "aes|des|aes256|aes256cisco"},
			{name: "priv-pwd", usage: "privacy password"},
			{name: "queries", usage: "enable|disable"},
			{name: "notify-hosts", usage: "trap destination IP(s)"},
		},
	}
	snmp := &cobra.Command{Use: "snmp", Short: "Manage SNMP configuration"}
	snmp.AddCommand(
		a.newResourceCmd(sysinfo),
		a.newResourceCmd(community),
		a.newResourceCmd(user),
	)

	centralMgmt := resource{
		use: "central-management", aliases: []string{"central-mgmt"}, short: "Manage central management (FortiManager/FortiGuard)", single: true,
		path: "system/central-management",
		fields: []fieldSpec{
			{name: "type", usage: "fortimanager|fortiguard|none"},
			{name: "fmg", usage: "FortiManager IP or serial"},
			{name: "fmg-source-ip", usage: "source IP for FortiManager traffic"},
			{name: "mode", usage: "normal|backup"},
			{name: "serial-number", usage: "FortiManager serial"},
			{name: "allow-monitor", usage: "enable|disable"},
			{name: "allow-push-configuration", usage: "enable|disable"},
			{name: "include-default-servers", usage: "enable|disable"},
		},
	}

	fortiguard := resource{
		use: "fortiguard", short: "Manage FortiGuard settings", single: true,
		path: "system/fortiguard",
		fields: []fieldSpec{
			{name: "protocol", usage: "udp|http|https"},
			{name: "port", usage: "53|8888|443"},
			{name: "service-account-id", usage: "FortiGuard service account"},
			{name: "auto-firmware-upgrade", usage: "enable|disable"},
			{name: "update-server-location", usage: "automatic|usa|eu"},
			{name: "fortiguard-anycast", usage: "enable|disable"},
			{name: "sdns-server-ip", usage: "FortiGuard DNS server IP(s)"},
		},
	}

	zone := resource{
		use: "zone", short: "Manage interface zones",
		path: "system/zone", mkey: "name",
		columns: []column{{field: "name"}, {field: "interface"}, {field: "intrazone"}},
		fields: []fieldSpec{
			{name: "interface", kind: kindIfaceList, usage: "member interface(s), comma-separated"},
			{name: "intrazone", usage: "allow|deny (intra-zone traffic)"},
			{name: "description", usage: "free-text description"},
		},
	}

	return []*cobra.Command{
		a.newResourceCmd(ntp), dhcp, snmp,
		a.newResourceCmd(centralMgmt),
		a.newResourceCmd(fortiguard),
		a.newResourceCmd(zone),
	}
}
