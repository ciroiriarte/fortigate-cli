package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func findCmd(root *cobra.Command, path ...string) *cobra.Command {
	curr := root
	for _, name := range path {
		var next *cobra.Command
		for _, c := range curr.Commands() {
			if c.Name() == name {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		curr = next
	}
	return curr
}

func TestRouterCommandTree(t *testing.T) {
	a := &app{}
	cmd := newRouterCmd(a)

	// Assert singletons expose show/set, but NOT list/create/delete.
	singletons := []string{"bgp", "ospf"}
	for _, s := range singletons {
		sub := findCmd(cmd, s)
		if sub == nil {
			t.Fatalf("expected subcommand router %s to exist", s)
		}
		if findCmd(sub, "show") == nil {
			t.Errorf("router %s: expected 'show' command", s)
		}
		if findCmd(sub, "set") == nil {
			t.Errorf("router %s: expected 'set' command", s)
		}
		for _, forbidden := range []string{"list", "create", "delete"} {
			if findCmd(sub, forbidden) != nil {
				t.Errorf("singleton router %s: should not have '%s' command", s, forbidden)
			}
		}
	}

	// Assert multi-instance resources expose full CRUD (list/show/create/set/delete).
	multi := []string{"route-map", "prefix-list", "access-list"}
	for _, m := range multi {
		sub := findCmd(cmd, m)
		if sub == nil {
			t.Fatalf("expected subcommand router %s to exist", m)
		}
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if findCmd(sub, action) == nil {
				t.Errorf("router %s: expected '%s' command", m, action)
			}
		}
	}

	// Verify typed flags on bgp set.
	bgpSet := findCmd(cmd, "bgp", "set")
	if bgpSet == nil {
		t.Fatal("router bgp set command missing")
	}
	for _, flag := range []string{"as", "router-id", "keepalive-timer", "holdtime-timer", "ebgp-multipath", "ibgp-multipath", "graceful-restart"} {
		if bgpSet.Flags().Lookup(flag) == nil {
			t.Errorf("router bgp set: expected flag --%s", flag)
		}
	}

	// Verify typed flags on ospf set.
	ospfSet := findCmd(cmd, "ospf", "set")
	if ospfSet == nil {
		t.Fatal("router ospf set command missing")
	}
	for _, flag := range []string{"router-id", "default-information-originate", "distance", "abr-type"} {
		if ospfSet.Flags().Lookup(flag) == nil {
			t.Errorf("router ospf set: expected flag --%s", flag)
		}
	}

	// Verify comments flag on route-map, prefix-list, access-list.
	for _, m := range multi {
		for _, action := range []string{"create", "set"} {
			actionCmd := findCmd(cmd, m, action)
			if actionCmd == nil {
				t.Fatalf("router %s %s missing", m, action)
			}
			if actionCmd.Flags().Lookup("comments") == nil {
				t.Errorf("router %s %s: expected flag --comments", m, action)
			}
		}
	}
}

func TestRootIncludesRouterTree(t *testing.T) {
	root := NewRootCmd()
	cases := [][]string{
		{"router", "static", "list"},
		{"router", "bgp", "show"},
		{"router", "bgp", "set"},
		{"router", "ospf", "show"},
		{"router", "ospf", "set"},
		{"router", "route-map", "list"},
		{"router", "route-map", "create"},
		{"router", "route-map", "delete"},
		{"router", "prefix-list", "list"},
		{"router", "access-list", "list"},
	}
	for _, c := range cases {
		sub, _, err := root.Find(c)
		if err != nil || sub == nil {
			t.Errorf("root command failed to find %v: %v", c, err)
		}
	}
}
