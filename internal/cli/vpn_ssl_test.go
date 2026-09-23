package cli

import (
	"io"
	"testing"
)

func TestVpnSSLCommandTree(t *testing.T) {
	tp := &testProvider{}
	cmd := newVpnSSLCmd(&app{prov: tp, assumeYes: true})
	cmd.SetOut(io.Discard)

	settings := findCmd(cmd, "settings")
	if settings == nil {
		t.Fatal("expected ssl settings command")
	}
	for _, action := range []string{"show", "set"} {
		if findCmd(settings, action) == nil {
			t.Errorf("ssl settings missing %q", action)
		}
	}
	for _, action := range []string{"list", "create", "delete"} {
		if findCmd(settings, action) != nil {
			t.Errorf("ssl settings singleton should not have %q", action)
		}
	}

	for _, name := range []string{"authentication-rule", "portal"} {
		resource := findCmd(cmd, name)
		if resource == nil {
			t.Errorf("expected ssl %s command", name)
			continue
		}
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if findCmd(resource, action) == nil {
				t.Errorf("ssl %s missing %q", name, action)
			}
		}
	}

	create := findCmd(cmd, "authentication-rule", "create")
	if create == nil {
		t.Fatal("ssl authentication-rule create command missing")
	}
	if err := create.RunE(create, nil); err != nil {
		t.Errorf("ssl authentication-rule create without id: %v", err)
	}
	if tp.lastPath != "vpn.ssl/settings/authentication-rule" {
		t.Errorf("ssl authentication-rule create path = %q", tp.lastPath)
	}
}
