package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newBackupCmd downloads the device configuration (monitor surface). Backup is a
// privileged operation — a read-only admin/token is rejected by FortiOS; use a
// super_admin token or session auth.
func newBackupCmd(a *app) *cobra.Command {
	var scope, vdom, out string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Download the device configuration (monitor surface)",
		Long: "Download the running configuration as a text file. Requires a\n" +
			"privileged admin (read-only credentials are rejected by FortiOS).\n" +
			"Writes to --output, or stdout when omitted.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			cfg, err := p.ConfigBackup(cmd.Context(), scope, vdom)
			if err != nil {
				return err
			}
			if out == "" {
				_, err = cmd.OutOrStdout().Write(cfg)
				return err
			}
			if err := os.WriteFile(out, cfg, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s\n", len(cfg), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "backup scope: global|vdom")
	cmd.Flags().StringVar(&vdom, "vdom", "", "VDOM to back up (scope vdom)")
	cmd.Flags().StringVarP(&out, "output", "O", "", "write config to this file (default: stdout)")
	return cmd
}

// newRestoreCmd uploads a configuration to the device. This REPLACES the running
// config and usually reboots the unit — hence a confirmation gate.
func newRestoreCmd(a *app) *cobra.Command {
	var scope, vdom string
	cmd := &cobra.Command{
		Use:   "restore <config-file>",
		Short: "Restore a device configuration (REPLACES running config; usually reboots)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			cfg, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			if err := confirmWrite(a, "RESTORE", "system config (replaces running config; the unit usually reboots)"); err != nil {
				return err
			}
			if err := p.ConfigRestore(cmd.Context(), scope, vdom, cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "config restore submitted")
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "restore scope: global|vdom")
	cmd.Flags().StringVar(&vdom, "vdom", "", "VDOM to restore (scope vdom)")
	return cmd
}
