# 07 — Settings — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, view-model, theming, validation, dirty-tracking, and save flow** for the Settings view. Single sidebar route. Lives in `internal/ui/views/settings.go` — the only file in `internal/ui` permitted to compose Settings widgets.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.7
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.7
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`
- Theme consumer: `internal/ui/theme/theme.go`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/settings.go
package views

type SettingsVM struct {
    svc      *core.Settings
    nav      NavRouter
    log      Logger

    // Snapshot loaded at Enter; baseline for dirty diff
    loaded   binding.Untyped // *core.SettingsSnapshot

    // Editable bindings (one per field)
    pollSeconds            binding.Int
    chromePath             binding.String
    chromePathDetected     binding.String   // resolved by DetectChrome
    chromePathSource       binding.String   // enum name, display only
    incognitoArg           binding.String
    incognitoArgPlaceholder binding.String  // suggested by detector
    theme                  binding.String   // "Dark" | "Light" | "System"
    openUrlAllowedSchemes  binding.StringList
    allowLocalhostUrls     binding.Bool
    autoStartWatch         binding.Bool

    // Read-only display
    configPath      binding.String
    dataDir         binding.String
    emailArchiveDir binding.String
    uiStateFile     binding.String

    // Per-field validation messages (empty = clean)
    errPoll       binding.String
    errChrome     binding.String
    errIncognito  binding.String
    errSchemes    binding.String

    // Composite state
    dirty       binding.Bool   // any field differs from `loaded`
    saving      binding.Bool   // Save in flight
    saveError   binding.String // banner-level error (e.g., ER-SET-21778)

    cancelSubscribe func()
}
```

`NewSettingsVM` requires `svc`, `nav`, `log` non-nil; panics in dev build, returns wrapped error in release build (per `04-coding-standards.md`).

---

## 2. Lifecycle

| Hook | Behavior |
|---|---|
| `Enter(ctx)` | Calls `svc.Get(ctx)` once; populates all bindings; subscribes to `SettingsEvent`; calls `svc.DetectChrome` async to fill placeholder. |
| `Leave()` | Calls `cancelSubscribe()`; if `dirty.Get() == true`, shows confirm dialog (`Discard / Stay`); navigation is blocked until resolved. |
| External `SettingsEvent` while open | Diff: if a field is **not** dirty locally, update its binding to the new value (live-reload). If dirty, leave the local edit alone and show a non-blocking banner: "Settings were changed elsewhere. [Reload]". |

The `Leave` confirm uses `dialog.NewConfirm` and is the **only** modal in this view.

---

## 3. Widget Tree

Single vertical scroll, four `widget.Card` sections separated by `widget.Separator`. Layout uses `container.NewVBox` inside a `container.NewVScroll`.

```
container.NewBorder(
    top    = nil,
    bottom = bottomBar,                // Save / Reset / saving spinner
    left   = nil,
    right  = nil,
    center = container.NewVScroll(
        container.NewVBox(
            cardPaths,                 // §3.1
            cardWatcher,               // §3.2
            cardBrowser,               // §3.3
            cardAppearance,            // §3.4
        ),
    ),
)
```

### 3.1 Paths card (read-only)

`widget.Card{Title: "Paths"}` containing a `container.New(layout.NewFormLayout(), …)` with four rows. Each row is a clickable `widget.Hyperlink`-styled label that, on tap, calls `nav.OpenInOsFileManager(path)`. No edit affordance.

| Label | Bound to |
|---|---|
| Config file | `configPath` |
| Data directory | `dataDir` |
| Email archive | `emailArchiveDir` |
| UI state file | `uiStateFile` |

### 3.2 Watcher card

| Widget | Binding | Validation |
|---|---|---|
| `widget.Entry` (numeric) — "Poll interval (seconds)" | `pollSeconds` | `errPoll` shown beneath via `widget.Label` with `theme.ErrorColor` |
| `widget.Check` — "Auto-start watcher on launch" | `autoStartWatch` | n/a |

Numeric entry uses `entry.Validator = validatePollSeconds` (1..60 inclusive). On invalid input, `errPoll` is set immediately; Save remains disabled while any `err*` binding is non-empty.

### 3.3 Browser card

