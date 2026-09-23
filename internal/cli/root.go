// Package cli builds the cobra command tree — the curated, stable UX surface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/config"
	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
	"github.com/ciroiriarte/fortigate-cli/internal/version"

	// Register the FortiGate backend.
	_ "github.com/ciroiriarte/fortigate-cli/internal/provider/fortigate"
)

// app is the shared runtime carried across commands.
type app struct {
	// global flags
	profile     string
	context     string
	server      string
	vdom        string
	token       string
	user        string
	password    string
	format      string
	columns     []string
	noHeaders   bool
	wide        bool
	sortBy      string
	insecure    bool
	fingerprint string
	debug       bool
	assumeYes   bool

	settings *config.Settings
	prov     provider.Provider
}

// Provider lazily builds and caches the backend from resolved settings.
func (a *app) Provider() (provider.Provider, error) {
	if a.prov != nil {
		return a.prov, nil
	}
	s, err := a.resolvedSettings()
	if err != nil {
		return nil, err
	}
	a.settings = s
	if a.format == "" {
		a.format = s.Output
	}
	p, err := provider.New(s, a.debug)
	if err != nil {
		return nil, err
	}
	a.prov = p
	return p, nil
}

// resolvedSettings resolves effective settings (config + env + flags) without
// building a client.
func (a *app) resolvedSettings() (*config.Settings, error) {
	f, err := config.Load(config.DefaultPath())
	if err != nil {
		return nil, err
	}
	return config.Resolve(f, a.overrides())
}

// overrides builds the config Overrides from the parsed global flags.
func (a *app) overrides() config.Overrides {
	ov := config.Overrides{
		Profile:     a.profile,
		Context:     a.context,
		Server:      a.server,
		VDOM:        a.vdom,
		TokenSecret: a.token,
		User:        a.user,
		Password:    a.password,
		Output:      a.format,
		Fingerprint: a.fingerprint,
	}
	if a.insecure {
		ov.Insecure = &a.insecure
	}
	return ov
}

// outputOptions builds render options from the resolved global flags.
func (a *app) outputOptions() (output.Options, error) {
	fmtStr := a.format
	if fmtStr == "" {
		fmtStr = "table"
	}
	f, err := output.ParseFormat(fmtStr)
	if err != nil {
		return output.Options{}, err
	}
	// Cap table columns by default so a single wide field (e.g. a policy's
	// address list) can't blow out the layout; --wide restores full width.
	// json/yaml/csv are never truncated.
	maxCol := 40
	if a.wide {
		maxCol = 0
	}
	return output.Options{
		Format:      f,
		Columns:     a.columns,
		NoHeaders:   a.noHeaders,
		SortBy:      a.sortBy,
		MaxColWidth: maxCol,
	}, nil
}

// render writes a Tabular using the resolved global output options.
func (a *app) render(t output.Tabular) error {
	opts, err := a.outputOptions()
	if err != nil {
		return err
	}
	return output.Render(os.Stdout, t, opts)
}

// NewRootCmd assembles the full command tree.
func NewRootCmd() *cobra.Command {
	a := &app{}

	root := &cobra.Command{
		Use:   "fgt",
		Short: "Remote CLI for FortiGate / FortiOS (and FortiLink-managed FortiSwitch)",
		Long: "fgt is a remote-first, OpenStack-Client-inspired CLI for managing FortiGate\n" +
			"firewalls entirely over the FortiOS REST API (/api/v2/cmdb for config,\n" +
			"/api/v2/monitor for status). Nothing is installed on the device.\n\n" +
			version.Disclaimer,
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if a.format != "" {
				if _, err := output.ParseFormat(a.format); err != nil {
					return err
				}
			}
			return nil
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")

	pf := root.PersistentFlags()
	pf.StringVar(&a.profile, "profile", "", "config profile to use")
	pf.StringVar(&a.context, "context", "", "config context to use")
	pf.StringVar(&a.server, "server", "", "FortiGate API base URL (e.g. https://fw.example.com)")
	pf.StringVar(&a.vdom, "vdom", "", "target VDOM (default: device global/root)")
	pf.StringVar(&a.token, "token", "", "REST API token (prefer a config secret_ref or FGT_CLI_TOKEN)")
	pf.StringVar(&a.user, "user", "", "admin username for session auth (or FGT_CLI_USER)")
	pf.StringVar(&a.password, "password", "", "admin password for session auth (prefer a secret_ref or FGT_CLI_PASSWORD)")
	pf.StringVarP(&a.format, "format", "o", "", "output format: table|json|yaml|csv|value")
	pf.StringArrayVarP(&a.columns, "column", "c", nil, "select/order output columns (repeatable)")
	pf.BoolVar(&a.noHeaders, "no-headers", false, "omit table/csv headers")
	pf.BoolVar(&a.wide, "wide", false, "do not truncate wide table columns")
	pf.StringVar(&a.sortBy, "sort", "", "sort by column (NAME[:asc|desc])")
	pf.BoolVar(&a.insecure, "insecure", false, "skip TLS verification (footgun)")
	pf.StringVar(&a.fingerprint, "tls-fingerprint", "", "pin the server cert SHA-256 fingerprint")
	pf.BoolVar(&a.debug, "debug", false, "log request/response metadata to stderr")
	pf.BoolVarP(&a.assumeYes, "yes", "y", false, "assume yes for destructive confirmations")

	root.AddCommand(
		newSystemCmd(a),
		newFirewallCmd(a),
		newRouterCmd(a),
		newSDWANCmd(a),
		newUserCmd(a),
		newLogCmd(a),
		newVpnCmd(a),
		newSwitchCmd(a),
		newAPICmd(a),
		newSchemaCmd(a),
		newConfigCmd(a),
		newVersionCmd(a),
	)
	return root
}

// Execute runs the root command and maps errors to exit codes.
func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return ExitCodeFor(err)
	}
	return 0
}
