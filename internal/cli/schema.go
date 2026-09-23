package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// newSchemaCmd exposes a cmdb object's self-described field schema for the
// target build (GET cmdb/<path>?action=schema). Without flags it renders the
// field table; with --gen it emits a resource{} skeleton (see resource.go) to
// scaffold a new curated command — the maintainer trims and refines it.
func newSchemaCmd(a *app) *cobra.Command {
	var gen bool
	cmd := &cobra.Command{
		Use:   "schema <cmdb-path>",
		Short: "Show a cmdb object's field schema for the target build (M5 scaffolding)",
		Long: "Fetch the per-build field schema FortiOS describes about a cmdb object\n" +
			"(GET cmdb/<path>?action=schema). --gen emits a resource{} declaration\n" +
			"skeleton to scaffold a curated command. Example paths: firewall/address,\n" +
			"vpn.ssl/settings, system/sdwan/members.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			sch, err := p.Schema(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if gen {
				fmt.Fprint(cmd.OutOrStdout(), genResource(args[0], sch))
				return nil
			}
			return a.render(schemaTable(sch))
		},
	}
	cmd.Flags().BoolVar(&gen, "gen", false, "emit a resource{} declaration skeleton instead of the field table")
	return cmd
}

// schemaField is one flattened field from a schema's children map.
type schemaField struct {
	name     string
	typ      string
	multi    bool
	child    bool     // child-table (has its own children/mkey)
	childKey string   // child-table's mkey (e.g. name/range/seq-num)
	options  []string // enum values for option types
	def      string
}

// schemaChildren flattens and sorts the schema's children, skipping internals.
func schemaChildren(sch provider.Object) []schemaField {
	raw, _ := sch["children"].(map[string]any)
	out := make([]schemaField, 0, len(raw))
	for name, v := range raw {
		if name == "q_origin_key" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		f := schemaField{name: name}
		f.typ, _ = m["type"].(string)
		f.multi, _ = m["multiple_values"].(bool)
		if d, ok := m["default"]; ok {
			f.def = cellValue(d)
		}
		if _, hasKids := m["children"]; hasKids {
			f.child = true
			f.childKey, _ = m["mkey"].(string)
			f.typ = "table"
		}
		if opts, ok := m["options"].([]any); ok {
			for _, o := range opts {
				if om, ok := o.(map[string]any); ok {
					if n, ok := om["name"].(string); ok {
						f.options = append(f.options, n)
					}
				}
			}
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func schemaHeader(sch provider.Object) (mkey, mkeyType, category string) {
	mkey, _ = sch["mkey"].(string)
	mkeyType, _ = sch["mkey_type"].(string)
	category, _ = sch["category"].(string)
	return
}

// schemaTable renders the fields as a table; Raw keeps the full schema for json/yaml.
func schemaTable(sch provider.Object) output.Tabular {
	mkey, mkeyType, _ := schemaHeader(sch)
	t := output.Tabular{
		Columns: []string{"FIELD", "TYPE", "MULTI", "OPTIONS", "DEFAULT", "MKEY"},
		Raw:     sch,
	}
	for _, f := range schemaChildren(sch) {
		typ := f.typ
		if f.child && f.childKey != "" {
			typ = "table[" + f.childKey + "]"
		}
		isMkey := ""
		if f.name == mkey {
			isMkey = fmt.Sprintf("* (%s)", mkeyType)
		}
		t.Rows = append(t.Rows, []string{
			f.name, typ, boolMark(f.multi), strings.Join(f.options, "|"), f.def, isMkey,
		})
	}
	return t
}

func boolMark(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

// genResource emits a resource{} declaration skeleton derived from the schema.
// It is a starting point, not a finished command: the maintainer picks columns,
// trims fields to the common set, and verifies child-table kinds.
func genResource(path string, sch provider.Object) string {
	mkey, mkeyType, category := schemaHeader(sch)
	fields := schemaChildren(sch)

	use := path
	if i := strings.LastIndexAny(path, "/."); i >= 0 {
		use = path[i+1:]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// generated from %s?action=schema — review before use.\n", path)
	fmt.Fprintf(&b, "resource{\n")
	fmt.Fprintf(&b, "\tuse: %q, short: %q,\n", use, "Manage "+path+" objects")
	fmt.Fprintf(&b, "\tpath: %q,", path)
	switch {
	case mkey == "" || category == "complete":
		fmt.Fprintf(&b, " single: true,\n")
	case mkeyType == "integer":
		fmt.Fprintf(&b, " mkey: %q, mkeyArg: %q, numeric: true,\n", mkey, mkey)
	default:
		fmt.Fprintf(&b, " mkey: %q,\n", mkey)
	}
	fmt.Fprintf(&b, "\tfields: []fieldSpec{\n")
	for _, f := range fields {
		if f.name == mkey {
			continue // the mkey is the positional arg, not a flag
		}
		usage := strings.Join(f.options, "|")
		switch {
		case f.child:
			// The framework only models child-tables keyed by "name" (kindRefList)
			// or "range" (kindRangeList). Any other key — or an unknown one —
			// can't round-trip through a typed flag, so flag it for --set/api
			// rather than emit a wrong kind.
			note := "child-table"
			if f.childKey != "" {
				note = "child-table keyed by " + f.childKey
			}
			switch f.childKey {
			case "name":
				fmt.Fprintf(&b, "\t\t{name: %q, kind: kindRefList, usage: %q},\n", f.name, note)
			case "range":
				fmt.Fprintf(&b, "\t\t{name: %q, kind: kindRangeList, usage: %q},\n", f.name, note)
			case "interface-name":
				fmt.Fprintf(&b, "\t\t{name: %q, kind: kindIfaceList, usage: %q},\n", f.name, note)
			default:
				fmt.Fprintf(&b, "\t\t// TODO %q is a %s — reach via --set/api\n", f.name, note)
			}
		default:
			fmt.Fprintf(&b, "\t\t{name: %q, usage: %q},\n", f.name, usage)
		}
	}
	fmt.Fprintf(&b, "\t},\n")
	fmt.Fprintf(&b, "}\n")
	return b.String()
}
