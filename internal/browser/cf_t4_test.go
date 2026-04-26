// cf_t4_test.go locks CF-T4: per-browser auto-pick of the incognito flag
// vs. an explicit user override. The override (when non-empty) MUST win
// over the basename-based auto-pick; when empty, basename selection
// returns "-private-window" for Firefox and "--incognito" for every
// Chromium-family binary.
//
// Spec: spec/21-app/02-features/06-tools/99-consistency-report.md CF-T4.
package browser

import "testing"

func TestCF_T4_IncognitoArg_OverrideVsAuto(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		override string
		want     string
	}{
		{"firefox-auto", "/usr/bin/firefox", "", "-private-window"},
		{"chrome-auto", "/usr/bin/google-chrome", "", "--incognito"},
		{"chromium-auto", "/usr/bin/chromium-browser", "", "--incognito"},
		{"edge-auto", "/usr/bin/microsoft-edge", "", "--incognito"},
		{"brave-auto", "/usr/bin/brave-browser", "", "--incognito"},
		{"override-beats-firefox-auto", "/usr/bin/firefox", "--private", "--private"},
		{"override-beats-chrome-auto", "/usr/bin/google-chrome", "-private-window", "-private-window"},
		{"override-empty-string-falls-back", "/usr/bin/firefox", "", "-private-window"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickIncognitoArg(tc.path, tc.override)
			if got != tc.want {
				t.Fatalf("pickIncognitoArg(%q,%q) = %q, want %q", tc.path, tc.override, got, tc.want)
			}
		})
	}
}
