# Consistency Report — 21-app

**Version:** 1.0.0
**Last Updated:** 2026-04-25

---

## Module Health

| Criterion | Status |
|-----------|--------|
| `00-overview.md` present | ✅ |
| `99-consistency-report.md` present | ✅ |
| Lowercase kebab-case naming | ✅ |
| Unique numeric sequence prefixes | ✅ |

**Health Score:** 100/100 (A+)

---

## File Inventory

| # | Path | Status |
|---|------|--------|
| 00 | `00-overview.md` | ✅ Present |
| 01 | `01-fundamentals.md` | ✅ Present |
| 02 | `02-features/00-overview.md` | ✅ Present |
| 02.01 | `02-features/01-dashboard/00-overview.md` | ✅ Present |
| 02.02 | `02-features/02-emails/00-overview.md` | ✅ Present |
| 02.03 | `02-features/03-rules/00-overview.md` | ✅ Present |
| 02.04 | `02-features/04-accounts/00-overview.md` | ✅ Present |
| 02.05 | `02-features/05-watch/00-overview.md` | ✅ Present |
| 02.06 | `02-features/06-tools/00-overview.md` | ✅ Present |
| 02.07 | `02-features/07-settings/00-overview.md` | ✅ Present |
| 03 | `03-issues/00-overview.md` | ✅ Present |
| 97 | `97-acceptance-criteria.md` | ✅ Present |
| 99 | `99-consistency-report.md` | ✅ Present |
| — | `legacy/spec.md` | 📦 Archived (was `21-golang-email-reader/spec.md`) |
| — | `legacy/plan-cli.md` | 📦 Archived (was `21-golang-email-reader/plan.md`) |
| — | `legacy/plan-fyne-ui.md` | 📦 Archived (was `22-fyne-ui/plan.md`) |

**Total active files:** 12. **Legacy:** 3.

---

## Migration Notes (2026-04-25)

| Action | Detail |
|---|---|
| Renamed | `spec/21-golang-email-reader/` → `spec/21-app/` (per spec authoring guide §"Required Root Folders") |
| Folded | `spec/22-fyne-ui/plan.md` → `spec/21-app/legacy/plan-fyne-ui.md` |
| Authored | New App Project Template structure: 00-overview, 01-fundamentals, 02-features/* (7 features), 03-issues, 97-acceptance, 99-consistency |
| Decision | Existing partial Fyne code in `internal/ui/` and `cmd/email-read-ui/` is treated as throwaway — spec is the authoritative target |

---

## Cross-Reference Validation

All internal links use file-relative paths (`./` and `../`). Manually verified for this initial draft. Run `node .lovable/linter-scripts/generate-dashboard-data.cjs` to confirm zero broken links once the dashboard scanner picks up the new folder.

---

## Validation History

| Date | Version | Action |
|------|---------|--------|
| 2026-04-25 | 1.0.0 | Initial spec authored after `21-golang-email-reader` + `22-fyne-ui` merge |
