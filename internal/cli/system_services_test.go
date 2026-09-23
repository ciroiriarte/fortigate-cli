package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSystemServiceCommands(t *testing.T) {
	// Build via bare app to ensure commands instantiate without provider.
	bareCmds := systemServiceCommands(&app{})
	if len(bareCmds) != 6 {
		t.Fatalf("systemServiceCommands(&app{}) returned %d commands, want 6", len(bareCmds))
	}

	tp := &testProvider{}
	a := &app{prov: tp, assumeYes: true}
	root := &cobra.Command{Use: "system"}
	root.SetOut(io.Discard)
	root.AddCommand(systemServiceCommands(a)...)

	// Singletons: ntp and snmp sysinfo must expose show and set only (no list, create, delete).
	singletons := [][]string{
		{"ntp"},
		{"snmp", "sysinfo"},
		{"central-management"},
		{"fortiguard"},
	}
	for _, s := range singletons {
		for _, action := range []string{"show", "set"} {
			if findCmd(root, append(s, action)...) == nil {
				t.Errorf("%v missing %s subcommand", s, action)
			}
		}
		for _, action := range []string{"list", "create", "delete"} {
			if findCmd(root, append(s, action)...) != nil {
				t.Errorf("%v singleton should not have %s subcommand", s, action)
			}
		}
	}

	// Full CRUD resources: dhcp server, snmp community, snmp user.
	crudResources := [][]string{
		{"dhcp", "server"},
		{"snmp", "community"},
		{"snmp", "user"},
	}
	crud := []string{"list", "show", "create", "set", "delete"}
	for _, r := range crudResources {
		for _, action := range crud {
			if findCmd(root, append(r, action)...) == nil {
				t.Errorf("%v missing %s subcommand", r, action)
			}
		}
	}

	// Numeric mkey resources: dhcp server and snmp community can be created without args (auto-assigned).
	serverCreate := findCmd(root, "dhcp", "server", "create")
	if serverCreate == nil {
		t.Fatal("dhcp server create command missing")
	}
	if err := serverCreate.RunE(serverCreate, nil); err != nil {
		t.Errorf("dhcp server create without arg: %v", err)
	}
	if tp.lastPath != "system.dhcp/server" {
		t.Errorf("dhcp server create path = %q, want system.dhcp/server", tp.lastPath)
	}
	if err := serverCreate.RunE(serverCreate, []string{"10"}); err != nil {
		t.Errorf("dhcp server create with id: %v", err)
	}
	if id, ok := tp.lastObj["id"].(int); !ok || id != 10 {
		t.Errorf("dhcp server id = %v (%T), want int 10", tp.lastObj["id"], tp.lastObj["id"])
	}

	commCreate := findCmd(root, "snmp", "community", "create")
	if commCreate == nil {
		t.Fatal("snmp community create command missing")
	}
	if err := commCreate.RunE(commCreate, nil); err != nil {
		t.Errorf("snmp community create without arg: %v", err)
	}
	if tp.lastPath != "system.snmp/community" {
		t.Errorf("snmp community create path = %q, want system.snmp/community", tp.lastPath)
	}
	if err := commCreate.RunE(commCreate, []string{"5"}); err != nil {
		t.Errorf("snmp community create with id: %v", err)
	}
	if id, ok := tp.lastObj["id"].(int); !ok || id != 5 {
		t.Errorf("snmp community id = %v (%T), want int 5", tp.lastObj["id"], tp.lastObj["id"])
	}

	// String mkey resource: snmp user requires a name argument to create.
	userCreate := findCmd(root, "snmp", "user", "create")
	if userCreate == nil {
		t.Fatal("snmp user create command missing")
	}
	if err := userCreate.RunE(userCreate, nil); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("snmp user create without arg expected error, got %v", err)
	}
	if err := userCreate.RunE(userCreate, []string{"adminv3"}); err != nil {
		t.Errorf("snmp user create with name: %v", err)
	}
	if tp.lastPath != "system.snmp/user" {
		t.Errorf("snmp user create path = %q, want system.snmp/user", tp.lastPath)
	}
	if tp.lastObj["name"] != "adminv3" {
		t.Errorf("snmp user name = %v, want adminv3", tp.lastObj["name"])
	}

	// Verify typed flags on curated commands.
	flagChecks := []struct {
		path  []string
		flags []string
	}{
		{[]string{"ntp", "set"}, []string{"type", "ntpsync", "syncinterval", "server-mode", "source-ip"}},
		{[]string{"dhcp", "server", "create"}, []string{"status", "interface", "default-gateway", "netmask", "dns-service", "dns-server1", "dns-server2", "domain", "lease-time"}},
		{[]string{"snmp", "sysinfo", "set"}, []string{"status", "description", "contact-info", "location", "trap-high-cpu-threshold"}},
		{[]string{"snmp", "community", "create"}, []string{"name", "status", "query-v1-status", "query-v2c-status", "trap-v1-status", "trap-v2c-status", "events"}},
		{[]string{"snmp", "user", "create"}, []string{"status", "security-level", "auth-proto", "auth-pwd", "priv-proto", "priv-pwd", "queries", "notify-hosts"}},
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

	// Verify notify-hosts is encoded as scalar string (kindString), not refList.
	userCreateFlags := findCmd(root, "snmp", "user", "create")
	if err := userCreateFlags.Flags().Set("notify-hosts", "192.0.2.1,192.0.2.2"); err != nil {
		t.Fatal(err)
	}
	if err := userCreateFlags.Flags().Set("security-level", "auth-priv"); err != nil {
		t.Fatal(err)
	}
	if err := userCreateFlags.RunE(userCreateFlags, []string{"trapuser"}); err != nil {
		t.Fatal(err)
	}
	if tp.lastObj["notify-hosts"] != "192.0.2.1,192.0.2.2" {
		t.Errorf("notify-hosts = %v (%T), want scalar string", tp.lastObj["notify-hosts"], tp.lastObj["notify-hosts"])
	}
	if tp.lastObj["security-level"] != "auth-priv" {
		t.Errorf("security-level = %v, want auth-priv", tp.lastObj["security-level"])
	}
}
