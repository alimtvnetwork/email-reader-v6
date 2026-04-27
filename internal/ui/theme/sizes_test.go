// sizes_test.go locks the §3 typography scale, §4 spacing scale, and
// §8 density resolver from spec/24-app-design-system-and-ui/01-tokens.md.
package theme

import (
	"testing"
)

// resetDensityForTest restores Comfortable so test order doesn't matter.
func resetDensityForTest() { SetDensity(DensityComfortable) }

// Test_Size_ComfortableScale locks every spec value from §3.1 and §4.
// Single source of truth for the typography + spacing contract.
func Test_Size_ComfortableScale(t *testing.T) {
	t.Cleanup(resetDensityForTest)
	SetDensity(DensityComfortable)
	cases := map[SizeName]float32{
		// §3.1 typography
		SizeTextPageTitle:    30,
		SizeTextSectionTitle: 20,
		SizeTextCardTitle:    16,
		SizeTextBody:         14,
		SizeTextCaption:      12,
		SizeTextCode:         13,
		SizeTextButton:       14,
		// §4 spacing (4 px base unit)
		SizeSpacing0: 0,
		SizeSpacing1: 4,
		SizeSpacing2: 8,
		SizeSpacing3: 12,
		SizeSpacing4: 16,
		SizeSpacing5: 24,
		SizeSpacing6: 32,
		SizeSpacing7: 48,
	}
	for name, want := range cases {
		if got := Size(name); got != want {
			t.Errorf("%s = %v, want %v (Comfortable)", name, got, want)
		}
	}
}

// Test_Size_CompactScale verifies the §8 multiplier (×0.875, rounded).
// Spot-checks every group; full table not needed because the formula is
// uniform.
func Test_Size_CompactScale(t *testing.T) {
	t.Cleanup(resetDensityForTest)
	SetDensity(DensityCompact)
	cases := map[SizeName]float32{
		SizeTextBody:         12, // 14 * 0.875 = 12.25 → 12
		SizeTextPageTitle:    26, // 30 * 0.875 = 26.25 → 26
		SizeTextSectionTitle: 18, // 20 * 0.875 = 17.50 → 18
		SizeTextCardTitle:    14, // 16 * 0.875 = 14.00 → 14
		SizeTextCaption:      11, // 12 * 0.875 = 10.50 → 11 (banker → away-from-zero)
		SizeSpacing0:         0,
		SizeSpacing2:         7,  //  8 * 0.875 = 7.00
		SizeSpacing4:         14, // 16 * 0.875 = 14.00
		SizeSpacing7:         42, // 48 * 0.875 = 42.00
	}
	for name, want := range cases {
		if got := Size(name); got != want {
			t.Errorf("%s = %v, want %v (Compact)", name, got, want)
		}
	}
}

// Test_Size_DensityToggleNoStateLeak confirms switching density flips
// the resolved value live without re-Apply.
func Test_Size_DensityToggleNoStateLeak(t *testing.T) {
	t.Cleanup(resetDensityForTest)
	SetDensity(DensityComfortable)
	if Size(SizeTextBody) != 14 {
		t.Fatalf("baseline Comfortable wrong: %v", Size(SizeTextBody))
	}
	SetDensity(DensityCompact)
	if Size(SizeTextBody) != 12 {
		t.Fatalf("after Compact: %v, want 12", Size(SizeTextBody))
	}
	SetDensity(DensityComfortable)
	if Size(SizeTextBody) != 14 {
		t.Fatalf("back to Comfortable: %v, want 14", Size(SizeTextBody))
	}
}

// Test_Size_UnknownReturnsZero confirms the no-panic contract — unknown
// tokens return 0 (a safe layout default) and log ER-UI-21900 once.
func Test_Size_UnknownReturnsZero(t *testing.T) {
	t.Cleanup(resetDensityForTest)
	if got := Size(SizeName("DoesNotExist")); got != 0 {
		t.Errorf("unknown size = %v, want 0", got)
	}
}

// Test_AllSizeNames_CoversBase asserts the canonical iteration list and
// the resolver table stay in sync. Adding a token to one without the
// other fails the build.
//
// Satisfies the SizeName half of AC-DS-02 (every SizeName /
// RadiusName / ElevName / MotionName matches 01-tokens.md §3–§7).
// The MotionName branch is locked by Test_ReducedMotion_CollapsesTokens
// in motion_test.go; RadiusName and ElevName are gated by the same
// AllNames()-style parity contract embedded in their resolvers.
func Test_AllSizeNames_CoversBase(t *testing.T) {
	if got, want := len(AllSizeNames()), len(sizeBase); got != want {
		t.Errorf("AllSizeNames len=%d, sizeBase len=%d — drift", got, want)
	}
	for _, name := range AllSizeNames() {
		if _, ok := sizeBase[name]; !ok {
			t.Errorf("AllSizeNames contains %q but sizeBase has no entry", name)
		}
	}
}

// Test_Fonts_FallbackWhenAbsent verifies the embed-or-nil contract:
// when no .ttf files have been committed under fonts/, both accessors
// return nil so the AppTheme.Font router can fall back to Fyne defaults.
// This test stays valid once fonts ship — it just stops returning nil
// and the assertion is relaxed via the dual-branch check below.
func Test_Fonts_FallbackWhenAbsent(t *testing.T) {
	inter := TextFont()
	mono := TextMonospaceFont()
	// Either both are present (assets committed) or both are nil
	// (fresh checkout). A mixed state means one filename drifted.
	if (inter == nil) != (mono == nil) {
		t.Errorf("font asset state mismatch: inter=%v mono=%v "+
			"— check fonts.go path constants vs fonts/*.ttf", inter != nil, mono != nil)
	}
	if inter != nil && len(inter.Content()) == 0 {
		t.Errorf("Inter resource present but empty — embed glob misconfigured")
	}
	if mono != nil && len(mono.Content()) == 0 {
		t.Errorf("JetBrains Mono resource present but empty — embed glob misconfigured")
	}
}

// Test_Fonts_PresentAndValidTTF locks the fact that the OFL .ttf assets
// have shipped — both fonts must load and start with the TrueType magic
// signature 0x00010000. Drops to t.Skip on a fresh checkout so the
// test pair stays compatible with the intentional fallback contract.
func Test_Fonts_PresentAndValidTTF(t *testing.T) {
	cases := []struct {
		name string
		load func() (resourceContent, bool)
	}{
		{"Inter", func() (resourceContent, bool) {
			r := TextFont()
			if r == nil {
				return nil, false
			}
			return r.Content(), true
		}},
		{"JetBrainsMono", func() (resourceContent, bool) {
			r := TextMonospaceFont()
			if r == nil {
				return nil, false
			}
			return r.Content(), true
		}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			data, ok := c.load()
			if !ok {
				t.Skipf("%s asset not committed yet — skip per fallback contract", c.name)
			}
			if len(data) < 4 {
				t.Fatalf("%s payload too small: %d bytes", c.name, len(data))
			}
			// TrueType magic: 0x00010000 (TTF) or "OTTO" (OpenType CFF).
			isTTF := data[0] == 0x00 && data[1] == 0x01 && data[2] == 0x00 && data[3] == 0x00
			isOTF := data[0] == 'O' && data[1] == 'T' && data[2] == 'T' && data[3] == 'O'
			if !isTTF && !isOTF {
				t.Fatalf("%s: not a valid TTF/OTF (magic=%x %x %x %x)",
					c.name, data[0], data[1], data[2], data[3])
			}
		})
	}
}

// resourceContent is a tiny alias to keep the table-driven test above
// readable without importing fyne types into the test signature line.
type resourceContent = []byte
