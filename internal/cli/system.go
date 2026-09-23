package cli

import (
	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

func newSystemCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "system",
		Aliases: []string{"sys"},
		Short:   "Inspect and manage system-level configuration and status",
	}

	admin := resource{
		use: "admin", short: "Manage administrator accounts",
		path: "system/admin", mkey: "name",
		columns: []column{{field: "name"}, {field: "accprofile"},
			{field: "trusthost1", header: "TRUSTHOST1"}, {field: "two-factor", header: "2FA"}},
		fields: []fieldSpec{
			{name: "accprofile", usage: "access profile (e.g. super_admin)"},
			{name: "password", usage: "admin password"},
			{name: "trusthost1", usage: `"<ip> <mask>" source restriction`},
			{name: "two-factor", usage: "disable|fortitoken|email|sms"},
			{name: "comments", usage: "free-text comment"},
		},
	}

	dns := resource{
		use: "dns", short: "Manage DNS settings", single: true,
		path: "system/dns",
		fields: []fieldSpec{
			{name: "primary", usage: "primary DNS server IPv4"},
			{name: "secondary", usage: "secondary DNS server IPv4"},
			{name: "protocol", usage: "cleartext|dot|doh"},
			{name: "dns-over-tls", usage: "disable|enable|enforce"},
		},
	}

	vdom := resource{
		use: "vdom", short: "Manage virtual domains (VDOMs)",
		path: "system/vdom", mkey: "name",
		columns: []column{{field: "name"}, {field: "short-name", header: "SHORT-NAME"},
			{field: "vcluster-id", header: "VCLUSTER"}},
		fields: []fieldSpec{
			{name: "short-name", usage: "abbreviated VDOM name"},
			{name: "vcluster-id", usage: "virtual cluster ID (HA)"},
			{name: "temporary", usage: "temporary VDOM flag"},
		},
	}

	cmd.AddCommand(
		newInterfaceCmd(a),
		a.newResourceCmd(admin),
		a.newResourceCmd(dns),
		a.newResourceCmd(vdom),
		newHACmd(a),
	)
	// ntp/dhcp/snmp live in system_services.go to keep this file focused.
	cmd.AddCommand(systemServiceCommands(a)...)
	// config backup/restore (system_backup.go).
	cmd.AddCommand(newBackupCmd(a), newRestoreCmd(a))
	return cmd
}

// newInterfaceCmd exposes system interfaces. `list` reads the monitor surface for
// live status/IP the cmdb object alone does not carry; show/create/set/delete
// operate on the cmdb config object. Aggregate `member` and other child-tables
// are reachable via --set / the api escape hatch.
func newInterfaceCmd(a *app) *cobra.Command {
	iface := &cobra.Command{
		Use:     "interface",
		Aliases: []string{"if", "intf"},
		Short:   "Manage/inspect system interfaces",
	}
	r := resource{
		use:  "interface",
		path: "system/interface", mkey: "name",
		fields: []fieldSpec{
			{name: "type", usage: "physical|vlan|aggregate|redundant|loopback|tunnel|..."},
			{name: "mode", usage: "static|dhcp|pppoe (IPv4 addressing)"},
			{name: "ip", usage: `"<ip> <mask>" or <ip>/<pfx>`},
			{name: "allowaccess", usage: `space list, e.g. "ping https ssh"`},
			{name: "role", usage: "lan|wan|dmz|undefined"},
			{name: "status", usage: "up|down (admin status)"},
			{name: "alias", usage: "short alias"},
			{name: "description", usage: "free-text description"},
			{name: "vlanid", usage: "VLAN id 1-4094 (type vlan)"},
			{name: "interface", usage: "parent interface (VLAN/aggregate)"},
			{name: "vdom", usage: "owning VDOM"},
			{name: "mtu", usage: "MTU (with --set mtu-override=enable)"},
		},
	}
	iface.AddCommand(
		interfaceListCmd(a),
		a.resShow(r),
		a.resCreate(r),
		a.resSet(r),
		a.resDelete(r),
	)
	return iface
}

// interfaceListCmd lists interfaces with live status from the monitor surface.
func interfaceListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List interfaces with live status (monitor surface)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			ifaces, err := p.ListInterfaces(cmd.Context())
			if err != nil {
				return err
			}
			t := output.Tabular{
				Columns: []string{"NAME", "TYPE", "IP", "STATUS", "VDOM", "ALIAS"},
				Raw:     ifaces,
			}
			for _, i := range ifaces {
				t.Rows = append(t.Rows, []string{i.Name, i.Type, i.IP, i.Status, i.VDOM, i.Alias})
			}
			return a.render(t)
		},
	}
}

// newHACmd manages HA: the cmdb `system/ha` config singleton (show/set) plus a
// `status` read of the cluster members from the monitor surface.
func newHACmd(a *app) *cobra.Command {
	ha := &cobra.Command{
		Use:   "ha",
		Short: "Manage HA (High Availability) clustering",
	}
	r := resource{
		use: "ha", single: true,
		path: "system/ha",
		fields: []fieldSpec{
			{name: "mode", usage: "standalone|a-a|a-p"},
			{name: "group-id", usage: "cluster group id 0-1023"},
			{name: "group-name", usage: "cluster group name"},
			{name: "password", usage: "cluster password"},
			{name: "hbdev", usage: `heartbeat interfaces, e.g. "port3 50"`},
			{name: "priority", usage: "device priority (higher wins)"},
			{name: "override", usage: "enable|disable"},
			{name: "session-pickup", usage: "enable|disable"},
		},
	}
	ha.AddCommand(a.resShow(r), a.resSet(r), haStatusCmd(a))
	return ha
}

// haStatusCmd reports the HA cluster members and their live utilization.
func haStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show HA cluster members and status (monitor surface)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			members, err := p.HAStatus(cmd.Context())
			if err != nil {
				return err
			}
			return a.render(objectsTable(members))
		},
	}
}
