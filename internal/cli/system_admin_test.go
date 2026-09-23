package cli

import (
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSystemAdminCommands(t *testing.T) {
	// Build via bare app to ensure commands instantiate without provider.
	bareCmds := systemAdminCommands(&app{})
	if len(bareCmds) != 3 {
		t.Fatalf("systemAdminCommands(&app{}) returned %d commands, want 3", len(bareCmds))
	}

	tp := &testProvider{}
	a := &app{prov: tp, assumeYes: true}
	root := &cobra.Command{Use: "system"}
	root.SetOut(io.Discard)
	root.AddCommand(systemAdminCommands(a)...)

	// Verify api-user command and alias.
	apiUserCmd := findCmd(root, "api-user")
	if apiUserCmd == nil {
		t.Fatal("expected api-user command")
	}
	if !slices.Contains(apiUserCmd.Aliases, "api-admin") {
		t.Errorf("api-user aliases = %v, want api-admin", apiUserCmd.Aliases)
	}

	// Full CRUD: api-user exposes list, show, create, set, delete.
	crud := []string{"list", "show", "create", "set", "delete"}
	for _, action := range crud {
		if findCmd(root, "api-user", action) == nil {
			t.Errorf("api-user missing %s subcommand", action)
		}
	}

	// Singletons: global and settings expose show and set only (no list, create, delete).
	singletons := []string{"global", "settings"}
	for _, s := range singletons {
		for _, action := range []string{"show", "set"} {
			if findCmd(root, s, action) == nil {
				t.Errorf("%s missing %s subcommand", s, action)
			}
		}
		for _, action := range []string{"list", "create", "delete"} {
			if findCmd(root, s, action) != nil {
				t.Errorf("%s singleton should not have %s subcommand", s, action)
			}
		}
	}

	// String mkey: api-user requires a name to create.
	apiUserCreate := findCmd(root, "api-user", "create")
	if apiUserCreate == nil {
		t.Fatal("api-user create command missing")
	}
	if err := apiUserCreate.RunE(apiUserCreate, nil); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("api-user create without arg expected error, got %v", err)
	}

	// Assert typed flags and that vdom ref-list encodes as [{name:...}].
	if err := apiUserCreate.ParseFlags([]string{
		"--accprofile", "super_admin",
		"--vdom", "root,mgmt",
		"--comments", "CI bot",
	}); err != nil {
		t.Fatal(err)
	}
	if err := apiUserCreate.RunE(apiUserCreate, []string{"ci-bot"}); err != nil {
		t.Fatalf("api-user create: %v", err)
	}
	if tp.lastPath != "system/api-user" {
		t.Errorf("api-user create path = %q, want system/api-user", tp.lastPath)
	}
	if tp.lastObj["name"] != "ci-bot" {
		t.Errorf("name = %v, want ci-bot", tp.lastObj["name"])
	}
	if tp.lastObj["accprofile"] != "super_admin" {
		t.Errorf("accprofile = %v, want super_admin", tp.lastObj["accprofile"])
	}
	if tp.lastObj["comments"] != "CI bot" {
		t.Errorf("comments = %v, want CI bot", tp.lastObj["comments"])
	}
	wantVdom := []map[string]string{{"name": "root"}, {"name": "mgmt"}}
	if !reflect.DeepEqual(tp.lastObj["vdom"], wantVdom) {
		t.Errorf("vdom = %v, want %v", tp.lastObj["vdom"], wantVdom)
	}

	// Verify typed flags on curated commands.
	flagChecks := []struct {
		path  []string
		flags []string
	}{
		{[]string{"api-user", "create"}, []string{"accprofile", "vdom", "comments", "cors-allow-origin", "peer-auth", "peer-group"}},
		{[]string{"global", "set"}, []string{"hostname", "admin-sport", "admin-ssh-port", "admintimeout", "timezone", "admin-https-redirect", "language", "gui-theme"}},
		{[]string{"settings", "set"}, []string{"opmode", "inspection-mode", "central-nat", "allow-subnet-overlap"}},
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
}
