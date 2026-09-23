package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/output"
	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// A resource declares a curated cobra command tree over one cmdb object. The
// generic CRUD implementation lives here, so each object is just data: its path,
// mkey, the columns to list, and the fields to expose as create/set flags. Any
// field not modeled as a typed flag is still reachable via `--set field=value`,
// and any object at all is reachable via `fgt api`.
type resource struct {
	use     string   // command name, e.g. "address"
	short   string   // one-line help
	aliases []string // command aliases, e.g. {"addr"}
	path    string   // cmdb path, e.g. "firewall/address" or "firewall.service/custom"
	mkey    string   // mkey field name, e.g. "name" or "policyid"
	mkeyArg string   // arg label in help, e.g. "name" (defaults to mkey)
	numeric bool     // mkey is numeric (policyid/seq-num) and may be auto-assigned
	single  bool     // singleton object (no mkey): only show/set
	columns []column // list/show table columns
	fields  []fieldSpec
}

type column struct {
	field  string // cmdb field name
	header string // table header (defaults to upper(field))
}

type fieldKind int

const (
	kindString    fieldKind = iota // scalar string/number, sent as-is
	kindRefList                    // comma-separated refs, sent as [{"name":x},...]
	kindRangeList                  // comma-separated values, sent as [{"range":x},...]
)

type fieldSpec struct {
	name  string // cmdb field name
	flag  string // CLI flag (defaults to name)
	usage string // flag help
	kind  fieldKind
}

func (f fieldSpec) flagName() string {
	if f.flag != "" {
		return f.flag
	}
	return f.name
}

func (r resource) mkeyLabel() string {
	if r.mkeyArg != "" {
		return r.mkeyArg
	}
	return r.mkey
}

// newResourceCmd builds the full command tree for a resource.
func (a *app) newResourceCmd(r resource) *cobra.Command {
	root := &cobra.Command{Use: r.use, Short: r.short, Aliases: r.aliases}
	if r.single {
		root.AddCommand(a.resShow(r), a.resSet(r))
	} else {
		root.AddCommand(a.resList(r), a.resShow(r), a.resCreate(r), a.resSet(r), a.resDelete(r))
	}
	return root
}

func (a *app) resList(r resource) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List " + r.use + " objects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			objs, err := p.CmdbList(cmd.Context(), r.path)
			if err != nil {
				return err
			}
			t := output.Tabular{Columns: r.headers(), Raw: objs}
			for _, o := range objs {
				t.Rows = append(t.Rows, r.row(o))
			}
			return a.render(t)
		},
	}
}

func (a *app) resShow(r resource) *cobra.Command {
	use := "show"
	args := cobra.NoArgs
	if !r.single {
		use = "show <" + r.mkeyLabel() + ">"
		args = cobra.ExactArgs(1)
	}
	return &cobra.Command{
		Use:   use,
		Short: "Show one " + r.use + " object",
		Args:  args,
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			mkey := ""
			if !r.single {
				mkey = cmdArgs[0]
			}
			obj, err := p.CmdbGet(cmd.Context(), r.path, mkey)
			if err != nil {
				return err
			}
			return a.render(keyValueTable(obj))
		},
	}
}

func (a *app) resCreate(r resource) *cobra.Command {
	vals := map[string]*string{}
	cmd := &cobra.Command{
		Use:   "create [" + r.mkeyLabel() + "]",
		Short: "Create a " + r.use + " object",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			body := r.body(cmd, vals)
			if len(cmdArgs) == 1 {
				body[r.mkey] = r.mkeyValue(cmdArgs[0])
			} else if !r.numeric {
				return fmt.Errorf("a %s is required to create a %s", r.mkeyLabel(), r.use)
			}
			if err := confirmWrite(a, "CREATE", r.path); err != nil {
				return err
			}
			mkey, err := p.CmdbCreate(cmd.Context(), r.path, body)
			if err != nil {
				return err
			}
			if mkey == "" && len(cmdArgs) == 1 {
				mkey = cmdArgs[0]
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s %s\n", r.use, mkey)
			return nil
		},
	}
	r.registerFlags(cmd, vals)
	return cmd
}

func (a *app) resSet(r resource) *cobra.Command {
	vals := map[string]*string{}
	use := "set <" + r.mkeyLabel() + ">"
	args := cobra.ExactArgs(1)
	if r.single {
		use = "set"
		args = cobra.NoArgs
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: "Update fields on a " + r.use + " object",
		Args:  args,
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			body := r.body(cmd, vals)
			if len(body) == 0 {
				return fmt.Errorf("nothing to set: pass at least one field flag or --set key=value")
			}
			mkey := ""
			if !r.single {
				mkey = cmdArgs[0]
			}
			if err := confirmWrite(a, "SET", r.path); err != nil {
				return err
			}
			if err := p.CmdbUpdate(cmd.Context(), r.path, mkey, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s %s\n", r.use, mkey)
			return nil
		},
	}
	r.registerFlags(cmd, vals)
	return cmd
}

