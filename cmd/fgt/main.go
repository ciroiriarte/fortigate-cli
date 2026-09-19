// SPDX-License-Identifier: Apache-2.0

// Command fgt is the fortigate-cli binary: a remote-first CLI for FortiOS.
package main

import (
	"os"

	"github.com/ciroiriarte/fortigate-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
