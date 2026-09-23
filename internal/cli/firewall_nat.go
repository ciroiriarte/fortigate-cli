package cli

import "github.com/spf13/cobra"

// firewallNATCommands returns the curated NAT-related firewall subcommands
// (vip, vip-group, ippool, central-snat-map). newFirewallCmd wires these in.
func firewallNATCommands(a *app) []*cobra.Command {
	vip := resource{
		use: "vip", short: "Manage firewall virtual IP objects",
		path: "firewall/vip", mkey: "name",
		columns: []column{{field: "name"}, {field: "extip"}, {field: "mappedip"},
			{field: "extport"}, {field: "mappedport"}, {field: "portforward"}},
		fields: []fieldSpec{
			{name: "type", usage: "static-nat|load-balance|server-load-balance|dns-translation|fqdn|access-proxy"},
			{name: "extip", usage: "external IP address or range"},
			{name: "extintf", usage: "external interface, e.g. any"},
			{name: "mappedip", kind: kindRangeList, usage: "mapped IP(s)/range(s), comma-separated (cmdb child-table keyed by range)"},
			{name: "extport", usage: "external port or range"},
			{name: "mappedport", usage: "mapped port or range"},
			{name: "portforward", usage: "enable|disable"},
			{name: "protocol", usage: "tcp|udp|sctp|icmp"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	vipGroup := resource{
		use: "vip-group", aliases: []string{"vipgrp"}, short: "Manage firewall virtual IP groups",
		path: "firewall/vipgrp", mkey: "name",
		columns: []column{{field: "name"}, {field: "member"}, {field: "comments"}},
		fields: []fieldSpec{
			{name: "interface", usage: "interface shared by member VIPs"},
			{name: "member", kind: kindRefList, usage: "comma-separated VIP members"},
			{name: "comments", usage: "free-text comment"},
		},
	}

	ippool := resource{
		use: "ippool", short: "Manage firewall IP pools",
		path: "firewall/ippool", mkey: "name",
		columns: []column{{field: "name"}, {field: "type"}, {field: "startip", header: "START-IP"}, {field: "endip", header: "END-IP"}},
		fields: []fieldSpec{
			{name: "type", usage: "overload|one-to-one|fixed-port-range|port-block-allocation"},
			{name: "startip", usage: "pool start IP"},
			{name: "endip", usage: "pool end IP"},
			{name: "arp-reply", usage: "enable|disable"},
			{name: "comments", usage: "free-text comment"},
		},
	}

	centralSNATMap := resource{
		use: "central-snat-map", aliases: []string{"snat", "central-snat"}, short: "Manage central SNAT maps",
		path: "firewall/central-snat-map", mkey: "policyid", mkeyArg: "policyid", numeric: true,
		columns: []column{{field: "policyid", header: "ID"}, {field: "srcintf"},
			{field: "dstintf"}, {field: "orig-addr"}, {field: "dst-addr"}, {field: "nat"}},
		fields: []fieldSpec{
			{name: "srcintf", kind: kindRefList, usage: "source interface(s)/zone(s)"},
			{name: "dstintf", kind: kindRefList, usage: "destination interface(s)/zone(s)"},
			{name: "orig-addr", kind: kindRefList, usage: "original source address(es)"},
			{name: "dst-addr", kind: kindRefList, usage: "destination address(es)"},
			{name: "nat-ippool", kind: kindRefList, usage: "SNAT pool(s)"},
			{name: "protocol", usage: "IP protocol number"},
			{name: "orig-port", usage: "original source port"},
			{name: "nat-port", usage: "translated source port"},
			{name: "nat", usage: "enable|disable"},
			{name: "status", usage: "enable|disable"},
		},
	}

	return []*cobra.Command{
		a.newResourceCmd(vip),
		a.newResourceCmd(vipGroup),
		a.newResourceCmd(ippool),
		a.newResourceCmd(centralSNATMap),
	}
}
