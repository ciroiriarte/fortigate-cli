package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/config"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
)

func newConfigCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect CLI configuration and resolved settings",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "path",
			Short: "Print the config file path",
			Args:  cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				fmt.Fprintln(os.Stdout, config.DefaultPath())
				return nil
			},
		},
		&cobra.Command{
			Use:   "current",
			Short: "Show the effective settings (config + env + flags), secret redacted",
			Args:  cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				s, err := a.resolvedSettings()
				if err != nil {
					return err
				}
				t := output.Tabular{
					Columns: []string{"KEY", "VALUE"},
					Raw:     redact(s),
					Rows: [][]string{
						{"profile", s.ProfileName},
						{"context", s.ContextName},
						{"server", s.Server},
						{"vdom", orNone(s.VDOM)},
						{"auth_type", s.AuthType},
						{"token_name", orNone(s.TokenName)},
						{"secret", redactSecret(s.Secret)},
						{"output", s.Output},
						{"tls_insecure", fmt.Sprintf("%t", s.TLSInsecure)},
						{"tls_fingerprint", orNone(s.TLSFinger)},
					},
				}
				return a.render(t)
			},
		},
	)
	return cmd
}

// redact returns a copy of settings with the secret masked, for json/yaml output.
func redact(s *config.Settings) map[string]any {
	return map[string]any{
		"profile":         s.ProfileName,
		"context":         s.ContextName,
		"server":          s.Server,
		"vdom":            s.VDOM,
		"auth_type":       s.AuthType,
		"token_name":      s.TokenName,
		"secret":          redactSecret(s.Secret),
		"output":          s.Output,
		"tls_insecure":    s.TLSInsecure,
		"tls_fingerprint": s.TLSFinger,
	}
}

func redactSecret(s string) string {
	if s == "" {
		return "(unset)"
	}
	return "(set)"
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
