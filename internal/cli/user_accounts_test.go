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
	want := []map[string]string{{"name": "alice"}, {"name": "ldap-users"}}
	if !reflect.DeepEqual(tp.lastObj["member"], want) {
		t.Errorf("member = %v, want %v", tp.lastObj["member"], want)
	}
}
