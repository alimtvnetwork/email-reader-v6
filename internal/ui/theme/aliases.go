// aliases.go declares the normative registry of legitimate duplicate-RGB
// pairs in the design palette. Used by Test_Tokens_NoDuplicateValues
// (AC-DS-05) to distinguish "this surface IS that surface" intentional
// re-use (allowed) from accidental colour-tune collisions (forbidden).
//
// Spec: spec/24-app-design-system-and-ui/01-tokens.md §2.12.
//
// To regenerate after a palette tune, run the Python audit in
// /tmp/dup.py against palette_dark.go + palette_light.go and reconcile
// the output against the three blocks below.
package theme

// AliasScope tags whether a registered alias holds in Dark, Light, or
// both variants. A typed enum (vs raw string) makes misspellings a
// compile-time error and lets the test message print a stable label.
type AliasScope int

const (
	// AliasBoth — the From/To pair shares the same RGB in both Dark and
	// Light variants. The most common case (13 of 22 entries).
	AliasBoth AliasScope = iota
	// AliasDarkOnly — RGBs match in Dark; the Light variants differ.
	// The Light-mode distinction is normative — promoting to AliasBoth
	// requires a palette change.
	AliasDarkOnly
	// AliasLightOnly — RGBs match in Light; the Dark variants differ.
	AliasLightOnly
)

// String returns a stable label used in test failure messages.
func (s AliasScope) String() string {
	switch s {
	case AliasBoth:
		return "both"
	case AliasDarkOnly:
		return "darkOnly"
	case AliasLightOnly:
		return "lightOnly"
	default:
		return "unknown"
	}
}

// Alias is a single registry row. The (From, To) ordering is informative
// only — canonicaliseAlias() puts them in alphabetical order before
// duplicate-detection.
type Alias struct {
	From  ColorName
	To    ColorName
	Scope AliasScope
}

// canonicaliseAlias returns the (From, To) pair in alphabetical order so
// (A, B) and (B, A) collide as duplicates in the hygiene check.
func canonicaliseAlias(a Alias) (ColorName, ColorName) {
	if a.From <= a.To {
		return a.From, a.To
	}
	return a.To, a.From
}

// NamedAliases is the full normative registry. Every legitimate
// duplicate-RGB pair in the active palettes MUST appear here exactly
// once; every entry MUST hold per its declared scope. Both directions
// are enforced by Test_Tokens_NoDuplicateValues.
var NamedAliases = []Alias{
	// ---- AliasBoth (13 pairs) -------------------------------------------
	// Accent → RuleMatchBadge: rule-match badge IS the accent purple.
	{From: ColorRuleMatchBadge, To: ColorAccent, Scope: AliasBoth},
	// Border-clique 4-clique = 6 unordered pairs: Border, SidebarBorder,
	// CodeBorder, BadgeNeutralBg are all the same separator surface.
	{From: ColorBadgeNeutralBg, To: ColorBorder, Scope: AliasBoth},
	{From: ColorBadgeNeutralBg, To: ColorCodeBorder, Scope: AliasBoth},
	{From: ColorBadgeNeutralBg, To: ColorSidebarBorder, Scope: AliasBoth},
	{From: ColorBorder, To: ColorCodeBorder, Scope: AliasBoth},
	{From: ColorBorder, To: ColorSidebarBorder, Scope: AliasBoth},
	{From: ColorCodeBorder, To: ColorSidebarBorder, Scope: AliasBoth},
	// Neutral badge text IS sidebar foreground.
	{From: ColorBadgeNeutralFg, To: ColorSidebarForeground, Scope: AliasBoth},
	// Error 3-clique: Error, RawLogError, WatchDotError.
	{From: ColorError, To: ColorRawLogError, Scope: AliasBoth},
	{From: ColorError, To: ColorWatchDotError, Scope: AliasBoth},
	{From: ColorRawLogError, To: ColorWatchDotError, Scope: AliasBoth},
	// Raw-log "new mail" IS primary text.
	{From: ColorForeground, To: ColorRawLogNewMail, Scope: AliasBoth},
	// Reconnecting dot IS the warning amber.
	{From: ColorWarning, To: ColorWatchDotReconnecting, Scope: AliasBoth},

	// ---- AliasDarkOnly (5 pairs) ----------------------------------------
	// Dark code-surface IS Dark sidebar; Light tones diverge.
	{From: ColorCodeBg, To: ColorSidebar, Scope: AliasDarkOnly},
	// Dark line-highlight IS Dark muted surface; Light line-highlight is blue-tinted.
	{From: ColorCodeLineHighlight, To: ColorSurfaceMuted, Scope: AliasDarkOnly},
	// Dark disabled IS Dark heartbeat; Light heartbeat sits darker.
	{From: ColorForegroundDisabled, To: ColorRawLogHeartbeat, Scope: AliasDarkOnly},
	// Both white in Dark; Light active-sidebar shows the primary blue.
	{From: ColorPrimaryForeground, To: ColorSidebarItemActiveForeground, Scope: AliasDarkOnly},
	// Dark watching IS Dark success green; Light watching is brighter.
	{From: ColorSuccess, To: ColorWatchDotWatching, Scope: AliasDarkOnly},

	// ---- AliasLightOnly (4 pairs) ---------------------------------------
	// Light code-bg IS Light muted; Dark code-bg is the deepest surface.
	{From: ColorCodeBg, To: ColorSurfaceMuted, Scope: AliasLightOnly},
	// Light primary IS Light active-sidebar text; Dark active is white.
	{From: ColorPrimary, To: ColorSidebarItemActiveForeground, Scope: AliasLightOnly},
	// Light surface IS Light primary-fg (both white); Dark surface is near-black.
	{From: ColorPrimaryForeground, To: ColorSurface, Scope: AliasLightOnly},
	// Light idle IS Light timestamp; Dark timestamp lifted by Slice #118d.
	{From: ColorRawLogTimestamp, To: ColorWatchDotIdle, Scope: AliasLightOnly},
}
