package config

import (
	"strings"
	"testing"
)

// Regress_Issue01_TestConnectionSurfacesEr21430 — Issue 01 (wrong password):
// the underlying bug was a user-data error, but the spec invariant that came
// out of it is "EncodePassword must be a faithful round-trip; do NOT investigate
// Base64 logic when AUTHENTICATIONFAILED appears". This test locks that
// invariant: a known-good password round-trips byte-for-byte through Encode →
// Decode regardless of Unicode/whitespace, so any future "auth failed" report
// can never be blamed on the encoding layer.
//
// Maps to AC-PROJ-23.
func Regress_Issue01_TestConnectionSurfacesEr21430(t *testing.T) {
	cases := []string{
		"ZPb*sz=d!cEE_Wgc",
		"plain",
		"with space",       // internal space MUST survive
		"unicode-café-π",   // multibyte must survive
		"!@#$%^&*()_+-=[]", // shell metas must survive
	}
	for _, plain := range cases {
		enc := EncodePassword(plain)
		if enc == plain {
			t.Errorf("EncodePassword returned plaintext for %q — encoding is a no-op", plain)
			continue
		}
		dec, err := DecodePassword(enc)
		if err != nil {
			t.Errorf("DecodePassword(%q) err=%v", plain, err)
			continue
		}
		if dec != plain {
			t.Errorf("round-trip mismatch: in=%q out=%q (encoding layer is NOT the bug source)", plain, dec)
		}
	}
}

// Regress_Issue03_SanitizePasswordStripsCfRunes — Issue 03 (hidden Unicode):
// pasted passwords surrounded by U+2060 WORD JOINER, U+200B ZWSP, NBSP, BOM,
// or stray ASCII whitespace must be cleaned at the boundary so IMAP receives
// only the visible bytes.
//
// Maps to AC-PROJ-25.
func Regress_Issue03_SanitizePasswordStripsCfRunes(t *testing.T) {
	want := "ZPb*sz=d!cEE_Wgc"
	dirties := []string{
		"\u2060" + want + "\u2060",         // WORD JOINER both sides
		"\u200B" + want,                    // ZERO-WIDTH SPACE leading
		want + "\u200B",                    // ZERO-WIDTH SPACE trailing
		"\uFEFF" + want,                    // BOM leading
		"\u00A0" + want + "\u00A0",         // NBSP both sides
		" \t" + want + "\n",                // ASCII whitespace
		"\u2060 " + want + " \u2060",       // WORD JOINER + ASCII space (the original issue)
	}
	for _, dirty := range dirties {
		got := SanitizePassword(dirty)
		if got != want {
			t.Errorf("SanitizePassword(%q) = %q (len %d), want %q (len %d) — invisible runes leaked into IMAP login",
				dirty, got, len(got), want, len(want))
		}
	}

	// Defense-in-depth: stored passwords from before the fix should
	// still come out clean via Decode.
	for _, dirty := range dirties {
		enc := EncodePassword(dirty)
		dec, err := DecodePassword(enc)
		if err != nil {
			t.Fatalf("decode err: %v", err)
		}
		if dec != want {
			t.Errorf("decode-after-encode for dirty %q = %q, want %q", dirty, dec, want)
		}
	}

	// Sanity: nothing inside the visible password should be removed.
	internalSpace := "ab cd"
	if got := SanitizePassword(internalSpace); got != internalSpace {
		t.Errorf("internal space stripped: got %q want %q", got, internalSpace)
	}

	// And we never want an empty result for non-empty visible input.
	if got := SanitizePassword("\u2060x\u2060"); !strings.Contains(got, "x") {
		t.Errorf("SanitizePassword stripped the visible payload too: %q", got)
	}
}
