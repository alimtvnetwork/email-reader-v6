// motion_test.go — Slice #118b lit-up implementation of spec
// `05-accessibility.md` §8 #5 (`Test_ReducedMotion_CollapsesTokens`).
// Confirms the `theme.Motion(MotionFast)` resolver collapses to
// `0` (the `MotionInstant` value) when the reduced-motion probe
// returns true, and returns the spec-§7 baseline duration when
// the probe returns false.
//
// The matching skip-stub in
// `internal/ui/accessibility/a11y_skipped_test.go` redirects here:
// the test must live in the `theme` package because
// `accessibility` cannot import `theme` (the dependency edge runs
// the other way — `theme/motion.go` imports `accessibility` for
// the reduced-motion probe).
package theme

import (
	"testing"
	"time"

	"github.com/lovable/email-read/internal/ui/accessibility"
)

// Test_ReducedMotion_CollapsesTokens locks the spec contract that
// every non-instant motion token resolves to 0 when reduced motion
// is on. Iterates all four tokens (Instant + 3 baselines) so a
// future addition (e.g. MotionExtraSlow) that forgot to plug into
// the resolver fails immediately.
func Test_ReducedMotion_CollapsesTokens(t *testing.T) {
	// Defer cleanup BEFORE installing the override so a panic in
	// the test body still restores production behaviour.
	defer accessibility.SetReducedMotionProbe(nil)
	defer accessibility.ResetReducedMotionCache()

	cases := []MotionName{MotionInstant, MotionFast, MotionMedium, MotionSlow}

	t.Run("reduced motion on", func(t *testing.T) {
		accessibility.SetReducedMotionProbe(func() bool { return true })
		for _, name := range cases {
			if got := Motion(name); got != 0 {
				t.Errorf("Motion(%s) under reduced-motion = %v; want 0", name, got)
			}
		}
	})

	t.Run("reduced motion off — baselines apply", func(t *testing.T) {
		accessibility.SetReducedMotionProbe(func() bool { return false })
		want := map[MotionName]time.Duration{
			MotionInstant: 0,
			MotionFast:    120 * time.Millisecond,
			MotionMedium:  220 * time.Millisecond,
			MotionSlow:    320 * time.Millisecond,
		}
		for _, name := range cases {
			if got, w := Motion(name), want[name]; got != w {
				t.Errorf("Motion(%s) under normal motion = %v; want %v", name, got, w)
			}
		}
	})

	t.Run("unknown name falls back to MotionFast", func(t *testing.T) {
		accessibility.SetReducedMotionProbe(func() bool { return false })
		if got := Motion(MotionName("bogus")); got != 120*time.Millisecond {
			t.Errorf("unknown token fallback = %v; want MotionFast (120ms)", got)
		}
	})
}
