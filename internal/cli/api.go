package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newAPICmd(a *app) *cobra.Command {
	var (
		data     []string
		bodyFlag string
	)
	cmd := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Make a raw authenticated API call (escape hatch)",
		Long: "Issue an arbitrary call against the FortiOS REST API, handling auth, base\n" +
			"URL, TLS and VDOM for you. The path is relative to /api/v2/, so use\n" +
			"cmdb/... for configuration and monitor/... for status. Use this for\n" +
			"endpoints the curated commands don't cover yet — every endpoint is reachable.\n\n" +
			"For a mutating method (POST/PUT/DELETE) you are prompted to confirm first\n" +
			"(pass --yes to skip). Write bodies are JSON: pass --body '<json>' / --body\n" +
			"@file.json, or build a flat object with repeated --data key=value.",
		Example: "  fgt api GET cmdb/firewall/address\n" +
			"  fgt api GET monitor/system/interface\n" +
			"  fgt api POST cmdb/firewall/address --data name=web --data subnet=\"10.0.0.0 255.255.255.0\"\n" +
			"  fgt api PUT cmdb/firewall/address/web --body '{\"comment\":\"updated\"}'\n" +
			"  fgt api DELETE cmdb/firewall/address/web",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			method := strings.ToUpper(args[0])
			path := args[1]

			// Reads put --data in the query string; writes assemble a JSON body.
			write := !isReadMethod(method)
			params := url.Values{}
			var body []byte

			switch {
			case bodyFlag != "":
				body, err = readBody(bodyFlag)
				if err != nil {
					return err
				}
			case write && len(data) > 0:
				obj := map[string]string{}
				for _, kv := range data {
					k, v, ok := strings.Cut(kv, "=")
					if !ok {
						return fmt.Errorf("invalid --data %q: expected key=value", kv)
					}
					obj[k] = v
				}
				if body, err = json.Marshal(obj); err != nil {
					return err
				}
			default:
				for _, kv := range data {
					k, v, ok := strings.Cut(kv, "=")
					if !ok {
						return fmt.Errorf("invalid --data %q: expected key=value", kv)
					}
					params.Add(k, v)
				}
			}

			if write {
				if err := confirmWrite(a, method, path); err != nil {
					return err
				}
			}

			out, err := p.Raw(cmd.Context(), method, path, params, body)
			if err != nil {
				return err
			}
			os.Stdout.Write(out)
			if len(out) > 0 && out[len(out)-1] != '\n' {
				fmt.Fprintln(os.Stdout)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&data, "data", "d", nil, "request parameter key=value (repeatable)")
	cmd.Flags().StringVar(&bodyFlag, "body", "", "raw JSON request body, or @file.json")
	return cmd
}

func isReadMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

// readBody returns the literal string, or the contents of the named file when
// the value starts with '@'.
func readBody(v string) ([]byte, error) {
	if strings.HasPrefix(v, "@") {
		return os.ReadFile(strings.TrimPrefix(v, "@"))
	}
	return []byte(v), nil
}

// confirmWrite prompts before a mutating call unless --yes was given.
func confirmWrite(a *app, method, path string) error {
	if a.assumeYes {
		return nil
	}
	fmt.Fprintf(os.Stderr, "About to %s %s — continue? [y/N] ", method, path)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return errCanceled
	}
}
