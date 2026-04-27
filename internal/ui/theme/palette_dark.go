package theme

import "image/color"

// paletteDark holds every ColorName → NRGBA mapping for ThemeDark, taken
// verbatim from spec/24-app-design-system-and-ui/01-tokens.md §2.
//
// Values are alpha=255 unless explicitly noted in the spec (no alpha-<255
// tokens ship in the MVP slice). RGB tuples in the spec are quoted as
// `R G B`; we convert space-separated decimals to color.NRGBA literals
// here so the pure-Go `palette_test.go` parity check has a flat shape.
var paletteDark = map[ColorName]color.NRGBA{
	// §2.1 surface/foreground
	ColorBackground:         {15, 17, 21, 255},
	ColorSurface:            {23, 25, 31, 255},
	ColorSurfaceMuted:       {30, 33, 40, 255},
	ColorBorder:             {46, 49, 58, 255},
	ColorForeground:         {235, 237, 242, 255},
	ColorForegroundMuted:    {155, 160, 170, 255},
	ColorForegroundDisabled: {90, 96, 110, 255},

	// §2.2 brand
	ColorPrimary:           {82, 136, 255, 255},
	ColorPrimaryForeground: {255, 255, 255, 255},
	ColorAccent:            {170, 130, 255, 255},

	// §2.3 status
	ColorSuccess: {74, 200, 130, 255},
	ColorWarning: {240, 175, 60, 255},
	ColorError:   {240, 90, 105, 255},
	ColorInfo:    {82, 170, 240, 255},

	// §2.4 sidebar
	ColorSidebar:                     {19, 21, 26, 255},
	ColorSidebarForeground:           {200, 205, 215, 255},
	ColorSidebarItemHover:            {34, 37, 46, 255},
	ColorSidebarItemActive:           {46, 56, 90, 255},
	ColorSidebarItemActiveForeground: {255, 255, 255, 255},
	ColorSidebarBorder:               {46, 49, 58, 255},

	// §2.5 watch dots
	ColorWatchDotIdle:         {120, 125, 135, 255},
	ColorWatchDotStarting:     {100, 170, 240, 255},
	ColorWatchDotWatching:     {74, 200, 130, 255},
	ColorWatchDotReconnecting: {240, 175, 60, 255},
	ColorWatchDotStopping:     {145, 195, 245, 255},
	ColorWatchDotError:        {240, 90, 105, 255},

	// §2.6 raw log
	ColorRawLogHeartbeat: {90, 96, 110, 255},
	ColorRawLogNewMail:   {235, 237, 242, 255},
	ColorRawLogError:     {240, 90, 105, 255},
	// Slice #118d palette tune: lightened from (120,125,135) — WCAG
	// ratio against ColorCodeBg rose from 4.42 → 5.77, clearing the
	// `RawLogTimestamp on CodeBg (Dark)` row's `knownDrift` flag.
	ColorRawLogTimestamp: {140, 145, 155, 255},

	// §2.7 badges
	ColorRuleMatchBadge: {170, 130, 255, 255},
	ColorBadgeNeutralBg: {46, 49, 58, 255},
	ColorBadgeNeutralFg: {200, 205, 215, 255},

	// §2.8 code surfaces
	ColorCodeBg:            {19, 21, 26, 255},
	ColorCodeBorder:        {46, 49, 58, 255},
	ColorCodeLineHighlight: {30, 33, 40, 255},
	ColorCodeSelection:     {46, 72, 130, 255},
}
