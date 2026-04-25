# 21-app — email-read

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Draft
**AI Confidence:** High
**Ambiguity:** Low

---

## Overview

`email-read` is a Go application that watches IMAP inboxes, persists every email to SQLite + disk, evaluates regex rules, and auto-opens matching URLs in Chrome incognito. This folder is the canonical spec for the application — both the existing CLI (`cmd/email-read`) and the new Fyne desktop UI (`cmd/email-read-ui`) — using the App Project Template (`01-fundamentals.md` + `02-features/` + `03-issues/`).

Per the spec authoring guide, folder #21 is reserved for the canonical app folder (`21-app/`). Earlier names (`21-golang-email-reader`, `22-fyne-ui`) have been merged here. Their original content is preserved verbatim under `legacy/` for reference but is **not** authoritative — this overview and the files below are.

---

## Keywords

`email-read` · `imap` · `fyne` · `desktop-ui` · `cli` · `sqlite` · `rules-engine` · `chrome-launcher` · `cross-platform` · `app-spec`

---

## Scoring

| Metric | Value |
|--------|-------|
| AI Confidence | High |
| Ambiguity | Low |
| Health Score | 100/100 |

---

## Folder Structure

```
21-app/
├── 00-overview.md                   ← this file
├── 01-fundamentals.md               core architecture (CLI + UI share internal/core)
│
├── 02-features/
│   ├── 00-overview.md               feature index (status table)
│   ├── 01-dashboard/                Counts, recent events, Start Watch CTA
│   ├── 02-emails/                   List + detail + extracted links
│   ├── 03-rules/                    Table with enable/disable + add form
│   ├── 04-accounts/                 Table + add/remove inline forms
│   ├── 05-watch/                    Live watcher: structured cards + raw log tabs
│   ├── 06-tools/                    Inline forms: read, export-csv, diagnose
│   └── 07-settings/                 Config paths, theme, poll interval
│
├── 03-issues/
│   └── 00-overview.md               issue index (empty for now)
│
├── legacy/                          archived — NOT authoritative
│   ├── spec.md                      original CLI spec from 21-golang-email-reader
│   ├── plan-cli.md                  10-step CLI plan (all complete)
│   └── plan-fyne-ui.md              28-step Fyne plan (will be re-derived from features/)
│
├── 97-acceptance-criteria.md        consolidated criteria for all features
└── 99-consistency-report.md         structural health report
```

---

## Files

| # | File | Description |
|---|------|-------------|
| 01 | [01-fundamentals.md](./01-fundamentals.md) | Core architecture: shared `internal/core`, dual entrypoints, data flow |
| 02 | [02-features/00-overview.md](./02-features/00-overview.md) | Feature index with status table |
| 03 | [03-issues/00-overview.md](./03-issues/00-overview.md) | Issue index (empty initially) |
| 97 | [97-acceptance-criteria.md](./97-acceptance-criteria.md) | Consolidated acceptance criteria |
| 99 | [99-consistency-report.md](./99-consistency-report.md) | Structural health report |

---

## Cross-References

| Reference | Location |
|-----------|----------|
| App Issues | [../22-app-issues/00-overview.md](../22-app-issues/00-overview.md) |
| App Database | [../23-app-database/00-overview.md](../23-app-database/00-overview.md) |
| App Design System | [../24-app-design-system-and-ui/00-overview.md](../24-app-design-system-and-ui/00-overview.md) |
| Coding Guidelines | [../02-coding-guidelines/00-overview.md](../02-coding-guidelines/00-overview.md) |
| Error Management | [../03-error-manage/00-overview.md](../03-error-manage/00-overview.md) |
| Database Conventions | [../04-database-conventions/00-overview.md](../04-database-conventions/00-overview.md) |
| Spec Authoring Guide | [../01-spec-authoring-guide/00-overview.md](../01-spec-authoring-guide/00-overview.md) |
