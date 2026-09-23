package cli

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// TestCVECommandTree asserts the cve group exposes list + check with the
// documented persistent flags.
func TestCVECommandTree(t *testing.T) {
	a := &app{}
	root := newCVECmd(a)

	if findCmd(root, "list") == nil {
		t.Error("cve missing list subcommand")
	}
	check := findCmd(root, "check")
	if check == nil {
		t.Fatal("cve missing check subcommand")
	}

	for _, f := range []string{"source", "min-severity", "nvd-api-key", "exit-code"} {
		if root.PersistentFlags().Lookup(f) == nil {
			t.Errorf("cve missing persistent flag --%s", f)
		}
	}

	// The long help must carry the PSIRT disclaimer, the exit-code contract, and
	// the CIRCL fallback caveat.
	for _, sub := range []string{"PSIRT", "unofficial", "NVD", "Exit codes", "CIRCL fallback"} {
		if !strings.Contains(root.Long, sub) {
			t.Errorf("cve long help missing %q, got: %q", sub, root.Long)
		}
	}
}

// cveCmdArgs builds the flag pointers a cve subcommand constructor needs.
func cveCmdArgs(source string, exitCode int) (*string, *string, *string, *int) {
	src := source
	minSev := "none"
	key := ""
	code := exitCode
	return &src, &minSev, &key, &code
}

// TestCVECheckValidatesID asserts an obviously malformed CVE id is rejected
// before any device/network call.
func TestCVECheckValidatesID(t *testing.T) {
	src, minSev, key, code := cveCmdArgs("auto", 0)
	cmd := newCVECheckCmd(&app{}, src, minSev, key, code)
	if err := cmd.RunE(cmd, []string{"not-a-cve"}); err == nil {
		t.Error("expected an error for a malformed CVE id")
	}
}

// TestCVEEmptyVersionRefuses asserts both subcommands refuse to assess CVEs when
// the device reports no FortiOS version (finding 1: avoid a false "not affected").
// The refusal happens before any external source call, so no network is used.
func TestCVEEmptyVersionRefuses(t *testing.T) {
	tp := &testProvider{deviceVersion: ""} // no version reported

	src, minSev, key, code := cveCmdArgs("auto", 0)
	list := newCVEListCmd(&app{prov: tp}, src, minSev, key, code)
	list.SetOut(io.Discard)
	list.SetErr(io.Discard)
	if err := list.RunE(list, nil); err == nil {
		t.Error("cve list must error on an empty device version, not report not-affected")
	}

	src2, minSev2, key2, code2 := cveCmdArgs("auto", 0)
	check := newCVECheckCmd(&app{prov: tp}, src2, minSev2, key2, code2)
	check.SetOut(io.Discard)
	check.SetErr(io.Discard)
	if err := check.RunE(check, []string{"CVE-2024-1111"}); err == nil {
		t.Error("cve check must error on an empty device version, not report not-affected")
	}
}

// TestVulnExitError covers the --exit-code contract (finding 3): a found
// vulnerability with a non-zero code yields that exit code; otherwise nil.
func TestVulnExitError(t *testing.T) {
	// Affected + non-zero code => exitCodeError carrying that code.
	err := vulnExitError(true, 10)
	var ece exitCodeError
	if !errors.As(err, &ece) || ece.code != 10 {
		t.Fatalf("vulnExitError(true, 10) = %v, want exitCodeError{10}", err)
	}
	if got := ExitCodeFor(err); got != 10 {
		t.Errorf("ExitCodeFor(exitCodeError{10}) = %d, want 10", got)
	}

	// Not found => no signal even with a code set.
	if err := vulnExitError(false, 10); err != nil {
		t.Errorf("vulnExitError(false, 10) = %v, want nil", err)
	}
	// Found but exit-code disabled (0) => no signal (default: always exit 0).
	if err := vulnExitError(true, 0); err != nil {
		t.Errorf("vulnExitError(true, 0) = %v, want nil", err)
	}
}
