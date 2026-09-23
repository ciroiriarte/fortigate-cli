package cli

import (
	"io"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestSystemAutomationCommands(t *testing.T) {
	if cmds := systemAutomationCommands(&app{}); len(cmds) != 1 {
		t.Fatalf("systemAutomationCommands(&app{}) returned %d commands, want 1", len(cmds))
	}

	tp := &testProvider{}
	a := &app{prov: tp, assumeYes: true}
	root := &cobra.Command{Use: "system"}
	root.SetOut(io.Discard)
	root.AddCommand(systemAutomationCommands(a)...)

	for _, resource := range []string{"trigger", "action", "stitch"} {
		for _, operation := range []string{"list", "show", "create", "set", "delete"} {
			if cmd := findCmd(root, "automation", resource, operation); cmd == nil {
				t.Errorf("%s missing %s subcommand", resource, operation)
			}
		}
	}

	create := findCmd(root, "automation", "action", "create")
	if err := create.Flags().Set("email-to", "ops,oncall"); err != nil {
		t.Fatal(err)
	}
	if err := create.RunE(create, []string{"notify"}); err != nil {
		t.Fatal(err)
	}
	want := []map[string]string{{"name": "ops"}, {"name": "oncall"}}
	if got := tp.lastObj["email-to"]; !reflect.DeepEqual(got, want) {
		t.Errorf("email-to = %v, want %v", got, want)
	}
}
