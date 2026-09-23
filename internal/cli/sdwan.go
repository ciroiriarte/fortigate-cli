package cli

import "github.com/spf13/cobra"

// newSDWANCmd builds the `sdwan` command group over system/sdwan and its
// child-table sub-paths (zone, members, health-check, service).
func newSDWANCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "sdwan", Short: "Manage SD-WAN configuration (cmdb surface)"}

	settings := resource{
		use: "settings", short: "Manage SD-WAN settings", single: true,
		path: "system/sdwan",
		fields: []fieldSpec{
			{name: "status", usage: "enable|disable"},
			{name: "load-balance-mode", usage: "source-ip-based|weight-based|usage-based|source-dest-ip-based|measured-volume-based"},
			{name: "neighbor-hold-down", usage: "enable|disable"},
			{name: "neighbor-hold-down-time", usage: "seconds"},
			{name: "fail-detect", usage: "enable|disable"},
		},
	}

	zone := resource{
		use: "zone", short: "Manage SD-WAN zones",
		path: "system/sdwan/zone", mkey: "name",
		columns: []column{{field: "name"}, {field: "service-sla-tie-break"}},
		fields: []fieldSpec{
			{name: "service-sla-tie-break", usage: "cfg-order|fib-best-match|input-device"},
			{name: "minimum-sla-meet-members", usage: "minimum number of SLA-meeting members"},
		},
	}

	member := resource{
		use: "member", aliases: []string{"members"}, short: "Manage SD-WAN members",
		path: "system/sdwan/members", mkey: "seq-num", mkeyArg: "seq-num", numeric: true,
		columns: []column{{field: "seq-num", header: "SEQ"}, {field: "interface"}, {field: "gateway"},
			{field: "zone"}, {field: "priority"}, {field: "status"}},
		fields: []fieldSpec{
			{name: "interface", usage: "egress interface"},
			{name: "gateway", usage: "next-hop IPv4"},
			{name: "source", usage: "source IP for health-check"},
			{name: "zone", usage: "SD-WAN zone"},
			{name: "cost", usage: "link cost"},
			{name: "priority", usage: "route priority"},
			{name: "weight", usage: "load-balance weight"},
			{name: "status", usage: "enable|disable"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	healthCheck := resource{
		use: "health-check", short: "Manage SD-WAN health checks",
		path: "system/sdwan/health-check", mkey: "name",
		columns: []column{{field: "name"}, {field: "server"}, {field: "protocol"}, {field: "interval"}},
		fields: []fieldSpec{
			{name: "server", usage: "probe target(s)"},
			{name: "protocol", usage: "ping|tcp-echo|udp-echo|http|https|twamp|dns|ftp"},
			{name: "port", usage: "probe port"},
			{name: "interval", usage: "probe interval"},
			{name: "failtime", usage: "failures before down"},
			{name: "recoverytime", usage: "successes before up"},
		},
	}

	service := resource{
		use: "service", short: "Manage SD-WAN services",
		path: "system/sdwan/service", mkey: "id", mkeyArg: "id", numeric: true,
		columns: []column{{field: "id"}, {field: "name"}, {field: "mode"}, {field: "dst"}, {field: "src"}, {field: "status"}},
		fields: []fieldSpec{
			{name: "name", usage: "rule name"},
			{name: "mode", usage: "auto|manual|priority|sla|load-balance"},
			{name: "dst", kind: kindRefList, usage: "destination address(es)"},
			{name: "src", kind: kindRefList, usage: "source address(es)"},
			{name: "health-check", kind: kindRefList, usage: "health-check name(s)"},
			{name: "status", usage: "enable|disable"},
		},
	}

	cmd.AddCommand(
		a.newResourceCmd(settings),
		a.newResourceCmd(zone),
		a.newResourceCmd(member),
		a.newResourceCmd(healthCheck),
		a.newResourceCmd(service),
	)
	return cmd
}
