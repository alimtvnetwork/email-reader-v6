# 02 — Theme Implementation

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the Go package layout for `internal/ui/theme/`, the Fyne `fyne.Theme` implementation, the live theme-switch protocol, and the AST guards that keep colors out of feature views.

Cross-references:
- Tokens: [`./01-tokens.md`](./01-tokens.md)
- Settings (theme persistence + live switch): [`../21-app/02-features/07-settings/01-backend.md`](../21-app/02-features/07-settings/01-backend.md), `02-frontend.md` §3.4
- Coding standards: [`../21-app/04-coding-standards.md`](../21-app/04-coding-standards.md)

---

## 1. Package Layout

```
internal/ui/theme/
  theme.go              // exported Apply(mode), Active() — public API
  fyne_theme.go         // type appTheme struct{} ; Color/Font/Icon/Size methods
  tokens.go             // typed enums + constants from 01-tokens.md
  palette_dark.go       // map[ColorName]color.NRGBA  for ThemeDark
  palette_light.go      // same shape, ThemeLight
  density.go            // Comfortable / Compact size resolver
  fonts/
    Inter-Variable.ttf
    JetBrainsMono-Variable.ttf
  fonts.go              // embed.FS + ResourceFor(...) loader
  theme_test.go
  ast_test.go           // §6 enforcement
```

`theme.go` is the **only** file outside this package permitted to import `fyne.io/fyne/v2/theme`. Verified in §6.

---

## 2. Public API

```go
// Package theme — file: internal/ui/theme/theme.go
package theme

import (
    "image/color"
    "sync"
    "fyne.io/fyne/v2"
    fynetheme "fyne.io/fyne/v2/theme"
)

// Apply switches the active palette and notifies Fyne.
// Safe to call from any goroutine; marshals to UI thread internally.
func Apply(mode core.ThemeMode) errtrace.Result[struct{}]

// Active returns the currently applied mode.
func Active() core.ThemeMode

// Color returns the resolved color for the active mode.
// Unknown name → ER-UI-21900 + ColorForeground fallback (never panic).
func Color(name ColorName) color.Color

// Size returns the resolved px size for the active mode + density.
func Size(name SizeName) float32

// Radius / Elev / Motion: same shape, different return type.
func Radius(name RadiusName) float32
func Elev(name ElevName)   ElevSpec
func Motion(name MotionName) MotionSpec
```

`appTheme` (in `fyne_theme.go`) implements the `fyne.Theme` interface and routes Fyne's built-in `theme.ColorName` constants to our `ColorName` set:

| Fyne built-in | Routes to |
|---|---|
| `theme.ColorNameBackground` | `ColorBackground` |
| `theme.ColorNameForeground` | `ColorForeground` |
| `theme.ColorNamePrimary`    | `ColorPrimary` |
| `theme.ColorNameButton`     | `ColorSurface` |
| `theme.ColorNameInputBackground` | `ColorSurface` |
| `theme.ColorNameDisabled`   | `ColorForegroundDisabled` |
| `theme.ColorNameError`      | `ColorError` |
| `theme.ColorNameSuccess`    | `ColorSuccess` |
| `theme.ColorNameWarning`    | `ColorWarning` |
| `theme.ColorNameSeparator`  | `ColorBorder` |
| `theme.ColorNameSelection`  | `ColorPrimary` (alpha 0.30) |

Tokens with no Fyne built-in equivalent (e.g. `ColorWatchDotWatching`, `ColorRawLogNewMail`) are read directly via our `theme.Color(...)` accessor — Fyne never sees them.

---

## 3. Apply / Live Switch Protocol

```go
var (
    mu     sync.RWMutex
    active core.ThemeMode = core.ThemeDark  // bootstrap default
)

func Apply(mode core.ThemeMode) errtrace.Result[struct{}] {
    if !validMode(mode) {
        return errtrace.Errf[struct{}]("ER-UI-21901", "unknown theme mode")
    }
    mu.Lock(); active = mode; mu.Unlock()
    fyne.Do(func() {
        a := fyne.CurrentApp()
        if a == nil { return }
        a.Settings().SetTheme(&appTheme{})
    })
    return errtrace.Ok(struct{}{})
}
```

