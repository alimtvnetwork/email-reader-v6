# 21-app spec authoring — atomic task list

**Created:** 2026-04-25
**Source of truth** for the spec/21-app authoring round. On each `next` command, find the first unchecked item, do exactly that one task, mark it `[x]`, and stop.

Citations are pinned to `spec/12-consolidated-guidelines/` (read 2026-04-25):
- Coding: `02-coding-guidelines.md` (PascalCase keys §1.1, 15-line fn §3, strict typing §5, no `any`/`interface{}` §6)
- Errors: `03-error-management.md` (apperror.Wrap, Result[T], envelope, code registry ranges)
- DB: `18-database-conventions.md` (singular PascalCase tables, FK rules, positive booleans)
- App design: `16-app-design-system-and-ui.md` (sidebar tokens, type scale, z-index)
- CLI: `23-generic-cli.md` (project structure, subcommand dispatch, exit codes)
- App folder: `13-app.md` (placement guide, required files)

## Tasks

- [x] 01 — Read selected guideline folders (01, 02, 03, 04, 07, 10, 12, 13, 16) + linters
- [x] 02 — Move `.lovable/linters/` → `linters/` and `.lovable/linter-scripts/` → `linter-scripts/`; update references (no refs found — clean rename)
- [x] 03 — Author `spec/21-app/04-coding-standards.md` (central code-style reference, cites 02-coding)
- [x] 04 — Author `spec/21-app/05-logging-strategy.md` (log format, levels, trace IDs, heartbeat invariant)
- [x] 05 — Author `spec/21-app/06-error-registry.md` (every error code, message, layer, recovery via internal/errtrace)
- [x] 06 — Author `spec/21-app/07-architecture.md` (internal/core API surface, package dependency graph)
- [x] 07 — Dashboard feature: write `02-features/01-dashboard/00-overview.md`
- [x] 08 — Dashboard feature: write `01-backend.md` (core.Dashboard signatures + queries)
- [x] 09 — Dashboard feature: write `02-frontend.md` (Fyne widget tree, every container/widget)
- [x] 10 — Dashboard feature: write `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 11 — Emails feature: write `02-features/02-emails/00-overview.md`
- [x] 12 — Emails feature: write `01-backend.md`
- [x] 13 — Emails feature: write `02-frontend.md`
- [x] 14 — Emails feature: write `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 15 — Rules feature: write `02-features/03-rules/00-overview.md`
- [x] 16 — Rules feature: write `01-backend.md`
- [x] 17 — Rules feature: write `02-frontend.md`
- [x] 18 — Rules feature: write `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 19 — Accounts feature: write `02-features/04-accounts/00-overview.md`
- [x] 20 — Accounts feature: write `01-backend.md`
- [x] 21 — Accounts feature: write `02-frontend.md`
- [x] 22 — Accounts feature: write `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 23 — Watch feature: write `02-features/05-watch/00-overview.md`
- [x] 24 — Watch feature: write `01-backend.md` (poll loop, heartbeat, event fan-out)
- [x] 25 — Watch feature: write `02-frontend.md` (live log + structured cards)
- [x] 26 — Watch feature: write `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 27 — Tools feature: write `02-features/06-tools/00-overview.md`
- [x] 28 — Tools feature: write `01-backend.md`
- [x] 29 — Tools feature: write `02-frontend.md`
- [x] 30 — Tools feature: write `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 31 — Settings feature: write `02-features/07-settings/00-overview.md` (pre-existing) + `01-backend.md` + `02-frontend.md` + `97-acceptance-criteria.md` + `99-consistency-report.md`
- [x] 32 — Author `spec/23-app-database/` (overview rewrite + 01-schema + 02-queries + 03-migrations + 04-retention-and-vacuum + 97-acceptance-criteria + 99-consistency-report)
- [x] 33 — Author `spec/24-app-design-system-and-ui/` (overview rewrite + 01-tokens + 02-theme-implementation + 03-layout-and-shell + 04-components + 05-accessibility + 97-acceptance-criteria + 99-consistency-report; **resolves Watch OI-1**)
- [x] 34 — Populate `spec/21-app/03-issues/` from `.lovable/solved-issues/` + `.lovable/pending-issues/` (8 solved issues migrated to `solved/`, empty `pending/` seeded, `00-overview.md` rewritten as severity-tagged index, Watch OI-1 referenced as closed in issue 08)
- [x] 35 — Finalize `spec/21-app/97-acceptance-criteria.md` + `99-consistency-report.md` (v1.0 → v2.0; added 35 AC-PROJ cross-feature criteria, indexed 700 total criteria across 9 source files, sign-off ladder, full inventory of 71 spec files, audited error-code / named-query / forward-reference resolution, recorded 7 known spec-vs-implementation deltas, confirmed Watch OI-1 / OI-2 closed)

---

## Status: Spec authoring round COMPLETE ✅

All 35 tasks done. The next phase (implementation work) is tracked under `spec/21-app/99-consistency-report.md` §6 "Implementation deltas" and is **out of scope** for this tasklist.
