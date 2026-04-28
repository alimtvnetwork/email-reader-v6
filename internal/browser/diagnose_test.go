package browser

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// Test_Diagnose_PicksConfigOverrideWhenItExists verifies the resolver
// order: a real file at config.browser.chromePath wins over env/OS/PATH,
// and the report marks exactly one Picked candidate.
func Test_Diagnose_PicksConfigOverrideWhenItExists(t *testing.T) {
	dir := t.TempDir()
	p := writeFakeBrowser(t, dir, "chrome-stub")

	rep := Diagnose(config.Browser{ChromePath: p})

	if rep.ResolvedPath != p {
		t.Fatalf("ResolvedPath = %q, want %q", rep.ResolvedPath, p)
	}
	if rep.Error != "" {
		t.Fatalf("unexpected Error: %s", rep.Error)
	}
	picked := 0
	for _, c := range rep.Candidates {
		if c.Picked {
			picked++
			if c.Source != SourceConfigOverride {
				t.Errorf("Picked candidate Source = %q, want %q", c.Source, SourceConfigOverride)
			}
		}
	}
	if picked != 1 {
		t.Fatalf("Picked count = %d, want 1", picked)
	}
	if !strings.Contains(rep.WouldSpawn, p) {
		t.Errorf("WouldSpawn = %q, want it to contain resolved path", rep.WouldSpawn)
	}
	if !strings.Contains(rep.WouldSpawn, "<url>") {
		t.Errorf("WouldSpawn = %q, want trailing <url> placeholder", rep.WouldSpawn)
	}
}

// Test_Diagnose_NothingFound returns a populated Error and no Picked.
func Test_Diagnose_NothingFound(t *testing.T) {
	rep := Diagnose(config.Browser{ChromePath: filepath.Join(t.TempDir(), "nope")})

	// Note: this test trusts that the sandbox has no Chrome on PATH.
	// If a real Chrome is present, the resolver SHOULD find it and Error
	// must be empty — that's still a valid outcome, just not what we're
	// testing here. Skip in that case rather than fail spuriously.
	if rep.ResolvedPath != "" {
		t.Skipf("sandbox has a real browser at %s; skipping no-browser path test", rep.ResolvedPath)
	}
	if rep.Error == "" {
		t.Fatal("Error empty when no browser resolved")
	}
	for _, c := range rep.Candidates {
		if c.Picked {
			t.Errorf("no candidate should be Picked when Error is set; got %+v", c)
		}
	}
}

// Test_FormatReport_ContainsKeySections smoke-tests the renderer so the
// step-numbered output stays grep-able for the user.
func Test_FormatReport_ContainsKeySections(t *testing.T) {
	rep := Diagnose(config.Browser{ChromePath: writeFakeBrowser(t, t.TempDir(), "x")})
	out := FormatReport(rep)
	for _, want := range []string{"Browser launch diagnostic", "GOOS:", "Candidates", "Result:"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatReport missing %q in:\n%s", want, out)
		}
	}
}
