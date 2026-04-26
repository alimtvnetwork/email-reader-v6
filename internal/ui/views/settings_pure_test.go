// settings_pure_test.go covers the framework-agnostic helpers from
// settings.go that don't require a running Fyne canvas. Building the full
// form needs cgo + display libs, so we restrict these tests to the input
// projection / validation logic that runs everywhere.
//go:build nofyne

package views

import (
	"testing"

	"github.com/lovable/email-read/internal/core"
)

func Test_ReadSettingsInput_Valid(t *testing.T) {
	w := &settingsWidgets{
		themeSelect:   stubSelect("Light"),
		pollEntry:     stubEntry("7"),
		chromeEntry:   stubEntry("/usr/bin/chromium"),
		densitySelect: stubSelect("Compact"),
		initial: core.SettingsSnapshot{
			BrowserOverride:       core.BrowserOverride{IncognitoArg: "--incognito"},
			OpenUrlAllowedSchemes: []string{"https"},
			AllowLocalhostUrls:    true,
			AutoStartWatch:        false,
		},
	}
	in, err := readSettingsInput(w)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if in.PollSeconds != 7 {
		t.Errorf("PollSeconds=%d want 7", in.PollSeconds)
	}
	if in.Theme != core.ThemeLight {
		t.Errorf("Theme=%v want Light", in.Theme)
	}
	if in.BrowserOverride.ChromePath != "/usr/bin/chromium" {
		t.Errorf("ChromePath=%q", in.BrowserOverride.ChromePath)
	}
	// Invariants preserved from initial snapshot:
	if in.BrowserOverride.IncognitoArg != "--incognito" {
		t.Errorf("IncognitoArg lost: %q", in.BrowserOverride.IncognitoArg)
	}
	if !in.AllowLocalhostUrls || in.AutoStartWatch {
		t.Errorf("preserved bools wrong: %+v", in)
	}
}

func Test_ReadSettingsInput_PollOutOfRange(t *testing.T) {
	cases := []string{"0", "61", "-3", "abc", ""}
	for _, c := range cases {
		w := &settingsWidgets{
			themeSelect: stubSelect("Dark"),
			pollEntry:   stubEntry(c),
			chromeEntry: stubEntry(""),
		}
		if _, err := readSettingsInput(w); err == nil {
			t.Errorf("poll=%q expected error, got nil", c)
		}
	}
}

func Test_DensityLabel_Roundtrip(t *testing.T) {
	if got := densityLabel(0); got != "Comfortable" {
		t.Errorf("Comfortable label=%q", got)
	}
	if got := densityLabel(1); got != "Compact" {
		t.Errorf("Compact label=%q", got)
	}
}
