package cli

import "testing"

func TestVpnIPsecCommandTree(t *testing.T) {
	root := NewRootCmd()
	for _, path := range [][]string{
		{"vpn", "ipsec", "phase1-interface"},
		{"vpn", "ipsec", "phase2-interface"},
	} {
		resource, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %q: %v", path, err)
		}
		for _, action := range []string{"list", "show", "create", "set", "delete"} {
			if cmd, _, err := resource.Find([]string{action}); err != nil || cmd.Name() != action {
				t.Errorf("find %q %q = %v, %v, want %q command", path, action, cmd, err, action)
			}
		}
	}
}
