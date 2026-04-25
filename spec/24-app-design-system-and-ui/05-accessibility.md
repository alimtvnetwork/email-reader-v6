# 05 — Accessibility

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines accessibility requirements for the Mailpulse Fyne UI: contrast, keyboard navigation, focus order, color-blind safety, screen-reader hints, and reduced-motion behavior.

Cross-references:
- Tokens: [`./01-tokens.md`](./01-tokens.md)
- Components: [`./04-components.md`](./04-components.md)

---

## 1. Contrast Matrix

All text/background pairs MUST meet WCAG 2.1 AA: **4.5:1** for body text, **3:1** for large text (≥ 18 px regular or ≥ 14 px bold). Verified by `Test_Contrast_Matrix` which computes `(L1+0.05)/(L2+0.05)` for every pair below.

| Token pair | Mode | Ratio | Threshold | Pass |
|---|---|---|---|---|
| `ColorForeground` on `ColorBackground` | Dark | 13.5 | 4.5 | ✅ |
| `ColorForeground` on `ColorBackground` | Light | 14.8 | 4.5 | ✅ |
| `ColorForegroundMuted` on `ColorBackground` | Dark | 4.7 | 4.5 | ✅ |
| `ColorForegroundMuted` on `ColorBackground` | Light | 4.6 | 4.5 | ✅ |
| `ColorPrimaryForeground` on `ColorPrimary` | Dark | 5.1 | 4.5 | ✅ |
| `ColorPrimaryForeground` on `ColorPrimary` | Light | 5.6 | 4.5 | ✅ |
| `ColorError` on `ColorBackground` | Dark | 4.8 | 4.5 | ✅ |
| `ColorError` on `ColorBackground` | Light | 5.2 | 4.5 | ✅ |
| `ColorSuccess` on `ColorBackground` | Dark | 5.3 | 4.5 | ✅ |
| `ColorSuccess` on `ColorBackground` | Light | 4.6 | 4.5 | ✅ |
| `ColorWarning` on `ColorBackground` | Dark | 8.1 | 4.5 | ✅ |
| `ColorRawLogTimestamp` on `ColorCodeBg` | Dark | 4.6 | 4.5 | ✅ |
| `ColorSidebarItemActiveForeground` on `ColorSidebarItemActive` | Dark | 6.7 | 4.5 | ✅ |
| `ColorBadgeNeutralFg` on `ColorBadgeNeutralBg` | Dark | 4.7 | 4.5 | ✅ |

Adding/changing any color token requires updating this matrix and re-running `Test_Contrast_Matrix`. CI fails if any pair drops below threshold.

`ColorForegroundDisabled` on `ColorBackground` is intentionally **below 4.5:1** — disabled text is not required to meet AA per WCAG 1.4.3 (informative). It MUST always be paired with another non-color cue (greyed icon, "(disabled)" tooltip).

---

## 2. Keyboard Navigation

| Key | Behavior |
|---|---|
| `Tab` / `Shift+Tab` | Move focus forward / back through interactive elements in DOM order. |
| `Enter` | Activate focused button / submit focused form. |
| `Space` | Toggle focused checkbox / radio. |
| `Esc` | Close focused dialog (Cancel) or dismiss focused toast. |
| `Cmd/Ctrl+1..7` | Jump to sidebar route 1..7 (Dashboard, Emails, Rules, Accounts, Watch, Tools, Settings). |
| `Cmd/Ctrl+S` | When in Settings, save (same as clicking Save). |
| `Cmd/Ctrl+R` | When in Watch, force one immediate poll (debug shortcut, gated). |
| `Arrow keys` | Navigate within virtualized lists (Emails, raw log, OpenedUrl history). |

All sidebar items must be reachable via Tab without entering hidden focus traps. Focus ring uses `ColorPrimary` at alpha 0.40, 2 px outline, offset 2 px — implemented in `internal/ui/theme/focusring.go`.

---

## 3. Focus Order

Per-view focus order is **declared explicitly** in the view file via a `func (vm *View) FocusOrder() []fyne.Focusable` method. The shell calls it on `Enter` and uses it instead of Fyne's default DOM walk (which is brittle when widgets are nested in `*fyne.Container`).

Order rules:

1. Page header actions (left → right).
2. Body interactive elements (top → bottom, left → right per row).
3. Footer / bottom-bar actions (left → right).

Hidden / disabled elements are skipped. Focus wraps from last → first.

---

## 4. Color-Blind Safety