- `Apply` is synchronous from the caller's view but the Fyne broadcast happens on the next UI tick (`fyne.Do`).
- `core.ThemeSystem` is resolved to `Dark` or `Light` via `app.Settings().ThemeVariant()` at call time and re-resolved on Fyne's `theme.VariantChanged` signal — no manual subscription needed.
- A successful `Apply` triggers `widget.Refresh` cascades automatically. Custom widgets (e.g. `WatchDot`) MUST implement `widget.Renderer.Refresh()` to re-read tokens.

---

## 4. Mapping `ThemeMode` → palette

```go
func paletteFor(mode core.ThemeMode) map[ColorName]color.NRGBA {
    switch mode {
    case core.ThemeLight:   return paletteLight
    case core.ThemeDark:    return paletteDark
    case core.ThemeSystem:
        if isSystemLight() { return paletteLight }
        return paletteDark
    }
    return paletteDark
}
```

`isSystemLight()` reads `fyne.CurrentApp().Settings().ThemeVariant()` and treats `theme.VariantLight` as light.

---

## 5. Bootstrap Order

In `cmd/mailpulse/main.go`:

```go
1. config.Load()                            // reads Settings
2. theme.Apply(config.UI.Theme)             // BEFORE any window construction
3. ui.NewMainWindow(...)
4. window.Show()
5. core.Settings.Subscribe → theme.Apply(...)  // live updates
```

If step 2 fails (`ER-UI-21901`), the app logs at WARN and continues with `ThemeDark` — never crashes.

---

## 6. AST Guards (`ast_test.go`)

| # | Guard | Mechanism |
|---|---|---|
| AST-T1 | Only `internal/ui/theme/` constructs `color.NRGBA{...}` or `color.RGBA{...}` literals. | `go/parser` walk over all packages; whitelist exactly one path. |
| AST-T2 | Only `internal/ui/theme/` imports `fyne.io/fyne/v2/theme`. | `go list -deps -json` cross-reference. |
| AST-T3 | Only `internal/ui/anim/` imports `fyne.io/fyne/v2/canvas` for `NewColorRGBAAnimation`. | string-match in AST identifiers. |
| AST-T4 | No file under `internal/ui/views/` declares a `var _ = color.…` or imports `image/color`. | import scan. |
| AST-T5 | Every `ColorName` constant in `tokens.go` has an entry in BOTH `palette_dark.go` and `palette_light.go`. | reflection over the package's exported constants vs. `len(palette*)`. |

---

## 7. Error Registry — Block 21900–21909

| Code | Meaning | Recovery |
|---|---|---|
| `ER-UI-21900` | Unknown `ColorName` requested | Returns `ColorForeground`, logs WARN |
| `ER-UI-21901` | `Apply` called with invalid mode | Caller fixes input |
| `ER-UI-21902` | Font resource embed missing | Build-time check; should never reach runtime |
| `ER-UI-21903..21909` | Reserved | — |

Block 21900–21909 is reserved exclusively for the theme/UI shell layer.

---

## 8. Test Contract

`theme_test.go` — 12 cases:

1. `Active()` returns `ThemeDark` on first call (bootstrap).
2. `Apply(ThemeLight)` succeeds and `Active() == ThemeLight`.
3. `Apply(ThemeMode(99))` returns `ER-UI-21901`.
4. `Color(ColorBackground)` returns the dark variant after `Apply(ThemeDark)`.
5. `Color(ColorBackground)` returns the light variant after `Apply(ThemeLight)`.
6. `Color(ColorName("nonsense"))` returns `ColorForeground` and logs `ER-UI-21900`.
7. `Size(SizeTextBody)` returns 14 in Comfortable, 12 in Compact (when density support lands).
8. `Apply` is goroutine-safe under `go test -race` (1000 concurrent calls).
9. `paletteFor(ThemeSystem)` resolves via `ThemeVariant()`.
10. Theme changes propagate to `widget.Refresh` for a representative custom widget.
11. Every Fyne built-in `ColorName` in §2 returns a non-zero color in both variants.
12. Font resources load (`theme.TextFont()` returns non-nil).

`ast_test.go` — 5 cases, one per AST-T# above.
