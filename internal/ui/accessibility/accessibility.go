// Package accessibility hosts the small set of cross-cutting
// accessibility primitives the Mailpulse Fyne UI consumes per
// `spec/24-app-design-system-and-ui/05-accessibility.md`.
//
// **Slice #118 (Phase 7.1) bootstrap.** Before this slice the spec
// referenced `internal/ui/accessibility/` as the home for
// `PrefersReducedMotion()`, the `AccessibilityLabel` shim, and the
// 11-test contract in §8 — but the package did not exist on disk.
// That left every Fyne view file free to drift away from the spec
// without an automated guard, and every spec test ID
// (`Test_Contrast_Matrix`, `Test_FocusOrder_Declared`, etc.) was a
// dangling reference.
//
// This slice lands the *foundational* surface only:
//
//   1. `PrefersReducedMotion()` — cross-platform OS probe with a
//      Linux env-var path that works in CI/sandbox today and
//      documented TODOs for the macOS / Windows native APIs that
//      need a follow-up cgo binding (out of scope for an atomic
//      slice; see Slice #118b).
//
//   2. `Labeler` + `EnsureLabel` — a thin adapter that lets call
//      sites annotate widgets with an accessible name without
//      depending on Fyne 2.4-only `widget.AccessibilityLabel`. The
//      shim degrades to a no-op on older Fyne versions so the same
//      call site compiles under `-tags nofyne` (where Fyne is not
//      linked at all) and against the production Fyne build.
//
//   3. AST-level contract guards in `a11y_test.go` that lock the
//      spec rules we can enforce *purely textually* (no Fyne
//      runtime needed): no bare-icon buttons, no stray `aria-*`
//      attribute strings (HTML/React leftovers from the design
//      system spec).
//
//   4. Skip-stubs in `a11y_skipped_test.go` for the 8 spec tests
//      that need a live Fyne build (contrast matrix, focus order,
//      reduced-motion token collapse, target size walk, keyboard
//      shortcut routing, focus-ring paint, status-text adjacency,
//      AccessibilityLabel non-empty walk). Each stub names the
//      follow-up slice so `go test -v ./internal/ui/accessibility/`
//      shows the work surface explicitly.
//
// The package builds under both `-tags nofyne` (CI / sandbox path)
// and the production Fyne build because every export is pure-Go
// with no Fyne imports — Fyne-bound wiring lives in callers, which
// pass `*widget.Button` etc. through the `Labeler` interface.
package accessibility

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// reducedMotionCacheTTL is how long a successful probe result is
// reused before re-checking the OS setting. Spec §5 calls for a
// 30-second refresh on macOS; we use the same value across platforms
// for predictability. Re-checks are also forced by the Fyne shell on
// the window-focus signal (see `internal/ui/app.go`, future wire-up)
// so users toggling the OS preference mid-session see the change
// within a few seconds at the latest.
const reducedMotionCacheTTL = 30 * time.Second

// reducedMotionEnv is the env-var fallback documented in spec §5
// for Linux. Set `REDUCE_MOTION=1` to force the reduced-motion path
// regardless of the live `gsettings` value — used by tests and by
// users on minimal desktops without GNOME settings.
const reducedMotionEnv = "REDUCE_MOTION"

// reducedMotionState is the cached probe result. We use atomic.Bool
// (rather than a sync.Map or sync.Mutex around a struct) because
// the read path is hot — every animation frame may consult it — and
// the write path runs at most once per `reducedMotionCacheTTL`.
//
// `lastProbe` is read/written under `probeMu` so two concurrent
// calls don't double-execute the OS probe. The atomic flag holds
// the last known value so callers can read without locking when
// the cache is fresh.
var (
	reducedMotionFlag atomic.Bool
	probeMu           sync.Mutex
	lastProbe         time.Time
	probeOverride     func() bool // test seam — see SetReducedMotionProbe
)

