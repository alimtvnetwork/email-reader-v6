package views

import (
	"reflect"
	"testing"
)

func TestExtractUrls(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"none", "no urls here", nil},
		{"single", "see https://example.com now", []string{"https://example.com"}},
		{"trim punct", "go to https://example.com/x).", []string{"https://example.com/x"}},
		{"dedupe", "https://a.test and https://a.test again", []string{"https://a.test"}},
		{"order preserved", "first https://b.test then https://a.test",
			[]string{"https://b.test", "https://a.test"}},
		{"http and https", "http://x.test and https://y.test",
			[]string{"http://x.test", "https://y.test"}},
		{"in html", `<a href="https://h.test/path">link</a>`, []string{"https://h.test/path"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractUrls(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractUrls(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTrimTrailingPunct(t *testing.T) {
	if got := trimTrailingPunct("https://x.test/a)).,"); got != "https://x.test/a" {
		t.Errorf("got %q", got)
	}
	if got := trimTrailingPunct(""); got != "" {
		t.Errorf("empty → %q", got)
	}
	if got := trimTrailingPunct("...."); got != "" {
		t.Errorf("all punct → %q", got)
	}
}
