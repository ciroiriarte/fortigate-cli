package cli

import "testing"

func TestLogCommandTree(t *testing.T) {
	root := newLogCmd(&app{})

	has := func(path ...string) bool {
		c, _, err := root.Find(path)
		return err == nil && c.Name() == path[len(path)-1]
	}

	singletons := [][]string{
		{"setting"},
		{"syslogd", "setting"},
		{"syslogd", "filter"},
		{"fortianalyzer", "setting"},
	}
	for _, resource := range singletons {
		if !has(append(resource, "show")...) || !has(append(resource, "set")...) {
			t.Errorf("log %v should have show and set", resource)
		}
		for _, verb := range []string{"list", "create", "delete"} {
			if has(append(resource, verb)...) {
				t.Errorf("log %v singleton should not have %q", resource, verb)
			}
		}
	}

	for _, path := range [][]string{
		{"syslogd", "setting"},
		{"syslogd", "filter"},
		{"fortianalyzer", "setting"},
	} {
		if !has(path...) {
			t.Errorf("log %v should resolve", path)
		}
	}
}