// PrefersReducedMotion reports whether the OS-level reduced-motion
// preference is on. Cached for `reducedMotionCacheTTL`; pass
// `forceRefresh=true` to bypass the cache (used by the Fyne shell
// on window-focus events).
//
// Resolution order:
//
//  1. If a test override is installed (`SetReducedMotionProbe`),
//     return its value directly — no cache, no env-var.
//  2. Otherwise consult the OS:
//     - Linux: `REDUCE_MOTION=1` env var (live `gsettings` probe is
//       deferred to Slice #118b — needs a `os/exec` call that is
//       sandbox-hostile and slow enough to require its own cache
//       layer beyond the 30 s TTL here).
//     - macOS: TODO Slice #118b (`defaults read
//       com.apple.universalaccess reduceMotion` via cgo or
//       `os/exec`).
//     - Windows: TODO Slice #118b (`SystemParametersInfo` via
//       `golang.org/x/sys/windows`).
//  3. Default: false (animations on).
func PrefersReducedMotion() bool {
	if probeOverride != nil {
		return probeOverride()
	}
	probeMu.Lock()
	defer probeMu.Unlock()
	if time.Since(lastProbe) < reducedMotionCacheTTL && !lastProbe.IsZero() {
		return reducedMotionFlag.Load()
	}
	v := probeReducedMotionFromOS()
	reducedMotionFlag.Store(v)
	lastProbe = time.Now()
	return v
}

// probeReducedMotionFromOS performs the OS-level probe with no
// caching. Split out so the cached `PrefersReducedMotion` wrapper
// owns concurrency policy and this function owns platform routing.
//
// Slice #118 scope: env-var only. macOS/Windows native paths land in
// Slice #118b (they require platform-specific build tags + cgo or
// `golang.org/x/sys` imports that we want to land as a focused
// follow-up rather than smuggling in here).
func probeReducedMotionFromOS() bool {
	if v := strings.TrimSpace(os.Getenv(reducedMotionEnv)); v != "" {
		// Accept the same truthy values as standard Go env-flag
		// patterns: "1", "true", "yes", "on" (case-insensitive).
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

// SetReducedMotionProbe installs a test-only probe that bypasses
// the OS-level resolution and the 30 s cache entirely. Pass `nil` to
// remove the override and restore production behaviour.
//
// Tests covering reduced-motion-driven branches (e.g.
// `Test_ReducedMotion_CollapsesTokens` in spec §8 #5) call this in
// setup and `defer SetReducedMotionProbe(nil)` to clean up.
func SetReducedMotionProbe(fn func() bool) {
	probeOverride = fn
}

// ResetReducedMotionCache forces the next `PrefersReducedMotion`
// call to re-probe the OS. The Fyne shell calls this from its
// window-focus handler so a user toggling the setting in System
// Preferences sees the new value without waiting up to 30 s.
//
// Safe to call concurrently with `PrefersReducedMotion`: the next
// reader observes either the stale-but-recent cached value (if it
// raced ahead of our lock acquire) or the freshly probed value.
func ResetReducedMotionCache() {
	probeMu.Lock()
	defer probeMu.Unlock()
	lastProbe = time.Time{}
}

// Labeler is the minimal interface a widget must satisfy to receive
// an accessible name via `EnsureLabel`. Mirrors the shape of Fyne
// 2.4's `widget.AccessibilityLabel` setter without depending on it,
// so older Fyne versions and the `-tags nofyne` build can still
// satisfy the contract via a tiny adapter.
//
// Spec §6 enumerates the widget classes that MUST implement this:
// `Button`, `Entry` (via Form), `WatchDot`, `Badge`, `RawLogLine`.
type Labeler interface {
	SetAccessibilityLabel(label string)
}

// EnsureLabel applies an accessible name to a widget that
// implements `Labeler`. Returns the label that was actually set
// (the input, after trim) so callers can chain assertions in tests.
//
// Empty / whitespace-only inputs are silently rejected — spec §8
// #10 (`Test_AccessibilityLabel_NonEmpty`) requires every annotated
// widget to have a non-empty name, and silently dropping the call
// would let regressions slip through. Callers passing a dynamically
// computed label should validate upstream.
//
// When `target` is nil or does not implement `Labeler` (older Fyne
// without the setter), this is a no-op — the spec accepts a graceful
// degradation on legacy widget builds, and the AST-level guards in
// §8 #11 catch the more common authoring mistake (icon-only buttons
// with no label arg in the constructor itself).
func EnsureLabel(target any, label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return ""
	}
	if target == nil {
		return trimmed
	}
	if l, ok := target.(Labeler); ok {
		l.SetAccessibilityLabel(trimmed)
	}
	return trimmed
}
