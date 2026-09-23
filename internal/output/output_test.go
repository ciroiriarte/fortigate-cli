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

func TestJSONUnaffectedByMaxColWidth(t *testing.T) {
	long := strings.Repeat("x", 60)
	tab := Tabular{Columns: []string{"C"}, Rows: [][]string{{long}}, Raw: map[string]string{"c": long}}
	if out := render(t, tab, Options{Format: JSON, MaxColWidth: 10}); !strings.Contains(out, long) {
		t.Errorf("json output must be faithful (untruncated); got:\n%s", out)
	}
}
