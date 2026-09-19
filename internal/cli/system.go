package cli

import (
	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

func newSystemCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "system",
		Aliases: []string{"sys"},
		Short:   "Inspect system-level configuration and status",
	}
	cmd.AddCommand(newInterfaceCmd(a))
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
