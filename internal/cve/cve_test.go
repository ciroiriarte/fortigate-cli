package cve

import "testing"

func TestCPEFor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"v7.4.3", "cpe:2.3:o:fortinet:fortios:7.4.3:*:*:*:*:*:*:*"},
		{"7.4.3", "cpe:2.3:o:fortinet:fortios:7.4.3:*:*:*:*:*:*:*"},
		{"  v7.6.1  ", "cpe:2.3:o:fortinet:fortios:7.6.1:*:*:*:*:*:*:*"},
		{"7.4.3-build1396", "cpe:2.3:o:fortinet:fortios:7.4.3:*:*:*:*:*:*:*"},
	}
	for _, c := range cases {
		if got := CPEFor(c.in); got != c.want {
			t.Errorf("CPEFor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMinSeverity(t *testing.T) {
	cases := []struct {
		in   string
		want int
		err  bool
	}{
		{"", 0, false},
		{"none", 0, false},
		{"low", 1, false},
		{"medium", 2, false},
		{"HIGH", 3, false},
		{"critical", 4, false},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := ParseMinSeverity(c.in)
		if (err != nil) != c.err {
			t.Errorf("ParseMinSeverity(%q) err = %v, want err=%v", c.in, err, c.err)
		}
		if got != c.want {
			t.Errorf("ParseMinSeverity(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestMeetsSeverity(t *testing.T) {
	high := CVE{Severity: "HIGH"}
	low := CVE{Severity: "LOW"}
	none := CVE{Severity: "NONE"}
	if !MeetsSeverity(high, 3) { // HIGH >= high threshold
		t.Error("HIGH should meet a high threshold")
	}
	if MeetsSeverity(low, 3) {
		t.Error("LOW should not meet a high threshold")
	}
	if !MeetsSeverity(none, 0) { // none threshold admits everything
		t.Error("NONE should meet a none threshold")
	}
	if MeetsSeverity(none, 1) {
		t.Error("NONE should not meet a low threshold")
	}
}

func TestAffectedByRangeAllBounds(t *testing.T) {
	cases := []struct {
		name    string
		running string
		rng     versionRange
		want    bool
	}{
		{"startIncluding-below", "7.3.9", versionRange{startIncluding: "7.4.0"}, false},
		{"startIncluding-at", "7.4.0", versionRange{startIncluding: "7.4.0"}, true},
		{"startExcluding-at", "7.4.0", versionRange{startExcluding: "7.4.0"}, false},
		{"startExcluding-above", "7.4.1", versionRange{startExcluding: "7.4.0"}, true},
		{"endExcluding-at", "7.4.4", versionRange{endExcluding: "7.4.4"}, false},
		{"endExcluding-below", "7.4.3", versionRange{endExcluding: "7.4.4"}, true},
		{"endIncluding-at", "7.2.5", versionRange{endIncluding: "7.2.5"}, true},
		{"endIncluding-above", "7.2.6", versionRange{endIncluding: "7.2.5"}, false},
		{"full-window-inside", "7.4.2", versionRange{startIncluding: "7.4.0", endExcluding: "7.4.4"}, true},
		{"exact-match", "7.4.3", versionRange{exact: "7.4.3"}, true},
		{"exact-mismatch", "7.4.4", versionRange{exact: "7.4.3"}, false},
		{"no-bounds-no-exact", "7.4.3", versionRange{}, true},
	}
	for _, c := range cases {
		if got := affectedByRange(c.running, c.rng); got != c.want {
			t.Errorf("%s: affectedByRange(%q, %+v) = %v, want %v", c.name, c.running, c.rng, got, c.want)
		}
	}
}

func TestSeverityRankOrder(t *testing.T) {
	if !(SeverityRank("CRITICAL") > SeverityRank("HIGH") &&
		SeverityRank("HIGH") > SeverityRank("MEDIUM") &&
		SeverityRank("MEDIUM") > SeverityRank("LOW") &&
		SeverityRank("LOW") > SeverityRank("NONE")) {
		t.Error("severity ranks must be strictly ordered CRITICAL>HIGH>MEDIUM>LOW>NONE")
	}
	if SeverityRank("nonsense") != 0 {
		t.Error("unknown severity should rank 0")
	}
}
