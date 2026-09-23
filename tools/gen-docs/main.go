// Command gen-docs generates the man pages and shell-completion scripts for fgt
// from the cobra command tree. Run via `make docs`; the output is committed so
// packagers and users don't need the Go toolchain.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/ciroiriarte/fortigate-cli/internal/cli"
)

const (
	manDir         = "docs/man"
	completionsDir = "contrib/completions"
)

func main() {
	root := cli.NewRootCmd()
	// Deterministic output (no per-run date/version) so the committed docs only
	// change when the command tree does.
	root.DisableAutoGenTag = true
	root.Version = "" // keep the injected build metadata out of the generated docs

	if err := run(root); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
}

func run(root *cobra.Command) error {
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		return err
	}
	header := &doc.GenManHeader{
		Title:   "FGT",
		Section: "1",
		Source:  "fgt",
		Manual:  "fgt Manual",
	}
	if err := doc.GenManTree(root, header, manDir); err != nil {
		return fmt.Errorf("man pages: %w", err)
	}

	if err := os.MkdirAll(completionsDir, 0o755); err != nil {
		return err
	}
	completions := []struct {
		file string
		gen  func(string) error
	}{
		{"fgt.bash", func(p string) error { return root.GenBashCompletionFileV2(p, true) }},
		{"fgt.zsh", root.GenZshCompletionFile},
		{"_fgt.fish", func(p string) error { return root.GenFishCompletionFile(p, true) }},
	}
	for _, c := range completions {
		if err := c.gen(filepath.Join(completionsDir, c.file)); err != nil {
			return fmt.Errorf("%s: %w", c.file, err)
		}
	}

	fmt.Printf("generated man pages -> %s/ and completions -> %s/\n", manDir, completionsDir)
	return nil
}
