package theme

import (
	"image/color"
	"testing"

	"github.com/lovable/email-read/internal/core"
)

// Test_Palettes_Parity asserts both palettes define every token in
// AllColorNames(). Adding a token without updating both maps is a
// build-time bug per spec/24-…/01-tokens.md §2.9.
//
// Satisfies AC-DS-01 (every ColorName matches §2 of 01-tokens.md
// one-to-one) and AC-DS-03 (each color has both Dark and Light
// variants in palette_dark.go / palette_light.go) — both rows from
// spec/24-app-design-system-and-ui/97-acceptance-criteria.md.
func Test_Palettes_Parity(t *testing.T) {
	for _, name := range AllColorNames() {
		if _, ok := paletteDark[name]; !ok {
			t.Errorf("paletteDark missing %q", name)
		}
		if _, ok := paletteLight[name]; !ok {
			t.Errorf("paletteLight missing %q", name)
		}
	}
	if got, want := len(paletteDark), len(AllColorNames()); got != want {
		t.Errorf("paletteDark len=%d, want %d (extra/stale entries)", got, want)
	}
	if got, want := len(paletteLight), len(AllColorNames()); got != want {
		t.Errorf("paletteLight len=%d, want %d", got, want)
	}
}

// Test_Color_ResolvesPerMode walks a representative subset of tokens and
// confirms that switching ThemeMode flips the resolved color. Picks
// tokens whose Dark/Light values differ in the spec.
func Test_Color_ResolvesPerMode(t *testing.T) {
	t.Cleanup(resetForTest)
	cases := []ColorName{
		ColorBackground, ColorForeground, ColorPrimary,
		ColorSidebar, ColorWatchDotWatching, ColorWatchDotError,
	}
	for _, name := range cases {
		_ = Apply(core.ThemeDark)
		dark := Color(name)
		_ = Apply(core.ThemeLight)
		light := Color(name)
		if dark == light {
			t.Errorf("%s: dark==light (%v); spec requires distinct values", name, dark)
		}
	}
}

// Test_Color_KnownTokens checks exact RGB values for the two MVP groups
// called out by Delta #4: sidebar tokens and WatchDot tokens. Locks the
// public token contract so refactors don't drift the palette.
func Test_Color_KnownTokens(t *testing.T) {
	t.Cleanup(resetForTest)
	_ = Apply(core.ThemeDark)
	checks := map[ColorName]color.NRGBA{
		ColorSidebar:              {19, 21, 26, 255},
		ColorSidebarItemActive:    {46, 56, 90, 255},
		ColorWatchDotWatching:     {74, 200, 130, 255},
		ColorWatchDotReconnecting: {240, 175, 60, 255},
		ColorWatchDotError:        {240, 90, 105, 255},
	}
	for name, want := range checks {
		if got := Color(name); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

// Test_Color_RawLogBadgeCodeTokens locks every spec value in §2.6, §2.7,
// §2.8 (resolves OI-1 second half + AC-DS-04). Both modes exercised so a
// drift in either palette fails the build.
func Test_Color_RawLogBadgeCodeTokens(t *testing.T) {
	t.Cleanup(resetForTest)
	cases := []struct {
		mode core.ThemeMode
		name ColorName
		want color.NRGBA
	}{
		// §2.6 raw log — dark
		{core.ThemeDark, ColorRawLogHeartbeat, color.NRGBA{90, 96, 110, 255}},
		{core.ThemeDark, ColorRawLogNewMail, color.NRGBA{235, 237, 242, 255}},
		{core.ThemeDark, ColorRawLogError, color.NRGBA{240, 90, 105, 255}},
		// Slice #118d palette tune: was {120,125,135}; lifted for WCAG AA.
		{core.ThemeDark, ColorRawLogTimestamp, color.NRGBA{140, 145, 155, 255}},
		// §2.7 badges — dark
		{core.ThemeDark, ColorRuleMatchBadge, color.NRGBA{170, 130, 255, 255}},
		{core.ThemeDark, ColorBadgeNeutralBg, color.NRGBA{46, 49, 58, 255}},
		{core.ThemeDark, ColorBadgeNeutralFg, color.NRGBA{200, 205, 215, 255}},
		// §2.8 code — dark
		{core.ThemeDark, ColorCodeBg, color.NRGBA{19, 21, 26, 255}},
		{core.ThemeDark, ColorCodeBorder, color.NRGBA{46, 49, 58, 255}},
		{core.ThemeDark, ColorCodeLineHighlight, color.NRGBA{30, 33, 40, 255}},
		{core.ThemeDark, ColorCodeSelection, color.NRGBA{46, 72, 130, 255}},
		// light-mode spot checks (one per group)
		{core.ThemeLight, ColorRawLogNewMail, color.NRGBA{15, 17, 21, 255}},
		{core.ThemeLight, ColorRuleMatchBadge, color.NRGBA{122, 82, 220, 255}},
		{core.ThemeLight, ColorCodeBg, color.NRGBA{244, 245, 248, 255}},
	}
	for _, tc := range cases {
		_ = Apply(tc.mode)
		if got := Color(tc.name); got != tc.want {
			t.Errorf("[%v] %s = %v, want %v", tc.mode, tc.name, got, tc.want)
		}
	}
}

// Test_Color_UnknownFallback confirms the no-panic contract: any
// undefined token returns ColorForeground for the active mode and logs
// ER-UI-21900 (we don't assert log output here — visual smoke only).
func Test_Color_UnknownFallback(t *testing.T) {
	t.Cleanup(resetForTest)
	_ = Apply(core.ThemeDark)
	got := Color(ColorName("DoesNotExist"))
	want := paletteDark[ColorForeground]
	if got != want {
		t.Errorf("unknown token fallback = %v, want %v (ColorForeground/Dark)", got, want)
	}
}

// Test_Apply_RejectsInvalid covers the validation path: a zero-valued
// ThemeMode (uninitialised enum) must fail loudly with ER-SET-21772 and
// not mutate global state.
func Test_Apply_RejectsInvalid(t *testing.T) {
	t.Cleanup(resetForTest)
	_ = Apply(core.ThemeLight)
	before := Active()
	r := Apply(core.ThemeMode(0))
	if !r.HasError() {
		t.Fatalf("expected error for ThemeMode(0)")
	}
	if Active() != before {
		t.Errorf("Active mode mutated on invalid Apply: was %v now %v", before, Active())
	}
}

// Test_Apply_ConcurrentSafe smoke-tests the RWMutex contract: concurrent
// reads + writes must neither race nor panic.
func Test_Apply_ConcurrentSafe(t *testing.T) {
	t.Cleanup(resetForTest)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			if i%2 == 0 {
				_ = Apply(core.ThemeDark)
			} else {
				_ = Apply(core.ThemeLight)
			}
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		_ = Color(ColorPrimary)
	}
	<-done
}
