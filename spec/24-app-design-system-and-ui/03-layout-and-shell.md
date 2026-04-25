# 03 — Layout & Shell

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the application window shell (sidebar + main + footer), responsive breakpoints, and how every feature view plugs into the navigation grid. Implemented in `internal/ui/shell/`.

Cross-references:
- Tokens: [`./01-tokens.md`](./01-tokens.md)
- Components: [`./04-components.md`](./04-components.md)
- Per-feature views: every `02-features/*/02-frontend.md`

---

## 1. Window

| Property | Value |
|---|---|
| Title | `Mailpulse` |
| Initial size | 1280 × 800 |
| Min size | 960 × 600 |
| Resize | Yes |
| Max size | unlimited |
| Icon | `internal/ui/icons/AppIcon.png` (512 × 512) |
| Window padding | 0 (shell paints its own) |
| Window background | `ColorBackground` |

Window is constructed once in `cmd/mailpulse/main.go`. **No multi-window support in v1** (per `21-app/01-fundamentals.md` §5; revisit tracked under issues).

---

## 2. Shell Grid

```
┌──────────────────────────────────────────────────────────┐
│ [Sidebar 240]│        Main content                        │
│              │  ┌──────────────────────────────────────┐  │
│              │  │ Page header (height = 56)            │  │
│              │  ├──────────────────────────────────────┤  │
│              │  │ Page body (scroll if overflow)       │  │
│              │  │  padding-x = SizeSpacing5 (24)       │  │
│              │  │  padding-y = SizeSpacing5 (24)       │  │
│              │  └──────────────────────────────────────┘  │
│              ├──────────────────────────────────────────┤  │
│              │ Status footer (height = 28)              │  │
└──────────────┴──────────────────────────────────────────┘
```

Built in `internal/ui/shell/shell.go` using:

```go
container.NewBorder(
    /*top   */ nil,
    /*bottom*/ statusFooter,         // §5
    /*left  */ sidebar,              // §3
    /*right */ nil,
    /*center*/ container.NewBorder(
        pageHeader,                  // §4
        nil, nil, nil,
        container.NewVScroll(currentPageBody),
    ),
)
```

| Region | Width / height | Background token | Border token |
|---|---|---|---|
| Sidebar | 240 px (fixed in v1) | `ColorSidebar` | right edge `ColorSidebarBorder` |
| Page header | 56 px | `ColorBackground` | bottom edge `ColorBorder` |
| Page body | flex | `ColorBackground` | none |
| Status footer | 28 px | `ColorSurfaceMuted` | top edge `ColorBorder` |

**No collapsible sidebar in v1.** Reserved as `SizeSidebarCollapsed = 0` for a future toggle.

---

## 3. Sidebar

### 3.1 Items (top → bottom)

| # | Label | Icon | Route | Feature spec |
|---|---|---|---|---|
| 1 | Dashboard | `theme.HomeIcon()` | `/dashboard` | `02-features/01-dashboard` |
| 2 | Emails    | `theme.MailComposeIcon()` | `/emails` | `02-features/02-emails` |
| 3 | Rules     | `theme.SettingsIcon()` (custom: ListChecks) | `/rules` | `02-features/03-rules` |
| 4 | Accounts  | `theme.AccountIcon()` | `/accounts` | `02-features/04-accounts` |
| 5 | Watch     | `theme.MediaPlayIcon()` | `/watch` | `02-features/05-watch` |
| 6 | Tools     | `theme.ContentCutIcon()` (custom: Wrench) | `/tools` | `02-features/06-tools` |
| 7 | Settings  | `theme.SettingsIcon()` | `/settings` | `02-features/07-settings` |

Order is fixed. Adding/removing/reordering items requires a MAJOR bump.

### 3.2 Item layout

Each item is a `*widget.Button` (custom-rendered) with:

