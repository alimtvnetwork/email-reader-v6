// a11y_skipped_test.go — Slice #118 documented skip-stubs for the
// 8 spec-§8 tests that need a live Fyne build. Each `t.Skip` names
// the follow-up slice and the spec section so the work surface is
// discoverable via `go test -v ./internal/ui/accessibility/`.
//
// Pattern adopted from the errtrace package's "future-coverage
// stubs" — the test names exist in the test runner output today,
// reviewers can grep for them, and a single line edit (drop the
// Skip, add the body) lights each one up when its dependency is
// ready. Without these stubs the spec's 11-test contract would be
// invisible to anyone running the suite, leading to silent drift.
//
// Slice #118b will land the Fyne-bound runtime: `widget.Button`
// `AccessibilityLabel` plumbing, the `WatchDot` adjacent-text
// invariant, the focus-ring paint check, the keyboard-shortcut
// routing, the contrast matrix probe, the focus-order declaration
// audit, the target-size walk, and the reduced-motion token
// collapse. Each test below is one row from spec
// `spec/24-app-design-system-and-ui/05-accessibility.md` §8.
package accessibility

import "testing"

// Spec §8 #1 — Test_Contrast_Matrix
//
// Walks every (foreground, background) pair in §1's matrix,
// computes the WCAG `(L1+0.05)/(L2+0.05)` ratio, and fails if any
// drops below the listed threshold (4.5 for body, 3.0 for large).
// Needs the live `internal/ui/theme` palette (already exists) plus
// a small color-math helper that lives outside this slice's diff.
func Test_Contrast_Matrix(t *testing.T) {
	t.Skip("Slice #118b — needs WCAG luminance helper + theme palette walk")
}

// Spec §8 #2 — Test_FocusOrder_Declared
//
// AST scan asserting every view file under `internal/ui/views/`
// declares a `func (vm *View) FocusOrder() []fyne.Focusable` method.
// Today zero views declare it (verified with
// `rg -c '^func.*FocusOrder\(\)' internal/ui/views/`); landing the
// declarations in each view is what Slice #118b actually does, so
// the guard goes live in the same slice that satisfies it.
func Test_FocusOrder_Declared(t *testing.T) {
	t.Skip("Slice #118b — depends on FocusOrder() landing in every view file first")
}

// Spec §8 #3 — Test_FocusOrder_NoHiddenInOrder
//
// Once `FocusOrder()` exists, asserts the returned slice contains
// no widgets whose `Hidden` or `Disabled` is true. Pairs with #2.
func Test_FocusOrder_NoHiddenInOrder(t *testing.T) {
	t.Skip("Slice #118b — needs FocusOrder() declarations to scan")
}

// Spec §8 #4 — Test_StatusHasTextLabel
//
// Walks every rendered `WatchDot` widget and asserts an adjacent
// `*widget.Label` carrying a status word ("Watching",
// "Reconnecting…", "Error"). The colour-blind safety guarantee
// from §4 (color is never the only signal) hangs on this test.
func Test_StatusHasTextLabel(t *testing.T) {
	t.Skip("Slice #118b — needs WatchDot widget + Fyne render harness")
}

// Spec §8 #5 — Test_ReducedMotion_CollapsesTokens
//
// With `SetReducedMotionProbe(func() bool { return true })`,
// asserts `theme.Motion(theme.MotionFast)` returns `MotionInstant`.
// The `internal/ui/theme/motion` tokens that this hangs on do not
// exist yet — they land in Slice #118b alongside the reduced-motion
// wiring. The probe seam is in place today (see
// `accessibility.go::SetReducedMotionProbe`) so the runtime side is
// the only blocker.
func Test_ReducedMotion_CollapsesTokens(t *testing.T) {
	t.Skip("Slice #118b — needs theme/motion tokens + reduced-motion wiring")
}

// Spec §8 #6 — Test_ReducedMotion_WatchDotSteady
//
// Pairs with #5: when the probe returns true, the `WatchDot` pulse
// animation must not be started (steady solid colour instead).
func Test_ReducedMotion_WatchDotSteady(t *testing.T) {
	t.Skip("Slice #118b — needs WatchDot animation hook")
}

// Spec §8 #7 — Test_TargetSize_Min32
//
// Walks the widget tree of every view and asserts no interactive
// widget renders smaller than 32 px on either axis. Spec §7
// minimum target size.
func Test_TargetSize_Min32(t *testing.T) {
	t.Skip("Slice #118b — needs Fyne render harness + per-view widget tree walker")
}

// Spec §8 #8 — Test_KeyboardShortcuts_Sidebar
//
// Asserts `Cmd/Ctrl+1..7` invoke the documented sidebar routes
// (Dashboard, Emails, Rules, Accounts, Watch, Tools, Settings).
// Hangs on a shortcut-binding registry that does not exist yet.
func Test_KeyboardShortcuts_Sidebar(t *testing.T) {
	t.Skip("Slice #118b — needs internal/ui shortcut registry")
}

// Spec §8 #9 — Test_FocusRing_Visible
//
// Asserts the focused widget paints the focus ring with
// `ColorPrimary` at alpha 0.40, 2 px outline, offset 2 px. Hangs
// on `internal/ui/theme/focusring.go` which does not exist yet.
func Test_FocusRing_Visible(t *testing.T) {
	t.Skip("Slice #118b — needs internal/ui/theme/focusring.go")
}

// Spec §8 #10 — Test_AccessibilityLabel_NonEmpty
//
// Walks every rendered `Button` / `WatchDot` / `Badge` /
// `RawLogLine` instance and asserts a non-empty
// `AccessibilityLabel`. The `EnsureLabel` shim is in place today;
// the call sites that use it land alongside the Fyne 2.4 widget
// upgrade in Slice #118b.
func Test_AccessibilityLabel_NonEmpty(t *testing.T) {
	t.Skip("Slice #118b — needs Fyne render harness + Labeler call-site rollout")
}
