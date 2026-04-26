// density_test.go — covers the Density enum + persistence round-trip
// added in Slice #42. The Density field is the smallest persisted Settings
// field but it crosses three layers (core enum ↔ JSON ↔ UI int form), so
// each transition gets a focused test.
package core

import (
	"context"
	"testing"
	"time"
)

func TestParseDensity(t *testing.T) {
	cases := []struct {
		in    string
		want  Density
		wantOK bool
	}{
		{"", DensityComfortable, true},        // empty → default
		{"Comfortable", DensityComfortable, true},
		{"Compact", DensityCompact, true},
		{"compact", DensityComfortable, false}, // case-sensitive per ParseThemeMode contract
		{"bogus", DensityComfortable, false},
	}
	for _, tc := range cases {
		got, ok := ParseDensity(tc.in)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("ParseDensity(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestDensity_String(t *testing.T) {
	if DensityComfortable.String() != "Comfortable" {
		t.Errorf("Comfortable.String() = %q", DensityComfortable.String())
	}
	if DensityCompact.String() != "Compact" {
		t.Errorf("Compact.String() = %q", DensityCompact.String())
	}
	if Density(99).String() != "Comfortable" {
		t.Errorf("unknown Density.String() should default to Comfortable")
	}
}

func TestDefaultSettings_Density(t *testing.T) {
	if got := DefaultSettingsInput().Density; got != DensityComfortable {
		t.Errorf("default Density = %v, want Comfortable", got)
	}
}

// TestSettings_DensityRoundTrip is the integration test: Save with
// Compact, re-read via Get, observe Compact. Catches breakage in
// applyInputToRaw / projectExtension / snapshotFromRaw / writeConfig.
func TestSettings_DensityRoundTrip(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := NewSettings(time.Now)
		if s.HasError() {
			t.Fatalf("NewSettings: %v", s.Error())
		}
		// Start from defaults so all the maintenance knobs are valid.
		in := DefaultSettingsInput()
		in.Density = DensityCompact
		if r := s.Value().Save(context.Background(), in); r.HasError() {
			t.Fatalf("Save Compact: %v", r.Error())
		}

		// Re-construct to defeat any in-memory cache and force a disk read.
		s2 := NewSettings(time.Now)
		if s2.HasError() {
			t.Fatalf("NewSettings (reload): %v", s2.Error())
		}
		snap := s2.Value().Get(context.Background())
		if snap.HasError() {
			t.Fatalf("Get reload: %v", snap.Error())
		}
		if snap.Value().Density != DensityCompact {
			t.Errorf("Density did not persist: got %v want Compact", snap.Value().Density)
		}
	})
}

// TestSettings_DensityNormalizeZeroValue locks the lenient zero-value
// path that protects existing callers (and pre-Slice-#42 test fixtures)
// from suddenly tripping ER-SET-21783 when they omit Density.
func TestSettings_DensityNormalizeZeroValue(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := NewSettings(time.Now)
		if s.HasError() {
			t.Fatalf("NewSettings: %v", s.Error())
		}
		in := DefaultSettingsInput()
		in.Density = 0 // simulate a partial caller
		r := s.Value().Save(context.Background(), in)
		if r.HasError() {
			t.Fatalf("Save with Density=0 should succeed via normalize, got: %v", r.Error())
		}
		if r.Value().Density != DensityComfortable {
			t.Errorf("normalized Density = %v, want Comfortable", r.Value().Density)
		}
	})
}

// TestSettings_DensityValidationRejectsUnknown locks the validator so a
// caller that explicitly sets a junk Density (e.g. Density(99)) gets the
// dedicated ER-SET-21783 code, not silent coercion.
func TestSettings_DensityValidationRejectsUnknown(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := NewSettings(time.Now)
		if s.HasError() {
			t.Fatalf("NewSettings: %v", s.Error())
		}
		in := DefaultSettingsInput()
		in.Density = Density(99)
		r := s.Value().Save(context.Background(), in)
		if !r.HasError() {
			t.Fatal("expected validation error for Density=99")
		}
	})
}
