package cli

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
)

func TestUserServerCommandTree(t *testing.T) {
	tp := &testProvider{}
	a := &app{prov: tp, assumeYes: true}
	user := &cobra.Command{Use: "user"}
	user.SetOut(io.Discard)
	user.AddCommand(userServerCommands(a)...)

	for _, name := range []string{"ldap", "radius", "tacacs"} {
		resource := findCmd(user, name)
		if resource == nil {
			t.Fatalf("expected user %s command", name)
		}
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if findCmd(resource, action) == nil {
				t.Errorf("user %s missing %s subcommand", name, action)
			}
		}
	}

	user.SetArgs([]string{"tacacs+", "create", "auth-server"})
	if err := user.Execute(); err != nil {
		t.Fatalf("user tacacs+ create: %v", err)
	}
	if tp.lastPath != "user/tacacs+" {
		t.Errorf("user tacacs+ create path = %q, want user/tacacs+", tp.lastPath)
	}
}