| Widget | Binding |
|---|---|
| `widget.Entry` — "Chrome path override" with adjacent "Browse…" (`dialog.NewFileOpen`) and "Detect" buttons | `chromePath` |
| `widget.Label` (muted) — shows "Detected: {path} (from {source})" | `chromePathDetected` + `chromePathSource` |
| `widget.Entry` — "Incognito flag override" with placeholder = `incognitoArgPlaceholder` | `incognitoArg` |
| `widget.Entry` — "Allowed URL schemes" (comma-separated, normalized on blur) | `openUrlAllowedSchemes` (joined) |
| `widget.Check` — "Allow localhost URLs (requires `http` scheme)" | `allowLocalhostUrls` |

The "Detect" button calls `svc.DetectChrome(ctx)` on a goroutine, then updates `chromePathDetected` / `chromePathSource` on the UI thread via `fyne.Do`. It never overwrites `chromePath` — user choice is preserved.

### 3.4 Appearance card

| Widget | Binding |
|---|---|
| `widget.RadioGroup{Options: []string{"Dark", "Light", "System"}, Horizontal: true}` | `theme` |

Theme change is **previewed live** (calls `internal/ui/theme.Apply(theme.Get())` immediately on selection), but persistence happens only on Save. If the user discards via the Leave dialog, the previous theme is reapplied.

### 3.5 Bottom bar

```
container.NewBorder(nil, nil,
    resetBtn,       // left  — secondary style
    container.NewHBox(savingSpinner, saveBtn), // right — primary
    saveErrorBanner, // center — visible only when saveError != ""
)
```

- `saveBtn`: `widget.Button{Text: "Save", Importance: widget.HighImportance}`. Disabled when `!dirty.Get() || saving.Get() || anyError()`.
- `resetBtn`: `widget.Button{Text: "Reset to defaults"}`. Always enabled. Tapping shows a confirm dialog listing exactly which fields will be reset (poll interval, browser overrides, theme, allowed schemes, allow-localhost, auto-start). Accounts / Rules are explicitly mentioned as **not** affected.
- `savingSpinner`: `widget.NewActivity()`, `Show()`/`Hide()` driven by `saving` binding.

---

## 4. Save Flow

```go
func (vm *SettingsVM) onSave(ctx context.Context) {
    vm.saving.Set(true)
    defer vm.saving.Set(false)
    in := vm.collectInput()                 // pure: bindings → core.SettingsInput
    res := vm.svc.Save(ctx, in)             // backend §5
    if err := res.Err(); err != nil {
        vm.saveError.Set(friendly(err))     // §5
        return
    }
    vm.applySnapshot(res.Value())           // refresh `loaded`, clear dirty
}
```

`onSave` is debounced — re-entry while `saving == true` is a no-op. The function body is exactly 7 lines (per `04-coding-standards.md` §3).

---

## 5. Error → Friendly Message Mapping

| Code | Surface | Friendly text |
|---|---|---|
| `ER-SET-21771` | inline (`errPoll`) | "Must be between 1 and 60 seconds." |
| `ER-SET-21772` | inline (`saveError`) | "Unknown theme. Choose Dark, Light, or System." |
| `ER-SET-21773` | inline (`errSchemes`) | "Schemes must be lowercase letters/digits and cannot include `file`, `javascript`, `data`, or `vbscript`." |
| `ER-SET-21774` | inline (`errChrome`) | "Chrome path must be an existing executable file." |
| `ER-SET-21775` | inline (`errIncognito`) | "Incognito argument must look like `--incognito` or `-private`." |
| `ER-SET-21777` | inline (`errSchemes`) | "Allow-localhost requires `http` in the scheme list." |
| `ER-SET-21778` | banner (`saveError`) | "Could not save settings. [Retry]" |
| `ER-SET-21779` | banner (`saveError`) | "Settings file changed on disk. [Reload]" — the action button reloads instead of forcing the user to re-enter values. |
| `ER-SET-21780` | toast (transient) | "Could not detect Chrome automatically." |

Stack traces are never shown in the UI; only `code` + friendly text. Full error logged via `vm.log.Error`.

---

## 6. Dirty Tracking

`dirty` is recomputed on every binding change via `binding.NewSprintf` listeners that call `vm.recomputeDirty()`:

```go
func (vm *SettingsVM) recomputeDirty() {
    base, _ := vm.loaded.Get()
    cur := vm.collectInput()
    vm.dirty.Set(!equalSnapshot(base.(*core.SettingsSnapshot), &cur))
}
```

