// settings_logic_test.go pins the framework-agnostic Settings helpers.
// These run under the headless `-tags nofyne` build matrix.
package views

import (
	"testing"

	"github.com/lovable/email-read/internal/core"
)

func Test_ParsePollSeconds_Bounds(t *testing.T) {
	good := map[string]uint16{"1": 1, "3": 3, "30": 30, "60": 60}
	for in, want := range good {
		got, err := ParsePollSeconds(in)
		if err != nil || got != want {
			t.Errorf("ParsePollSeconds(%q)=%d,%v want %d,nil", in, got, err, want)
		}
	}
	bad := []string{"0", "61", "-1", "", "abc", "3.5"}
	for _, in := range bad {
		if _, err := ParsePollSeconds(in); err == nil {
			t.Errorf("ParsePollSeconds(%q) expected error", in)
		}
	}
}

func Test_DensityChoice_Roundtrip(t *testing.T) {
	if DensityLabelFor(0) != "Comfortable" {
		t.Errorf("0 should be Comfortable, got %q", DensityLabelFor(0))
	}
	if DensityLabelFor(1) != "Compact" {
		t.Errorf("1 should be Compact, got %q", DensityLabelFor(1))
	}
	if DensityLabelFor(99) != "Comfortable" {
		t.Errorf("unknown density should default Comfortable")
	}
	if ParseDensityChoice("Compact") != 1 {
		t.Errorf("Compact should parse to 1")
	}
	if ParseDensityChoice("Comfortable") != 0 {
		t.Errorf("Comfortable should parse to 0")
	}
	if ParseDensityChoice("anything-else") != 0 {
		t.Errorf("unknown label should default to 0")
	}
}

func Test_ProjectSettingsInput_PreservesInvariants(t *testing.T) {
	prev := core.SettingsSnapshot{
		BrowserOverride:       core.BrowserOverride{IncognitoArg: "--incognito"},
		OpenUrlAllowedSchemes: []string{"https", "mailto"},
		AllowLocalhostUrls:    true,
		AutoStartWatch:        false,
	}
	in := ProjectSettingsInput("Light", 7, "/opt/chrome", 30, prev)
	if in.Theme != core.ThemeLight {
		t.Errorf("Theme=%v want Light", in.Theme)
	}
	if in.PollSeconds != 7 {
		t.Errorf("PollSeconds=%d want 7", in.PollSeconds)
	}
	if in.BrowserOverride.ChromePath != "/opt/chrome" {
		t.Errorf("ChromePath wrong: %q", in.BrowserOverride.ChromePath)
	}
	if in.BrowserOverride.IncognitoArg != "--incognito" {
		t.Errorf("IncognitoArg lost: %q", in.BrowserOverride.IncognitoArg)
	}
	if len(in.OpenUrlAllowedSchemes) != 2 || in.OpenUrlAllowedSchemes[0] != "https" {
		t.Errorf("schemes lost: %+v", in.OpenUrlAllowedSchemes)
	}
	if !in.AllowLocalhostUrls || in.AutoStartWatch {
		t.Errorf("bools lost: %+v", in)
	}
	if in.OpenUrlsRetentionDays != 30 {
		t.Errorf("OpenUrlsRetentionDays=%d want 30", in.OpenUrlsRetentionDays)
	}
}

func Test_ProjectSettingsInput_UnknownThemeFallsBackDark(t *testing.T) {
	in := ProjectSettingsInput("nonsense", 3, "", 90, core.SettingsSnapshot{})
	if in.Theme != core.ThemeDark {
		t.Errorf("unknown theme should fall back to Dark, got %v", in.Theme)
	}
}

func Test_ParseRetentionDays_Bounds(t *testing.T) {
	good := map[string]uint16{"0": 0, "1": 1, "30": 30, "90": 90, "3650": 3650}
	for in, want := range good {
		got, err := ParseRetentionDays(in)
		if err != nil || got != want {
			t.Errorf("ParseRetentionDays(%q)=%d,%v want %d,nil", in, got, err, want)
		}
	}
	bad := []string{"-1", "3651", "9999", "", "abc", "3.5"}
	for _, in := range bad {
		if _, err := ParseRetentionDays(in); err == nil {
			t.Errorf("ParseRetentionDays(%q) expected error", in)
		}
	}
}

func Test_ProjectSettingsInput_RetentionZeroDisablesPruning(t *testing.T) {
	// Round-trips the spec semantic that 0 = never prune.
	in := ProjectSettingsInput("Dark", 3, "", 0, core.SettingsSnapshot{})
	if in.OpenUrlsRetentionDays != 0 {
		t.Errorf("0 should round-trip as 0 (disabled), got %d", in.OpenUrlsRetentionDays)
	}
}
