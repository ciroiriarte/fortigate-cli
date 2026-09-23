package cli

import "github.com/spf13/cobra"

// newFirewallCmd builds the `firewall` command group from declarative resource
// definitions (see resource.go). Each object gets list/show/create/set/delete.
func newFirewallCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "firewall",
		Aliases: []string{"fw"},
		Short:   "Manage firewall configuration objects (cmdb surface)",
	}

	address := resource{
		use: "address", aliases: []string{"addr"}, short: "Manage firewall address objects",
		path: "firewall/address", mkey: "name",
		columns: []column{{field: "name"}, {field: "type"}, {field: "subnet"},
			{field: "start-ip", header: "START-IP"}, {field: "fqdn"}, {field: "comment"}},
		fields: []fieldSpec{
			{name: "type", usage: "ipmask|iprange|fqdn|geography|wildcard|interface-subnet|mac"},
			{name: "subnet", usage: `"<ip> <mask>" or CIDR (type ipmask)`},
			{name: "start-ip", usage: "range start (type iprange)"},
			{name: "end-ip", usage: "range end (type iprange)"},
			{name: "fqdn", usage: "FQDN (type fqdn)"},
			{name: "country", usage: "ISO country code (type geography)"},
			{name: "associated-interface", flag: "interface", usage: "associated interface"},
			{name: "allow-routing", usage: "enable|disable — usable as a route destination"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	addrgrp := resource{
		use: "addrgrp", aliases: []string{"addr-group"}, short: "Manage firewall address groups",
		path: "firewall/addrgrp", mkey: "name",
		columns: []column{{field: "name"}, {field: "member"}, {field: "comment"}},
		fields: []fieldSpec{
			{name: "member", kind: kindRefList, usage: "comma-separated address/group members"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	serviceCustom := resource{
		use: "custom", short: "Manage custom firewall services",
		path: "firewall.service/custom", mkey: "name",
		columns: []column{{field: "name"}, {field: "protocol"},
			{field: "tcp-portrange", header: "TCP"}, {field: "udp-portrange", header: "UDP"}, {field: "comment"}},
		fields: []fieldSpec{
			{name: "protocol", usage: "TCP/UDP/SCTP|ICMP|ICMP6|IP|..."},
			{name: "tcp-portrange", usage: `TCP ports, e.g. "443 8080-8090"`},
			{name: "udp-portrange", usage: "UDP ports"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	serviceGroup := resource{
		use: "group", short: "Manage firewall service groups",
		path: "firewall.service/group", mkey: "name",
		columns: []column{{field: "name"}, {field: "member"}, {field: "comment"}},
		fields: []fieldSpec{
			{name: "member", kind: kindRefList, usage: "comma-separated service members"},
			{name: "comment", usage: "free-text comment"},
		},
	}

	service := &cobra.Command{Use: "service", Aliases: []string{"svc"}, Short: "Manage firewall services"}
	service.AddCommand(a.newResourceCmd(serviceCustom), a.newResourceCmd(serviceGroup))

	policy := resource{
		use: "policy", aliases: []string{"pol"}, short: "Manage firewall policies",
		path: "firewall/policy", mkey: "policyid", mkeyArg: "policyid", numeric: true,
		columns: []column{{field: "policyid", header: "ID"}, {field: "name"},
			{field: "srcintf", header: "SRCINTF"}, {field: "dstintf", header: "DSTINTF"},
			{field: "srcaddr", header: "SRC"}, {field: "dstaddr", header: "DST"},
			{field: "service"}, {field: "action"}, {field: "status"}},
		fields: []fieldSpec{
			{name: "name", usage: "policy name"},
			{name: "srcintf", kind: kindRefList, usage: "source interface(s)/zone(s), comma-separated"},
			{name: "dstintf", kind: kindRefList, usage: "destination interface(s)/zone(s)"},
			{name: "srcaddr", kind: kindRefList, usage: "source address(es), e.g. all"},
			{name: "dstaddr", kind: kindRefList, usage: "destination address(es)"},
			{name: "service", kind: kindRefList, usage: "service(s), e.g. ALL,HTTPS"},
			{name: "action", usage: "accept|deny|ipsec"},
			{name: "status", usage: "enable|disable"},
			{name: "schedule", usage: "schedule name (default always)"},
			{name: "nat", usage: "enable|disable — source NAT"},
		},
	}

	cmd.AddCommand(
		a.newResourceCmd(address),
		a.newResourceCmd(addrgrp),
		service,
		a.newResourceCmd(policy),
	)
	// NAT (vip/vip-group/ippool/central-snat-map) and traffic-shaping/schedule
	// resources live in firewall_nat.go / firewall_shaping.go to keep this file
	// focused; wire their command sets in here.
	cmd.AddCommand(firewallNATCommands(a)...)
	cmd.AddCommand(firewallShapingCommands(a)...)
	return cmd
}
