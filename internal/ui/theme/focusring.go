// focusring.go defines the focus-ring geometry + opacity tokens per
// `spec/24-app-design-system-and-ui/05-accessibility.md` §2:
//
//   "Focus ring uses ColorPrimary at alpha 0.40, 2 px outline,
//    offset 2 px — implemented in internal/ui/theme/focusring.go."
//
// **Slice #118b** lands the *data* for the focus-ring contract. The
// Fyne paint code that consumes these tokens (drawing the outline
// around the focused widget on the canvas) lands alongside the
// `Test_FocusRing_Visible` runtime test in a follow-up that needs
// the live Fyne render harness — out of scope for this slice
// because the sandbox cannot link Fyne's GL/X11 stack.
//
// What ships here:
//   - Two `SizeName` tokens (`SizeFocusRingWidth`, `SizeFocusRingOffset`)
//     wired into the existing `sizeBase` map so `Size()` resolves
//     them like every other spacing token.
//   - One `float32` constant `FocusRingAlpha` consumed by the
//     `AlphaBlend(ColorPrimary, FocusRingAlpha)` call site that
//     paints the ring. Constant rather than a token because alpha
//     scaling is a property of the paint, not of the palette.
//   - `FocusRingColor()` helper that returns the resolved
//     `color.NRGBA` for the active mode — the single call site the
//     paint code consumes.
//
// Pure Go (fyne-free) — builds under `-tags nofyne`.
package theme

import "image/color"

// FocusRingAlpha is the constant alpha multiplier applied to
// `ColorPrimary` when painting the focus ring. Sourced from spec
// §2 ("ColorPrimary at alpha 0.40"). Kept as a `float32` to match
// the `AlphaBlend` signature without conversion at the call site.
//
// Why a constant rather than a `SizeName`-style token: the alpha
// value is a property of the paint operation, not a per-mode
// palette entry. Both Light and Dark modes use the same 0.40 ratio
// because the underlying `ColorPrimary` already carries the
// per-mode hue. A token here would force a `palette_test.go`
// extension that adds no signal.
const FocusRingAlpha float32 = 0.40

// SizeName extensions for the focus ring. These plug into the
// existing `sizeBase` map (see `sizes.go` patch below) so `Size()`
// resolves them with the standard Comfortable/Compact density
// scaling. Spec §2 calls for 2 px width + 2 px offset at the
// Comfortable baseline.
const (
	SizeFocusRingWidth  SizeName = "FocusRingWidth"
	SizeFocusRingOffset SizeName = "FocusRingOffset"
)

// FocusRingColor returns the resolved focus-ring colour for the
// currently-active theme mode. Single call site for the paint code:
//
//   c := theme.FocusRingColor()
//   canvas.DrawRect(bounds.Inset(-Size(SizeFocusRingOffset)), c, Size(SizeFocusRingWidth))
//
// Returns `color.NRGBA` so AST-T1's literal guard does not flag
// downstream consumers — they receive a fully-formed colour value,
// not a hex literal.
func FocusRingColor() color.NRGBA {
	return AlphaBlend(ColorPrimary, FocusRingAlpha)
}
