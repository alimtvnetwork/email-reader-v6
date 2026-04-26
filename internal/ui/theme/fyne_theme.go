// fyne_theme.go is the only file in the project that imports the Fyne theme
// package (enforced by AST-T2 in ast_test.go). It implements the
// `fyne.Theme` interface and routes Fyne's built-in `ThemeColorName`
// constants to our typed `ColorName` tokens, per
// spec/24-app-design-system-and-ui/02-theme-implementation.md §2 table.
//
// Built behind `!nofyne` so headless CI (`go test -tags nofyne`) can still
// build the rest of the package — token data + Color() resolution live in
// theme.go and are fyne-free.
//go:build !nofyne

package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/lovable/email-read/internal/core"
)

// AppTheme implements fyne.Theme. Construct via NewAppTheme or just use
// the package-level Apply() helper which installs it on the current Fyne
// app. The struct is empty: all state lives in the package-level `state`
// variable so live theme switches don't require swapping the instance.
type AppTheme struct{}

// NewAppTheme returns the singleton-style Fyne adapter. Cheap to call.
func NewAppTheme() fyne.Theme { return &AppTheme{} }

// Color routes Fyne's built-in name + variant to one of our tokens. The
// `variant` parameter is honored only when the active mode is ThemeSystem
// — explicit ThemeDark/ThemeLight pin the palette regardless of OS pref,
// matching spec §4 (paletteFor).
func (AppTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	pal := paletteFor(resolvedMode(variant))
	if tok, ok := fyneColorRoute[name]; ok {
		return pal[tok]
	}
	// Unknown Fyne builtin → defer to Fyne defaults so unmapped chrome
	// (e.g. ColorNameOverlayBackground in dialog overlays) still renders.
	// We do NOT log here: each Fyne release adds names and we don't want
	// log-spam for every unmapped one.
	return fynetheme.DefaultTheme().Color(name, variant)
}

// Font / Icon / Size delegate to Fyne defaults for now. Custom font
// embedding (Inter, JetBrains Mono) and the Size scale (`SizeText*`,
// `SizeSpacing*`) land with the typography MVP — tracked outstanding in
// the consistency report Delta #4 follow-ups.
func (AppTheme) Font(s fyne.TextStyle) fyne.Resource { return fynetheme.DefaultTheme().Font(s) }
func (AppTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return fynetheme.DefaultTheme().Icon(n)
}
func (AppTheme) Size(n fyne.ThemeSizeName) float32 { return fynetheme.DefaultTheme().Size(n) }

// resolvedMode reconciles the active mode with the OS-supplied variant.
// Explicit Dark/Light wins; ThemeSystem defers to Fyne.
func resolvedMode(variant fyne.ThemeVariant) core.ThemeMode {
	m := Active()
	if m == core.ThemeSystem {
		if variant == fynetheme.VariantLight {
			return core.ThemeLight
		}
		return core.ThemeDark
	}
	return m
}

// fyneColorRoute is the §2 routing table: every Fyne built-in
// ThemeColorName we want to override is mapped to one of our tokens.
// Unmapped names fall through to fynetheme.DefaultTheme().Color(...).
var fyneColorRoute = map[fyne.ThemeColorName]ColorName{
	fynetheme.ColorNameBackground:      ColorBackground,
	fynetheme.ColorNameForeground:      ColorForeground,
	fynetheme.ColorNamePrimary:         ColorPrimary,
	fynetheme.ColorNameButton:          ColorSurface,
	fynetheme.ColorNameInputBackground: ColorSurface,
	fynetheme.ColorNameDisabled:        ColorForegroundDisabled,
	fynetheme.ColorNameError:           ColorError,
	fynetheme.ColorNameSuccess:         ColorSuccess,
	fynetheme.ColorNameWarning:         ColorWarning,
	fynetheme.ColorNameSeparator:       ColorBorder,
	// Selection: spec wants ColorPrimary at alpha 0.30 — alpha-mixing
	// requires a wrapped color value, deferred until the elevation/alpha
	// helper lands. For now use the solid Primary so selection is at
	// least theme-aware (better than Fyne's hard-coded blue).
	fynetheme.ColorNameSelection: ColorPrimary,
}
