package cli

import "github.com/spf13/cobra"

// newRouterCmd builds the `router` command group.
func newRouterCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "router",
		Short: "Manage routing configuration (cmdb surface)",
	}

	static := resource{
		use: "static", short: "Manage IPv4 static routes",
		path: "router/static", mkey: "seq-num", mkeyArg: "seq-num", numeric: true,
		columns: []column{{field: "seq-num", header: "SEQ"}, {field: "dst"},
			{field: "gateway", header: "GATEWAY"}, {field: "device"},
			{field: "distance"}, {field: "status"}},
		fields: []fieldSpec{
			{name: "dst", usage: `destination "<ip> <mask>" or CIDR`},
			{name: "gateway", usage: "next-hop gateway IP"},
			{name: "device", usage: "egress interface"},
			{name: "distance", usage: "administrative distance (default 10)"},
			{name: "priority", usage: "route priority"},
			{name: "status", usage: "enable|disable"},
			{name: "blackhole", usage: "enable|disable"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	cmd.AddCommand(a.newResourceCmd(static))
	return cmd
}
