# 04 — Components

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Specifies every reusable widget under `internal/ui/widgets/` and the variant taxonomy. Feature views MUST use these — never construct ad-hoc styled containers.

Cross-references:
- Tokens: [`./01-tokens.md`](./01-tokens.md)
- Watch consumer: [`../21-app/02-features/05-watch/02-frontend.md`](../21-app/02-features/05-watch/02-frontend.md)

---

## 1. Inventory

| Widget | File | Purpose |
|---|---|---|
| `Button` (variants: `Primary`, `Secondary`, `Ghost`, `Danger`) | `widgets/button.go` | Standard buttons |
| `Entry` (variants: `Text`, `Password`, `Numeric`) | `widgets/entry.go` | Single-line inputs |
| `Card` | `widgets/card.go` | Elevated panel with optional title + actions |
| `Badge` (variants: `Neutral`, `Success`, `Warning`, `Error`, `Info`, `RuleMatch`) | `widgets/badge.go` | Inline status chips |
| `Toast` (variants: `Info`, `Success`, `Warning`, `Error`) | `widgets/toast.go` | Transient bottom-right notifications |
| `Dialog` (variants: `Confirm`, `Alert`, `Form`) | `widgets/dialog.go` | Modal dialogs |
| `WatchDot` | `widgets/watchdot.go` | Animated status dot (resolves OI-1) |
| `RawLogLine` | `widgets/rawlogline.go` | One styled line in the Watch raw log |
| `EmptyState` | `widgets/emptystate.go` | Centered icon + headline + body for empty lists |
| `KeyValueRow` | `widgets/kvrow.go` | Read-only labeled rows (used by Settings paths card) |

Total: 10 widgets. Adding one requires updating §2 and `97-acceptance-criteria.md`.

---

## 2. Button

```go
package widgets

type ButtonVariant uint8
const (
    ButtonPrimary   ButtonVariant = 1
    ButtonSecondary ButtonVariant = 2
    ButtonGhost     ButtonVariant = 3
    ButtonDanger    ButtonVariant = 4
)

func NewButton(label string, variant ButtonVariant, onTap func()) *fyne.Container
```

| Variant | Background | Foreground | Border | Hover behavior |
|---|---|---|---|---|
| `Primary`   | `ColorPrimary`            | `ColorPrimaryForeground` | none | brightness +6 % over `MotionFast` |
| `Secondary` | `ColorSurface`            | `ColorForeground` | 1 px `ColorBorder` | bg → `ColorSurfaceMuted` |
| `Ghost`     | transparent               | `ColorForeground` | none | bg → `ColorSurfaceMuted` |
| `Danger`    | `ColorError`              | `ColorPrimaryForeground` | none | brightness +6 % |

All buttons: `RadiusButton` (6 px), padding `SizeSpacing2` vertical / `SizeSpacing3` horizontal, `SizeTextButton` font.

---

## 3. Entry

Wraps `widget.Entry` with consistent border, padding, and validation slot.

| Visual | Token |
|---|---|
| Background | `ColorSurface` |
| Border | 1 px `ColorBorder` (focus → 2 px `ColorPrimary`) |
| Padding | `SizeSpacing2` |
| Radius | `RadiusSmall` |
| Placeholder text | `ColorForegroundMuted` |
| Error text below | `ColorError`, `SizeTextCaption`, prefix `⚠` glyph |

`NumericEntry` enforces digit-only input; `PasswordEntry` masks with `•`.

---

## 4. Card

```go
func NewCard(title string, body fyne.CanvasObject, actions ...fyne.CanvasObject) *fyne.Container
```

| Visual | Token |
|---|---|
| Background | `ColorSurface` |
| Border + shadow | `ElevCard` |
| Radius | `RadiusCard` |
| Padding | `SizeSpacing5` |
| Title font | `SizeTextCardTitle`, `ColorForeground` |
| Title-to-body gap | `SizeSpacing4` |
| Actions row | bottom-right, `SizeSpacing2` gap |

On hover (when card is interactive), elevation transitions `ElevCard → ElevHover` over `MotionFast`.

---

## 5. Badge

Pill-shaped, `RadiusPill`, `SizeTextCaption`, padding `SizeSpacing1` × `SizeSpacing2`.

| Variant | Background | Foreground |
|---|---|---|
| `Neutral`    | `ColorBadgeNeutralBg`  | `ColorBadgeNeutralFg` |
| `Success`    | `ColorSuccess` (alpha 0.20) | `ColorSuccess` |
| `Warning`    | `ColorWarning` (alpha 0.20) | `ColorWarning` |
| `Error`      | `ColorError`   (alpha 0.20) | `ColorError`   |
| `Info`       | `ColorInfo`    (alpha 0.20) | `ColorInfo`    |
| `RuleMatch`  | `ColorRuleMatchBadge` (alpha 0.25) | `ColorRuleMatchBadge` |

---

## 6. WatchDot — resolves OI-1

The animated status indicator used by the Watch feature, the status footer, and the Dashboard.

```go
package widgets

type WatchDot struct {
    widget.BaseWidget
    status core.WatchStatus  // bound externally
}

func NewWatchDot(initial core.WatchStatus) *WatchDot
func (d *WatchDot) SetStatus(s core.WatchStatus)  // safe from any goroutine
```

