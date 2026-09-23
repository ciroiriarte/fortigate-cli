package cli

import (
	"strings"
	"testing"

	"github.com/ciroiriarte/fortigate-cli/internal/provider"
)

// sampleSchema mimics the shape of GET cmdb/<path>?action=schema.
func sampleSchema() provider.Object {
	return provider.Object{
		"mkey":      "policyid",
		"mkey_type": "integer",
		"category":  "table",
		"children": map[string]any{
			"policyid":     map[string]any{"type": "integer"},
			"q_origin_key": map[string]any{"type": "integer"},
			"action":       map[string]any{"type": "option", "default": "deny", "options": []any{map[string]any{"name": "accept"}, map[string]any{"name": "deny"}}},
			"srcaddr":      map[string]any{"multiple_values": true, "mkey": "name", "children": map[string]any{"name": map[string]any{"type": "string"}}},
			"mappedip":     map[string]any{"multiple_values": true, "mkey": "range", "children": map[string]any{"range": map[string]any{"type": "string"}}},
			"members":      map[string]any{"multiple_values": true, "mkey": "seq-num", "children": map[string]any{"seq-num": map[string]any{"type": "integer"}}},
		},
	}
}

func TestSchemaChildren(t *testing.T) {
	fields := schemaChildren(sampleSchema())
	got := map[string]schemaField{}
	for _, f := range fields {
		got[f.name] = f
	}
	if _, ok := got["q_origin_key"]; ok {
		t.Error("q_origin_key should be skipped")
	}
	if a := got["action"]; a.typ != "option" || strings.Join(a.options, "|") != "accept|deny" || a.def != "deny" {
		t.Errorf("action parsed wrong: %+v", a)
	}
	if s := got["srcaddr"]; !s.child || s.childKey != "name" {
		t.Errorf("srcaddr should be a name-keyed child-table: %+v", s)
	}
	if m := got["mappedip"]; !m.child || m.childKey != "range" {
		t.Errorf("mappedip should be a range-keyed child-table: %+v", m)
	}
}

func TestGenResource(t *testing.T) {
	out := genResource("firewall/policy", sampleSchema())
	// numeric mkey
	if !strings.Contains(out, `mkey: "policyid", mkeyArg: "policyid", numeric: true`) {
		t.Errorf("expected numeric mkey decl, got:\n%s", out)
	}
	// the mkey itself is not emitted as a flag
	if strings.Contains(out, `{name: "policyid"`) {
		t.Errorf("mkey should not be a fieldSpec:\n%s", out)
	}
	// option field carries its enum as usage
	if !strings.Contains(out, `{name: "action", usage: "accept|deny"}`) {
		t.Errorf("expected action enum usage, got:\n%s", out)
	}
	// name-keyed child -> kindRefList; range-keyed -> kindRangeList
	if !strings.Contains(out, `{name: "srcaddr", kind: kindRefList`) {
		t.Errorf("expected srcaddr kindRefList, got:\n%s", out)
	}
	if !strings.Contains(out, `{name: "mappedip", kind: kindRangeList`) {
		t.Errorf("expected mappedip kindRangeList, got:\n%s", out)
	}
	// a non-name/range child key (seq-num) is left to --set with a TODO, not a wrong kind
	if !strings.Contains(out, `TODO`) || strings.Contains(out, `{name: "members", kind:`) {
		t.Errorf("expected members left as TODO, got:\n%s", out)
	}
}

func TestSchemaTable(t *testing.T) {
	tab := schemaTable(sampleSchema())
	row := map[string][]string{}
	for _, r := range tab.Rows {
		row[r[0]] = r
	}
	// policyid is the mkey -> MKEY column marks it with the type.
	if r := row["policyid"]; r == nil || !strings.Contains(r[5], "integer") {
		t.Errorf("policyid should be marked as the integer mkey: %v", r)
	}
	// child-tables render their key in the TYPE column.
	if r := row["mappedip"]; r == nil || r[1] != "table[range]" {
		t.Errorf("mappedip type = %v, want table[range]", r)
	}
	// enum options render in the OPTIONS column.
	if r := row["action"]; r == nil || r[3] != "accept|deny" {
		t.Errorf("action options = %v, want accept|deny", r)
	}
	// Raw carries the full schema for json/yaml.
	if tab.Raw == nil {
		t.Error("schemaTable must set Raw for json/yaml")
	}
}

func TestGenResourceSingleton(t *testing.T) {
	sch := provider.Object{"mkey": "", "category": "complete", "children": map[string]any{
		"status": map[string]any{"type": "option", "options": []any{map[string]any{"name": "enable"}}},
	}}
	if out := genResource("system/dns", sch); !strings.Contains(out, "single: true") {
		t.Errorf("expected singleton decl, got:\n%s", out)
	}
}
