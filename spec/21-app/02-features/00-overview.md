# Feature Index

**Version:** 1.0.0
**Updated:** 2026-04-25

---

## Overview

All user-facing features of `email-read` (CLI + Fyne UI). Each feature is specified once and consumed by both binaries via `internal/core`.

---

## Feature Index

| # | Feature | UI status | CLI status | Files |
|---|---------|-----------|------------|-------|
| 01 | [Dashboard](./01-dashboard/00-overview.md) | Planned | N/A (UI-only) | 1 |
| 02 | [Emails](./02-emails/00-overview.md) | Planned | Partial (read) | 1 |
| 03 | [Rules](./03-rules/00-overview.md) | Planned | ✅ Done (`rules list/enable/disable`) | 1 |
| 04 | [Accounts](./04-accounts/00-overview.md) | Planned | ✅ Done (`add/list/remove`) | 1 |
| 05 | [Watch](./05-watch/00-overview.md) | Planned | ✅ Done (`<alias>`) | 1 |
| 06 | [Tools](./06-tools/00-overview.md) | Planned | ✅ Done (`export-csv`, `diagnose`, `read`) | 1 |
| 07 | [Settings](./07-settings/00-overview.md) | Planned | N/A (manual JSON edit) | 1 |

---

## Conventions per feature folder

Each `0N-{feature}/` contains a single `00-overview.md` (this round). When a feature grows past one page, expand into:

- `00-overview.md` — scope, user stories, dependencies
- `01-backend.md` — `internal/core` surface used + any new core function
- `02-frontend.md` — Fyne layout, components, inline forms
- `97-acceptance-criteria.md`
- `99-consistency-report.md`

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Fundamentals | [../01-fundamentals.md](../01-fundamentals.md) |
| App Issues | [../03-issues/00-overview.md](../03-issues/00-overview.md) |
