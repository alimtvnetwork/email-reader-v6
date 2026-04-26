// tools_recent_opens_format_test.go is a headless table-driven test for
// the Recent-opens filter + format helpers. Runs under `-tags nofyne`.
package views

import (
	"strings"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/core"
)

func TestParseRecentOpensLimit(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 100},
		{"   ", 100},
		{"abc", 100},
		{"0", 100},
		{"-5", 100},
		{"50", 50},
		{"1000", 1000},
		{"5000", 1000},
	}
	for _, c := range cases {
		if got := ParseRecentOpensLimit(c.in); got != c.want {
			t.Errorf("ParseRecentOpensLimit(%q)=%d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildRecentOpensSpec(t *testing.T) {
	cases := []struct {
		name       string
		f          RecentOpensFilter
		wantAlias  string
		wantOrigin core.OpenUrlOrigin
		wantLimit  int
	}{
		{"empty", RecentOpensFilter{}, "", "", 100},
		{"all sentinel", RecentOpensFilter{Origin: "All"}, "", "", 100},
		{"alias trim", RecentOpensFilter{Alias: "  work  "}, "work", "", 100},
		{"manual", RecentOpensFilter{Origin: "manual", LimitStr: "25"}, "", core.OriginManual, 25},
		{"both", RecentOpensFilter{Alias: "w", Origin: "rule", LimitStr: "999"}, "w", core.OriginRule, 999},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := BuildRecentOpensSpec(c.f)
			if s.Alias != c.wantAlias {
				t.Errorf("Alias=%q, want %q", s.Alias, c.wantAlias)
			}
			if s.Origin != c.wantOrigin {
				t.Errorf("Origin=%q, want %q", s.Origin, c.wantOrigin)
			}
			if s.Limit != c.wantLimit {
				t.Errorf("Limit=%d, want %d", s.Limit, c.wantLimit)
			}
		})
	}
}

func TestFormatRecentOpensRow(t *testing.T) {
	at := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	cases := []struct {
		name     string
		row      core.OpenedUrlRow
		contains []string
	}{
		{
			"plain",
			core.OpenedUrlRow{Alias: "work", Origin: core.OriginManual, Url: "https://x.test/p", OpenedAt: at, RuleName: "r1"},
			[]string{"2026-04-26 10:30:00", "[work/manual]", "rule=r1", "https://x.test/p"},
		},
		{
			"flags",
			core.OpenedUrlRow{Alias: "w", Origin: core.OriginRule, Url: "https://y/", OpenedAt: at, IsDeduped: true, IsIncognito: true},
			[]string{"(deduped,incognito)", "rule=—"},
		},
		{
			"missing alias/origin",
			core.OpenedUrlRow{Url: "https://z/", OpenedAt: at},
			[]string{"[—/—]"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := FormatRecentOpensRow(c.row)
			for _, sub := range c.contains {
				if !strings.Contains(out, sub) {
					t.Errorf("row missing %q in %q", sub, out)
				}
			}
		})
	}
}

func TestFormatRecentOpensSummary(t *testing.T) {
	spec := core.OpenedUrlListSpec{Limit: 50}
	got := FormatRecentOpensSummary(spec, 7, 12*time.Millisecond)
	for _, sub := range []string{"7 row(s)", "all aliases", "all origins", "limit=50", "12ms"} {
		if !strings.Contains(got, sub) {
			t.Errorf("summary missing %q in %q", sub, got)
		}
	}
	spec2 := core.OpenedUrlListSpec{Alias: "work", Origin: core.OriginRule, Limit: 25}
	got2 := FormatRecentOpensSummary(spec2, 0, 0)
	for _, sub := range []string{"alias=work", "origin=rule", "limit=25"} {
		if !strings.Contains(got2, sub) {
			t.Errorf("summary2 missing %q in %q", sub, got2)
		}
	}
}
