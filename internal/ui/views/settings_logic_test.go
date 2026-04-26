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
	// New semantic (Slice #42): DensityLabelFor / ParseDensityChoice operate
	// on core.Density's int form (Comfortable=1, Compact=2). The legacy
	// zero value still maps to Comfortable so callers built before the
	// migration don't render "Compact" by surprise.
	if DensityLabelFor(int(core.DensityComfortable)) != "Comfortable" {
		t.Errorf("DensityComfortable should map to Comfortable")
	}
	if DensityLabelFor(int(core.DensityCompact)) != "Compact" {
		t.Errorf("DensityCompact should map to Compact")
	}
	if DensityLabelFor(0) != "Comfortable" {
		t.Errorf("legacy zero value should default to Comfortable")
	}
	if DensityLabelFor(99) != "Comfortable" {
		t.Errorf("unknown density should default Comfortable")
	}
	if ParseDensityChoice("Compact") != int(core.DensityCompact) {
		t.Errorf("Compact should parse to DensityCompact")
	}
	if ParseDensityChoice("Comfortable") != int(core.DensityComfortable) {
		t.Errorf("Comfortable should parse to DensityComfortable")
	}
	if ParseDensityChoice("anything-else") != int(core.DensityComfortable) {
		t.Errorf("unknown label should default to Comfortable")
	}
}

// Test_CoreDensityToThemeDensity locks the bridge between core.Density
// (Comfortable=1, Compact=2) and theme.Density (Comfortable=0, Compact=1).
// The bridge is the only place this offset matters; getting it wrong here
// would silently flip the user's saved preference on every cold boot.
func Test_CoreDensityToThemeDensity(t *testing.T) {
	if got := CoreDensityToThemeDensity(int(core.DensityComfortable)); got != 0 {
		t.Errorf("Comfortable should map to theme 0, got %d", got)
	}
	if got := CoreDensityToThemeDensity(int(core.DensityCompact)); got != 1 {
		t.Errorf("Compact should map to theme 1, got %d", got)
	}
	if got := CoreDensityToThemeDensity(0); got != 0 {
		t.Errorf("legacy zero should map to theme 0 (Comfortable), got %d", got)
	}
}

func Test_ProjectSettingsInput_PreservesInvariants(t *testing.T) {
	prev := core.SettingsSnapshot{
		BrowserOverride:       core.BrowserOverride{IncognitoArg: "--incognito"},
		OpenUrlAllowedSchemes: []string{"https", "mailto"},
		AllowLocalhostUrls:    true,
		AutoStartWatch:        false,
	}
	maint := MaintenanceFields{WeekdayLabel: "Sunday", HourLocal: 3, WalHours: 6, PruneBatchSize: 5000}
	in := ProjectSettingsInput("Light", 7, "/opt/chrome", 30, maint, prev)
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
	maint := MaintenanceFields{WeekdayLabel: "Sunday", HourLocal: 3, WalHours: 6, PruneBatchSize: 5000}
	in := ProjectSettingsInput("nonsense", 3, "", 90, maint, core.SettingsSnapshot{})
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
	maint := MaintenanceFields{WeekdayLabel: "Sunday", HourLocal: 3, WalHours: 6, PruneBatchSize: 5000}
	in := ProjectSettingsInput("Dark", 3, "", 0, maint, core.SettingsSnapshot{})
	if in.OpenUrlsRetentionDays != 0 {
		t.Errorf("0 should round-trip as 0 (disabled), got %d", in.OpenUrlsRetentionDays)
	}
}

func Test_ParseVacuumHourLocal_Bounds(t *testing.T) {
	for _, in := range []string{"0", "3", "23"} {
		if _, err := ParseVacuumHourLocal(in); err != nil {
			t.Errorf("ParseVacuumHourLocal(%q) unexpected error: %v", in, err)
		}
	}
	for _, in := range []string{"-1", "24", "99", "", "abc"} {
		if _, err := ParseVacuumHourLocal(in); err == nil {
			t.Errorf("ParseVacuumHourLocal(%q) expected error", in)
		}
	}
}

func Test_ParseWalCheckpointHours_Bounds(t *testing.T) {
	for _, in := range []string{"1", "6", "168"} {
		if _, err := ParseWalCheckpointHours(in); err != nil {
			t.Errorf("ParseWalCheckpointHours(%q) unexpected error: %v", in, err)
		}
	}
	for _, in := range []string{"0", "169", "-1", "", "abc"} {
		if _, err := ParseWalCheckpointHours(in); err == nil {
			t.Errorf("ParseWalCheckpointHours(%q) expected error", in)
		}
	}
}

func Test_ParsePruneBatchSize_Bounds(t *testing.T) {
	for _, in := range []string{"100", "5000", "50000"} {
		if _, err := ParsePruneBatchSize(in); err != nil {
			t.Errorf("ParsePruneBatchSize(%q) unexpected error: %v", in, err)
		}
	}
	for _, in := range []string{"99", "50001", "0", "-5", "", "abc"} {
		if _, err := ParsePruneBatchSize(in); err == nil {
			t.Errorf("ParsePruneBatchSize(%q) expected error", in)
		}
	}
}

func Test_ParseWeekdayLabel_RoundTrip(t *testing.T) {
	labels := WeekdayLabels()
	if len(labels) != 7 {
		t.Fatalf("expected 7 weekday labels, got %d", len(labels))
	}
	for i, label := range labels {
		if got := ParseWeekdayLabel(label); got != i {
			t.Errorf("ParseWeekdayLabel(%q)=%d want %d", label, got, i)
		}
	}
	if got := ParseWeekdayLabel("nonsense"); got != 0 {
		t.Errorf("unknown label should default to 0 (Sunday), got %d", got)
	}
}

func Test_ProjectSettingsInput_MaintenanceFields(t *testing.T) {
	maint := MaintenanceFields{WeekdayLabel: "Wednesday", HourLocal: 4, WalHours: 12, PruneBatchSize: 2000}
	in := ProjectSettingsInput("Dark", 3, "", 90, maint, core.SettingsSnapshot{})
	if in.WeeklyVacuumOn != 3 { // time.Wednesday
		t.Errorf("WeeklyVacuumOn=%d want 3 (Wednesday)", in.WeeklyVacuumOn)
	}
	if in.WeeklyVacuumHourLocal != 4 || in.WalCheckpointHours != 12 || in.PruneBatchSize != 2000 {
		t.Errorf("maintenance fields lost: %+v", in)
	}
}
