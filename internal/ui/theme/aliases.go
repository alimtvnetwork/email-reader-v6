// Package theme — aliases.go declares the named-alias carve-out for
// AC-DS-05 ("no two distinct tokens share the same RGB triple in the
// same variant"). The carve-out narrows the rule from binary to
// registry-aware: a duplicate is allowed iff its (canonicalised,
// unordered) pair is listed in NamedAliases below.
//
// Spec: spec/24-app-design-system-and-ui/01-tokens.md §2.12.
//
// Adding or removing an entry here MUST come with: (a) a matching row
// in §2.12 of 01-tokens.md, (b) a one-line rationale in this file's
// table comments, (c) a passing run of Test_Tokens_NoDuplicateValues.
package theme

// AliasScope captures whether a registered alias holds in both
// variants, only in Dark, or only in Light. The asymmetric scopes
// (`AliasDarkOnly` / `AliasLightOnly`) are a test signal, not a
// suggestion: the registry validator REQUIRES the off-variant pair to
// be distinct, otherwise the row is misleading and should be promoted
// to `AliasBoth`.
//
// Typed enum (vs raw string) chosen per Slice #168 verdict 2(a) —
// compile-time misspellings impossible.
type AliasScope int

const (
	// AliasBoth — pair shares RGB in both Dark and Light. Most-stable.
	AliasBoth AliasScope = iota
	// AliasDarkOnly — pair shares RGB in Dark; Light variants MUST differ.
	AliasDarkOnly
	// AliasLightOnly — pair shares RGB in Light; Dark variants MUST differ.
	AliasLightOnly
)

// String returns a stable label for test failure messages.
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

// Alias is one row of the registry: an unordered pair of ColorNames
// that may legally collide in the named scope.
//
// Order of A/B is not significant — the test canonicalises pairs
// alphabetically before comparison. Duplicate rows (e.g. `(A,B)` and
// `(B,A)`) are caught by the duplicate-detection clause of the
// validator.
type Alias struct {
	// A, B are the two colliding ColorNames. A != B (reflexive entries
	// are rejected by the validator).
	A, B ColorName
	// Scope declares which variant the collision is legal in.
	Scope AliasScope
}

// NamedAliases is the canonical alias registry. Spec mirror: 22 rows
// total, matching `spec/24-app-design-system-and-ui/01-tokens.md`
// §2.12 (13 Both + 5 DarkOnly + 4 LightOnly).
//
// Struct-of-structs shape (vs map) chosen per Slice #168 verdict 1(a):
// explicit and grep-friendly ("who aliases Border?" → one ripgrep
// over this slice).
//
// Cliques (e.g. `Border = SidebarBorder = CodeBorder = BadgeNeutralBg`,
// or `Error = RawLogError = WatchDotError`) are listed as every
// unordered pair so the test message can name the precise colliding
// pair, not just the canonical representative.
var NamedAliases = []Alias{
	// ----- Both-variant aliases (13 pairs) -----
	// Brand re-use
	{A: ColorAccent, B: ColorRuleMatchBadge, Scope: AliasBoth},

	// Error 3-clique: Error = RawLogError = WatchDotError
	{A: ColorError, B: ColorRawLogError, Scope: AliasBoth},
	{A: ColorError, B: ColorWatchDotError, Scope: AliasBoth},
	{A: ColorRawLogError, B: ColorWatchDotError, Scope: AliasBoth},

	// Status re-use
	{A: ColorWarning, B: ColorWatchDotReconnecting, Scope: AliasBoth},
	{A: ColorForeground, B: ColorRawLogNewMail, Scope: AliasBoth},

	// Border 4-clique: Border = SidebarBorder = CodeBorder = BadgeNeutralBg
	{A: ColorBorder, B: ColorSidebarBorder, Scope: AliasBoth},
	{A: ColorBorder, B: ColorCodeBorder, Scope: AliasBoth},
	{A: ColorBorder, B: ColorBadgeNeutralBg, Scope: AliasBoth},
	{A: ColorSidebarBorder, B: ColorCodeBorder, Scope: AliasBoth},
	{A: ColorSidebarBorder, B: ColorBadgeNeutralBg, Scope: AliasBoth},
	{A: ColorCodeBorder, B: ColorBadgeNeutralBg, Scope: AliasBoth},

	// Sidebar / badge text
	{A: ColorBadgeNeutralFg, B: ColorSidebarForeground, Scope: AliasBoth},

	// ----- Dark-only aliases (5 pairs) -----
	// Healthy watcher dot = Success-green on Dark; Light Success was
	// darkened by Slice #118d for WCAG, watch-dot kept brighter.
	{A: ColorSuccess, B: ColorWatchDotWatching, Scope: AliasDarkOnly},

	// Heartbeat = disabled grey on Dark; Light heartbeat lifted for
	// legibility against ColorCodeBg.
	{A: ColorForegroundDisabled, B: ColorRawLogHeartbeat, Scope: AliasDarkOnly},

	// Active sidebar item label = pure white on Dark.
	{A: ColorPrimaryForeground, B: ColorSidebarItemActiveForeground, Scope: AliasDarkOnly},

	// Sidebar and code surfaces share the deep panel tone on Dark.
	{A: ColorCodeBg, B: ColorSidebar, Scope: AliasDarkOnly},

	// Code line-highlight matches the muted panel tone on Dark.
	{A: ColorCodeLineHighlight, B: ColorSurfaceMuted, Scope: AliasDarkOnly},

	// ----- Light-only aliases (4 pairs) -----
	// Cards on white pages: PrimaryForeground = Surface = white.
	{A: ColorPrimaryForeground, B: ColorSurface, Scope: AliasLightOnly},

	// Active item label = primary-action blue on Light.
	{A: ColorPrimary, B: ColorSidebarItemActiveForeground, Scope: AliasLightOnly},

	// Code surfaces match the muted panel tone on Light.
	{A: ColorCodeBg, B: ColorSurfaceMuted, Scope: AliasLightOnly},

	// Idle dot = timestamp grey on Light. Dark RawLogTimestamp was
	// lifted by Slice #118d, breaking the Dark equality.
	{A: ColorRawLogTimestamp, B: ColorWatchDotIdle, Scope: AliasLightOnly},
}

// canonicalise returns the alias pair as an alphabetically ordered
// `(low, high)` tuple. Used by the validator to detect duplicate rows
// and to look up "is this colliding pair registered?" in O(N).
func (a Alias) canonicalise() (ColorName, ColorName) {
	if a.A < a.B {
		return a.A, a.B
	}
	return a.B, a.A
}
