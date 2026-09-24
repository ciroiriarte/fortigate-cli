package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

// newNetworkCmd is the top-level `network` group for L2/L3 neighbor-discovery and
// related monitor surfaces. It currently exposes the LLDP neighbor table.
func newNetworkCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "network",
		Aliases: []string{"net"},
		Short:   "Inspect network discovery surfaces (LLDP neighbors)",
	}
	cmd.AddCommand(newLLDPCmd(a))
	return cmd
}

// newLLDPCmd groups LLDP subcommands.
func newLLDPCmd(a *app) *cobra.Command {
	lldp := &cobra.Command{
		Use:   "lldp",
		Short: "Inspect LLDP neighbor discovery (monitor surface)",
	}
	lldp.AddCommand(lldpNeighborsCmd(a))
	return lldp
}

// lldpNeighborsCmd lists the observed LLDP neighbor table.
func lldpNeighborsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "neighbors",
		Aliases: []string{"neighbor", "neigh"},
		Short:   "List observed LLDP neighbors (monitor surface, VDOM-scoped)",
		Long: "List the observed LLDP neighbor table from monitor/network/lldp/neighbors.\n\n" +
			"LOCAL-PORT is this FortiGate's interface the neighbor was seen on; NEIGHBOR /\n" +
			"NEIGHBOR-PORT / CHASSIS-ID identify the remote device, its port, and its\n" +
			"chassis; MGMT-IP lists the neighbor's advertised management addresses.\n\n" +
			"This is VDOM-scoped: only neighbors on interfaces in the queried VDOM are\n" +
			"shown (use --vdom / --global to change scope), and only when LLDP reception\n" +
			"is enabled on those interfaces. json/yaml output shows the device's original\n" +
			"records verbatim.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			neighbors, err := p.ListLLDPNeighbors(cmd.Context())
			if err != nil {
				return err
			}
			sort.SliceStable(neighbors, func(i, j int) bool {
				return neighbors[i].LocalPort < neighbors[j].LocalPort
			})
			t := output.Tabular{
				Columns: []string{"LOCAL-PORT", "NEIGHBOR", "NEIGHBOR-PORT", "MGMT-IP", "CHASSIS-ID", "SYSTEM-DESC"},
				Raw:     rawRecords(lldpRaws(neighbors)),
			}
			for _, n := range neighbors {
				t.Rows = append(t.Rows, []string{
					n.LocalPort, n.NeighborName, n.NeighborPort,
					strings.Join(n.MgmtIPs, ","), n.ChassisID, n.SystemDesc,
				})
			}
			return a.render(t)
		},
	}
}

// lldpRaws collects the verbatim device records for json/yaml passthrough,
// matching the transceiver/sensor pattern.
func lldpRaws(in []domain.LLDPNeighbor) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, n := range in {
		if n.Raw != nil {
			out = append(out, n.Raw)
		}
	}
	return out
}
