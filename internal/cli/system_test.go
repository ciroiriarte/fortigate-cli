package cli

import "testing"

// TestSystemCommandTree asserts the system subcommands expose the intended
// verbs: interface and vdom get full CRUD, dns and ha are singletons (show/set,
// no list/create/delete), and ha additionally exposes a monitor `status` read.
func TestSystemCommandTree(t *testing.T) {
	a := &app{}
	root := newSystemCmd(a)

	has := func(path ...string) bool {
		c, _, err := root.Find(path)
		if err != nil {
			return false
		}
		// Find returns the deepest match; require an exact leaf hit.
		return c.Name() == path[len(path)-1]
	}

	crud := []string{"list", "show", "create", "set", "delete"}
	for _, v := range crud {
		if !has("interface", v) {
			t.Errorf("system interface missing %q", v)
		}
		if !has("vdom", v) {
			t.Errorf("system vdom missing %q", v)
		}
	}

	// dns singleton: show/set only.
	if !has("dns", "show") || !has("dns", "set") {
		t.Error("system dns should have show and set")
	}
	if has("dns", "list") || has("dns", "create") || has("dns", "delete") {
		t.Error("system dns singleton should not have list/create/delete")
	}

	// ha singleton config plus a status read; no list/create/delete.
	if !has("ha", "show") || !has("ha", "set") || !has("ha", "status") {
		t.Error("system ha should have show, set and status")
	}
	if has("ha", "list") || has("ha", "create") || has("ha", "delete") {
		t.Error("system ha singleton should not have list/create/delete")
	}
}
