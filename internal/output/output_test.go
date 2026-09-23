package output

import (
	"bytes"
	"strings"
	"testing"
)

func render(t *testing.T, tab Tabular, opts Options) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(&b, tab, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return b.String()
}

func TestTableTruncatesWideCells(t *testing.T) {
	tab := Tabular{
		Columns: []string{"NAME", "MEMBERS"},
		Rows:    [][]string{{"pol1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, // 30 a's
	}
	out := render(t, tab, Options{Format: Table, MaxColWidth: 10})
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis in truncated output, got:\n%s", out)
	}
	// The truncated cell must be exactly 10 display columns (9 runes + "…").
	var widest int
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		for _, f := range strings.Fields(line) {
			if n := len([]rune(f)); n > widest {
				widest = n
			}
		}
	}
	if widest > 10 {
		t.Errorf("cell wider than MaxColWidth: widest field = %d runes", widest)
	}
}

func TestNoTruncationWhenZero(t *testing.T) {
	long := strings.Repeat("x", 60)
	tab := Tabular{Columns: []string{"C"}, Rows: [][]string{{long}}}
	if out := render(t, tab, Options{Format: Table, MaxColWidth: 0}); !strings.Contains(out, long) {
		t.Errorf("MaxColWidth 0 should not truncate")
	}
}

func TestStructuredFormatsUnaffectedByMaxColWidth(t *testing.T) {
	long := strings.Repeat("x", 60)
	// CSV/value render from the (pre-stringified) rows; json/yaml from Raw. None
	// may be truncated — they are the stable scripting contract.
	cases := []struct {
		name string
		f    Format
		tab  Tabular
	}{
		{"json", JSON, Tabular{Columns: []string{"C"}, Rows: [][]string{{long}}, Raw: map[string]string{"c": long}}},
		{"yaml", YAML, Tabular{Columns: []string{"C"}, Rows: [][]string{{long}}, Raw: map[string]string{"c": long}}},
		{"csv", CSV, Tabular{Columns: []string{"C"}, Rows: [][]string{{long}}}},
		{"value", Value, Tabular{Columns: []string{"C"}, Rows: [][]string{{long}}}},
	}
	for _, c := range cases {
		if out := render(t, c.tab, Options{Format: c.f, MaxColWidth: 10}); !strings.Contains(out, long) {
			t.Errorf("%s output must be faithful (untruncated); got:\n%s", c.name, out)
		}
	}
}

func TestTableAlignmentWithTruncatedCell(t *testing.T) {
	// A truncated multibyte "…" cell must still pad so the next column aligns.
	tab := Tabular{
		Columns: []string{"A", "B"},
		Rows: [][]string{
			{"0123456789abc", "x"}, // long -> truncated to 10 runes
			{"short", "y"},
		},
	}
	out := render(t, tab, Options{Format: Table, NoHeaders: true, MaxColWidth: 10})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 rows, got %d:\n%s", len(lines), out)
	}
	// First col padded to 10 display cols + 3-space gap => "B" cell starts at index 13.
	if r := []rune(lines[0]); string(r[:10]) != "012345678…" || string(r[13:]) != "x" {
		t.Errorf("truncated row misaligned: %q", lines[0])
	}
	if r := []rune(lines[1]); string(r[13:]) != "y" {
		t.Errorf("short row misaligned: %q", lines[1])
	}
}
