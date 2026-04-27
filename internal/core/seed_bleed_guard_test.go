// seed_bleed_guard_test.go — Slice #141 follow-up to Task #10.
//
// Pins that BOTH `withIsolatedConfig` (T) and `withIsolatedConfigTB`
// (TB) set EMAIL_READ_DISABLE_SEED=1 inside the supplied closure.
// Without this gate, the first `config.Load()` after a config wipe
// re-injects DefaultSeedAccounts, which silently bleeds the demo
// account into:
//   - tests asserting "no accounts configured" (Diagnose)
//   - benchmarks measuring cold-load cost (skews the first iteration)
//
// The original guard lived only in `withIsolatedConfig`; the bench
// twin shipped without it (workspace revert noted in mem://workspace-
// revert-on-resume). This test prevents the gap from re-opening.
package core

import (
	"os"
	"testing"
)

func TestWithIsolatedConfig_SetsSeedDisable(t *testing.T) {
	withIsolatedConfig(t, func() {
		if got := os.Getenv("EMAIL_READ_DISABLE_SEED"); got != "1" {
			t.Fatalf("withIsolatedConfig must set EMAIL_READ_DISABLE_SEED=1 inside fn, got %q", got)
		}
	})
}

func TestWithIsolatedConfigTB_SetsSeedDisable(t *testing.T) {
	withIsolatedConfigTB(t, func() {
		if got := os.Getenv("EMAIL_READ_DISABLE_SEED"); got != "1" {
			t.Fatalf("withIsolatedConfigTB must set EMAIL_READ_DISABLE_SEED=1 inside fn, got %q", got)
		}
	})
}
