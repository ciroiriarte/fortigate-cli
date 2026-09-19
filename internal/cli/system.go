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

	cmd.AddCommand(
		newInterfaceCmd(a),
		a.newResourceCmd(admin),
		a.newResourceCmd(dns),
	)
	return cmd
}

func newInterfaceCmd(a *app) *cobra.Command {
	iface := &cobra.Command{
		Use:     "interface",
		Aliases: []string{"if", "intf"},
		Short:   "Manage/inspect system interfaces",
	}
	list := &cobra.Command{
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
	iface.AddCommand(list)
	return iface
}
