package cli

import (
	"io"
	"reflect"
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
	// A ref-list flag must encode as [{"name":...}] and the dotted sub-path must
	// be hit verbatim.
	if err := create.Flags().Set("groups", "APP-VPN-GTPY,APP-VPN-TIPS"); err != nil {
		t.Fatal(err)
	}
	if err := create.RunE(create, nil); err != nil {
		t.Errorf("ssl authentication-rule create without id: %v", err)
	}
	if tp.lastPath != "vpn.ssl/settings/authentication-rule" {
		t.Errorf("ssl authentication-rule create path = %q", tp.lastPath)
	}
	wantGroups := []map[string]string{{"name": "APP-VPN-GTPY"}, {"name": "APP-VPN-TIPS"}}
	if !reflect.DeepEqual(tp.lastObj["groups"], wantGroups) {
		t.Errorf("groups encoding = %v, want %v", tp.lastObj["groups"], wantGroups)
	}

	// The portal resource must target the dotted vpn.ssl.web category.
	pcreate := findCmd(cmd, "portal", "create")
	if pcreate == nil {
		t.Fatal("ssl portal create command missing")
	}
	if err := pcreate.RunE(pcreate, []string{"test-portal"}); err != nil {
		t.Errorf("ssl portal create: %v", err)
	}
	if tp.lastPath != "vpn.ssl.web/portal" {
		t.Errorf("ssl portal create path = %q, want vpn.ssl.web/portal", tp.lastPath)
	}
}
