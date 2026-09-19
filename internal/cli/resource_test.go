package cli

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

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
