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
