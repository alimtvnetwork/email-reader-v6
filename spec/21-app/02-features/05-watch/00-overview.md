# Feature 05 — Watch

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI + CLI (`email-read <alias>`)

---

## Purpose

The headline UI feature. Two-tab live view of the watcher: structured event cards on one side, raw log stream on the other. Both tabs subscribe to the **same** `core.Event` channel — no double polling.

## User stories

- I press "▶ Start" and watch begins for the selected alias.
- Tab 1 shows a card per new email: from, subject, snippet, extracted links (clickable → incognito), rule-match badge.
- Tab 2 shows the same events as scrolling text — identical to the CLI log.
- I press "■ Stop" and the watcher cancels its context cleanly.
- A status indicator (●) shows idle / watching / error.

## Layout

```
┌─ Watch  alias: [work ▾]  ●watching  [▶ Start] [■ Stop] ┐
│ ┌─Cards─┬─Raw log─────────────────────────────────────┐│
│ │ ▣ noreply@…  Sign in link        rule:open-magic ✓  ││
│ │   "Click below to log in…"                          ││
│ │   [↗ https://app.example.com/auth?token=…]          ││
│ │ ─────────────────────────────────────────────────── ││
│ │ ▣ alice@…    Reschedule          (no match)         ││
│ │   …                                                 ││
│ └──────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────┘
```

Tabs share the event stream. Cards capped at 200 (oldest dropped). Raw log capped at 2000 lines with "Pause" + "Clear" buttons.

## Backend (core API)

Watcher already exists in `internal/watcher`. Required additions in `internal/core/watch.go`:

```go
type Event struct {
    Time     time.Time
    Alias    string
    Kind     EventKind   // poll | new-mail | rule-hit | error | idle
    Message  string
    Email    *EmailSummary  // for new-mail / rule-hit
    URLs     []string
    RuleName string
}

func StartWatch(ctx context.Context, alias string) (<-chan Event, error)
func StopWatch(alias string) error
func IsWatching(alias string) bool
```

The CLI subscriber formats events with the existing readable formatter. The UI subscribes to the same channel via a fan-out so multiple views (Watch + Dashboard recent-events) can read without dropping events.

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-W1 | Sending a test email produces a card AND a raw log line within 5s. |
| AC-W2 | "■ Stop" causes the IMAP LOGOUT to complete within 2s and status flips to idle. |
| AC-W3 | Switching alias while watching shows a confirm strip ("Stop watching <old> first?"). |
| AC-W4 | Heartbeat poll lines (`messages=N uidNext=M`) appear in raw log even when no new mail. |
| AC-W5 | Clicking a link inside a card opens Chrome incognito and records to `OpenedUrls` with `RuleName = "manual"` (matches Emails feature). |
| AC-W6 | If the IMAP connection drops, status flips to `error`, an error card appears, and the watcher auto-retries with backoff (1s, 2s, 5s, 10s, capped). |
