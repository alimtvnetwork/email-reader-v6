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
	mode := resolvedMode(variant)
	// Selection is special-cased: spec §2 wants ColorPrimary at alpha
	// 0.30 so the selection band tints the underlying surface instead
	// of opaquely covering it. AlphaBlend reads the active mode, so we
	// pin it via a brief Apply round-trip if Fyne's variant disagrees
	// — cheaper path: build the NRGBA inline from the resolved palette
	// to avoid mutating global state during a render.
	if name == fynetheme.ColorNameSelection {
		base := paletteFor(mode)[ColorPrimary]
		return color.NRGBA{R: base.R, G: base.G, B: base.B, A: scaleAlpha(base.A, 0.30)}
	}
	pal := paletteFor(mode)
	if tok, ok := fyneColorRoute[name]; ok {
		return pal[tok]
	}
	// Unknown Fyne builtin → defer to Fyne defaults so unmapped chrome
	// (e.g. ColorNameOverlayBackground in dialog overlays) still renders.
	// We do NOT log here: each Fyne release adds names and we don't want
	// log-spam for every unmapped one.
	return fynetheme.DefaultTheme().Color(name, variant)
}

// Font routes Fyne's text-style request to one of our embedded
// variable fonts. Monospace + monospace-bold map to JetBrains Mono;
// every other style (regular, bold, italic, bold-italic) maps to Inter
// Variable — the variable axis lets a single TTF cover all weights /
// italics. When the embedded asset is absent (fresh checkout, no .ttf
// files committed), we fall back to Fyne's bundled default font so the
// UI still renders.
func (AppTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace {
		if r := TextMonospaceFont(); r != nil {
			return r
		}
		return fynetheme.DefaultTheme().Font(s)
	}
	if r := TextFont(); r != nil {
		return r
	}
	return fynetheme.DefaultTheme().Font(s)
}

// Icon delegates to Fyne defaults — the icon set is shared. Custom
// icons (if any) land per-widget, not via the theme.
func (AppTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return fynetheme.DefaultTheme().Icon(n)
}

// IconEdit returns the canonical "edit / pencil" icon resource.
// Exposed so non-theme packages can use built-in Fyne icons without
// importing fyne.io/fyne/v2/theme directly (AST-T2 enforcement —
// see ast_test.go).
func IconEdit() fyne.Resource { return fynetheme.DocumentCreateIcon() }

// IconDelete returns the canonical "delete / trash" icon resource.
// See IconEdit for why this lives here instead of in callers.
func IconDelete() fyne.Resource { return fynetheme.DeleteIcon() }

// Size routes Fyne's built-in size names to our typography + spacing
// scale (01-tokens.md §3 / §4) honoring the active density. Names with
// no spec equivalent (e.g. ScrollBar, Separator) fall through to Fyne
// defaults so chrome rendering stays consistent.
func (AppTheme) Size(n fyne.ThemeSizeName) float32 {
	if tok, ok := fyneSizeRoute[n]; ok {
		return Size(tok)
	}
	return fynetheme.DefaultTheme().Size(n)
}

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
	// Slice #175 — popup/menu/overlay surfaces. Without these the
	// dropdown popup, right-click menu, and dialog overlay all fall
	// through to fynetheme.DefaultTheme().Color(name, variant), which
	// keys off the OS variant (NOT our explicit Active() mode). Result
	// in Light mode on a dark-OS machine: the panel is white but the
	// Theme dropdown popup paints dark — the bug visible in the user's
	// screenshot. Routing these to our own surface tokens makes the
	// chrome follow the chosen palette regardless of the OS hint.
	fynetheme.ColorNameMenuBackground:    ColorSurface,
	fynetheme.ColorNameOverlayBackground: ColorSurface,
	fynetheme.ColorNameHeaderBackground:  ColorSurfaceMuted,
	fynetheme.ColorNameHover:             ColorSurfaceMuted,
	fynetheme.ColorNamePressed:           ColorSurfaceMuted,
	// Selection is intentionally absent: it's special-cased in Color()
	// to apply the §2 alpha-0.30 blend on ColorPrimary via scaleAlpha.
}

// fyneSizeRoute is the §3/§4 routing table: each Fyne built-in size
// name maps to one of our typography or spacing tokens. Unmapped names
// fall through to fynetheme.DefaultTheme().Size(...).
//
// Mapping rationale (per 01-tokens.md §3.1):
//   - SizeNameText            → body text (14 px Comfortable)
//   - SizeNameHeadingText     → page-title scale
//   - SizeNameSubHeadingText  → section-title scale
//   - SizeNameCaptionText     → caption / helper text
//   - SizeNamePadding         → default form gap (Spacing2 = 8 px)
//   - SizeNameInnerPadding    → inner control padding (Spacing3 = 12 px)
//   - SizeNameInlineIcon      → caption-text scale (matches Fyne ratio)
var fyneSizeRoute = map[fyne.ThemeSizeName]SizeName{
	fynetheme.SizeNameText:           SizeTextBody,
	fynetheme.SizeNameHeadingText:    SizeTextPageTitle,
	fynetheme.SizeNameSubHeadingText: SizeTextSectionTitle,
	fynetheme.SizeNameCaptionText:    SizeTextCaption,
	fynetheme.SizeNamePadding:        SizeSpacing2,
	fynetheme.SizeNameInnerPadding:   SizeSpacing3,
	fynetheme.SizeNameInlineIcon:     SizeTextCaption,
}
