// sizes.go defines the typography + spacing scale per
// spec/24-app-design-system-and-ui/01-tokens.md §3 and §4, plus the
// Comfortable/Compact density resolver from §8.
//
// Pure Go (fyne-free) so it builds under -tags nofyne alongside the
// rest of the token contract. The Fyne adapter in fyne_theme.go routes
// fyne.ThemeSizeName → SizeName via sizeRouteForFyne.
package theme

import "math"

// SizeName is a typed enum for every typography + spacing token. Using
// a typed string (instead of raw `string`) makes misspellings a
// compile-time error and lets ast_test scan accidental literal usage.
//
// Spec: 01-tokens.md §1 (Naming Convention).
type SizeName string

// Typography tokens — 01-tokens.md §3.1.
const (
	SizeTextPageTitle    SizeName = "TextPageTitle"
	SizeTextSectionTitle SizeName = "TextSectionTitle"
	SizeTextCardTitle    SizeName = "TextCardTitle"
	SizeTextBody         SizeName = "TextBody"
	SizeTextCaption      SizeName = "TextCaption"
	SizeTextCode         SizeName = "TextCode"
	SizeTextButton       SizeName = "TextButton"
)

// Spacing tokens — 01-tokens.md §4. 4 px base unit.
const (
	SizeSpacing0 SizeName = "Spacing0"
	SizeSpacing1 SizeName = "Spacing1"
	SizeSpacing2 SizeName = "Spacing2"
	SizeSpacing3 SizeName = "Spacing3"
	SizeSpacing4 SizeName = "Spacing4"
	SizeSpacing5 SizeName = "Spacing5"
	SizeSpacing6 SizeName = "Spacing6"
	SizeSpacing7 SizeName = "Spacing7"
)

// sizeBase holds the Comfortable (default) px values from the spec.
// Compact mode multiplies by 0.875 and rounds to the nearest px (§8).
var sizeBase = map[SizeName]float32{
	// §3.1 typography
	SizeTextPageTitle:    30,
	SizeTextSectionTitle: 20,
	SizeTextCardTitle:    16,
	SizeTextBody:         14,
	SizeTextCaption:      12,
	SizeTextCode:         13,
	SizeTextButton:       14,
	// §4 spacing
	SizeSpacing0: 0,
	SizeSpacing1: 4,
	SizeSpacing2: 8,
	SizeSpacing3: 12,
	SizeSpacing4: 16,
	SizeSpacing5: 24,
	SizeSpacing6: 32,
	SizeSpacing7: 48,
}

// allSizeNames is the canonical iteration order for parity tests.
// Adding a new token requires appending here AND extending sizeBase.
var allSizeNames = []SizeName{
	SizeTextPageTitle, SizeTextSectionTitle, SizeTextCardTitle,
	SizeTextBody, SizeTextCaption, SizeTextCode, SizeTextButton,
	SizeSpacing0, SizeSpacing1, SizeSpacing2, SizeSpacing3,
	SizeSpacing4, SizeSpacing5, SizeSpacing6, SizeSpacing7,
}

// AllSizeNames returns a defensive copy of the canonical list.
func AllSizeNames() []SizeName {
	out := make([]SizeName, len(allSizeNames))
	copy(out, allSizeNames)
	return out
}

// Density picks Comfortable (default) or Compact. Compact multiplies
// every Size by 0.875, rounded to the nearest px (§8). Token IDs are
// unchanged; only the resolved value differs.
type Density int

const (
	DensityComfortable Density = 0 // default
	DensityCompact     Density = 1
)

// activeDensity is read on every Size() call so density toggles take
// effect on the next refresh without re-applying the theme.
var activeDensity Density = DensityComfortable

// SetDensity updates the active density. Safe to call from any
// goroutine — it's a single atomic-ish int write under the same RWMutex
// as the theme mode (Size() reads it under RLock). v1 ships with
// Comfortable as the only user-visible option; Compact is wired and
// covered by tests so the Settings UI can flip it later.
func SetDensity(d Density) {
	state.mu.Lock()
	activeDensity = d
	state.mu.Unlock()
}

// ActiveDensity returns the currently applied density. Cheap RLock read.
func ActiveDensity() Density {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return activeDensity
}

// Size resolves a typography or spacing token in px, honoring the
// active density. Unknown name → 0 (no panic on the hot path) and a
// dedup-logged ER-UI-21900 via the same warnUnknown channel as Color.
func Size(name SizeName) float32 {
	base, ok := sizeBase[name]
	if !ok {
		warnUnknownSize(name)
		return 0
	}
	if ActiveDensity() == DensityCompact {
		return float32(math.Round(float64(base) * 0.875))
	}
	return base
}

// warnUnknownSize reuses the warnedTokens dedup channel for size tokens
// by namespacing them under "size:" so an unknown size and an unknown
// color of the same name don't collide.
func warnUnknownSize(name SizeName) {
	warnUnknown(ColorName("size:" + string(name)))
}
