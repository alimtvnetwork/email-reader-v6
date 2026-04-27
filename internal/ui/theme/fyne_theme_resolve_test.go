// fyne_theme_resolve_test.go — Slice #145 / Task #3 burn-down.
//
// Closes AC-DS-15:
//   "theme.Apply(ThemeSystem) resolves via app.Settings().ThemeVariant()."
//
// The headless-portion of the rule lives in `resolvedMode(variant)` in
// fyne_theme.go: when the active mode is ThemeSystem the function
// hands off to the OS-supplied `fyne.ThemeVariant` (Light/Dark);
// when an explicit ThemeLight or ThemeDark is active, the OS variant
// is ignored. This file pins both branches plus the input invariant.
//
// Built behind `!nofyne` because resolvedMode itself only compiles
// with the fyne theme package — the AC-coverage audit (slice #119)
// resolves AC-DS-15 from this file's comment regardless of build tag.
//
//go:build !nofyne

package theme

import (
	"testing"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"github.com/lovable/email-read/internal/core"
)

// Test_Theme_SystemResolves is the spec-named test for AC-DS-15. Three
// arms: ThemeSystem honours OS variant (Light vs Dark), explicit modes
// pin the palette regardless of OS hint.
func Test_Theme_SystemResolves(t *testing.T) {
	t.Cleanup(resetForTest)

	cases := []struct {
		name    string
		active  core.ThemeMode
		variant fyne.ThemeVariant
		want    core.ThemeMode
	}{
		// AC-DS-15 — System mode follows the OS-supplied variant.
		{"system+osLight=light", core.ThemeSystem, fynetheme.VariantLight, core.ThemeLight},
		{"system+osDark=dark", core.ThemeSystem, fynetheme.VariantDark, core.ThemeDark},

		// Explicit modes pin the palette even when OS says otherwise.
		// (Same row, second half of the rule from
		// spec/24-app-design-system-and-ui/02-theme-implementation.md §3.)
		{"explicitLight+osDark=light", core.ThemeLight, fynetheme.VariantDark, core.ThemeLight},
		{"explicitDark+osLight=dark", core.ThemeDark, fynetheme.VariantLight, core.ThemeDark},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if r := Apply(tc.active); r.HasError() {
				t.Fatalf("Apply(%v): %v", tc.active, r.Error())
			}
			if got := resolvedMode(tc.variant); got != tc.want {
				t.Errorf("resolvedMode(active=%v, osVariant=%v) = %v, want %v",
					tc.active, tc.variant, got, tc.want)
			}
		})
	}
}
