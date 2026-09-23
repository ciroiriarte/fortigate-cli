package cli

import "testing"

// TestUserWiredIntoRoot guards newUserCmd + its root wiring: every user resource
// (accounts and auth servers) must resolve from the assembled root tree. The
// per-file suites build the builder functions directly, so this covers the
// composition in user.go and root.go.
func TestUserWiredIntoRoot(t *testing.T) {
	root := NewRootCmd()
	for _, path := range [][]string{
		{"user", "local"},
		{"user", "group"},
		{"user", "ldap"},
		{"user", "radius"},
		{"user", "tacacs"},
	} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd.Name() != path[len(path)-1] {
			t.Errorf("find %q = %v, %v; want leaf %q", path, cmd.Name(), err, path[len(path)-1])
		}
		// each should expose the generic CRUD leaf set
		if c, _, err := cmd.Find([]string{"list"}); err != nil || c.Name() != "list" {
			t.Errorf("%q missing list subcommand", path)
		}
	}
}
