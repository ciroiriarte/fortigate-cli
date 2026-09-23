package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestRestoreGateAndOrdering locks in the safety of the destructive restore:
// with --yes the config is sent verbatim, and a missing file errors BEFORE any
// restore is attempted (confirmWrite runs after the local read, never after a
// partial device action).
func TestRestoreGateAndOrdering(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "cfg")
	if err := os.WriteFile(cfgFile, []byte("config x\nend\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// 1. --yes + a real file => ConfigRestore called with the file content.
	tp := &testProvider{}
	cmd := newRestoreCmd(&app{prov: tp, assumeYes: true})
	cmd.SetOut(io.Discard)
	if err := cmd.RunE(cmd, []string{cfgFile}); err != nil {
		t.Fatalf("restore with --yes: %v", err)
	}
	if !tp.restoreCalled {
		t.Fatal("ConfigRestore was not called")
	}
	if string(tp.restoreCfg) != "config x\nend\n" {
		t.Errorf("restore config = %q, want the file content", tp.restoreCfg)
	}
	if tp.restoreScope != "global" {
		t.Errorf("restore scope = %q, want default global", tp.restoreScope)
	}

	// 2. Missing file => error, and ConfigRestore must NOT be called.
	tp2 := &testProvider{}
	cmd2 := newRestoreCmd(&app{prov: tp2, assumeYes: true})
	cmd2.SetOut(io.Discard)
	if err := cmd2.RunE(cmd2, []string{filepath.Join(dir, "nope")}); err == nil {
		t.Error("expected an error for a missing config file")
	}
	if tp2.restoreCalled {
		t.Error("ConfigRestore must not run when the config file cannot be read")
	}
}