Color is **never** the only signal. Every status indicator pairs color with one of: text, glyph, or shape.

| Indicator | Color | Secondary cue |
|---|---|---|
| `WatchDot` Watching | green | static circle + "Watching" text in adjacent label |
| `WatchDot` Reconnecting | amber | pulsing + "Reconnecting…" text + animated `↻` glyph in label |
| `WatchDot` Error | red | static circle + `⚠` glyph + "Error" text |
| Validation error in form | red text | leading `⚠` glyph (per `04-components.md` §3) |
| `Decision='Blocked'` row in OpenedUrl history | red dot | "Blocked: ER-…" text in same row |
| Rules "matched" badge | purple | "matched" word in badge |

Verified visually by the QA checklist (manual color-blind simulation in macOS Display settings) and structurally by `Test_StatusHasTextLabel` (asserts every `WatchDot` instance has an adjacent `*widget.Label` with a status word).

---

## 5. Reduced Motion

Detected via `accessibility.PrefersReducedMotion() bool` in `internal/ui/accessibility/`. Implementation:

| OS | Source |
|---|---|
| macOS | `defaults read com.apple.universalaccess reduceMotion` (cached, refreshed every 30 s) |
| Windows | `SystemParametersInfo(SPI_GETCLIENTAREAANIMATION, ...)` returns false ⇒ reduced |
| Linux | `gsettings get org.gnome.desktop.interface enable-animations` returns `false` ⇒ reduced; otherwise check env `REDUCE_MOTION=1` |

When reduced motion is true:

- All `Motion*` tokens collapse to `MotionInstant`.
- `WatchDot` pulse becomes a steady solid color (no alpha cycling).
- Card hover elevation transition is instant.
- Toasts slide-in animation becomes an instant appear.

The setting is re-checked on Fyne's window-focus signal so users toggling it mid-session see updated behavior within seconds.

---

## 6. Screen Reader Hints

Fyne's screen-reader story is limited. Within those limits:

| Widget | Hint |
|---|---|
| `Button` | Label text is the accessible name. Icon-only buttons MUST set `Importance` and a `Title` via `widget.NewButtonWithIcon(label, ...)` where `label` is non-empty. No bare-icon buttons. |
| `Entry` | Wrapped in `widget.Form` with a non-empty label so the OS announces "field name, edit text". |
| `WatchDot` | Adjacent `*widget.Label` provides the spoken state (see §4). The dot itself sets `widget.BaseWidget.AccessibilityLabel = "Watch status: " + statusWord`. |
| `Badge` | Sets `AccessibilityLabel = variant + ": " + text` so "matched" reads as "Rule match: matched". |
| `RawLogLine` | Sets `AccessibilityLabel = kindWord + " at " + timestamp + ": " + message`. |

(Fyne 2.4 introduces `widget.AccessibilityLabel`; older versions get a no-op shim.)

---

## 7. Touch / Click Targets

Minimum target size: **32 × 32 px** for any interactive element. Sidebar items, buttons, form controls all meet this. Verified by `Test_TargetSize_Min32` walking the widget tree of every view.

---

## 8. Test Contract

`internal/ui/accessibility/a11y_test.go` — 11 required cases:

1. `Test_Contrast_Matrix` — every pair in §1 passes its threshold.
2. `Test_FocusOrder_Declared` — every view file declares `FocusOrder()`.
3. `Test_FocusOrder_NoHiddenInOrder` — disabled / hidden widgets not in order list.
4. `Test_StatusHasTextLabel` — every `WatchDot` has adjacent text label.
5. `Test_ReducedMotion_CollapsesTokens` — when probe returns true, `Motion(MotionFast)` returns `MotionInstant`.
6. `Test_ReducedMotion_WatchDotSteady` — pulse animation is not started.
7. `Test_TargetSize_Min32` — no interactive widget renders smaller than 32 px in any axis.
8. `Test_KeyboardShortcuts_Sidebar` — `Cmd/Ctrl+1..7` navigate to documented routes.
9. `Test_FocusRing_Visible` — focused widget paints the focus ring with `ColorPrimary` alpha 0.40.
10. `Test_AccessibilityLabel_NonEmpty` — every `Button` / `WatchDot` / `Badge` / `RawLogLine` instance has a non-empty `AccessibilityLabel`.
11. `Test_NoIconOnlyButtons_WithoutLabel` — AST scan: no `widget.NewButtonWithIcon("", ...)` calls.
