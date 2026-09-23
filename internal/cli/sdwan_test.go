package cli

import (
	"io"
	"testing"
)

func TestSDWANCommandTree(t *testing.T) {
	tp := &testProvider{}
	cmd := newSDWANCmd(&app{prov: tp, assumeYes: true})
	cmd.SetOut(io.Discard)

	settings := findCmd(cmd, "settings")
	if settings == nil {
		t.Fatal("expected sdwan settings command")
	}
	for _, action := range []string{"show", "set"} {
		if findCmd(settings, action) == nil {
			t.Errorf("sdwan settings missing %q", action)
		}
	}
	for _, action := range []string{"list", "create", "delete"} {
		if findCmd(settings, action) != nil {
			t.Errorf("sdwan settings singleton should not have %q", action)
		}
	}

	for _, name := range []string{"zone", "member", "health-check", "service"} {
		resource := findCmd(cmd, name)
		if resource == nil {
			t.Errorf("expected sdwan %s command", name)
			continue
		}
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if findCmd(resource, action) == nil {
				t.Errorf("sdwan %s missing %q", name, action)
			}
		}
	}

	for _, name := range []string{"member", "service"} {
		create := findCmd(cmd, name, "create")
		if create == nil {
			t.Fatalf("sdwan %s create command missing", name)
		}
		if err := create.RunE(create, nil); err != nil {
			t.Errorf("sdwan %s create without mkey: %v", name, err)
		}
	}
}
