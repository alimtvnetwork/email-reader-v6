// Package theme is the single source of truth for design tokens consumed by
// the Fyne UI. Color tokens, palette maps, and the live-switch protocol
// live here per spec/24-app-design-system-and-ui/01-tokens.md and
// 02-theme-implementation.md.
//
// This file is intentionally fyne-free: token names + palettes are pure
// Go data so the token contract can be unit-tested under `-tags nofyne`
// alongside the rest of internal/core. The Fyne adapter (appTheme) lives
// in fyne_theme.go behind the `!nofyne` build tag.
package theme

// ColorName is a typed enum for every design-token color. Using a typed
// string (instead of raw `string`) makes misspellings a compile-time
// error and lets ast_test scan accidental literal usage in feature views.
//
// Spec: 01-tokens.md §1 (Naming Convention).
type ColorName string

// Sidebar tokens — 01-tokens.md §2.4. Cited by Delta #4 of the project
// consistency report as one of the two MVP groups.
const (
	ColorSidebar                     ColorName = "Sidebar"
	ColorSidebarForeground           ColorName = "SidebarForeground"
	ColorSidebarItemHover            ColorName = "SidebarItemHover"
	ColorSidebarItemActive           ColorName = "SidebarItemActive"
	ColorSidebarItemActiveForeground ColorName = "SidebarItemActiveForeground"
	ColorSidebarBorder               ColorName = "SidebarBorder"
)

// Watch status-dot tokens — 01-tokens.md §2.5. Resolves OI-1 ("watch dot
// has no semantic color"). Used by the WatchDot widget (04-components §6).
const (
	ColorWatchDotIdle         ColorName = "WatchDotIdle"
	ColorWatchDotStarting     ColorName = "WatchDotStarting"
	ColorWatchDotWatching     ColorName = "WatchDotWatching"
	ColorWatchDotReconnecting ColorName = "WatchDotReconnecting"
	ColorWatchDotStopping     ColorName = "WatchDotStopping"
	ColorWatchDotError        ColorName = "WatchDotError"
)

// Surface + foreground tokens — 01-tokens.md §2.1. Required as fallbacks
// for the Fyne built-in routing table even before the full palette ships.
const (
	ColorBackground         ColorName = "Background"
	ColorSurface            ColorName = "Surface"
	ColorSurfaceMuted       ColorName = "SurfaceMuted"
	ColorBorder             ColorName = "Border"
	ColorForeground         ColorName = "Foreground"
	ColorForegroundMuted    ColorName = "ForegroundMuted"
	ColorForegroundDisabled ColorName = "ForegroundDisabled"
)

// Brand + status tokens — 01-tokens.md §2.2 / §2.3.
const (
	ColorPrimary           ColorName = "Primary"
	ColorPrimaryForeground ColorName = "PrimaryForeground"
	ColorAccent            ColorName = "Accent"
	ColorSuccess           ColorName = "Success"
	ColorWarning           ColorName = "Warning"
	ColorError             ColorName = "Error"
	ColorInfo              ColorName = "Info"
)

// Raw-log tokens — 01-tokens.md §2.6. Resolves OI-1 second half. Used by
// the Watch raw-log virtualized list (04-components §RawLogLine).
const (
	ColorRawLogHeartbeat ColorName = "RawLogHeartbeat"
	ColorRawLogNewMail   ColorName = "RawLogNewMail"
	ColorRawLogError     ColorName = "RawLogError"
	ColorRawLogTimestamp ColorName = "RawLogTimestamp"
)

// Badge tokens — 01-tokens.md §2.7. Used by the Badge widget variants
// (Neutral / RuleMatch); Success/Warning/Error/Info badges reuse §2.3.
const (
	ColorRuleMatchBadge ColorName = "RuleMatchBadge"
	ColorBadgeNeutralBg ColorName = "BadgeNeutralBg"
	ColorBadgeNeutralFg ColorName = "BadgeNeutralFg"
)

// Code / monospace surface tokens — 01-tokens.md §2.8. Used by raw email
// viewer, log viewer, and any <pre>-style block.
const (
	ColorCodeBg            ColorName = "CodeBg"
	ColorCodeBorder        ColorName = "CodeBorder"
	ColorCodeLineHighlight ColorName = "CodeLineHighlight"
	ColorCodeSelection     ColorName = "CodeSelection"
)

// allColorNames is the canonical iteration order for parity tests
// (palette_dark and palette_light must each define every entry).
//
// Adding a new token requires appending here AND extending both palette
// maps — palette_test.go enforces this at build-time.
var allColorNames = []ColorName{
	// §2.1 surface/foreground
	ColorBackground, ColorSurface, ColorSurfaceMuted, ColorBorder,
	ColorForeground, ColorForegroundMuted, ColorForegroundDisabled,
	// §2.2 brand
	ColorPrimary, ColorPrimaryForeground, ColorAccent,
	// §2.3 status
	ColorSuccess, ColorWarning, ColorError, ColorInfo,
	// §2.4 sidebar
	ColorSidebar, ColorSidebarForeground, ColorSidebarItemHover,
	ColorSidebarItemActive, ColorSidebarItemActiveForeground, ColorSidebarBorder,
	// §2.5 watch dots
	ColorWatchDotIdle, ColorWatchDotStarting, ColorWatchDotWatching,
	ColorWatchDotReconnecting, ColorWatchDotStopping, ColorWatchDotError,
	// §2.6 raw log
	ColorRawLogHeartbeat, ColorRawLogNewMail, ColorRawLogError, ColorRawLogTimestamp,
	// §2.7 badges
	ColorRuleMatchBadge, ColorBadgeNeutralBg, ColorBadgeNeutralFg,
	// §2.8 code surfaces
	ColorCodeBg, ColorCodeBorder, ColorCodeLineHighlight, ColorCodeSelection,
}

// AllColorNames returns a defensive copy of the canonical list. Used by
// tests + the palette parity guard. Returning a copy prevents callers
// from mutating the package-private slice.
func AllColorNames() []ColorName {
	out := make([]ColorName, len(allColorNames))
	copy(out, allColorNames)
	return out
}
