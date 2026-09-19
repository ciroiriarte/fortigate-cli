package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/version"
)

func newVersionCmd(_ *app) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and supported FortiOS matrix",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintln(os.Stdout, version.String())
			fmt.Fprintf(os.Stdout, "supported: %s\n", version.SupportedFortiOS)
			fmt.Fprintln(os.Stdout, version.Disclaimer)
			return nil
		},
	}
}
