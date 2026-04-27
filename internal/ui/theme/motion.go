// motion.go defines the motion-duration tokens per
// `spec/24-app-design-system-and-ui/01-tokens.md` §7 and the
// reduced-motion collapse contract from `05-accessibility.md` §5.
//
// **Slice #118b (Phase 7.2) bootstrap.** Before this slice the Fyne
// UI used hard-coded `time.Duration` literals in widget animations
// (most prominently the `WatchDot` pulse and the toast slide-in).
// That made it impossible to honour the OS-level reduced-motion
// preference: every animation site would have needed its own
// per-call `if accessibility.PrefersReducedMotion()` branch, easy
// to forget on the next widget added.
//
// The token-routed approach centralises the policy: every animation
// site asks `theme.Motion(theme.MotionFast)` for its duration, and
// the resolver collapses *every* non-instant token to
// `MotionInstant` (zero) when the reduced-motion probe returns true.
// One contract, one test (`Test_ReducedMotion_CollapsesTokens` in
// `internal/ui/accessibility/a11y_skipped_test.go` — now lit up by
// this slice).
//
// Pure Go (fyne-free) so it builds under `-tags nofyne` alongside
// the rest of the token contract.
package theme

import (
	"time"

	"github.com/lovable/email-read/internal/ui/accessibility"
)

// MotionName is a typed enum for every motion-duration token.
// Mirrors the `ColorName` / `SizeName` shape so AST-T1's literal
// guard can scan accidental hard-coded `time.Millisecond * N`
// literals at animation sites in a future slice.
//
// Spec: 01-tokens.md §7.
type MotionName string

const (
	// MotionInstant is a zero-duration token that resolves to
	// `0 * time.Nanosecond`. Reserved for the reduced-motion
	// collapse target — call sites should NOT request it directly
	// outside the resolver.
	MotionInstant MotionName = "MotionInstant"
	// MotionFast is the default snap duration for hover / focus
	// state changes (≤ 120 ms — perceived as "immediate" per §5).
	MotionFast MotionName = "MotionFast"
	// MotionMedium is the standard transition duration for layout
	// shifts, panel reveals, and the WatchDot pulse cycle.
	MotionMedium MotionName = "MotionMedium"
	// MotionSlow is reserved for full-screen transitions (route
	// switches, modal scrim fade). Used sparingly.
	MotionSlow MotionName = "MotionSlow"
)

// motionBase holds the production durations from spec §7.
// Re-exported via `Motion()` rather than directly so the
// reduced-motion collapse runs on every read.
var motionBase = map[MotionName]time.Duration{
	MotionInstant: 0,
	MotionFast:    120 * time.Millisecond,
	MotionMedium:  220 * time.Millisecond,
	MotionSlow:    320 * time.Millisecond,
}

// Motion resolves a motion token to its `time.Duration`. When the
// OS-level reduced-motion preference is on (probed via
// `accessibility.PrefersReducedMotion()`), every non-instant token
// collapses to `0` so animation sites receive an instant duration
// they can interpret as "skip the tween".
//
// Unknown names fall through to `MotionFast` to keep the shape
// identical to `Color()` / `Size()` (graceful degradation, not a
// panic). A `warnUnknown`-style log line is intentionally omitted
// here because animation sites resolve durations on every frame —
// flooding the log with one warning per frame would be worse than
// silently degrading.
func Motion(name MotionName) time.Duration {
	if accessibility.PrefersReducedMotion() {
		return 0
	}
	if d, ok := motionBase[name]; ok {
		return d
	}
	return motionBase[MotionFast]
}
