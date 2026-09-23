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

	bgp := resource{
		use: "bgp", short: "Manage BGP configuration", single: true,
		path: "router/bgp",
		fields: []fieldSpec{
			{name: "as", usage: "local AS number (supports asdot)"},
			{name: "router-id", usage: "BGP router-id (IPv4)"},
			{name: "keepalive-timer", usage: "keepalive timer in seconds"},
			{name: "holdtime-timer", usage: "holdtime timer in seconds"},
			{name: "ebgp-multipath", usage: "enable|disable"},
			{name: "ibgp-multipath", usage: "enable|disable"},
			{name: "graceful-restart", usage: "enable|disable"},
		},
	}

	ospf := resource{
		use: "ospf", short: "Manage OSPF configuration", single: true,
		path: "router/ospf",
		fields: []fieldSpec{
			{name: "router-id", usage: "OSPF router-id (IPv4)"},
			{name: "default-information-originate", usage: "enable|always|disable"},
			{name: "distance", usage: "administrative distance"},
			{name: "abr-type", usage: "cisco|ibm|standard|shortcut"},
		},
	}

	routeMap := resource{
		use: "route-map", short: "Manage route maps",
		path: "router/route-map", mkey: "name",
		columns: []column{{field: "name"}, {field: "comments"}},
		fields: []fieldSpec{
			{name: "comments", usage: "free-text comment"},
		},
	}

	prefixList := resource{
		use: "prefix-list", short: "Manage prefix lists",
		path: "router/prefix-list", mkey: "name",
		columns: []column{{field: "name"}, {field: "comments"}},
		fields: []fieldSpec{
			{name: "comments", usage: "free-text comment"},
		},
	}

	accessList := resource{
		use: "access-list", short: "Manage access lists",
		path: "router/access-list", mkey: "name",
		columns: []column{{field: "name"}, {field: "comments"}},
		fields: []fieldSpec{
			{name: "comments", usage: "free-text comment"},
		},
	}

	cmd.AddCommand(
		a.newResourceCmd(static),
		a.newResourceCmd(bgp),
		a.newResourceCmd(ospf),
		a.newResourceCmd(routeMap),
		a.newResourceCmd(prefixList),
		a.newResourceCmd(accessList),
	)
	return cmd
}
