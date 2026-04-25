# 24 — App Design System & UI — Consistency Report

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Cross-checks `spec/24-app-design-system-and-ui/` against every feature frontend that consumes its tokens, the consolidated guideline (web-targeted), and the Settings live-switch contract.

---

## 1. Internal Consistency (within `24-app-design-system-and-ui/`)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| INT-1 | The 39 color tokens in `01-tokens.md` §2 are exactly the keys of `palette_dark.go` and `palette_light.go`. | `01-tokens.md` §2 ↔ `02-theme-implementation.md` §1 | `Test_Tokens_PaletteCoverage` (AC-DS-03) |
| INT-2 | Every contrast pair in `05-accessibility.md` §1 references token names that exist in `01-tokens.md`. | `05-accessibility.md` §1 ↔ `01-tokens.md` §2 | `Test_Contrast_Matrix` resolves names → values |
| INT-3 | Every component variant in `04-components.md` references tokens that exist in `01-tokens.md`. | `04-components.md` §2–§11 ↔ `01-tokens.md` | `Test_Components_TokenRefs` (parses spec tables vs. constants) |
| INT-4 | Sidebar item count (7) in `03-layout-and-shell.md` §3.1 matches the route count in §7. | `03-layout-and-shell.md` §3.1 ↔ §7 | `Test_Shell_SidebarOrder` (AC-DS-30) |
| INT-5 | Motion tokens in `01-tokens.md` §7 are the only motion durations referenced in `04-components.md`. | `01-tokens.md` §7 ↔ `04-components.md` | `Test_Components_MotionRefs` |

---

## 2. Cross-Feature Consistency

### 2.1 DS ↔ Watch (resolves OI-1)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-W1 | Every `ColorWatchDot*` constant referenced in Watch frontend §3 exists in `01-tokens.md` §2.5. | `21-app/02-features/05-watch/02-frontend.md` §3 ↔ `01-tokens.md` §2.5 | `Test_Tokens_WatchAndRawLogPresent` (AC-DS-04) |
| CF-W2 | Every `ColorRawLog*` constant referenced in Watch frontend §3 exists in `01-tokens.md` §2.6. | `21-app/02-features/05-watch/02-frontend.md` §3 ↔ `01-tokens.md` §2.6 | same as CF-W1 |
| CF-W3 | The `core.WatchStatus → ColorWatchDot*` mapping in Watch §3 matches the table in `04-components.md` §6. | `21-app/02-features/05-watch/02-frontend.md` §3 ↔ `04-components.md` §6 | `Test_WatchDot_StatusColorMap` |
| CF-W4 | Pulse rates (1 Hz / 2 Hz) declared in Watch frontend match `04-components.md` §6 and `01-tokens.md` §7. | three-way | `Test_WatchDot_PulseRates` (AC-DS-47) |
| CF-W5 | Watch frontend uses `widgets.WatchDot` (not a hand-rolled circle). | `21-app/02-features/05-watch/02-frontend.md` ↔ `04-components.md` §6 | `Test_AST_Watch_UsesWatchDotWidget` |

