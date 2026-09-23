package cli

import "github.com/spf13/cobra"

// firewallShapingCommands returns the curated schedule + traffic-shaping
// firewall subcommands. Note the dotted REST categories: firewall.schedule and
// firewall.shaper vs top-level firewall/shaping-policy.
// newFirewallCmd wires these in.
func firewallShapingCommands(a *app) []*cobra.Command {
	onetime := resource{
		use:   "onetime",
		short: "Manage one-time firewall schedules",
		path:  "firewall.schedule/onetime",
		mkey:  "name",
		columns: []column{
			{field: "name"},
			{field: "start"},
			{field: "end"},
		},
		fields: []fieldSpec{
			{name: "start", usage: `start "hh:mm yyyy/mm/dd"`},
			{name: "end", usage: `end "hh:mm yyyy/mm/dd"`},
			{name: "expiration-days", usage: "days before expiry to warn"},
			{name: "color", usage: "GUI color index"},
		},
	}

	recurring := resource{
		use:   "recurring",
		short: "Manage recurring firewall schedules",
		path:  "firewall.schedule/recurring",
		mkey:  "name",
		columns: []column{
			{field: "name"},
			{field: "day"},
			{field: "start"},
			{field: "end"},
		},
		fields: []fieldSpec{
			{name: "day", usage: "space list: sunday monday tuesday wednesday thursday friday saturday"},
			{name: "start", usage: `"hh:mm"`},
			{name: "end", usage: `"hh:mm"`},
			{name: "color", usage: "GUI color index"},
		},
	}

	schedule := &cobra.Command{
		Use:     "schedule",
		Aliases: []string{"sched"},
		Short:   "Manage firewall schedules",
	}
	schedule.AddCommand(a.newResourceCmd(onetime), a.newResourceCmd(recurring))

	trafficShaper := resource{
		use:   "traffic-shaper",
		short: "Manage traffic shapers",
		path:  "firewall.shaper/traffic-shaper",
		mkey:  "name",
		columns: []column{
			{field: "name"},
			{field: "guaranteed-bandwidth", header: "GUARANTEED"},
			{field: "maximum-bandwidth", header: "MAXIMUM"},
			{field: "bandwidth-unit", header: "UNIT"},
			{field: "priority"},
		},
		fields: []fieldSpec{
			{name: "guaranteed-bandwidth", usage: "guaranteed rate"},
			{name: "maximum-bandwidth", usage: "max rate"},
			{name: "bandwidth-unit", usage: "kbps|mbps|gbps"},
			{name: "priority", usage: "low|medium|high"},
			{name: "per-policy", usage: "enable|disable"},
			{name: "diffserv", usage: "enable|disable"},
			{name: "diffservcode", usage: "DSCP bits"},
		},
	}

	perIPShaper := resource{
		use:   "per-ip-shaper",
		short: "Manage per-IP traffic shapers",
		path:  "firewall.shaper/per-ip-shaper",
		mkey:  "name",
		columns: []column{
			{field: "name"},
			{field: "max-bandwidth", header: "MAX-BW"},
			{field: "bandwidth-unit", header: "UNIT"},
			{field: "max-concurrent-session", header: "MAX-SESS"},
		},
		fields: []fieldSpec{
			{name: "max-bandwidth", usage: "per-IP max rate"},
			{name: "bandwidth-unit", usage: "kbps|mbps|gbps"},
			{name: "max-concurrent-session", usage: "per-IP session cap"},
		},
	}

	shaper := &cobra.Command{
		Use:   "shaper",
		Short: "Manage traffic shapers",
	}
	shaper.AddCommand(a.newResourceCmd(trafficShaper), a.newResourceCmd(perIPShaper))

	shapingPolicy := resource{
		use:     "shaping-policy",
		aliases: []string{"shaping-pol"},
		short:   "Manage firewall shaping policies",
		path:    "firewall/shaping-policy",
		mkey:    "id",
		mkeyArg: "id",
		numeric: true,
		columns: []column{
			{field: "id"},
			{field: "srcaddr", header: "SRC"},
			{field: "dstaddr", header: "DST"},
			{field: "service"},
			{field: "traffic-shaper", header: "SHAPER"},
			{field: "status"},
		},
		fields: []fieldSpec{
			{name: "srcaddr", kind: kindRefList, usage: "source address(es), comma-separated"},
			{name: "dstaddr", kind: kindRefList, usage: "destination address(es), comma-separated"},
			{name: "srcintf", kind: kindRefList, usage: "source interface(s)/zone(s), comma-separated"},
			{name: "dstintf", kind: kindRefList, usage: "destination interface(s)/zone(s), comma-separated"},
			{name: "service", kind: kindRefList, usage: "service(s), comma-separated"},
			{name: "traffic-shaper", usage: "forward traffic shaper name"},
			{name: "traffic-shaper-reverse", usage: "reverse traffic shaper name"},
			{name: "per-ip-shaper", usage: "per-IP traffic shaper name"},
			{name: "status", usage: "enable|disable"},
		},
	}

	return []*cobra.Command{
		schedule,
		shaper,
		a.newResourceCmd(shapingPolicy),
	}
}
