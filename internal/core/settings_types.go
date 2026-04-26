// settings_types.go declares the public types and defaults for the Settings
// service per spec/21-app/02-features/07-settings/01-backend.md §3.
//
// Note: the on-disk JSON schema currently uses the legacy camelCase keys
// (`pollSeconds`, `chromePath`, `incognitoArg`) per `internal/config/config.go`
// to remain backward-compatible with existing user `data/config.json` files.
// The PascalCase migration described in the spec is tracked separately under
// Delta #1 (table rename) and will be done as a coordinated config migration.
package core

import "time"

// SettingsSnapshot is the read-side projection returned by Get / Save / Reset.
// All path fields are absolute and read-only (populated from the config layer
// on every Get; ignored if a caller passes them in via SettingsInput).
type SettingsSnapshot struct {
	PollSeconds           uint16
	BrowserOverride       BrowserOverride
	Theme                 ThemeMode
	OpenUrlAllowedSchemes []string // canonical lower-case, sorted, deduped
	AllowLocalhostUrls    bool
	AutoStartWatch        bool
	OpenUrlsRetentionDays uint16 // 0 = disabled (never prune); else age cutoff in days
	ConfigPath            string // read-only, absolute
	DataDir               string // read-only, absolute
	EmailArchiveDir       string // read-only, absolute
	UpdatedAt             time.Time
}

// SettingsInput is the write-side payload for Save. Read-only display fields
// (paths, UpdatedAt) are intentionally absent.
type SettingsInput struct {
	PollSeconds           uint16
	BrowserOverride       BrowserOverride
	Theme                 ThemeMode
	OpenUrlAllowedSchemes []string
	AllowLocalhostUrls    bool
	AutoStartWatch        bool
	OpenUrlsRetentionDays uint16
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
		PollSeconds:           3,
		Theme:                 ThemeDark,
		OpenUrlAllowedSchemes: []string{"https"},
		AllowLocalhostUrls:    false,
		AutoStartWatch:        true,
		OpenUrlsRetentionDays: 90,
	}
}
