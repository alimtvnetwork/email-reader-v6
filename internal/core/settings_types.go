// settings_types.go declares the public types and defaults for the Settings
// service per spec/21-app/02-features/07-settings/01-backend.md §3.
//
// Note: the on-disk JSON schema currently uses the legacy camelCase keys
// (`pollSeconds`, `chromePath`, `incognitoArg`) per `internal/config/config.go`
// to remain backward-compatible with existing user `data/config.json` files.
// The PascalCase migration described in the spec is tracked separately under
// Delta #1 (table rename) and will be done as a coordinated config migration.
package core

import (
	"time"

	"github.com/lovable/email-read/internal/config"
)

// SettingsSnapshot is the read-side projection returned by Get / Save / Reset.
// All path fields are absolute and read-only (populated from the config layer
// on every Get; ignored if a caller passes them in via SettingsInput).
type SettingsSnapshot struct {
	PollSeconds           uint16
	BrowserOverride       BrowserOverride
	Theme                 ThemeMode
	Density               Density
	OpenUrlAllowedSchemes []string // canonical lower-case, sorted, deduped
	AllowLocalhostUrls    bool
	AutoStartWatch        bool
	OpenUrlsRetentionDays uint16 // 0 = disabled (never prune); else age cutoff in days
	// Maintenance knobs (spec/23-app-database/04 §5).
	WeeklyVacuumOn        time.Weekday // 0..6 (Sunday..Saturday)
	WeeklyVacuumHourLocal uint8        // 0..23
	WalCheckpointHours    uint8        // 1..168
	PruneBatchSize        uint32       // 100..50000
	ConfigPath            string       // read-only, absolute
	DataDir               string       // read-only, absolute
	EmailArchiveDir       string       // read-only, absolute
	UpdatedAt             time.Time
}

// SettingsInput is the write-side payload for Save. Read-only display fields
// (paths, UpdatedAt) are intentionally absent.
type SettingsInput struct {
	PollSeconds           uint16
	BrowserOverride       BrowserOverride
	Theme                 ThemeMode
	Density               Density
	OpenUrlAllowedSchemes []string
	AllowLocalhostUrls    bool
	AutoStartWatch        bool
	OpenUrlsRetentionDays uint16
	// Maintenance knobs (spec/23-app-database/04 §5).
	WeeklyVacuumOn        time.Weekday
	WeeklyVacuumHourLocal uint8
	WalCheckpointHours    uint8
	PruneBatchSize        uint32
}

// BrowserOverride mirrors config.Browser but is owned by Settings so the UI
// has a stable type that is independent of the on-disk schema name.
type BrowserOverride struct {
	ChromePath   string // "" → auto-detect at launch time
	IncognitoArg string // "" → auto-pick per detected browser family
}

// ThemeMode is the in-memory enum form of the Ui.Theme JSON string.
type ThemeMode uint8

const (
	ThemeDark   ThemeMode = 1
	ThemeLight  ThemeMode = 2
	ThemeSystem ThemeMode = 3
)

// String returns the canonical JSON form ("Dark" / "Light" / "System").
func (t ThemeMode) String() string {
	switch t {
	case ThemeLight:
		return "Light"
	case ThemeSystem:
		return "System"
	default:
		return "Dark"
	}
}

// ParseThemeMode parses the canonical string form. Empty / unknown → ThemeDark
// to match the documented default behaviour.
func ParseThemeMode(s string) (ThemeMode, bool) {
	switch s {
	case "", "Dark":
		return ThemeDark, true
	case "Light":
		return ThemeLight, true
	case "System":
		return ThemeSystem, true
	}
	return ThemeDark, false
}

// Density is the in-memory enum form of the Ui.Density JSON string.
// Mirrors theme.Density (which lives in internal/ui/theme to avoid pulling
// the theme package into core); we keep our own enum so core has no
// dependency on the UI layer. The integer values must match
// theme.Density's int form so the UI bootstrap can cast directly.
type Density uint8

const (
	DensityComfortable Density = 1 // default
	DensityCompact     Density = 2
)

// String returns the canonical JSON form ("Comfortable" / "Compact").
func (d Density) String() string {
	if d == DensityCompact {
		return "Compact"
	}
	return "Comfortable"
}

// ParseDensity parses the canonical string form. Empty / unknown →
// DensityComfortable to match the documented default behaviour.
func ParseDensity(s string) (Density, bool) {
	switch s {
	case "", "Comfortable":
		return DensityComfortable, true
	case "Compact":
		return DensityCompact, true
	}
	return DensityComfortable, false
}

// ChromeDetection is the result of Settings.DetectChrome.
type ChromeDetection struct {
	Path   string
	Source ChromeDetectionSource
}

// ChromeDetectionSource identifies which probe step found the browser.
type ChromeDetectionSource uint8

const (
	ChromeFromConfig    ChromeDetectionSource = 1
	ChromeFromEnv       ChromeDetectionSource = 2
	ChromeFromOsDefault ChromeDetectionSource = 3
	ChromeFromPath      ChromeDetectionSource = 4
	ChromeNotFound      ChromeDetectionSource = 5
)

// String returns the enum name for log fields.
func (c ChromeDetectionSource) String() string {
	switch c {
	case ChromeFromConfig:
		return "Config"
	case ChromeFromEnv:
		return "Env"
	case ChromeFromOsDefault:
		return "OsDefault"
	case ChromeFromPath:
		return "Path"
	default:
		return "NotFound"
	}
}

// SettingsEvent is published on the Subscribe channel after a successful
// Save / Reset.
type SettingsEvent struct {
	Kind     SettingsEventKind
	Snapshot SettingsSnapshot
	At       time.Time
}

// SettingsEventKind discriminates Save vs Reset events.
type SettingsEventKind uint8

const (
	SettingsSaved        SettingsEventKind = 1
	SettingsResetApplied SettingsEventKind = 2
)

// DefaultSettingsInput returns the documented defaults from §3.
// OpenUrlsRetentionDays defaults to 90 per spec/23-app-database/04-retention-and-vacuum.md
// (the conservative blocked-decision retention; we apply it uniformly here
// because OpenedUrls v1 has no Decision column to split on yet).
func DefaultSettingsInput() SettingsInput {
	return SettingsInput{
		PollSeconds:           config.MinWatchPollSeconds,
		Theme:                 ThemeDark,
		Density:               DensityComfortable,
		OpenUrlAllowedSchemes: []string{"https"},
		AllowLocalhostUrls:    false,
		AutoStartWatch:        true,
		OpenUrlsRetentionDays: 90,
		// Maintenance defaults (spec/23-app-database/04 §5).
		WeeklyVacuumOn:        time.Sunday,
		WeeklyVacuumHourLocal: 3,
		WalCheckpointHours:    6,
		PruneBatchSize:        5000,
	}
}

// ParseWeekday parses the canonical capitalised weekday name
// ("Sunday".."Saturday"). Empty/unknown returns (Sunday, false).
func ParseWeekday(s string) (time.Weekday, bool) {
	switch s {
	case "Sunday":
		return time.Sunday, true
	case "Monday":
		return time.Monday, true
	case "Tuesday":
		return time.Tuesday, true
	case "Wednesday":
		return time.Wednesday, true
	case "Thursday":
		return time.Thursday, true
	case "Friday":
		return time.Friday, true
	case "Saturday":
		return time.Saturday, true
	}
	return time.Sunday, false
}
