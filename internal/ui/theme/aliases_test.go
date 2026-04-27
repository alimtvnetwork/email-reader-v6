// Package theme — aliases_test.go implements the AC-DS-05
// "Test_Tokens_NoDuplicateValues" contract from
// spec/24-app-design-system-and-ui/97-acceptance-criteria.md.
//
// 3-clause contract (per spec/24-…/01-tokens.md §2.12):
//
//  1. Pairwise scan: for every distinct (a, b) in AllColorNames() and
//     every variant v ∈ {Dark, Light}, if palette[v][a] == palette[v][b]
//     and (a, b) is NOT in NamedAliases for that variant's scope, fail.
//
//  2. Registered-alias parity: every NamedAliases row's RGB equality
//     must hold in its declared scope. Asymmetric rows
//     (DarkOnly/LightOnly) MUST also have the off-variant DIFFER —
//     otherwise the row is misleading and should be promoted to
//     AliasBoth.
//
//  3. Registry hygiene: no reflexive entries (A == B) and no duplicate
//     rows (canonicalised unordered).
package theme

import (
	"image/color"
	"testing"
)

// Test_Tokens_NoDuplicateValues — AC-DS-05.
func Test_Tokens_NoDuplicateValues(t *testing.T) {
	// Registry hygiene first — bad registry rows would poison the
	// lookup tables built from them.
	checkRegistryHygiene(t)

	// Build per-scope lookup sets keyed by canonicalised (A,B).
	allowedDark := allowedPairsForVariant(AliasBoth, AliasDarkOnly)
	allowedLight := allowedPairsForVariant(AliasBoth, AliasLightOnly)

	// Clause 1 — pairwise scan.
	scanForUnregisteredDuplicates(t, "Dark", paletteDark, allowedDark)
	scanForUnregisteredDuplicates(t, "Light", paletteLight, allowedLight)

	// Clause 2 — registered-alias parity (RGB equality + asymmetry).
	checkRegisteredAliasParity(t)
}

// pairKey is the canonical (low, high) ColorName pair used as a map key.
type pairKey struct{ A, B ColorName }

func canonicalPair(a, b ColorName) pairKey {
	if a < b {
		return pairKey{a, b}
	}
	return pairKey{b, a}
}

// allowedPairsForVariant returns the set of canonical pairs that may
// legally collide in the named variant's palette. Pass AliasBoth plus
// the variant-specific scope (AliasDarkOnly or AliasLightOnly).
func allowedPairsForVariant(scopes ...AliasScope) map[pairKey]bool {
	want := map[AliasScope]bool{}
	for _, s := range scopes {
		want[s] = true
	}
	out := map[pairKey]bool{}
	for _, al := range NamedAliases {
		if !want[al.Scope] {
			continue
		}
		lo, hi := al.canonicalise()
		out[pairKey{lo, hi}] = true
	}
	return out
}

// scanForUnregisteredDuplicates implements clause 1.
func scanForUnregisteredDuplicates(t *testing.T, label string, p map[ColorName]color.NRGBA, allowed map[pairKey]bool) {
	t.Helper()
	names := AllColorNames()
	for i, a := range names {
		for j := i + 1; j < len(names); j++ {
			b := names[j]
			if p[a] != p[b] {
				continue
			}
			key := canonicalPair(a, b)
			if allowed[key] {
				continue
			}
			t.Errorf("AC-DS-05: %s palette has unregistered duplicate: "+
				"%s and %s both map to %v. Either pick a distinct RGB "+
				"or register the pair in NamedAliases (aliases.go) "+
				"with a one-line rationale.",
				label, a, b, p[a])
		}
	}
}

// checkRegisteredAliasParity implements clause 2.
func checkRegisteredAliasParity(t *testing.T) {
	t.Helper()
	for _, al := range NamedAliases {
		dEq := paletteDark[al.A] == paletteDark[al.B]
		lEq := paletteLight[al.A] == paletteLight[al.B]
		switch al.Scope {
		case AliasBoth:
			if !dEq {
				t.Errorf("NamedAliases[%s↔%s] scope=both but Dark differs: %v vs %v",
					al.A, al.B, paletteDark[al.A], paletteDark[al.B])
			}
			if !lEq {
				t.Errorf("NamedAliases[%s↔%s] scope=both but Light differs: %v vs %v",
					al.A, al.B, paletteLight[al.A], paletteLight[al.B])
			}
		case AliasDarkOnly:
			if !dEq {
				t.Errorf("NamedAliases[%s↔%s] scope=darkOnly but Dark differs: %v vs %v",
					al.A, al.B, paletteDark[al.A], paletteDark[al.B])
			}
			if lEq {
				t.Errorf("NamedAliases[%s↔%s] scope=darkOnly but Light is also equal (%v) — "+
					"promote to AliasBoth or pick a distinct Light RGB.",
					al.A, al.B, paletteLight[al.A])
			}
		case AliasLightOnly:
			if !lEq {
				t.Errorf("NamedAliases[%s↔%s] scope=lightOnly but Light differs: %v vs %v",
					al.A, al.B, paletteLight[al.A], paletteLight[al.B])
			}
			if dEq {
				t.Errorf("NamedAliases[%s↔%s] scope=lightOnly but Dark is also equal (%v) — "+
					"promote to AliasBoth or pick a distinct Dark RGB.",
					al.A, al.B, paletteDark[al.A])
			}
		default:
			t.Errorf("NamedAliases[%s↔%s] has unknown scope %d", al.A, al.B, al.Scope)
		}
	}
}

// checkRegistryHygiene implements clause 3.
func checkRegistryHygiene(t *testing.T) {
	t.Helper()
	seen := map[pairKey]int{}
	for i, al := range NamedAliases {
		if al.A == al.B {
			t.Errorf("NamedAliases[%d] is reflexive (%s == %s); reflexive entries are forbidden", i, al.A, al.B)
			continue
		}
		lo, hi := al.canonicalise()
		key := pairKey{lo, hi}
		if prev, dup := seen[key]; dup {
			t.Errorf("NamedAliases[%d] duplicates row %d (canonical pair %s↔%s)", i, prev, lo, hi)
		}
		seen[key] = i
	}
}
