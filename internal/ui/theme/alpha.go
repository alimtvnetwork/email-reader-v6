// alpha.go provides AlphaBlend — a small helper that returns a token's
// resolved color with a custom alpha. Used for the Selection token
// (spec/24-app-design-system-and-ui/02-theme-implementation.md §2 wants
// `ColorPrimary` at alpha 0.30) and for any future translucent surfaces
// (elevation shadows, overlay scrim, etc.).
//
// Pure Go (fyne-free) — builds under `-tags nofyne` alongside the rest
// of the token contract.
package theme

import "image/color"

// AlphaBlend returns the active-mode color for `name` with its alpha
// channel scaled by `alpha` (0.0..1.0). Out-of-range alphas are clamped
// to [0, 1]. The returned value is a `color.NRGBA` so AST-T1's literal
// guard does not flag downstream consumers — they receive a fully-formed
// color value, not a literal.
//
// Implementation note: we read the underlying NRGBA from the active
// palette directly (not via Color()) so the alpha math operates on the
// canonical un-pre-multiplied form. Unknown names fall through to
// ColorForeground at the requested alpha to keep the contract identical
// to Color().
func AlphaBlend(name ColorName, alpha float32) color.NRGBA {
	pal := paletteFor(Active())
	nrgba, ok := pal[name]
	if !ok {
		warnUnknown(name)
		nrgba = pal[ColorForeground]
	}
	return color.NRGBA{
		R: nrgba.R,
		G: nrgba.G,
		B: nrgba.B,
		A: scaleAlpha(nrgba.A, alpha),
	}
}

// scaleAlpha clamps `factor` to [0, 1] and multiplies the input alpha
// channel. Rounded to nearest, never panics.
func scaleAlpha(a uint8, factor float32) uint8 {
	if factor <= 0 {
		return 0
	}
	if factor >= 1 {
		return a
	}
	v := float32(a)*factor + 0.5
	if v >= 255 {
		return 255
	}
	return uint8(v)
}
