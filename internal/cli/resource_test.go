package cli

import (
	"io"
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
)

func TestSetUpsertFallback(t *testing.T) {
	r := resource{use: "addr", path: "firewall/address", mkey: "name",
		fields: []fieldSpec{{name: "comment"}}}

	// --upsert + a not-found update falls back to create carrying the mkey.
	tp := &testProvider{updateErr: &protocol.APIError{Kind: protocol.KindNotFound, Code: -3}}
	a := &app{prov: tp, assumeYes: true}
	cmd := a.resSet(r)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("comment", "x")
	_ = cmd.Flags().Set("upsert", "true")
	if err := cmd.RunE(cmd, []string{"web"}); err != nil {
		t.Fatalf("upsert set: %v", err)
	}
	if !tp.createCalled {
		t.Error("upsert should fall back to create on not-found")
	}
	if tp.lastObj["name"] != "web" {
		t.Errorf("upsert create body must carry the mkey, got %v", tp.lastObj)
	}

	// Without --upsert, the not-found error propagates and no create happens.
	tp2 := &testProvider{updateErr: &protocol.APIError{Kind: protocol.KindNotFound, Code: -3}}
	a2 := &app{prov: tp2, assumeYes: true}
	cmd2 := a2.resSet(r)
	cmd2.SetOut(io.Discard)
	_ = cmd2.Flags().Set("comment", "x")
	if err := cmd2.RunE(cmd2, []string{"web"}); err == nil {
		t.Error("without --upsert a not-found update should error")
	}
	if tp2.createCalled {
		t.Error("without --upsert there must be no create")
	}
}

func TestRefList(t *testing.T) {
	got := refList("all, web ,, db")
	want := []map[string]string{{"name": "all"}, {"name": "web"}, {"name": "db"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("refList = %v, want %v", got, want)
	}
	if len(refList("")) != 0 {
		t.Errorf("empty refList should be empty")
	}
}

func TestRangeListEncoding(t *testing.T) {
	got := encodeField(kindRangeList, "10.0.0.5, 10.0.0.6")
	want := []map[string]string{{"range": "10.0.0.5"}, {"range": "10.0.0.6"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kindRangeList = %v, want %v", got, want)
	}
}

func TestIfaceListEncoding(t *testing.T) {
	got := encodeField(kindIfaceList, "port1, port2")
	want := []map[string]string{{"interface-name": "port1"}, {"interface-name": "port2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kindIfaceList = %v, want %v", got, want)
	}
}

func TestChildCellValueNonNameKeys(t *testing.T) {
	// A child-table element keyed by "range" (VIP mappedip) must render as its
	// value, not Go map text.
	if got := cellValue([]any{map[string]any{"range": "10.0.0.5"}}); got != "10.0.0.5" {
		t.Errorf("range child = %q, want 10.0.0.5", got)
	}
	// A bare object keyed by "range" (mappedip can come back as a single object).
	if got := cellValue(map[string]any{"range": "10.0.0.5", "q_origin_key": "10.0.0.5"}); got != "10.0.0.5" {
		t.Errorf("bare range object = %q, want 10.0.0.5", got)
	}
	// name still wins when present.
	if got := cellValue([]any{map[string]any{"name": "all"}}); got != "all" {
		t.Errorf("name child = %q, want all", got)
	}
}

func TestCellValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"web", "web"},
		{true, "true"},
		{float64(17), "17"},   // integer-valued JSON number
		{float64(1.5), "1.5"}, // real
		{[]any{map[string]any{"name": "all"}, map[string]any{"name": "db"}}, "all,db"},
	}
	for _, c := range cases {
		if got := cellValue(c.in); got != c.want {
			t.Errorf("cellValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMkeyValueNumeric(t *testing.T) {
	r := resource{numeric: true}
	if v := r.mkeyValue("17"); v != 17 {
		t.Errorf("numeric mkey = %v (%T), want int 17", v, v)
	}
	rs := resource{}
	if v := rs.mkeyValue("web"); v != "web" {
		t.Errorf("string mkey = %v, want web", v)
	}
}

func TestBodyBuildsFromFlagsAndSet(t *testing.T) {
	r := resource{
		fields: []fieldSpec{
			{name: "type"},
			{name: "srcaddr", kind: kindRefList},
			{name: "comment"},
		},
	}
	vals := map[string]*string{}
	cmd := &cobra.Command{}
	r.registerFlags(cmd, vals)
	// type + srcaddr set via typed flags; comment left unset; extra field via --set
	if err := cmd.ParseFlags([]string{
		"--type", "ipmask",
		"--srcaddr", "all,any",
		"--set", "action=accept",
		"--set", "dstaddr=a,b",
	}); err != nil {
		t.Fatal(err)
	}
	body := r.body(cmd, vals)

	if body["type"] != "ipmask" {
		t.Errorf("type = %v, want ipmask", body["type"])
	}
	if _, ok := body["comment"]; ok {
		t.Errorf("unset field comment should be absent, got %v", body["comment"])
	}
	if body["action"] != "accept" {
		t.Errorf("--set scalar action = %v, want accept", body["action"])
	}
	// typed refList field
	wantSrc := []map[string]string{{"name": "all"}, {"name": "any"}}
	if !reflect.DeepEqual(body["srcaddr"], wantSrc) {
		t.Errorf("srcaddr = %v, want %v", body["srcaddr"], wantSrc)
	}
	// --set value with commas becomes a ref list too
	wantDst := []map[string]string{{"name": "a"}, {"name": "b"}}
	if !reflect.DeepEqual(body["dstaddr"], wantDst) {
		t.Errorf("dstaddr = %v, want %v", body["dstaddr"], wantDst)
	}
}