| `core.WatchStatus` | Color token | Animation |
|---|---|---|
| `WatchStatusIdle`         | `ColorWatchDotIdle`         | none |
| `WatchStatusStarting`     | `ColorWatchDotStarting`     | `MotionPulse2Hz` |
| `WatchStatusWatching`     | `ColorWatchDotWatching`     | none |
| `WatchStatusReconnecting` | `ColorWatchDotReconnecting` | `MotionPulse1Hz` |
| `WatchStatusStopping`     | `ColorWatchDotStopping`     | `MotionPulse2Hz` |
| `WatchStatusError`        | `ColorWatchDotError`        | none |

| Visual | Value |
|---|---|
| Default size | 10 × 10 px (footer) / 12 × 12 (Watch view) |
| Shape | circle (`canvas.NewCircle`) |
| Outline | 1 px `ColorBorder` |
| Animation | alpha pulse 1.0 ↔ 0.4 via `internal/ui/anim/pulse.go` |

When `accessibility.PrefersReducedMotion()` is true (`05-accessibility.md` §5), animation is replaced by a steady color and a small "•" glyph next to the dot indicates the live state in text — color is never the only signal.

`SetStatus` is concurrency-safe: it captures the new status under a `sync.Mutex` and calls `widget.Refresh` via `fyne.Do`. The animation loop reads the latest status on each tick.

---

## 7. RawLogLine

Single styled line for the Watch raw-log virtualized list.

```go
type RawLogLineKind uint8
const (
    RawLogHeartbeat RawLogLineKind = 1
    RawLogNewMail   RawLogLineKind = 2
    RawLogError     RawLogLineKind = 3
)

func NewRawLogLine(timestamp time.Time, kind RawLogLineKind, message string) fyne.CanvasObject
```

Layout: `[hh:mm:ss] message`, monospace (`SizeTextCode`).

| Element | Token |
|---|---|
| Timestamp prefix | `ColorRawLogTimestamp` |
| Heartbeat message | `ColorRawLogHeartbeat` |
| New-mail message | `ColorRawLogNewMail` |
| Error message | `ColorRawLogError` |
| Hovered line bg | `ColorCodeLineHighlight` (over `MotionFast`) |

---

## 8. Toast

Bottom-right stack, max 3 visible. Each toast auto-dismisses after a duration based on variant.

| Variant | Auto-dismiss (ms) | Color band (4 px left) |
|---|---|---|
| `Info`    | 3000 | `ColorInfo` |
| `Success` | 2500 | `ColorSuccess` |
| `Warning` | 4500 | `ColorWarning` |
| `Error`   | 6000 + dismiss button required | `ColorError` |

| Visual | Token |
|---|---|
| Background | `ColorSurface` |
| Elevation | `ElevPopover` |
| Radius | `RadiusCard` |
| Padding | `SizeSpacing4` |
| Z-index | always on top of dialogs (uses Fyne overlay layer) |

Implemented via `fyne.CurrentApp().Driver().AllWindows()[0].Canvas().Overlays()` — the toast manager owns this overlay slot.

---

## 9. Dialog

Wraps `dialog.NewConfirm` / `dialog.NewCustom` with our tokens applied (radius, elevation, action button order).

| Variant | Action layout |
|---|---|
| `Confirm` | `[ Cancel ]  [ Confirm (Primary) ]` — right-aligned |
| `Alert`   | `[ OK (Primary) ]` — right-aligned |
| `Form`    | `[ Cancel ]  [ Save (Primary) ]` — right-aligned, body is a form |

Modal backdrop alpha = 0.45. Backdrop click on `Confirm`/`Alert` triggers Cancel; `Form` requires explicit Cancel (because a stray click could discard typed data).

---

## 10. EmptyState

```go
func NewEmptyState(icon fyne.Resource, headline, body string, actions ...fyne.CanvasObject) fyne.CanvasObject
```

Centered vertical stack: icon (48 × 48, `ColorForegroundMuted`), gap `SizeSpacing4`, headline (`SizeTextSectionTitle`), gap `SizeSpacing2`, body (`SizeTextBody`, `ColorForegroundMuted`), gap `SizeSpacing4`, actions row.

Used by every list view when filtered/empty (Emails, Rules, Tools recent URLs, etc.).

---

## 11. KeyValueRow

Read-only row used by Settings paths card and Dashboard summary.

| Region | Style |
|---|---|
| Key   | `SizeTextBody`, `ColorForegroundMuted`, min-width 160 |
| Value | `SizeTextBody`, `ColorForeground`, hyperlink-styled if `OnTap != nil` |

`OnTap` (when set) calls `NavRouter.OpenInOsFileManager(value)` for paths.

---

## 12. AST Guards (in `widgets/ast_test.go`)

| # | Guard |
|---|---|
| AST-W1 | Only files under `internal/ui/widgets/` declare exported widget constructors with the names in §1. |
| AST-W2 | No file under `internal/ui/views/` calls `widget.NewButton` / `widget.NewEntry` directly — they MUST use `widgets.NewButton` / `widgets.NewEntry`. |
| AST-W3 | `WatchDot` is the only widget calling `internal/ui/anim.Pulse(...)`. |

---

## 13. Test Contract

`internal/ui/widgets/widgets_test.go` — 24 required cases (one per visual variant, plus the 3 AST guards). Watch-dot animation tests run with a deterministic clock (`anim/clock.go` interface).
