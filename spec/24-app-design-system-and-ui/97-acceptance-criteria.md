# 24 — App Design System & UI — Acceptance Criteria

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Binary, machine-checkable acceptance criteria for `spec/24-app-design-system-and-ui/`. Each item maps to ≥ 1 automated test in the file path listed.


<!-- sandbox-feasibility-legend v1 -->

## Sandbox feasibility legend

Each criterion below is tagged for the implementing AI so it can pick sandbox-implementable rows first:

| Tag | Meaning | Where it runs |
|---|---|---|
| 🟢 | **headless** — pure Go logic, AST scanner, SQL, registry, lint rule, errtrace, code-quality check | Sandbox: `nix run nixpkgs#go -- test -tags nofyne ./...` |
| 🟡 | **cgo-required** — Fyne canvas / widget render / focus ring / hover / pulse / pixel contrast / screen-reader runtime | Workstation only (CGO + display server) |
| 🔴 | **bench / E2E** — perf gate (`P-*`), benchmark, race detector under UI, multi-process integration | CI infra only |

See also: [`mem://design/schema-naming-convention.md`](mem://design/schema-naming-convention.md), `.lovable/cicd-issues/03-fyne-canvas-needs-cgo.md`, `.lovable/cicd-issues/05-no-bench-infra.md`.

---

## A. Tokens

| # | Criterion | Test ID |
|---|---|---|
| AC-DS-01 | 🟢 Every `ColorName` constant in `internal/ui/theme/tokens.go` matches §2 of `01-tokens.md` (one-to-one). | `Test_Tokens_ColorParity` |
| AC-DS-02 | 🟢 Every `SizeName`, `RadiusName`, `ElevName`, `MotionName` matches `01-tokens.md` §3–§7. | `Test_Tokens_NonColorParity` |
| AC-DS-03 | 🟢 Each color has both Dark and Light variants in `palette_dark.go` / `palette_light.go`. | `Test_Tokens_PaletteCoverage` |
| AC-DS-04 | 🟢 All `Watch*` and `RawLog*` tokens listed in `01-tokens.md` §2.5–§2.6 are present. (Resolves OI-1.) | `Test_Tokens_WatchAndRawLogPresent` |
| AC-DS-05 | 🟢 No two distinct tokens share the same RGB triple in the same variant. | `Test_Tokens_NoDuplicateValues` |

## B. Theme implementation

| # | Criterion | Test ID |
|---|---|---|
| AC-DS-10 | 🟡 `theme.Apply(ThemeDark)` then `theme.Color(ColorBackground)` returns the dark RGB. | `Test_Theme_ApplyDark` |
| AC-DS-11 | 🟡 `theme.Apply(ThemeLight)` then `theme.Color(ColorBackground)` returns the light RGB. | `Test_Theme_ApplyLight` |
| AC-DS-12 | 🟢 `theme.Apply(ThemeMode(99))` returns `ER-UI-21901`. | `Test_Theme_InvalidMode` |
| AC-DS-13 | 🟢 Unknown `ColorName` returns `ColorForeground` and logs `ER-UI-21900`. | `Test_Theme_UnknownNameFallback` |
| AC-DS-14 | 🔴 `theme.Apply` is goroutine-safe under `go test -race` (1000 concurrent calls). | `Test_Theme_Race` |
| AC-DS-15 | 🟡 `theme.Apply(ThemeSystem)` resolves via `app.Settings().ThemeVariant()`. | `Test_Theme_SystemResolves` |
| AC-DS-16 | 🟡 Embedded fonts (Inter + JetBrains Mono) load successfully. | `Test_Theme_FontsLoad` |
| AC-DS-17 | 🟢 AST: only `internal/ui/theme/` constructs `color.NRGBA{...}` / `color.RGBA{...}` literals. | `Test_AST_ColorLiteralsOnlyInTheme` |
| AC-DS-18 | 🟢 AST: only `internal/ui/theme/` imports `fyne.io/fyne/v2/theme`. | `Test_AST_FyneThemeImportLimit` |
| AC-DS-19 | 🟢 AST: only `internal/ui/anim/` imports `canvas.NewColorRGBAAnimation`. | `Test_AST_AnimImportLimit` |
| AC-DS-20 | 🟢 AST: no file under `internal/ui/views/` imports `image/color`. | `Test_AST_ViewsNoImageColor` |

## C. Layout & shell

| # | Criterion | Test ID |
|---|---|---|
| AC-DS-30 | 🟡 Sidebar renders 7 items in the order from `03-layout-and-shell.md` §3.1. | `Test_Shell_SidebarOrder` |
| AC-DS-31 | 🟡 Each sidebar item changes `NavRouter.Active()` on tap. | `Test_Shell_SidebarNav` |
| AC-DS-32 | 🟡 Active sidebar item uses `ColorSidebarItemActive` background. | `Test_Shell_ActiveStyle` |
| AC-DS-33 | 🟡 Status footer always visible and shows a `WatchDot`. | `Test_Shell_FooterPresent` |
| AC-DS-34 | 🟡 Window minimum size is enforced at 960 × 600. | `Test_Shell_MinSize` |
| AC-DS-35 | 🟢 Unknown route falls back to `/dashboard` and logs `ER-UI-21910`. | `Test_Shell_UnknownRouteFallback` |
| AC-DS-36 | 🟡 Outgoing view's `Leave()` is awaited before swap (dirty-confirm respected). | `Test_Shell_LeaveAwaited` |
| AC-DS-37 | 🟢 AST: only `internal/ui/shell/` constructs the top-level `container.NewBorder` for the window. | `Test_AST_ShellOnlyConstructsRoot` |

## D. Components

| # | Criterion | Test ID |
|---|---|---|
| AC-DS-40 | 🟡 `Button` rendered for each variant uses the documented background/foreground tokens. | `Test_Button_Variants` |
| AC-DS-41 | 🟡 `Entry` focus state shows 2 px `ColorPrimary` border. | `Test_Entry_FocusBorder` |
| AC-DS-42 | 🟡 `Card` hover transitions `ElevCard → ElevHover` over `MotionFast`. | `Test_Card_HoverElevation` |
| AC-DS-43 | 🟡 `Badge` renders all 6 variants with documented colors. | `Test_Badge_Variants` |
| AC-DS-44 | 🟡 `Toast` Error variant requires explicit dismiss; others auto-dismiss within their documented window. | `Test_Toast_AutoDismiss` |
| AC-DS-45 | 🟡 `Dialog` Confirm: backdrop click triggers Cancel; Form: backdrop click does NOT discard. | `Test_Dialog_BackdropBehavior` |
| AC-DS-46 | 🔴 `WatchDot.SetStatus` is concurrency-safe (`-race`). | `Test_WatchDot_Race` |
| AC-DS-47 | 🟡 `WatchDot` shows pulse for Reconnecting (1 Hz), Starting / Stopping (2 Hz). | `Test_WatchDot_PulseRates` |
| AC-DS-48 | 🟡 `WatchDot` reduces to steady color when reduced-motion is on. | `Test_WatchDot_ReducedMotion` |
| AC-DS-49 | 🟡 `RawLogLine` colors: heartbeat (dim), new-mail (full), error (red). | `Test_RawLog_LineColors` |
| AC-DS-50 | 🟢 AST: no view file calls `widget.NewButton` / `widget.NewEntry` directly. | `Test_AST_ViewsUseWidgetsPkg` |
| AC-DS-51 | 🟢 AST: only `WatchDot` calls `anim.Pulse(...)`. | `Test_AST_PulseOnlyInWatchDot` |

## E. Accessibility

| # | Criterion | Test ID |
|---|---|---|
| AC-DS-60 | 🟡 Every contrast pair in `05-accessibility.md` §1 meets its threshold (WCAG AA). | `Test_Contrast_Matrix` |
| AC-DS-61 | 🟡 Every view declares `FocusOrder()`. | `Test_FocusOrder_Declared` |
| AC-DS-62 | 🟢 Hidden / disabled widgets are not in any view's focus order. | `Test_FocusOrder_NoHiddenInOrder` |
| AC-DS-63 | 🟡 Every `WatchDot` instance has an adjacent text label with the status word. | `Test_StatusHasTextLabel` |
| AC-DS-64 | 🟡 Reduced-motion probe collapses all `Motion*` tokens to `MotionInstant`. | `Test_ReducedMotion_CollapsesTokens` |
| AC-DS-65 | 🟡 No interactive widget renders smaller than 32 × 32 px. | `Test_TargetSize_Min32` |
| AC-DS-66 | 🟡 `Cmd/Ctrl+1..7` map to the seven sidebar routes. | `Test_KeyboardShortcuts_Sidebar` |
| AC-DS-67 | 🟡 Focus ring uses `ColorPrimary` alpha 0.40, 2 px outline. | `Test_FocusRing_Visible` |
| AC-DS-68 | 🟡 Every `Button` / `WatchDot` / `Badge` / `RawLogLine` instance has a non-empty `AccessibilityLabel`. | `Test_AccessibilityLabel_NonEmpty` |
| AC-DS-69 | 🟢 AST: no `widget.NewButtonWithIcon("", ...)` calls. | `Test_NoIconOnlyButtons_WithoutLabel` |

## F. Definition of Done

All AC-DS-* automated tests pass on `linux/amd64`, `darwin/arm64`, `windows/amd64`. `make spec-check` reports zero TODOs in `spec/24-app-design-system-and-ui/`. Watch OI-1 is closed (the consistency report in §5 of `21-app/02-features/05-watch/99-consistency-report.md` updates to ✅).
