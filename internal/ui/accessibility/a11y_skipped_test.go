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
// **Slice #118b: lit up.** Lives in the `theme` package
// (`internal/ui/theme/contrast_test.go`) because `accessibility`
// cannot import `theme` (the dependency edge runs the other way).
// This stub stays as a discoverable redirect so `go test -v
// ./internal/ui/accessibility/` still surfaces the test ID — the
// real assertions run in the theme package's own test binary.
func Test_Contrast_Matrix(t *testing.T) {
	t.Skip("Slice #118b — live test runs in internal/ui/theme/contrast_test.go (cannot live here without import cycle)")
}

// Spec §8 #2 — Test_FocusOrder_Declared (AC-DS-61)
//
// **Slice #118c: lit up.** Lives in `a11y_render_harness_test.go`
// in this package as a pure AST scan (no Fyne runtime needed).
// Seeded with an allowlist of every existing view file; the
// allowlist must shrink monotonically as Slice #118e rolls out
// `FocusOrder()` declarations across the views package.

// Spec §8 #3 — Test_FocusOrder_NoHiddenInOrder (AC-DS-62)
//
// Once `FocusOrder()` declarations exist (Slice #118e), asserts the
// returned slice contains no widgets whose `Hidden` or `Disabled`
// is true at construction time. Pairs with #2 and needs the live
// widget tree to inspect — gated on `A11Y_RENDER=1`.
func Test_FocusOrder_NoHiddenInOrder(t *testing.T) {
	if !a11yRenderHarnessEnabled() {
		t.Skip("Slice #118e — needs A11Y_RENDER=1 + FocusOrder() declarations from #118e rollout")
	}
	t.Skip("Slice #118e — implementation pending (harness enabled but assertions not yet ported)")
}

// Spec §8 #4 — Test_StatusHasTextLabel (AC-DS-63)
//
// Walks every rendered `WatchDot` widget and asserts an adjacent
// `*widget.Label` carrying a status word ("Watching",
// "Reconnecting…", "Error"). The colour-blind safety guarantee
// from §4 (color is never the only signal) hangs on this test.
// Needs the `WatchDot` widget itself (not yet built) and a Fyne
// render canvas.
func Test_StatusHasTextLabel(t *testing.T) {
	if !a11yRenderHarnessEnabled() {
		t.Skip("Slice #118e — needs A11Y_RENDER=1 + WatchDot widget rollout")
	}
	t.Skip("Slice #118e — implementation pending (harness enabled but WatchDot widget not yet built)")
}

// Spec §8 #5 — Test_ReducedMotion_CollapsesTokens
//
// **Slice #118b: lit up.** Lives in the `theme` package
// (`internal/ui/theme/motion_test.go`) — same import-cycle reason
// as Test_Contrast_Matrix above. The probe seam exported from
// this package (`SetReducedMotionProbe`) is what the theme test
// drives to flip the resolver between collapsed and baseline modes.
func Test_ReducedMotion_CollapsesTokens(t *testing.T) {
	t.Skip("Slice #118b — live test runs in internal/ui/theme/motion_test.go (cannot live here without import cycle)")
}

// Spec §8 #6 — Test_ReducedMotion_WatchDotSteady (AC-DS-48)
//
// Pairs with #5: when the probe returns true, the `WatchDot` pulse
// animation must not be started (steady solid colour instead).
// Needs the WatchDot animation hook (Slice #118e).
func Test_ReducedMotion_WatchDotSteady(t *testing.T) {
	if !a11yRenderHarnessEnabled() {
		t.Skip("Slice #118e — needs A11Y_RENDER=1 + WatchDot animation hook")
	}
	t.Skip("Slice #118e — implementation pending (harness enabled but animation hook not yet built)")
}

// Spec §8 #7 — Test_TargetSize_Min32 (AC-DS-65)
//
// Walks the widget tree of every view and asserts no interactive
// widget renders smaller than 32 px on either axis. Spec §7
// minimum target size.
func Test_TargetSize_Min32(t *testing.T) {
	if !a11yRenderHarnessEnabled() {
		t.Skip("Slice #118e — needs A11Y_RENDER=1 + per-view widget tree walker")
	}
	t.Skip("Slice #118e — implementation pending (harness enabled but tree walker not yet built)")
}

// Spec §8 #8 — Test_KeyboardShortcuts_Sidebar (AC-DS-66)
//
// **Slice #118c: lit up.** Lives in `a11y_render_harness_test.go`
// in this package as a pure AST scan that walks the UI tree for
// `desktop.CustomShortcut{...}` constructions and tabulates the
// digit keys found alongside a Cmd/Ctrl modifier. Bootstrap-PASSes
// today (no shortcuts registered yet); enforces the full {1..7}
// set the moment the first binding lands in Slice #118e.

// Spec §8 #9 — Test_FocusRing_Visible (AC-DS-67)
//
// Asserts the focused widget paints the focus ring with
// `ColorPrimary` at alpha 0.40, 2 px outline, offset 2 px. Tokens
// are in place (`internal/ui/theme/focusring.go` from Slice #118b);
// the paint inspection itself needs a live canvas.
func Test_FocusRing_Visible(t *testing.T) {
	if !a11yRenderHarnessEnabled() {
		t.Skip("Slice #118e — needs A11Y_RENDER=1 + canvas paint inspection")
	}
	t.Skip("Slice #118e — implementation pending (harness enabled but paint inspector not yet built)")
}

// Spec §8 #10 — Test_AccessibilityLabel_NonEmpty (AC-DS-68)
//
// Walks every rendered `Button` / `WatchDot` / `Badge` /
// `RawLogLine` instance and asserts a non-empty
// `AccessibilityLabel`. The `EnsureLabel` shim is in place today
// (Slice #118); call-site rollout + tree walk land in #118e.
func Test_AccessibilityLabel_NonEmpty(t *testing.T) {
	if !a11yRenderHarnessEnabled() {
		t.Skip("Slice #118e — needs A11Y_RENDER=1 + Labeler call-site rollout")
	}
	t.Skip("Slice #118e — implementation pending (harness enabled but call-site rollout not yet done)")
}
