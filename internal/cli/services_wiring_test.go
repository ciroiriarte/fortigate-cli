package cli

import "testing"

// TestServicesWiredIntoRoot guards the root wiring of the log group and the
// system service subcommands (both attached in root.go / system.go, not in the
// per-file builder tests) — a dropped or misplaced AddCommand would be caught.
func TestServicesWiredIntoRoot(t *testing.T) {
	root := NewRootCmd()
	for _, path := range [][]string{
		{"log", "setting"},
		{"log", "syslogd", "setting"},
		{"log", "syslogd", "filter"},
		{"log", "fortianalyzer", "setting"},
		{"system", "ntp"},
		{"system", "dhcp", "server"},
		{"system", "snmp", "sysinfo"},
		{"system", "snmp", "community"},
		{"system", "snmp", "user"},
		{"system", "backup"},
		{"system", "restore"},
	} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd.Name() != path[len(path)-1] {
			t.Errorf("find %q = %v, %v; want leaf %q", path, cmd.Name(), err, path[len(path)-1])
		}
	}
}
