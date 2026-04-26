// alpha_test.go locks the AlphaBlend contract per
// spec/24-app-design-system-and-ui/02-theme-implementation.md §2
// (selection = ColorPrimary @ 0.30) and the tnum helper from §3.2.
package theme

import (
	"image/color"
	"testing"

	"github.com/lovable/email-read/internal/core"
)

// Test_AlphaBlend_PrimarySelection locks the spec's selection value:
// ColorPrimary in the active palette, with alpha multiplied by 0.30.
// Both modes covered so palette flips don't break the wrapper.
func Test_AlphaBlend_PrimarySelection(t *testing.T) {
	t.Cleanup(resetForTest)
	cases := []struct {
		mode core.ThemeMode
		want color.NRGBA
	}{
		// Dark Primary = (82, 136, 255, 255). 255 * 0.30 + 0.5 = 77.
		{core.ThemeDark, color.NRGBA{82, 136, 255, 77}},
		// Light Primary = (42, 100, 245, 255). Same alpha math.
		{core.ThemeLight, color.NRGBA{42, 100, 245, 77}},
	}
	for _, tc := range cases {
		_ = Apply(tc.mode)
		got := AlphaBlend(ColorPrimary, 0.30)
		if got != tc.want {
			t.Errorf("[%v] AlphaBlend(Primary, 0.30) = %+v, want %+v", tc.mode, got, tc.want)
		}
	}
}

// Test_AlphaBlend_ClampsOutOfRange asserts the [0, 1] clamp contract.
// A negative factor → fully transparent; a >1 factor → original alpha.
func Test_AlphaBlend_ClampsOutOfRange(t *testing.T) {
	t.Cleanup(resetForTest)
	_ = Apply(core.ThemeDark)
	if got := AlphaBlend(ColorPrimary, -0.5).A; got != 0 {
		t.Errorf("alpha < 0 → A = %d, want 0", got)
	}
	if got := AlphaBlend(ColorPrimary, 2.0).A; got != 255 {
		t.Errorf("alpha > 1 → A = %d, want 255", got)
	}
	if got := AlphaBlend(ColorPrimary, 1.0).A; got != 255 {
		t.Errorf("alpha == 1 → A = %d, want 255 (identity)", got)
	}
}

// Test_AlphaBlend_UnknownFalls­BackToForeground confirms the no-panic
// fallback contract: unknown name → ColorForeground at the requested
// alpha (matches Color()'s contract).
func Test_AlphaBlend_UnknownFallsBackToForeground(t *testing.T) {
	t.Cleanup(resetForTest)
	_ = Apply(core.ThemeDark)
	got := AlphaBlend(ColorName("DoesNotExist"), 0.50)
	fg := paletteDark[ColorForeground]
	want := color.NRGBA{R: fg.R, G: fg.G, B: fg.B, A: scaleAlpha(fg.A, 0.50)}
	if got != want {
		t.Errorf("unknown alpha-blend = %+v, want %+v", got, want)
	}
}

// Test_TabularNumFeatures locks the §3.2 OpenType feature tag list.
// Adding a tag is a minor bump; removing one breaks downstream numeric
// columns and must be deliberate.
func Test_TabularNumFeatures(t *testing.T) {
	got := TabularNumFeatures()
	if len(got) != 1 || got[0] != "tnum" {
		t.Errorf("TabularNumFeatures() = %v, want [tnum]", got)
	}
	if TabularNumFeature != "tnum" {
		t.Errorf("TabularNumFeature = %q, want \"tnum\"", TabularNumFeature)
	}
	// Defensive copy: mutating the returned slice must not affect the
	// next call (we currently return a fresh slice on every invocation).
	got[0] = "MUTATED"
	if TabularNumFeatures()[0] != "tnum" {
		t.Errorf("TabularNumFeatures returns aliased slice — caller mutation leaked")
	}
}
