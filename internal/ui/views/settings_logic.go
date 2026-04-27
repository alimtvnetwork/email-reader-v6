// settings_logic.go contains the framework-agnostic helpers used by the
// Settings view (settings.go). Splitting them out lets headless CI (which
// builds with `-tags nofyne`) verify input parsing, density mapping, and
// the SettingsInput projection without linking against Fyne / cgo.
//
// settings.go composes these helpers with widget.Entry / widget.Select.
package views

import (
	"strconv"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// densityChoice is the on-wire label form used by the Density Select.
// Defining named constants keeps the UI layer and the tests in lockstep.
type densityChoice string

const (
	DensityChoiceComfortable densityChoice = "Comfortable"
	DensityChoiceCompact     densityChoice = "Compact"
)

// ParsePollSeconds validates the user's poll-interval entry. Returns the
// uint16 value or a friendly error suitable for the status line.
func ParsePollSeconds(raw string) (uint16, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 60 {
		return 0, errtrace.New("poll interval must be 1–60 seconds")
	}
	return uint16(n), nil
}

// ParseRetentionDays validates the user's OpenedUrls retention entry.
// Returns the uint16 value (0 = never prune) or a friendly error.
// Bounds match core.validateRetentionDays: [0, 3650].
func ParseRetentionDays(raw string) (uint16, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 3650 {
		return 0, errtrace.New("retention must be 0–3650 days (0 = never prune)")
	}
	return uint16(n), nil
}

// ParseVacuumHourLocal validates the weekly-VACUUM hour-of-day entry.
// Range [0, 23] per spec/23-app-database/04 §5.
func ParseVacuumHourLocal(raw string) (uint8, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 23 {
		return 0, errtrace.New("vacuum hour must be 0–23 (24-hour clock)")
	}
	return uint8(n), nil
}

// ParseWalCheckpointHours validates the WAL-checkpoint cadence entry.
// Range [1, 168] per spec/23-app-database/04 §5.
func ParseWalCheckpointHours(raw string) (uint8, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 168 {
		return 0, errtrace.New("WAL checkpoint hours must be 1–168 (1h..1 week)")
	}
	return uint8(n), nil
}

// ParsePruneBatchSize validates the per-batch row count for chunked
// retention deletes. Range [100, 50000] per spec/23-app-database/04 §5.
func ParsePruneBatchSize(raw string) (uint32, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 100 || n > 50000 {
		return 0, errtrace.New("prune batch size must be 100–50000")
	}
	return uint32(n), nil
}

// WeekdayLabels returns the canonical Sunday-first weekday names used by
// the Settings dropdown. Matches core.ParseWeekday.
func WeekdayLabels() []string {
	return []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
}

// ParseWeekdayLabel maps a dropdown label back to time.Weekday's int form
// (Sunday=0..Saturday=6). Unknown labels default to 0 (Sunday).
func ParseWeekdayLabel(label string) int {
	for i, w := range WeekdayLabels() {
		if w == label {
			return i
		}
	}
	return 0
}

// DensityLabelFor maps a core.Density value (its int form) to the Select
// label. We accept an int rather than the core.Density type so this file
// stays import-light. core.DensityComfortable=1, core.DensityCompact=2.
// Any other value (including the legacy 0 zero-value) → Comfortable.
func DensityLabelFor(d int) string {
	if d == int(core.DensityCompact) {
		return string(DensityChoiceCompact)
	}
	return string(DensityChoiceComfortable)
}

// ParseDensityChoice returns the core.Density int form for a Select label.
// Unknown labels default to Comfortable.
func ParseDensityChoice(label string) int {
	if label == string(DensityChoiceCompact) {
		return int(core.DensityCompact)
	}
	return int(core.DensityComfortable)
}

// CoreDensityToThemeDensity translates a core.Density int form
// (Comfortable=1, Compact=2) into the theme.Density int form
// (Comfortable=0, Compact=1) consumed by theme.SetDensity. Pure so it's
// testable under -tags nofyne.
func CoreDensityToThemeDensity(coreDensity int) int {
	if coreDensity == int(core.DensityCompact) {
		return 1 // theme.DensityCompact
	}
	return 0 // theme.DensityComfortable
}

// MaintenanceFields bundles the four §5 knobs to keep ProjectSettingsInput
// readable and to give the Settings view a single value to thread through.
type MaintenanceFields struct {
	WeekdayLabel    string
	HourLocal       uint8
	WalHours        uint8
	PruneBatchSize  uint32
}

// ProjectSettingsInput merges user-edited fields with the invariant slice
// of the previous snapshot to produce a SettingsInput suitable for Save.
// Pure: no IO, no globals, no widget references.
//
// retentionDays is the user-edited OpenedUrls retention value (0 disables
// pruning). Other fields not exposed in the form (IncognitoArg,
// OpenUrlAllowedSchemes, AllowLocalhostUrls, AutoStartWatch) are carried
// forward from prev so a Save on this view never silently mutates them.
func ProjectSettingsInput(
	themeLabel string,
	pollSecs uint16,
	chromePath string,
	retentionDays uint16,
	maint MaintenanceFields,
	prev core.SettingsSnapshot,
	densityLabel string,
) core.SettingsInput {
	mode, _ := core.ParseThemeMode(themeLabel)
	weekday, ok := core.ParseWeekday(maint.WeekdayLabel)
	if !ok {
		weekday = prev.WeeklyVacuumOn
	}
	density, _ := core.ParseDensity(densityLabel)
	return core.SettingsInput{
		PollSeconds: pollSecs,
		Theme:       mode,
		Density:     density,
		BrowserOverride: core.BrowserOverride{
			ChromePath:   chromePath,
			IncognitoArg: prev.BrowserOverride.IncognitoArg,
		},
		OpenUrlAllowedSchemes: prev.OpenUrlAllowedSchemes,
		AllowLocalhostUrls:    prev.AllowLocalhostUrls,
		AutoStartWatch:        prev.AutoStartWatch,
		OpenUrlsRetentionDays: retentionDays,
		WeeklyVacuumOn:        weekday,
		WeeklyVacuumHourLocal: maint.HourLocal,
		WalCheckpointHours:    maint.WalHours,
		PruneBatchSize:        maint.PruneBatchSize,
	}
}
