package theme

import (
	"image/color"
	"sort"
	"testing"
)

// Test_Tokens_NoDuplicateValues enforces AC-DS-05 (spec/24/01-tokens.md
// §2.12): every duplicate-RGB pair across the Dark and Light palettes
// MUST be registered in NamedAliases, and every registered alias MUST
// actually hold per its declared scope.
//
// Three sub-tests so failures localize:
//
//   - clause1_pairwise — every observed duplicate is registered.
//   - clause2_scope    — every registered alias holds; asymmetric scopes
//                        are also asymmetric in the *other* variant.
//   - clause3_hygiene  — registry has no reflexive or duplicate entries.
func Test_Tokens_NoDuplicateValues(t *testing.T) {
	t.Run("clause1_pairwise", func(t *testing.T) {
		checkPairwise(t, "Dark", paletteDark, scopeCoversDark)
		checkPairwise(t, "Light", paletteLight, scopeCoversLight)
	})
	t.Run("clause2_scope", func(t *testing.T) {
		for _, a := range NamedAliases {
			d1, d2 := paletteDark[a.From], paletteDark[a.To]
			l1, l2 := paletteLight[a.From], paletteLight[a.To]
			darkEq, lightEq := rgbEqual(d1, d2), rgbEqual(l1, l2)
			switch a.Scope {
			case AliasBoth:
				if !darkEq {
					t.Errorf("%s↔%s: scope=both but Dark RGBs differ "+
						"(%v vs %v) — palette change broke the alias",
						a.From, a.To, rgbTuple(d1), rgbTuple(d2))
				}
				if !lightEq {
					t.Errorf("%s↔%s: scope=both but Light RGBs differ "+
						"(%v vs %v) — palette change broke the alias",
						a.From, a.To, rgbTuple(l1), rgbTuple(l2))
				}
			case AliasDarkOnly:
				if !darkEq {
					t.Errorf("%s↔%s: scope=darkOnly but Dark RGBs differ "+
						"(%v vs %v)", a.From, a.To, rgbTuple(d1), rgbTuple(d2))
				}
				if lightEq {
					t.Errorf("%s↔%s: scope=darkOnly but Light RGBs ALSO match "+
						"(%v) — promote to AliasBoth or pick a distinct Light RGB",
						a.From, a.To, rgbTuple(l1))
				}
			case AliasLightOnly:
				if !lightEq {
					t.Errorf("%s↔%s: scope=lightOnly but Light RGBs differ "+
						"(%v vs %v)", a.From, a.To, rgbTuple(l1), rgbTuple(l2))
				}
				if darkEq {
					t.Errorf("%s↔%s: scope=lightOnly but Dark RGBs ALSO match "+
						"(%v) — promote to AliasBoth or pick a distinct Dark RGB",
						a.From, a.To, rgbTuple(d1))
				}
			default:
				t.Errorf("%s↔%s: unknown AliasScope value %d", a.From, a.To, a.Scope)
			}
		}
	})
	t.Run("clause3_hygiene", func(t *testing.T) {
		seen := map[[2]ColorName]int{}
		for i, a := range NamedAliases {
			if a.From == a.To {
				t.Errorf("entry #%d: reflexive alias %s↔%s is forbidden",
					i, a.From, a.To)
				continue
			}
			x, y := canonicaliseAlias(a)
			key := [2]ColorName{x, y}
			if prev, ok := seen[key]; ok {
				t.Errorf("entry #%d: duplicate alias %s↔%s "+
					"(already declared at entry #%d)", i, a.From, a.To, prev)
				continue
			}
			seen[key] = i
		}
	})
}

// checkPairwise scans every unordered pair of color tokens in `pal` and
// fails for any duplicate RGB triple that is not covered by a registry
// entry whose Scope satisfies `scopeOK`.
func checkPairwise(t *testing.T, label string, pal map[ColorName]color.NRGBA,
	scopeOK func(AliasScope) bool) {
	t.Helper()
	names := AllColorNames()
	// Stable iteration so failures sort deterministically.
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })

	allowed := map[[2]ColorName]struct{}{}
	for _, a := range NamedAliases {
		if !scopeOK(a.Scope) {
			continue
		}
		x, y := canonicaliseAlias(a)
		allowed[[2]ColorName{x, y}] = struct{}{}
	}

	for i, a := range names {
		ca, ok := pal[a]
		if !ok {
			continue
		}
		for _, b := range names[i+1:] {
			cb, ok := pal[b]
			if !ok {
				continue
			}
			if !rgbEqual(ca, cb) {
				continue
			}
			x, y := a, b
			if x > y {
				x, y = y, x
			}
			if _, ok := allowed[[2]ColorName{x, y}]; ok {
				continue
			}
			t.Errorf("AC-DS-05 (%s): tokens %s and %s share RGB %v "+
				"but no NamedAliases entry covers this variant — either "+
				"pick a distinct value or register the alias in aliases.go",
				label, a, b, rgbTuple(ca))
		}
	}
}

func scopeCoversDark(s AliasScope) bool  { return s == AliasBoth || s == AliasDarkOnly }
func scopeCoversLight(s AliasScope) bool { return s == AliasBoth || s == AliasLightOnly }

func rgbEqual(a, b color.NRGBA) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B
}

func rgbTuple(c color.NRGBA) [3]uint8 { return [3]uint8{c.R, c.G, c.B} }
