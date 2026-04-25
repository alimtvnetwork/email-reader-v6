# 02 — Emails — Overview

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **Emails** feature is the primary work surface of `email-read`: a fast, searchable, alias-scoped list of every email the watcher has stored locally, plus a detail pane for reading and acting on individual messages.

It is the only feature with both **read** and **mutating** operations on the `Email` table (mark-read, delete, refresh-from-IMAP).

Cross-references:
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.2
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21200–21299
- Guidelines: `spec/12-consolidated-guidelines/13-app.md`, `16-app-design-system-and-ui.md`, `18-database-conventions.md`

---

## 1. Scope

### In scope
1. List emails for one or all aliases, with pagination (cursor or offset).
2. Full-text search over `Subject`, `FromAddr`, `BodyText` (SQLite `LIKE` for v1; FTS5 deferred).
3. Detail pane: headers (From / To / Cc / Subject / Received), body (text + HTML toggle), file path link, raw `.eml` download.
4. Mark one or many as read / unread (positive boolean `IsRead`).
5. Soft-delete one or many emails locally (sets `DeletedAt`, hides from default list).
6. Refresh-from-IMAP for the active alias (delegates to `core.Watch.PollOnce`).
7. Open URLs found in the body (delegates to `core.Tools.OpenUrl`, logs to `OpenedUrl` table).
8. Filter by `IsRead`, date range, attachment presence (placeholder column for v2).

### Out of scope
- Sending email (this app is read-only on the wire; SMTP support deferred to a future feature).
- Server-side flag sync (mark-read updates local DB only; server flags untouched in v1).
- Threading / conversation grouping (deferred — `MessageId` parent chain analysis is v2).
- Attachment extraction (the `.eml` file is preserved; in-app viewer deferred).

---

## 2. User Stories

| #  | As a … | I want to …                                                              | So that …                                              |
|----|--------|--------------------------------------------------------------------------|--------------------------------------------------------|
| 1  | User   | see all emails for a given alias sorted newest-first                     | I can scan recent activity at a glance                 |
| 2  | User   | search across subject + from + body                                      | I can find a specific notification fast                |
| 3  | User   | open an email and read its full body                                     | I do not need to switch to my real mail client         |
| 4  | User   | toggle between rendered HTML and plain text                              | I can detect tracking or formatting tricks             |
| 5  | User   | mark messages as read in bulk                                            | I can clear noise without opening each one             |
| 6  | User   | delete messages locally                                                  | I can prune the DB without re-fetching everything      |
| 7  | User   | click a URL in the body and have it open in my default browser           | I can act on links without copy/paste                  |
| 8  | User   | hit **Refresh** to pull the latest UIDs from IMAP for the active alias   | I do not have to wait for the next watcher poll        |
| 9  | User   | filter to "unread only"                                                  | I can triage what is new                               |

---

## 3. Dependencies

| Dependency             | Why                                                                |
|------------------------|--------------------------------------------------------------------|
| `core.Emails`          | All read/list/mutate operations                                    |
| `core.Watch.PollOnce`  | Manual refresh delegates here (does not bypass watcher)            |
| `core.Tools.OpenUrl`   | URL clicks logged to `OpenedUrl` table                             |
| `core.Rules`           | (read-only) tag preview — show which rules matched a given email   |
| `internal/store`       | (transitive) `Email`, `OpenedUrl` tables                           |
| `internal/ui/theme`    | Tokens for read/unread, selection, danger (delete confirmation)    |

The view **must not** import `internal/store` or `internal/mailclient` directly.

---

## 4. Data Model (read-only projections)

All names PascalCase (per `04-coding-standards.md` §1.1 and `18-database-conventions.md`).

```go
type EmailSummary struct {
    Id          int64
    Alias       string
    Uid         uint32
    FromAddr    string
    Subject     string
    Snippet     string     // first 140 chars of BodyText
    ReceivedAt  time.Time
    IsRead      bool
    IsDeleted   bool       // soft-delete flag
    HasAttachment bool     // v2 placeholder; always false in v1
    MatchedRules []string  // rule names that fired on this email (read-only)
}

type EmailDetail struct {
    EmailSummary
    ToAddr    string
    CcAddr    string
    BodyText  string
    BodyHtml  string
    FilePath  string       // path to .eml on disk
    MessageId string
}

type EmailQuery struct {
    Alias       string     // empty = all aliases
    Search      string     // empty = no search
    OnlyUnread  bool
    IncludeDeleted bool
    SinceAt     time.Time  // zero = no lower bound
    UntilAt     time.Time  // zero = no upper bound
    Limit       int        // default 50, max 200
    Offset      int        // default 0
    SortBy      EmailSortKey   // ReceivedAtDesc | ReceivedAtAsc | SubjectAsc
}

type EmailPage struct {
    Rows       []EmailSummary
    TotalCount int
    Limit      int
    Offset     int
    HasMore    bool
}

type RefreshReport struct {
    Alias        string
    NewCount     int
    UpdatedCount int
    DurationMs   int
    StartedAt    time.Time
    FinishedAt   time.Time
}

type EmailSortKey string  // PascalCase enum
```

---

## 5. Refresh Behavior

| Trigger                          | Action                                                                            |
|----------------------------------|-----------------------------------------------------------------------------------|
| Tab opened                       | `List` once with last-used `EmailQuery` (persisted to `Settings.LastEmailQuery`)  |
| User clicks **Refresh**          | `core.Watch.PollOnce(alias)` → on success, re-run `List`                          |
| `WatchEvent.Kind == EmailStored` matches active alias | Increment "N new" badge in toolbar; re-list on click       |
| Filter / search input changes    | Debounced 300 ms, then `List`                                                     |
| Pagination next/prev             | `List` with new `Offset`                                                          |
| Bulk mark-read / delete          | Optimistic UI update, then `MarkRead` / `Delete`; rollback on error                |

The Emails view **never** opens an IMAP connection itself — `core.Watch.PollOnce` is the sole entry point.

---

## 6. Acceptance Snapshot (full criteria in `97-acceptance-criteria.md`)

A merged Emails build is shippable iff:

1. `List` p95 ≤ **60 ms** with 100 000 stored emails and a 3-character `Search`.
2. Detail-pane open ≤ **30 ms** (cached headers; body lazy-loaded).
3. Bulk mark-read of 500 messages completes in ≤ **150 ms** (single `UPDATE … WHERE Id IN (...)`).
4. Soft-delete is reversible until app restart (in-memory undo stack, max 1 step).
5. Refresh failure shows the standard error envelope with a **Retry** button — list state preserved.
6. Zero `interface{}` / `any` in any new code (lint-enforced).
7. All toolbar icons via `theme.*Icon()` — no PNG assets shipped for this feature.
8. Mark-read and delete are **idempotent** (re-issuing the same op is a no-op, not a 2× update).

---

## 7. Open Questions

None. Confidence: Production-Ready.

---

**End of `02-emails/00-overview.md`**
