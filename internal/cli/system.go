package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/version"
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
	// REST-API admin + global/per-VDOM settings (system_admin.go) and automation
	// stitches (system_automation.go).
	cmd.AddCommand(systemAdminCommands(a)...)
	cmd.AddCommand(systemAutomationCommands(a)...)
	// config backup/restore (system_backup.go).
	cmd.AddCommand(newBackupCmd(a), newRestoreCmd(a))
	cmd.AddCommand(newStatusCmd(a))
	// physical health surface (system_health.go): optics, sensors, roll-up.
	cmd.AddCommand(newTransceiverCmd(a), newSensorCmd(a), newHealthCmd(a))
	return cmd
}

// newStatusCmd shows device identity + FortiOS version (monitor surface) and
// notes whether the detected version is in fgt's supported matrix.
func newStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show device identity + FortiOS version (monitor surface)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			st, err := p.DeviceStatus(cmd.Context())
			if err != nil {
				return err
			}
			supported, mm := version.SupportsVersion(st.Version)
			supNote := "yes"
			if !supported {
				supNote = "no (untested — fgt targets " + version.SupportedFortiOS + ")"
				fmt.Fprintf(cmd.ErrOrStderr(),
					"[fgt] warning: FortiOS %s (%s series) is outside the tested matrix (%s); commands should still work via the api escape hatch\n",
					st.Version, mm, version.SupportedFortiOS)
			}
			t := output.Tabular{Columns: []string{"FIELD", "VALUE"}, Raw: st, Rows: [][]string{
				{"hostname", st.Hostname},
				{"model", st.Model},
				{"serial", st.Serial},
				{"version", st.Version},
				{"build", strconv.Itoa(st.Build)},
				{"supported", supNote},
			}}
			return a.render(t)
		},
	}
}

// newInterfaceCmd exposes system interfaces. `list` merges the cmdb config
// inventory (the authoritative full set, including logical VLANs/tunnels/zones)
// with the monitor surface's live status/IP overlay; show/create/set/delete
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

// interfaceListCmd lists the full interface inventory: the cmdb config set merged
// with the monitor live-status overlay, so a VDOM whose interfaces are all
// logical (VLANs/tunnels/zones, absent from the monitor surface) still lists.
// ADMIN is the configured admin state (cmdb); LINK is the live link state
// (monitor) — either may be blank when that surface does not know the interface.
func interfaceListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List interfaces (cmdb config inventory + live monitor status)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			ifaces, err := p.ListInterfacesFull(cmd.Context())
			if err != nil {
				return err
			}
			t := output.Tabular{
				Columns: []string{"NAME", "TYPE", "ADMIN", "LINK", "SPEED", "DUPLEX", "IP", "VDOM", "ALIAS"},
				Raw:     ifaces,
			}
			for _, i := range ifaces {
				t.Rows = append(t.Rows, []string{i.Name, i.Type, i.AdminStatus, i.Status, i.Speed, i.Duplex, i.IP, i.VDOM, i.Alias})
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
	ha.AddCommand(a.resShow(r), a.resSet(r), haStatusCmd(a), haCheckCmd(a))
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
