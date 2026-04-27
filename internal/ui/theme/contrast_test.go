// contrast_test.go — Slice #118b lit-up implementation of spec
// `spec/24-app-design-system-and-ui/05-accessibility.md` §8 #1
// (`Test_Contrast_Matrix`). Computes the WCAG 2.1 contrast ratio
// for every (foreground, background) pair in §1's matrix and fails
// if any pair drops below the listed threshold.
//
// The skip-stub of the same name in
// `internal/ui/accessibility/a11y_skipped_test.go` redirects here:
// the test must live in the `theme` package itself because
// `accessibility` cannot import `theme` (the import edge runs the
// other way — `theme/motion.go` imports `accessibility` for the
// reduced-motion probe). Putting the test in `theme` keeps the
// dependency graph acyclic and gives the test direct access to the
// palette maps without forcing a public `RGBA(name)` export.
//
// Algorithm (WCAG 2.1 §1.4.3):
//
//   1. Linearise each sRGB channel: c′ = c/12.92 if c ≤ 0.03928
//      else ((c + 0.055)/1.055)^2.4 (c is the channel in [0,1]).
//   2. Relative luminance: L = 0.2126·R′ + 0.7152·G′ + 0.0722·B′.
//   3. Contrast: (L_lighter + 0.05) / (L_darker + 0.05).
//
// Ratios round to one decimal for matching against the spec table.
package theme

import (
	"image/color"
	"math"
	"testing"

	"github.com/lovable/email-read/internal/core"
)

// contrastCase pairs two tokens with their expected mode + minimum
// passing ratio. Mirrors the rows from spec §1.
type contrastCase struct {
	id        string // human label used in failure messages
	mode      core.ThemeMode
	fg, bg    ColorName
	threshold float64
}

// contrastMatrix mirrors the 14 rows from
// `spec/24-app-design-system-and-ui/05-accessibility.md` §1. WCAG
// 2.1 AA threshold is 4.5 for body text and 3.0 for large text.
// Every spec row uses 4.5 — the spec deliberately holds large-text
// pairs to the body-text bar so a future "make this label small"
// edit cannot silently drop below threshold.
var contrastMatrix = []contrastCase{
	{"Foreground on Background (Dark)", core.ThemeDark, ColorForeground, ColorBackground, 4.5},
	{"Foreground on Background (Light)", core.ThemeLight, ColorForeground, ColorBackground, 4.5},
	{"ForegroundMuted on Background (Dark)", core.ThemeDark, ColorForegroundMuted, ColorBackground, 4.5},
	{"ForegroundMuted on Background (Light)", core.ThemeLight, ColorForegroundMuted, ColorBackground, 4.5},
	{"PrimaryForeground on Primary (Dark)", core.ThemeDark, ColorPrimaryForeground, ColorPrimary, 4.5},
	{"PrimaryForeground on Primary (Light)", core.ThemeLight, ColorPrimaryForeground, ColorPrimary, 4.5},
	{"Error on Background (Dark)", core.ThemeDark, ColorError, ColorBackground, 4.5},
	{"Error on Background (Light)", core.ThemeLight, ColorError, ColorBackground, 4.5},
	{"Success on Background (Dark)", core.ThemeDark, ColorSuccess, ColorBackground, 4.5},
	{"Success on Background (Light)", core.ThemeLight, ColorSuccess, ColorBackground, 4.5},
	{"Warning on Background (Dark)", core.ThemeDark, ColorWarning, ColorBackground, 4.5},
	{"RawLogTimestamp on CodeBg (Dark)", core.ThemeDark, ColorRawLogTimestamp, ColorCodeBg, 4.5},
	{"SidebarItemActiveForeground on SidebarItemActive (Dark)", core.ThemeDark, ColorSidebarItemActiveForeground, ColorSidebarItemActive, 4.5},
	{"BadgeNeutralFg on BadgeNeutralBg (Dark)", core.ThemeDark, ColorBadgeNeutralFg, ColorBadgeNeutralBg, 4.5},
}

// Test_Contrast_Matrix lights up spec §8 #1. Iterates every row,
// resolves the colour from the matching palette, computes the WCAG
// ratio, and asserts it is >= threshold. Failure message names the
// row so a regression points straight at the offending edit.
func Test_Contrast_Matrix(t *testing.T) {
	for _, c := range contrastMatrix {
		c := c
		t.Run(c.id, func(t *testing.T) {
			pal := paletteFor(c.mode)
			fgN, ok := pal[c.fg]
			if !ok {
				t.Fatalf("foreground token %q missing from %s palette", c.fg, c.mode)
			}
			bgN, ok := pal[c.bg]
			if !ok {
				t.Fatalf("background token %q missing from %s palette", c.bg, c.mode)
			}
			ratio := wcagContrast(fgN, bgN)
			if ratio+1e-9 < c.threshold {
				t.Fatalf("contrast %.2f < threshold %.2f for %s vs %s in %s mode (fg=%v bg=%v)",
					ratio, c.threshold, c.fg, c.bg, c.mode, fgN, bgN)
			}
			t.Logf("ratio = %.2f (threshold %.2f) — %s", ratio, c.threshold, c.id)
		})
	}
}

// wcagContrast returns the WCAG 2.1 contrast ratio of two colours
// per §1.4.3. Symmetric — order of arguments does not matter, the
// formula divides the lighter luminance by the darker.
func wcagContrast(a, b color.NRGBA) float64 {
	la := relativeLuminance(a)
	lb := relativeLuminance(b)
	lighter, darker := la, lb
	if darker > lighter {
		lighter, darker = lb, la
	}
	return (lighter + 0.05) / (darker + 0.05)
}

// relativeLuminance computes the WCAG 2.1 relative luminance of a
// colour: 0.2126·R + 0.7152·G + 0.0722·B over the sRGB-linearised
// channels. The alpha channel is intentionally ignored — text is
// painted as a flat sample over its background, so the contrast
// math operates on the visible RGB only. Pre-multiplied alpha
// composition is the painter's responsibility, not the matrix's.
func relativeLuminance(c color.NRGBA) float64 {
	r := linearise(float64(c.R) / 255)
	g := linearise(float64(c.G) / 255)
	b := linearise(float64(c.B) / 255)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// linearise converts an sRGB channel value in [0,1] to linear-light
// per the WCAG 2.1 piecewise-gamma formula.
func linearise(c float64) float64 {
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
