package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/domain"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/topology"
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
	cmd.AddCommand(networkTopologyCmd(a))
	return cmd
}

// networkTopologyCmd renders an observed, one-hop topology graph of this
// FortiGate from live data. It does not route through output.Tabular (a graph
// isn't tabular): it owns a dedicated --format flag and prints the rendered
// string to stdout.
func networkTopologyCmd(a *app) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "topology",
		Aliases: []string{"topo"},
		Short:   "Render an observed one-hop topology graph (LLDP-based)",
		Long: "Render an OBSERVED, one-hop topology graph of this FortiGate from live data:\n" +
			"the device at the center (from monitor/system/status), one node per LLDP\n" +
			"neighbor (monitor/network/lldp/neighbors), and an HA peer node when the\n" +
			"cluster has more than one member. Edges are labeled with the local and\n" +
			"neighbor ports, enriched with the local link speed when known\n" +
			"(e.g. \"x1 (10G) - port57\").\n\n" +
			"This is NOT physical ground truth: LLDP is optional and can be stale, and\n" +
			"unmanaged devices (no LLDP) are invisible. It is VDOM-scoped — only neighbors\n" +
			"on interfaces in the queried VDOM appear (use --vdom / --global to change\n" +
			"scope). If LLDP is empty the center node (and HA peer, if any) still renders;\n" +
			"only a failure to read device status is fatal.\n\n" +
			"Formats (--format, default mermaid):\n" +
			"  mermaid  paste into mermaid.live or a GitHub ```mermaid block\n" +
			"  dot      Graphviz: fgt network topology --format dot | dot -Tsvg -o topo.svg\n" +
			"  json     stable machine-readable {device,nodes,edges}\n\n" +
			"No SVG/PNG is rendered by this command.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch format {
			case "mermaid", "dot", "json":
			default:
				return fmt.Errorf("invalid --format %q (want mermaid|dot|json)", format)
			}
			p, err := a.Provider()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			// DeviceStatus is the only hard dependency; everything else degrades.
			dev, err := p.DeviceStatus(ctx)
			if err != nil {
				return err
			}
			neighbors, _ := p.ListLLDPNeighbors(ctx)
			ha, _ := p.HAStatus(ctx)
			ifaces, _ := p.ListInterfacesFull(ctx)

			g := topology.Build(dev, neighbors, ha, ifaces)

			var out string
			switch format {
			case "mermaid":
				out = topology.RenderMermaid(g)
			case "dot":
				out = topology.RenderDOT(g)
			case "json":
				out, err = topology.RenderJSON(g)
				if err != nil {
					return err
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(out, "\n"))
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&format, "format", "mermaid", "graph output format: mermaid|dot|json")
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
