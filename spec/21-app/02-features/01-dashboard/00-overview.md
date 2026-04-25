# Feature 01 — Dashboard

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI only

---

## Purpose

The first view a user sees. Communicates project health at a glance and provides a one-click "Start Watch" CTA.

## User stories

- As a user opening the app cold, I see how many accounts/rules/emails I have without clicking anything.
- I see the **5 most recent watcher events** without leaving Dashboard.
- I can start watching the default account with one button.

## Layout

```
┌─ Dashboard ──────────────────────────────────────┐
│  ┌────────┐ ┌────────┐ ┌─────────────────┐        │
│  │ Accts  │ │ Rules  │ │ Emails today    │        │
│  │   3    │ │   7    │ │      42         │        │
│  └────────┘ └────────┘ └─────────────────┘        │
│                                                   │
│  Recent events                       [▶ Start]    │
│  ─────────────────────────────────────────        │
│  10:42  work    new mail  "Sign in link"          │
│  10:39  work    rule hit  open-magic-links        │
│  10:14  personal idle    messages=412             │
│  ...                                              │
└───────────────────────────────────────────────────┘
```

Three count cards across the top, recent-events list below, "▶ Start" button top-right of the events panel. Cards are clickable (Accounts → Accounts view, Rules → Rules view, Emails → Emails view).

## Backend (core API)

New functions in `internal/core/dashboard.go` (already exists — verify signature):

```go
type DashboardSnapshot struct {
    AccountsCount int
    RulesCount    int
    EmailsToday   int
    RecentEvents  []Event   // last 5
}

func GetDashboard(ctx context.Context) (DashboardSnapshot, error)
```

Reads from `internal/store` only. Does not start a watcher. The "Start Watch" button delegates to the **Watch** feature's start path (does not duplicate logic).

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-D1 | Dashboard renders within 100ms of a cold open on a DB with 10k emails. |
| AC-D2 | Counts refresh every 5s while the view is visible (no manual reload). |
| AC-D3 | Recent-events list shows the latest 5 events from any alias, newest first. |
| AC-D4 | "▶ Start" disabled when zero accounts configured; enabled otherwise. |
| AC-D5 | Clicking a count card navigates to the corresponding view, preserving the selected alias. |

## Out of scope

Charts, time-series, multi-day drill-down — defer to a future "Reports" feature.
