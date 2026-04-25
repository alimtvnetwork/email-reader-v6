# App Design System & UI

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Keywords

`app-design-system` · `app-ui` · `fyne` · `theme` · `tokens` · `components` · `layout` · `accessibility`

---

## Scoring

| Criterion | Status |
|-----------|--------|
| `00-overview.md` present | ✅ |
| AI Confidence assigned | ✅ |
| Ambiguity assigned | ✅ |
| Keywords present | ✅ |
| Scoring table present | ✅ |

---

## Purpose

Authoritative design system for the **Fyne desktop UI** of Mailpulse. Defines every color token, typography rule, spacing/elevation/radius scale, layout grid, animation primitive, and accessibility requirement. Every feature frontend (`02-features/*/02-frontend.md`) MUST source visual values from this spec — no hard-coded `color.RGBA{...}` literals anywhere outside `internal/ui/theme/`.

This spec is the **Fyne-native counterpart** to the web-targeted `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`. Where that file shows CSS variables / Tailwind classes, this spec shows the equivalent Fyne `theme.Theme` token names and Go constants. The **token semantics are identical** — only the binding form differs.

This spec **resolves Watch OI-1** (registers all `ColorWatchDot*` and `ColorRawLog*` tokens used by the Watch frontend).

---

## Document Inventory

| # | File | Purpose |
|---|------|---------|
| 1 | [`01-tokens.md`](./01-tokens.md) | Color, typography, spacing, radius, elevation, motion tokens — all enums + values for both `Dark` and `Light` variants. |
| 2 | [`02-theme-implementation.md`](./02-theme-implementation.md) | Fyne `fyne.Theme` implementation contract, `internal/ui/theme/` package layout, live-switch rules. |
| 3 | [`03-layout-and-shell.md`](./03-layout-and-shell.md) | Window shell, sidebar grid, content area, header/footer, breakpoints, density modes. |
| 4 | [`04-components.md`](./04-components.md) | Standard component variants (Button, Entry, Card, Badge, Toast, Dialog) and the watch-specific `WatchDot` widget. |
| 5 | [`05-accessibility.md`](./05-accessibility.md) | Contrast matrix, focus order, keyboard, color-blind safety, reduced-motion. |
| 6 | [`97-acceptance-criteria.md`](./97-acceptance-criteria.md) | Binary acceptance tests. |
| 7 | [`99-consistency-report.md`](./99-consistency-report.md) | Cross-checks against feature frontends + the consolidated guideline. |

---

## Ownership

- **`internal/ui/theme/`** is the **only** package permitted to construct `color.Color` values, define typography sizes, or import `fyne.io/fyne/v2/theme`. Verified by AST scan in `97-acceptance-criteria.md`.
- Feature views (`internal/ui/views/*.go`) call `theme.Color(theme.ColorName...)`, `theme.Size(theme.SizeName...)`, and pre-built widget factories from `internal/ui/widgets/`.
- All animation goes through `internal/ui/anim/` — feature views may NOT call `canvas.NewColorRGBAAnimation` directly.

---

## Cross-References

- Foundational rules: [`spec/07-design-system/00-overview.md`](../07-design-system/00-overview.md)
- Consolidated app guideline (web-targeted, semantic source of truth): [`spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`](../12-consolidated-guidelines/16-app-design-system-and-ui.md)
- Settings backend (theme persistence): [`spec/21-app/02-features/07-settings/01-backend.md`](../21-app/02-features/07-settings/01-backend.md)
- Settings frontend (live theme switch): [`spec/21-app/02-features/07-settings/02-frontend.md`](../21-app/02-features/07-settings/02-frontend.md) §3.4
- Watch frontend (consumer of `WatchDot`/`RawLog` tokens): [`spec/21-app/02-features/05-watch/02-frontend.md`](../21-app/02-features/05-watch/02-frontend.md)
- Every other feature frontend under `spec/21-app/02-features/*/02-frontend.md`

---

*App design system & UI — fully authored 2026-04-25 (replaces 2026-04-16 placeholder).*