func (a *app) resDelete(r resource) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <" + r.mkeyLabel() + ">",
		Aliases: []string{"del", "rm"},
		Short:   "Delete a " + r.use + " object",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			p, err := a.Provider()
			if err != nil {
				return err
			}
			if err := confirmWrite(a, "DELETE", r.path+"/"+cmdArgs[0]); err != nil {
				return err
			}
			if err := p.CmdbDelete(cmd.Context(), r.path, cmdArgs[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s %s\n", r.use, cmdArgs[0])
			return nil
		},
	}
}

// --- helpers -------------------------------------------------------------

// setFlagName is the generic passthrough flag present on every create/set.
const setFlagName = "set"

func (r resource) registerFlags(cmd *cobra.Command, vals map[string]*string) {
	for _, f := range r.fields {
		v := new(string)
		vals[f.name] = v
		cmd.Flags().StringVar(v, f.flagName(), "", f.usage)
	}
	// generic escape hatch for any field not modeled above
	cmd.Flags().StringArray(setFlagName, nil, "set an arbitrary field: --set key=value (repeatable; value may be a,b for ref lists)")
}

// body builds the request object from the changed typed flags plus any --set.
func (r resource) body(cmd *cobra.Command, vals map[string]*string) provider.Object {
	body := provider.Object{}
	for _, f := range r.fields {
		if !cmd.Flags().Changed(f.flagName()) {
			continue
		}
		body[f.name] = encodeField(f.kind, *vals[f.name])
	}
	if raw, err := cmd.Flags().GetStringArray(setFlagName); err == nil {
		for _, kv := range raw {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			// a value with commas becomes a ref list; otherwise a scalar string
			if strings.Contains(v, ",") {
				body[k] = refList(v)
			} else {
				body[k] = v
			}
		}
	}
	return body
}

func encodeField(kind fieldKind, v string) any {
	switch kind {
	case kindRefList:
		return refList(v)
	case kindRangeList:
		return refListKeyed(v, "range")
	default:
		return v
	}
}

// refList turns "a, b, c" into [{"name":"a"},{"name":"b"},{"name":"c"}], the
// shape FortiOS expects for reference child-tables (srcaddr, member, ...).
func refList(csv string) []map[string]string {
	return refListKeyed(csv, "name")
}

// refListKeyed turns "a, b, c" into [{key:"a"},{key:"b"},{key:"c"}] — the shape
// FortiOS expects for child-tables whose element key is not "name" (e.g. VIP
// mappedip uses "range").
func refListKeyed(csv, key string) []map[string]string {
	out := []map[string]string{}
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, map[string]string{key: s})
		}
	}
	return out
}

// mkeyValue coerces a positional mkey to the type FortiOS expects.
func (r resource) mkeyValue(s string) any {
	if r.numeric {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return s
}

func (r resource) headers() []string {
	if len(r.columns) == 0 {
		return []string{strings.ToUpper(r.mkey)}
	}
	h := make([]string, len(r.columns))
	for i, c := range r.columns {
		if c.header != "" {
			h[i] = c.header
		} else {
			h[i] = strings.ToUpper(c.field)
		}
	}
	return h
}

func (r resource) row(o provider.Object) []string {
	if len(r.columns) == 0 {
		return []string{cellValue(o[r.mkey])}
	}
	row := make([]string, len(r.columns))
	for i, c := range r.columns {
		row[i] = cellValue(o[c.field])
	}
	return row
}

// cellValue renders a JSON value for a table cell: scalars as-is, reference
// child-tables ([{name:...}]) as a comma-joined list.
func cellValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		var names []string
		for _, e := range t {
			names = append(names, cellValue(e))
		}
		return strings.Join(names, ",")
	case map[string]any:
		return childCellValue(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// childCellValue renders a single child-table element (a JSON object) for a
// table cell by pulling the value under the first recognized key. FortiOS keys
// child-tables by "name" for most objects, but by "range" (VIP mappedip),
// "subnet" (policy src/dst), or "interface-name" (aggregate members) elsewhere.
func childCellValue(m map[string]any) string {
	for _, k := range []string{"name", "range", "subnet", "interface-name", "seq-num"} {
		if v, ok := m[k]; ok {
			return cellValue(v)
		}
	}
	return fmt.Sprintf("%v", m)
}

// objectsTable renders a slice of untyped objects as a table whose columns are
// the sorted union of all keys present. Used for monitor reads whose per-build
// field set is not modeled as a Go struct; Raw carries the objects unchanged so
// json/yaml stays faithful to the device.
func objectsTable(objs []provider.Object) output.Tabular {
	seen := map[string]struct{}{}
	for _, o := range objs {
		for k := range o {
			seen[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	cols := make([]string, len(keys))
	for i, k := range keys {
		cols[i] = strings.ToUpper(k)
	}
	t := output.Tabular{Columns: cols, Raw: objs}
	for _, o := range objs {
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = cellValue(o[k])
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// keyValueTable renders a single object as a sorted KEY/VALUE table, keeping the
// full object as Raw so json/yaml output is complete.
func keyValueTable(obj provider.Object) output.Tabular {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t := output.Tabular{Columns: []string{"FIELD", "VALUE"}, Raw: obj}
	for _, k := range keys {
		t.Rows = append(t.Rows, []string{k, cellValue(obj[k])})
	}
	return t
}
