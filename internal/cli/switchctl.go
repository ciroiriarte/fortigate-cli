package cli

import (
	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

// newSwitchCmd exposes FortiLink-managed FortiSwitch units. They are
// administered through the FortiGate's switch-controller, never contacted
// directly, so this lives under the FortiGate CLI rather than a separate tool.
func newSwitchCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "switch",
		Aliases: []string{"sw", "fortiswitch"},
		Short:   "Manage FortiLink-managed FortiSwitch units",
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List managed FortiSwitch units and their status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			switches, err := p.ListManagedSwitches(cmd.Context())
			if err != nil {
				return err
			}
			t := output.Tabular{
				Columns: []string{"SERIAL", "NAME", "MODEL", "STATUS", "STATE", "FIRMWARE"},
				Raw:     switches,
			}
			for _, s := range switches {
				t.Rows = append(t.Rows, []string{s.Serial, s.Name, s.Model, s.Status, s.State, s.Firmware})
			}
			return a.render(t)
		},
	}
	cmd.AddCommand(list)
	return cmd
}
