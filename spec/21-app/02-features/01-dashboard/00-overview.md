# 01 — Dashboard — Overview

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **Dashboard** is the landing view of the Fyne UI (`cmd/email-read-ui`). It gives a one-glance answer to *"is the system healthy and what happened recently?"* — without requiring the user to open any other tab.

It is **read-only** (no mutations). Every number is sourced from `internal/core/dashboard.go` so the same projection can later power a `email-read status` CLI subcommand without code duplication.

Cross-references:
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.1
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21100–21199
- Guideline: `spec/12-consolidated-guidelines/13-app.md`, `16-app-design-system-and-ui.md`

---

## 1. Scope

### In scope
1. Account count (configured aliases).
2. Rule totals (`Total` / `Enabled` / `Disabled`).
3. Stored email count (global + per-active-alias).
4. Recent activity list (last N watcher events, default 20).
5. Per-account health badges (LastPollAt, LastErrorAt, ConsecutiveFailures).
6. A "Refresh now" button that re-runs `core.Dashboard.Summary` (does **not** trigger an IMAP poll).

### Out of scope
- Mutating actions (delete, mark-read, edit rule) — those live in their owning feature tab.
- Charts / graphs (deferred; the design system does not yet ship a chart widget — see `16-app-design-system-and-ui.md`).
- Per-rule hit counters (lives in Rules tab).

---

## 2. User Stories

| # | As a … | I want to …                                                        | So that …                                                  |
|---|--------|--------------------------------------------------------------------|------------------------------------------------------------|
| 1 | User   | open the app and see whether all accounts polled successfully      | I can trust the watcher is alive without reading logs      |
| 2 | User   | see how many emails are stored locally per account                 | I know the watcher actually wrote something                |
| 3 | User   | see the last 20 watcher events with timestamps                     | I can correlate an alert in another app to a delivery here |
| 4 | User   | click an account row and jump to the Emails tab filtered by alias  | I can drill into the offending account in one click        |
| 5 | User   | refresh the dashboard manually                                     | I can verify a fix without waiting for the next poll       |

---

## 3. Dependencies

| Dependency             | Why                                                          |
|------------------------|--------------------------------------------------------------|
| `core.Dashboard`       | All data access (Summary / RecentActivity / AccountHealth)   |
| `core.Watch`           | Subscribes to `WatchEvent` channel for live updates          |
| `internal/store`       | (transitive) reads `Email`, `WatchState`, `Rule` tables      |
| `internal/config`      | (transitive) reads `Accounts[]`, `Rules[]` from `config.json`|
| `internal/ui/theme`    | Tokens for status badge colors (Healthy/Warn/Error)          |

The Dashboard view **must not** import `internal/store` or `internal/mailclient` directly. Architecture DAG (§3 of `07-architecture.md`) is enforced.

---

## 4. Data Model (read-only projections)

All field names are **PascalCase** (per `04-coding-standards.md` §1.1).

```go
type DashboardSummary struct {
    GeneratedAt    time.Time
    Accounts       int
    RulesTotal     int
    RulesEnabled   int
    RulesDisabled  int
    EmailsTotal    int
    UnreadTotal    int
    LastPollAt     time.Time   // max across all accounts
    OverallHealth  HealthLevel // Healthy | Warning | Error
}

type ActivityRow struct {
    OccurredAt time.Time
    Alias      string
    Kind       ActivityKind  // PollStarted | PollSucceeded | PollFailed | EmailStored | RuleMatched
    Message    string
    ErrorCode  int           // 0 if Kind != PollFailed
}

type AccountHealthRow struct {
    Alias                string
    LastPollAt           time.Time
    LastErrorAt          time.Time   // zero value if never errored
    ConsecutiveFailures  int
    Health               HealthLevel
    EmailsStored         int
    UnreadCount          int
}

type HealthLevel string  // "Healthy" | "Warning" | "Error"
type ActivityKind string // PascalCase enum, per 04-enum-standards
```

---

## 5. Refresh & Live-Update Behavior

| Trigger                          | Action                                                                 |
|----------------------------------|------------------------------------------------------------------------|
| Tab opened                       | Call `Summary` + `RecentActivity` + `AccountHealth` once               |
| User clicks **Refresh**          | Same as above; debounce 500 ms                                         |
| `WatchEvent` arrives (live)      | Prepend to ActivityRows (cap 20); patch matching `AccountHealthRow`    |
| Tab loses focus                  | Unsubscribe from `WatchEvent` channel                                  |
| Tab regains focus                | Re-subscribe and re-fetch summary                                      |

The dashboard **never** triggers an IMAP poll itself — that is `core.Watch`'s job.

---

## 6. Acceptance Snapshot (full criteria in `97-acceptance-criteria.md`)

A merged Dashboard build is shippable iff:

1. Cold-start render of the Summary panel completes in **< 100 ms** with 10 accounts and 100 000 stored emails (in-memory store).
2. Live `WatchEvent` insertion happens in **< 16 ms** (one frame at 60 FPS).
3. All status badges respect the design tokens `--Status-Healthy`, `--Status-Warning`, `--Status-Error`.
4. Clicking an `AccountHealthRow` navigates to the Emails tab with `EmailQuery.Alias` pre-set.
5. Failing `core.Dashboard.Summary` shows the standard error envelope (per `06-error-registry.md`) and a **Retry** button — never a blank panel.
6. Zero `interface{}` / `any` in any new code (lint-enforced).

---

## 7. Open Questions

None. Confidence: Production-Ready.

---

**End of `01-dashboard/00-overview.md`**
