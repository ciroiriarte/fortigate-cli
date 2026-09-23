package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestFirewallNATCommandTree(t *testing.T) {
	root := &cobra.Command{}
	root.AddCommand(firewallNATCommands(&app{})...)

	for _, name := range []string{"vip", "vip-group", "ippool", "central-snat-map"} {
		resource, _, err := root.Find([]string{name})
		if err != nil || resource.Name() != name {
			t.Fatalf("find firewall %s: %v", name, err)
		}
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if cmd, _, err := resource.Find([]string{action}); err != nil || cmd.Name() != action {
				t.Errorf("find firewall %s %s: %v", name, action, err)
			}
		}
	}

	resource, _, err := root.Find([]string{"central-snat-map"})
	if err != nil {
		t.Fatalf("find firewall central-snat-map: %v", err)
	}
	create, _, err := resource.Find([]string{"create"})
	if err != nil {
		t.Fatalf("find firewall central-snat-map create: %v", err)
	}
	if err := create.Args(create, nil); err != nil {
		t.Errorf("central-snat-map create should allow no policyid: %v", err)
	}
}
