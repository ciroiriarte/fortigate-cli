package cli

import (
	"io"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestUserAccountCommandTree(t *testing.T) {
	tp := &testProvider{}
	a := &app{prov: tp, assumeYes: true}
	root := &cobra.Command{Use: "user"}
	root.SetOut(io.Discard)
	root.AddCommand(userAccountCommands(a)...)

	for _, resource := range []string{"local", "group"} {
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if cmd := findCmd(root, resource, action); cmd == nil {
				t.Errorf("%s missing %s subcommand", resource, action)
			}
		}
	}

	create := findCmd(root, "group", "create")
	if err := create.ParseFlags([]string{"--member", "alice,ldap-users"}); err != nil {
		t.Fatal(err)
	}
	if err := create.RunE(create, []string{"staff"}); err != nil {
		t.Fatal(err)
	}
	if tp.lastPath != "user/group" {
		t.Errorf("group create path = %q, want user/group", tp.lastPath)
	}
	want := []map[string]string{{"name": "alice"}, {"name": "ldap-users"}}
	if !reflect.DeepEqual(tp.lastObj["member"], want) {
		t.Errorf("member = %v, want %v", tp.lastObj["member"], want)
	}

	// user/local: assert the create path and the asymmetric flag/field mapping —
	// the CLI flag is --tacacs-server but the cmdb body key is tacacs+-server.
	lcreate := findCmd(root, "local", "create")
	if err := lcreate.ParseFlags([]string{"--type", "tacacs+", "--tacacs-server", "srv1"}); err != nil {
		t.Fatal(err)
	}
	if err := lcreate.RunE(lcreate, []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	if tp.lastPath != "user/local" {
		t.Errorf("local create path = %q, want user/local", tp.lastPath)
	}
	if tp.lastObj["tacacs+-server"] != "srv1" {
		t.Errorf("tacacs+-server = %v, want srv1 (flag --tacacs-server must map to cmdb key tacacs+-server)", tp.lastObj["tacacs+-server"])
	}
}
