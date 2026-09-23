package cli

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

type testProvider struct {
	provider.Provider
	lastPath string
	lastObj  provider.Object

	restoreCalled bool
	restoreScope  string
	restoreCfg    []byte
}

func (tp *testProvider) CmdbCreate(_ context.Context, path string, obj provider.Object) (string, error) {
	tp.lastPath = path
	tp.lastObj = obj
	return "1", nil
}

func (tp *testProvider) ConfigRestore(_ context.Context, scope, _ string, cfg []byte) error {
	tp.restoreCalled = true
	tp.restoreScope = scope
	tp.restoreCfg = cfg
	return nil
}

func TestFirewallShapingCommandTree(t *testing.T) {
	tp := &testProvider{}
	a := &app{prov: tp, assumeYes: true}
	root := &cobra.Command{Use: "firewall"}
	root.SetOut(io.Discard)
	root.AddCommand(firewallShapingCommands(a)...)

	// Verify group commands and aliases.
	schedCmd := findCmd(root, "schedule")
	if schedCmd == nil {
		t.Fatal("expected schedule group command")
	}
	if !slices.Contains(schedCmd.Aliases, "sched") {
		t.Errorf("schedule aliases = %v, want sched", schedCmd.Aliases)
	}

	shaperCmd := findCmd(root, "shaper")
	if shaperCmd == nil {
		t.Fatal("expected shaper group command")
	}

	policyCmd := findCmd(root, "shaping-policy")
	if policyCmd == nil {
		t.Fatal("expected shaping-policy command")
	}
	if !slices.Contains(policyCmd.Aliases, "shaping-pol") {
		t.Errorf("shaping-policy aliases = %v, want shaping-pol", policyCmd.Aliases)
	}

	// Verify all resources expose full CRUD verbs (list/show/create/set/delete).
	resources := [][]string{
		{"schedule", "onetime"},
		{"schedule", "recurring"},
		{"shaper", "traffic-shaper"},
		{"shaper", "per-ip-shaper"},
		{"shaping-policy"},
	}
	crud := []string{"list", "show", "create", "set", "delete"}
	for _, r := range resources {
		for _, action := range crud {
			cmd := findCmd(root, append(r, action)...)
			if cmd == nil {
				t.Errorf("resource %v missing %s subcommand", r, action)
			}
		}
	}

	// Verify typed flags on create.
	flagChecks := []struct {
		path  []string
		flags []string
	}{
		{[]string{"schedule", "onetime", "create"}, []string{"start", "end", "expiration-days", "color"}},
		{[]string{"schedule", "recurring", "create"}, []string{"day", "start", "end", "color"}},
		{[]string{"shaper", "traffic-shaper", "create"}, []string{"guaranteed-bandwidth", "maximum-bandwidth", "bandwidth-unit", "priority", "per-policy", "diffserv", "diffservcode"}},
		{[]string{"shaper", "per-ip-shaper", "create"}, []string{"max-bandwidth", "bandwidth-unit", "max-concurrent-session"}},
		{[]string{"shaping-policy", "create"}, []string{"srcaddr", "dstaddr", "srcintf", "dstintf", "service", "traffic-shaper", "traffic-shaper-reverse", "per-ip-shaper", "status"}},
	}
	for _, fc := range flagChecks {
		cmd := findCmd(root, fc.path...)
		if cmd == nil {
			t.Fatalf("command %v not found", fc.path)
		}
		for _, f := range fc.flags {
			if cmd.Flags().Lookup(f) == nil {
				t.Errorf("%v missing flag --%s", fc.path, f)
			}
		}
	}

	// Verify numeric vs string mkey behavior on create:
	// String mkeys (e.g. onetime) require a name argument to create.
	onetimeCreate := findCmd(root, "schedule", "onetime", "create")
	if err := onetimeCreate.RunE(onetimeCreate, nil); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("onetime create without arg expected error, got %v", err)
	}

	// Shaping-policy has numeric mkey -> create needs no arg (auto-assigned by FortiOS).
	shapingCreate := findCmd(root, "shaping-policy", "create")
	if err := shapingCreate.RunE(shapingCreate, nil); err != nil {
		t.Errorf("shaping-policy create without arg should succeed, got %v", err)
	}
	if tp.lastPath != "firewall/shaping-policy" {
		t.Errorf("shaping-policy create path = %q, want firewall/shaping-policy", tp.lastPath)
	}

	// Shaping-policy create with explicit numeric ID coerces to int.
	if err := shapingCreate.RunE(shapingCreate, []string{"42"}); err != nil {
		t.Errorf("shaping-policy create with id should succeed, got %v", err)
	}
	if id, ok := tp.lastObj["id"].(int); !ok || id != 42 {
		t.Errorf("shaping-policy id = %v (%T), want int 42", tp.lastObj["id"], tp.lastObj["id"])
	}
}