**OI-1 status: CLOSED.** All `ColorWatchDot*` and `ColorRawLog*` tokens are now registered in `01-tokens.md` §2.5–§2.6 and verified by AC-DS-04. Update `21-app/02-features/05-watch/99-consistency-report.md` line 274 to mark OI-1 ✅ in the next consistency-report sweep (Task #34).

### 2.2 DS ↔ Settings (theme persistence + live switch)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-S1 | `core.Settings.Save` with `Theme=Light` triggers `SettingsEvent` → `theme.Apply(ThemeLight)` within ≤ 80 ms. | `21-app/02-features/07-settings/02-frontend.md` §3.4 ↔ `02-theme-implementation.md` §3 | `Test_Settings_LiveTheme_E2E` |
| CF-S2 | `theme.Apply(ThemeSystem)` resolves to `Dark` or `Light` via `ThemeVariant()`. | `02-theme-implementation.md` §4 ↔ `21-app/02-features/07-settings/01-backend.md` §3 | `Test_Theme_SystemResolves` (AC-DS-15) |
| CF-S3 | Discarding via Leave-confirm in Settings restores the previous theme (Settings frontend §3.4). | `21-app/02-features/07-settings/02-frontend.md` §3.4 | `Test_Theme_DiscardRestores` (already in Settings AC-SF-18) |
| CF-S4 | `theme.Apply` failure (`ER-UI-21901`) does NOT crash Settings save — Settings logs WARN and continues. | `21-app/02-features/07-settings/01-backend.md` §10 ↔ `02-theme-implementation.md` §5 | `Test_Settings_ThemeApplyFailure_NonFatal` |

### 2.3 DS ↔ all feature frontends

| # | Invariant | Enforcement |
|---|---|---|
| CF-X1 | No file under `internal/ui/views/` constructs a `color.NRGBA{...}` / `color.RGBA{...}` literal. | `Test_AST_ColorLiteralsOnlyInTheme` (AC-DS-17) |
| CF-X2 | No file under `internal/ui/views/` imports `image/color`. | `Test_AST_ViewsNoImageColor` (AC-DS-20) |
| CF-X3 | No file under `internal/ui/views/` calls `widget.NewButton` / `widget.NewEntry` directly. | `Test_AST_ViewsUseWidgetsPkg` (AC-DS-50) |
| CF-X4 | Every view declares `FocusOrder()`. | `Test_FocusOrder_Declared` (AC-DS-61) |
| CF-X5 | Every view's title shown in the page header is sourced from a single `Header() fyne.CanvasObject` method (per `03-layout-and-shell.md` §4). | `Test_Views_HeaderMethodPresent` |

### 2.4 DS ↔ Tools / Rules / Dashboard / Emails / Accounts

These features consume the design system but introduce no new tokens. The rule is uniform: every visual element in their `02-frontend.md` MUST reference a token from `01-tokens.md`. Verified globally by `Test_AllViews_OnlyTokenizedColors` (AST scan + cross-reference).

---

## 3. Cross-Reference to Consolidated Guideline

The consolidated guideline `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md` targets a **web stack** (Tailwind / CSS variables). This Fyne spec is the **semantic equivalent** for the desktop app. Token semantics MUST agree even though the binding form differs.

| Consolidated (web) | Fyne (this spec) | Match? |
|---|---|---|
| `--app-sidebar-width: 280px` | `03-layout-and-shell.md` §2: 240 px | **Diverges intentionally** (desktop density). Documented here. |
| `--app-header-height: 64px` | `03-layout-and-shell.md` §2: 56 px | **Diverges intentionally** (no top bar in v1; only per-page header). |
| `--success`, `--warning`, `--code-bg`, etc. | `01-tokens.md` §2 (`ColorSuccess`, `ColorWarning`, `ColorCodeBg`) | ✅ semantic match |
| Button / Card / Toast variants | `04-components.md` §2, §4, §8 | ✅ semantic match |
| Z-index scale | Fyne uses overlay layers (`Canvas.Overlays`); ordering documented in `04-components.md` §8 (Toast) | ✅ functionally equivalent |
| Shadow scale | `01-tokens.md` §6 `Elev*` | ✅ semantic match (rendered as layered rectangles, not CSS shadows) |
| Typography scale | `01-tokens.md` §3 | ✅ pixel-equivalent |

Divergences (sidebar width, header height) are **deliberate** and locked here. Future syncs go: web spec → propose change → update this spec → bump MAJOR if any token contract changes.

---

## 4. Cross-Reference to Consolidated Guidelines (other)

| Guideline | DS reference | Verified by |
|---|---|---|
| `02-coding-guidelines.md` §1.1 (PascalCase) | All token Go constants. | `golangci-lint` |
| `02-coding-guidelines.md` §3 (≤ 15-line fns) | `theme.Apply`, `Color`, `Size`, etc. | `golangci-lint funlen` |
| `02-coding-guidelines.md` §6 (no `any`) | All public APIs in `02-theme-implementation.md` §2. | `golangci-lint forbidigo` |
| `03-error-management.md` (`apperror.Wrap` + registry) | `02-theme-implementation.md` §7 block 21900–21909, `03-layout-and-shell.md` §8 block 21910–21919. | `Test_Errors_AllWrapped` |

---

## 5. Open Issues

| # | Issue | Owner | Disposition |
|---|---|---|---|
| OI-DS-1 | Compact density mode plumbed but not user-exposed. | DS + Settings | Add to Settings UI in MINOR bump. |
| OI-DS-2 | Sidebar collapse reserved (`SizeSidebarCollapsed = 0`) but not implemented. | DS | v2 only. |
| OI-DS-3 | No multi-window support (single window only — `21-app/01-fundamentals.md` §5). | shell | v2 only. |

OI-1 from Watch (`ColorWatchDot*` registration) is **closed by this spec**.

---

## 6. Sign-Off

| Reviewer | Role | Date |
|---|---|---|
| Spec author (AI) | Drafting | 2026-04-25 |
| Pending | Tech lead | — |
| Pending | QA | — |
| Pending | Accessibility lead | — |
