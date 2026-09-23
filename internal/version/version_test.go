package version

import "testing"

func TestSupportsVersion(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		mm   string
	}{
		{"v7.4.12", true, "7.4"},
		{"7.4.12", true, "7.4"},
		{"v7.6.7", true, "7.6"},
		{"v8.0.1", true, "8.0"},
		{"v7.0.14", false, "7.0"}, // older minor, not in the matrix
		{"v6.4.9", false, "6.4"},
		{"garbage", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		got, mm := SupportsVersion(c.in)
		if got != c.want || mm != c.mm {
			t.Errorf("SupportsVersion(%q) = %v, %q; want %v, %q", c.in, got, mm, c.want, c.mm)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"7.4.3", "7.4.3", 0},
		{"v7.4.3", "7.4.3", 0}, // leading "v" tolerated on either side
		{"7.4.3", "v7.4.3", 0},
		{"7.4.3", "7.4.4", -1}, // patch diff
		{"7.4.4", "7.4.3", 1},
		{"7.4.3", "7.6.0", -1}, // minor diff
		{"8.0.0", "7.6.9", 1},  // major diff
		{"7.4", "7.4.0", 0},    // missing component == 0
		{"7.4.0", "7.4", 0},
		{"7.4.10", "7.4.9", 1},          // numeric (not lexical) compare
		{"7.4.3-build1396", "7.4.3", 0}, // build suffix ignored
		{"v7.4.3 beta", "7.4.3", 0},     // trailing suffix ignored
		{"7.4.3", "7.4.3.1", -1},        // extra component
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d; want %d", c.a, c.b, got, c.want)
		}
	}
}