`equalSnapshot` compares **canonical normalized** values (sorted/deduped schemes, trimmed strings) so cosmetic edits like trailing whitespace don't mark dirty.

---

## 7. Theming

- Theme switch calls `internal/ui/theme.Apply(mode)` which calls `fyne.CurrentApp().Settings().SetTheme(...)`. All views observe Fyne's theme-changed signal automatically.
- `System` mode resolves via `app.Settings().ThemeVariant()` and re-resolves on OS theme change (Fyne 2.4+ emits `theme.VariantChanged`).
- Settings view itself uses only design-system tokens: `theme.BackgroundColor`, `theme.ForegroundColor`, `theme.PrimaryColor`, `theme.ErrorColor`, `theme.DisabledColor`. **No hard-coded colors.** Verified by AST scan in §10.

---

## 8. Streaming / Async Patterns

Settings has no streaming output, but it has two async operations:

| Operation | Pattern |
|---|---|
| `DetectChrome` on tap | goroutine + `fyne.Do` to update UI; cancels on `Leave` via `ctx`. |
| `Save` | goroutine; `saving` binding gates UI; result marshaled back via `fyne.Do`. |

No goroutine outlives the view: `Leave` cancels the view-scoped `context.Context`.

---

## 9. Accessibility

- Every input has a visible label rendered via `widget.Form` items (Fyne announces them as form fields).
- Tab order: top-down, left-to-right per card; matches DOM order.
- "Detect" and "Browse…" buttons have `Importance: widget.MediumImportance` and explicit `widget.NewLabel` adjacency for screen-reader pairing.
- Error `widget.Label` uses `theme.ErrorColor` AND a leading `⚠` glyph (color is not the only signal — per `16-app-design-system-and-ui.md`).

---

## 10. Testing Contract

Tests live in `internal/ui/views/settings_test.go` using `fyne.io/fyne/v2/test`. 22 required cases:

**Rendering (4):**
1. Initial render shows all four cards in the documented order.
2. Read-only path rows are non-editable (no `*widget.Entry`).
3. Save button is disabled on first render.
4. `saveError` banner is hidden when empty.

**Validation (5):**
5. Typing `0` in poll seconds sets `errPoll` and disables Save.
6. Typing `61` in poll seconds sets `errPoll` and disables Save.
7. Typing `30` clears `errPoll` and enables Save (assuming dirty).
8. Adding `javascript` to schemes sets `errSchemes`.
9. Toggling Allow-localhost without `http` in schemes sets `errSchemes` (composite `21777`).

**Dirty / Save (5):**
10. Editing then reverting to original value clears `dirty`.
11. Whitespace-only edit to `chromePath` does NOT mark dirty (normalization).
12. Save calls `svc.Save` exactly once even with rapid double-click.
13. Successful Save updates `loaded` and re-disables Save.
14. Failed Save (`ER-SET-21778`) shows banner with Retry; does NOT clear `loaded`.

**Detect (3):**
15. Tap "Detect" calls `svc.DetectChrome` once and updates `chromePathDetected`/`chromePathSource`.
16. Detect failure shows transient toast, does not block UI.
17. Detect never overwrites the user-typed `chromePath`.

**Live-reload (2):**
18. External `SettingsEvent` while view is clean updates non-dirty fields in place.
19. External `SettingsEvent` while view is dirty leaves bindings alone and shows the reload banner.

**Theme (2):**
20. Selecting Light immediately calls `theme.Apply(ThemeLight)`.
21. Discarding via Leave-confirm restores the previous theme.

**Anti-features (1):**
22. AST scan: file `internal/ui/views/settings.go` contains no `color.RGBA{` or `color.NRGBA{` literals (only `theme.*Color()` references).

---

## 11. File Layout

```
internal/ui/views/
  settings.go            // SettingsVM + lifecycle (≤15-line fns)
  settings_widgets.go    // CardPaths / CardWatcher / CardBrowser / CardAppearance builders
  settings_validate.go   // UI-side mirror of backend §6 (instant feedback only)
  settings_test.go

internal/ui/theme/
  theme.go               // Apply(mode) — owned by theme pkg
```

`settings.go` is the only file in `internal/ui/views` permitted to call `core.Settings.Save` / `core.Settings.ResetToDefaults`. Verified by AST scan analogous to Tools §12.27.