| State | Background | Foreground |
|---|---|---|
| default  | transparent              | `ColorSidebarForeground` |
| hover    | `ColorSidebarItemHover`  | `ColorSidebarForeground` |
| active   | `ColorSidebarItemActive` | `ColorSidebarItemActiveForeground` |
| disabled | transparent              | `ColorForegroundDisabled` |

Padding: `SizeSpacing3` horizontal, `SizeSpacing2` vertical. Icon-to-label gap: `SizeSpacing2`.
Active state is mutually exclusive — exactly one item is active at any time, driven by the `NavRouter`.

### 3.3 Sidebar footer (bottom-anchored)

A small block above the window edge containing:

- Theme toggle (cycles Dark → Light → System)
- App version label (caption text)

Anchored via `container.NewBorder(navItems, themeToggleBlock, nil, nil, nil)`.

---

## 4. Page Header

| Region | Content |
|---|---|
| Left | Page title (`SizeTextPageTitle`, `ColorForeground`) |
| Right | Optional action buttons (e.g. "New rule", "Refresh") — max 3, right-aligned with `SizeSpacing2` gap |

Per-feature header content is provided by each view's `Header() fyne.CanvasObject` method. The shell never composes feature widgets.

---

## 5. Status Footer

Single horizontal band, always visible. Reads `core.Watcher` status via the `core.Dashboard` aggregator (so the footer stays in sync without subscribing to lower-level events).

| Region | Content |
|---|---|
| Far-left | `WatchDot` (size 12) + alias name + status word ("Watching", "Reconnecting…", etc.) |
| Center   | Last poll timestamp (caption) — `LastPolledAt` from any active alias, formatted "12 s ago" |
| Far-right| Connection-error count badge (only visible when > 0) |

Padding: `SizeSpacing3` horizontal, `SizeSpacing1` vertical. Background `ColorSurfaceMuted`. Top border `ColorBorder`.

---

## 6. Density / Breakpoints

| Width | Behavior |
|---|---|
| ≥ 960 px (always — min size) | Standard layout. |
| Future | Compact density mode (`01-tokens.md` §8) reduces spacing/text by 0.875×. Sidebar remains 240 px. |
| Future | Sidebar collapse (`SizeSidebarCollapsed = 0`) — not in v1. |

No mobile/touch layout — Mailpulse is desktop-only.

---

## 7. Routing

`internal/ui/shell/router.go`:

```go
type NavRouter interface {
    Navigate(route string)            // e.g. "/emails"
    Active() string
    OpenInOsFileManager(path string)  // shell helper, used by Settings paths card
}
```

- Routes are strings, prefixed `/`. Unknown routes fall back to `/dashboard` and log `ER-UI-21910` at WARN.
- Navigation is synchronous on the UI goroutine; the shell calls `view.Leave()` on the outgoing view (which may show its dirty-confirm dialog and cancel) before swapping in the new view.
- Deep linking is not supported in v1 (no command-line `--route=` flag).

---

## 8. Error Registry — Block 21910–21919 (shell / router)

| Code | Meaning |
|---|---|
| `ER-UI-21910` | Unknown route → fell back to `/dashboard` |
| `ER-UI-21911` | View `Enter` returned an error → fell back to `/dashboard` with toast |
| `ER-UI-21912` | View `Leave` blocked by dirty-confirm — navigation aborted (informational) |
| `ER-UI-21913..21919` | Reserved |

---

## 9. Test Contract

Tests live in `internal/ui/shell/shell_test.go`. 9 required cases:

1. Shell renders all 7 sidebar items in the documented order.
2. Clicking each item updates `NavRouter.Active()`.
3. Active item uses `ColorSidebarItemActive` background.
4. Status footer shows `WatchDot` color matching `core.Watcher` status.
5. Page header displays the active view's title.
6. Unknown route falls back to `/dashboard` and logs `ER-UI-21910`.
7. Outgoing view's `Leave()` is awaited before swap.
8. Window minimum size is enforced (resize below 960×600 is clamped).
9. AST scan: only `internal/ui/shell/` constructs the top-level `container.NewBorder` for the window.
