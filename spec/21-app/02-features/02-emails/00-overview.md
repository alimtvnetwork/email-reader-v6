# Feature 02 — Emails

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI (CLI has limited `read` only)

---

## Purpose

Browse persisted emails for the selected alias. Read a message in full, see extracted links, and re-open any link in Chrome incognito on demand.

## User stories

- I pick an alias from the sidebar, then see every email it has received, newest first.
- I click an email and read its decoded body inline (no popup).
- Links extracted from the body appear as clickable buttons → Chrome incognito.
- I can search by subject or sender.

## Layout (split pane inside the right detail area)

```
┌─ Emails ─────────────────────────────────────────┐
│ Search: [____________________]                   │
│ ┌──────────────────────┬────────────────────────┐│
│ │ uid  from  subject   │  Subject: Sign in link ││
│ │ 412  noreply...      │  From:    noreply@...  ││
│ │ 411  alice@... resched│  Date:    10:42 today  ││
│ │ 410  ...             │                        ││
│ │                      │  [body — decoded text] ││
│ │                      │                        ││
│ │                      │  Links                 ││
│ │                      │  [↗ https://app.exa…]  ││
│ │                      │  [↗ https://other…]    ││
│ └──────────────────────┴────────────────────────┘│
└───────────────────────────────────────────────────┘
```

## Backend (core API)

`internal/core/emails.go` — already exists. Required surface:

```go
func ListEmails(ctx context.Context, alias string, query string, limit int) ([]EmailSummary, error)
func GetEmail(ctx context.Context, id int64) (Email, error)
func ExtractLinks(body string) []string
func OpenLink(ctx context.Context, emailId int64, url string) error  // delegates to internal/browser
```

`OpenLink` MUST record the open in `OpenedUrls` (same dedup ledger the watcher uses).

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-E1 | Email list virtualizes — 50k rows scroll smoothly. |
| AC-E2 | Body renders plain text first; HTML stripped to text fallback if no plain part. |
| AC-E3 | Search filters list within 200ms of last keystroke (debounced). |
| AC-E4 | Link buttons label = full URL truncated to 60 chars; tooltip = full URL. |
| AC-E5 | Manually opened link records to `OpenedUrls` with `RuleName = "manual"`. |
| AC-E6 | Switching alias clears the detail pane to a placeholder ("Select an email"). |

## Out of scope

- Reply / forward / compose — never (read-only).
- Attachment download — defer.
- Threaded conversation view — defer.
