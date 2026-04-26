// settings_logic.go contains the framework-agnostic helpers used by the
// Settings view (settings.go). Splitting them out lets headless CI (which
// builds with `-tags nofyne`) verify input parsing, density mapping, and
// the SettingsInput projection without linking against Fyne / cgo.
//
// settings.go composes these helpers with widget.Entry / widget.Select.
package views

import (
	"fmt"
	"strconv"

	"github.com/lovable/email-read/internal/core"
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
		return 0, fmt.Errorf("poll interval must be 1–60 seconds")
	}
	return uint16(n), nil
}

// ParseRetentionDays validates the user's OpenedUrls retention entry.
// Returns the uint16 value (0 = never prune) or a friendly error.
// Bounds match core.validateRetentionDays: [0, 3650].
func ParseRetentionDays(raw string) (uint16, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 3650 {
		return 0, fmt.Errorf("retention must be 0–3650 days (0 = never prune)")
	}
	return uint16(n), nil
}

// DensityLabelFor maps an internal density code (theme.Density's int form)
// to the Select label. We accept an int rather than the theme.Density type
// to keep this file fyne-free.
func DensityLabelFor(d int) string {
	if d == 1 {
		return string(DensityChoiceCompact)
	}
	return string(DensityChoiceComfortable)
}

// ParseDensityChoice returns the int form (matches theme.Density) for a
// label. Unknown labels default to Comfortable (0).
func ParseDensityChoice(label string) int {
	if label == string(DensityChoiceCompact) {
		return 1
	}
	return 0
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
	prev core.SettingsSnapshot,
) core.SettingsInput {
	mode, _ := core.ParseThemeMode(themeLabel)
	return core.SettingsInput{
		PollSeconds: pollSecs,
		Theme:       mode,
		BrowserOverride: core.BrowserOverride{
			ChromePath:   chromePath,
			IncognitoArg: prev.BrowserOverride.IncognitoArg,
		},
		OpenUrlAllowedSchemes: prev.OpenUrlAllowedSchemes,
		AllowLocalhostUrls:    prev.AllowLocalhostUrls,
		AutoStartWatch:        prev.AutoStartWatch,
		OpenUrlsRetentionDays: retentionDays,
	}
}
