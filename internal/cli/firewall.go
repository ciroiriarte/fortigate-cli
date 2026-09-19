package cli

import (
	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

func newFirewallCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "firewall",
		Aliases: []string{"fw"},
		Short:   "Manage firewall configuration objects (cmdb surface)",
	}
	cmd.AddCommand(newAddressCmd(a))
	return cmd
}

func newAddressCmd(a *app) *cobra.Command {
	addr := &cobra.Command{
		Use:     "address",
		Aliases: []string{"addr"},
		Short:   "Manage firewall address objects",
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List firewall address objects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			addrs, err := p.ListFirewallAddresses(cmd.Context())
			if err != nil {
				return err
			}
			t := output.Tabular{
				Columns: []string{"NAME", "TYPE", "SUBNET", "FQDN", "COMMENT"},
				Raw:     addrs,
			}
			for _, x := range addrs {
				t.Rows = append(t.Rows, []string{x.Name, x.Type, x.Subnet, x.FQDN, x.Comment})
			}
			return a.render(t)
		},
	}
	addr.AddCommand(list)
	return addr
}
