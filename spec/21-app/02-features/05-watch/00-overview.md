# 05 — Watch — Overview

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **Watch** feature is the headline real-time surface of `email-read`. It owns the long-running poll loop that monitors one or more IMAP accounts, the heartbeat invariant that proves it is alive even when no new mail arrives, the cross-feature event bus that fans out `WatchEvent` to UI / Dashboard / Emails / Rules consumers, and the live two-tab view (structured cards + raw log) that lets a user *see* the watcher work.

This feature owns: start/stop lifecycle, poll-interval scheduling, IMAP IDLE upgrade when supported, heartbeat emission, exponential-backoff reconnect, status indicator (idle / watching / error / reconnecting), UID-cursor persistence in `WatchState`, and the **single canonical event channel** every other feature subscribes to.

Cross-references:
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.5
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- **Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.4 — heartbeat invariant 🔴** (every poll cycle MUST emit at least one INFO line, even when zero new mail)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21400–21499` (watcher) + wrapped `21200–21299` (mailclient)
- Sibling features that consume `WatchEvent`: `01-dashboard`, `02-emails`, `03-rules`, `04-accounts`
- Guidelines: `spec/12-consolidated-guidelines/13-app.md`, `16-app-design-system-and-ui.md`
- Regression to never repeat: `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`

---

## 1. Scope

### In scope
1. Start/stop the watcher per `Alias` (multi-account: independent watcher per account).
2. **Poll-or-IDLE mode selection**: try IMAP IDLE first; fall back to fixed-interval polling on `ER-MAIL-21209 ErrMailIdleUnsupported`.
3. Per-cycle **heartbeat** — INFO log line on every successful poll regardless of new-mail count (audit invariant).
4. New-mail detection via `UID > LastSeenUid`; persist new emails (delegated to `core.Emails`); update `WatchState.LastSeenUid`.
5. **Event fan-out**: every `WatchEvent` published once, every subscriber gets a copy via per-subscriber buffered channel (cap 256). Slow subscribers drop oldest + emit `WARN WatchSubscriberSlow`.
6. Status indicator: `Idle` (not started) / `Watching` (healthy) / `Reconnecting` (in backoff) / `Error` (terminal — user must restart).
7. **Exponential backoff** on connection drop: 1s → 2s → 5s → 10s → 30s → 60s (capped). Reset on first successful poll.
8. Clean `Stop`: `LOGOUT` IMAP within 2 s, cancel poll context, drain subscriber channels, close them.
9. `WatchEvent.Kind == RuleMatched` emission for every rule hit (consumed by Rules `BumpStat`).
10. Two-tab UI: **Cards** (structured per-event widgets, capped 200) + **Raw log** (scrolling text, capped 2000 lines, Pause + Clear buttons).
11. Sidebar status dot reflects current `WatchStatus` per active account.
12. Auto-start at app boot if `config.Settings.AutoStartWatch == true` (default false in v1).

### Out of scope
- Distributed coordination (multiple `email-read` processes for the same account). Out of project scope — single-process invariant.
- Push notifications / OS notifications. Deferred to v2.
- Per-account separate poll intervals. Single `config.Settings.PollSeconds` for all accounts in v1.
- Mailbox change events for moves/deletes. v1 only watches new mail (UID > cursor); deletions are out of scope.
- IMAP CONDSTORE / QRESYNC. Deferred to v2.
- `INBOX` is the only watched folder. Multi-folder deferred to v2.

---

## 2. User Stories

| #  | As a … | I want to …                                                          | So that …                                                       |
|----|--------|----------------------------------------------------------------------|-----------------------------------------------------------------|
| 1  | User   | press ▶ Start and immediately see activity                            | I have confidence the watcher is actually running               |
| 2  | User   | see a heartbeat line every poll cycle even when no mail               | I am never lied to by silence                                   |
| 3  | User   | see a card per new email with from / subject / snippet / links        | I read mail without leaving the app                             |
| 4  | User   | click a link in a card and have it open in incognito                   | rule-extracted URLs are auditable & non-tracking                |
| 5  | User   | press ■ Stop and have the watcher quit cleanly within 2 s             | I never have to force-kill the process                          |
| 6  | User   | switch the active alias while watching                                | the watcher swaps to the new account without manual restart     |
| 7  | User   | see status flip to Reconnecting when my Wi-Fi drops                    | I know it is the network, not the app                           |
| 8  | User   | see status flip to Error after backoff exhaustion                     | I know I need to act (re-enter credentials, etc.)               |
| 9  | User   | see a rule-match badge on cards that fired a rule                      | I can verify my automation works end-to-end                     |
| 10 | User   | pause the raw-log scroll while inspecting a line                       | the autoscroll does not steal my reading position               |
| 11 | User   | clear the raw log without restarting the watcher                       | I can isolate output for the next test message                  |
| 12 | User   | open multiple tabs (UI) and have all see the same events               | the dashboard recent-events feed never disagrees with Watch tab |
| 13 | User   | run the same watcher from CLI (`email-read <alias>`) headlessly        | I can run it on a server without the UI                         |

---

## 3. Dependencies

| Dependency             | Why                                                                    |
|------------------------|------------------------------------------------------------------------|
| `core.Watch`           | All start/stop/status/subscribe                                        |
| `internal/watcher`     | (transitive) the actual poll loop / IDLE upgrade / backoff             |
| `internal/mailclient`  | (transitive) IMAP dial, LOGIN, SELECT INBOX, FETCH, IDLE, LOGOUT       |
| `internal/store`       | (transitive) `WatchState` cursor + delegated `Email` insert            |
| `core.Emails`          | Persists new emails (the Watch feature **does not** insert into `Email` directly) |
| `core.Rules`           | `Engine.EvaluateAll` per new email; `BumpStat` on match                |
| `core.Tools.OpenUrl`   | Browser launch for `Action == OpenUrl` matches                         |
| `core.Accounts`        | Reads account list at start; subscribes to `AccountEvent` for live add/remove |
| `internal/eventbus`    | Fan-out publisher → multiple per-subscriber buffered channels          |
| `internal/ui/theme`    | Tokens for status dot colors, card backgrounds, rule-match badge       |

The view **must not** import `internal/watcher`, `internal/mailclient`, `internal/store`, or `internal/eventbus` directly. All access goes through `core.Watch`.

---

## 4. Data Model

All names PascalCase (per `04-coding-standards.md` §1.1).

### 4.1 Core types

```go
type WatchStatus string  // PascalCase enum
const (
    WatchStatusIdle         WatchStatus = "Idle"          // not started
    WatchStatusStarting     WatchStatus = "Starting"      // dialing + LOGIN in progress
    WatchStatusWatching     WatchStatus = "Watching"      // healthy: poll cycles emitting heartbeats
    WatchStatusReconnecting WatchStatus = "Reconnecting"  // in backoff after a drop; will retry
    WatchStatusStopping     WatchStatus = "Stopping"      // LOGOUT + ctx cancel in flight
    WatchStatusError        WatchStatus = "Error"         // terminal (backoff exhausted or auth failed); user must Start again
)

type WatchOptions struct {
    Alias        string         // required; selects account
    Mode         WatchMode      // default Auto = "try IDLE then fall back to Poll"
    PollSeconds  int            // default 3; ignored when Mode == Idle and IDLE supported
    AutoStart    bool           // for boot-time auto-start; not user-facing in v1
}

type WatchMode string
const (
    WatchModeAuto WatchMode = "Auto"   // IDLE if supported, else Poll
    WatchModeIdle WatchMode = "Idle"   // force IDLE; fail if unsupported
    WatchModePoll WatchMode = "Poll"   // force fixed-interval polling
)

type WatchState struct {                // current runtime status (in-memory; per-Alias)
    Alias         string
    Status        WatchStatus
    Mode          WatchMode             // negotiated actual mode (Auto resolves to Idle or Poll)
    StartedAt     time.Time             // zero when Status == Idle
    LastPollAt    time.Time             // zero before first poll
    LastSeenUid   uint32
    BackoffUntil  time.Time             // populated when Status == Reconnecting
    BackoffStep   int                   // 0..5 (1s, 2s, 5s, 10s, 30s, 60s); reset on healthy poll
    LastError     string                // empty when healthy
    LastErrorCode string                // e.g. "ER-MAIL-21208"
}

type WatchEvent struct {                // fan-out payload (the canonical event channel)
    Time      time.Time
    Alias     string
    Kind      WatchEventKind
    Message   string                    // human-readable; mirrors raw-log line
    Email     *EmailSummary             // populated when Kind ∈ {NewMail, RuleMatched}
    Urls      []string                  // populated when Kind == RuleMatched && Action == OpenUrl
    RuleName  string                    // populated when Kind == RuleMatched
    PollStat  *PollStat                 // populated when Kind == PollHeartbeat
    Err       *EventError               // populated when Kind ∈ {ConnectError, RuleEvalError, PersistError}
    TraceId   string                    // matches the structured log TraceId for the same op
}

type WatchEventKind string
const (
    WatchEventKindWatchStart        WatchEventKind = "WatchStart"
    WatchEventKindWatchStop         WatchEventKind = "WatchStop"
    WatchEventKindAccountConnected  WatchEventKind = "AccountConnected"
    WatchEventKindAccountConnectError WatchEventKind = "AccountConnectError"
    WatchEventKindPollHeartbeat     WatchEventKind = "PollHeartbeat"     // every cycle, even idle
    WatchEventKindNewMail           WatchEventKind = "NewMail"
    WatchEventKindRuleMatched       WatchEventKind = "RuleMatched"
    WatchEventKindReconnecting      WatchEventKind = "Reconnecting"
    WatchEventKindReconnectExhausted WatchEventKind = "ReconnectExhausted"  // → Status == Error
    WatchEventKindStatusChanged     WatchEventKind = "StatusChanged"     // emitted on every WatchStatus transition
)

type PollStat struct {                  // mirrors §6.4 of 05-logging-strategy
    MessagesCount int
    UidNext       uint32
    LastUid       uint32
    NewCount      int
    DurationMs    int
}

type EventError struct {
    Code    string                      // e.g. "ER-MAIL-21201"
    Message string                      // user-facing, redacted
}

type EmailSummary struct {              // re-used from Emails feature (do not redefine)
    Id       int64
    Alias    string
    FromAddr string
    Subject  string
    SnippetText string                  // first 160 chars, redacted of secrets
    ReceivedAt time.Time
}
```

### 4.2 Status transitions

Allowed transitions (any other = bug):

```
Idle ─► Starting ─► Watching ─┬─► Stopping ─► Idle
                              ├─► Reconnecting ─► Watching
                              └─► Reconnecting ─► Error (backoff exhausted)
Starting ─► Error              (LOGIN auth failure: ER-MAIL-21201)
Reconnecting ─► Stopping ─► Idle  (user pressed Stop while in backoff)
Error ─► Starting              (user pressed Start again)
```

Every transition emits `WatchEvent{Kind: StatusChanged}` with the previous + new status in `Message`.

### 4.3 Default values

```go
WatchOptions{
    Mode:        WatchModeAuto,
    PollSeconds: config.Settings.PollSeconds,  // default 3
    AutoStart:   config.Settings.AutoStartWatch, // default false
}
```

### 4.4 Persistence shape

The Watch feature persists exactly two things:
1. **`WatchState` SQLite row** per alias — `LastSeenUid`, `LastConnectedAt`, `LastConnectError`, `UpdatedAt`. Owned by Watch but read by Accounts (see `04-accounts/01-backend.md` §3).
2. **Nothing in `config.json`** — runtime `WatchStatus` is in-memory only and lost on app restart (intentional; restart is recovery).

Email data and rule-match audit go through `core.Emails` and `core.Rules.BumpStat` respectively — never written by Watch directly.

---

## 5. Heartbeat Invariant 🔴

**Source:** `05-logging-strategy.md` §6.4 + `.lovable/strictly-avoid.md`.

**Rule:** Every poll cycle MUST emit at least one `INFO` log line AND one `WatchEvent{Kind: PollHeartbeat}` — even when `NewCount == 0`. Silence on healthy idle is a **regression** (see `solved-issues/02-watcher-silent-on-healthy-idle.md`).

**Concrete contract:**
- After every successful FETCH (or IDLE wakeup that revealed no new mail), the watcher publishes:
  ```
  WatchEvent{
      Kind:    PollHeartbeat,
      Message: "poll: messages=N uidNext=M new=K",
      PollStat: &PollStat{MessagesCount: N, UidNext: M, LastUid: L, NewCount: K, DurationMs: D},
  }
  ```
- AND emits the structured log line per §6.4 of `05-logging-strategy.md`.
- The Watch UI's **Raw log** tab is the literal stream of `WatchEvent.Message` strings — heartbeat visible to user.
- The **Cards** tab does NOT render heartbeat events as cards (would be visual noise) but increments a footer counter `"{N} polls"`.

**Test gate:** `Watcher_HeartbeatEmittedEveryCycle_EvenWhenIdle` runs the watcher for 10 cycles against a fake mailbox with zero new mail and asserts `len(heartbeatEvents) == 10`.

---

## 6. Reconnect Backoff

| Step | Wait     | After                                 |
|------|----------|---------------------------------------|
| 0    | 1 s      | first connection drop                 |
| 1    | 2 s      | retry 1 fails                         |
| 2    | 5 s      | retry 2 fails                         |
| 3    | 10 s     | retry 3 fails                         |
| 4    | 30 s     | retry 4 fails                         |
| 5    | 60 s     | retry 5 fails                         |
| 6+   | (none)   | backoff exhausted → `WatchStatusError` + `WatchEvent{Kind: ReconnectExhausted}`; user must press Start again |

**Reset rule:** the backoff counter resets to 0 on the **first successful poll cycle after reconnect** (not on the successful LOGIN — a connection that immediately drops should keep climbing the ladder).

**Wait clock:** `BackoffUntil = clock.Now() + step` and the loop blocks on `select { ctx.Done(): … ; clock.After(step): retry }`. `Stop` cancels the wait immediately.

---

## 7. Refresh & Live-Update

| Trigger                                              | Action                                                      |
|------------------------------------------------------|-------------------------------------------------------------|
| Tab opened                                           | `core.Watch.Status` once + `Subscribe()`                    |
| `WatchEvent{StatusChanged}`                          | Update sidebar dot + status header in Watch view            |
| `WatchEvent{NewMail}`                                | Prepend card to Cards tab; append line to Raw log tab       |
| `WatchEvent{RuleMatched}`                            | Add rule-match badge to the matching card                   |
| `WatchEvent{PollHeartbeat}`                          | Append line to Raw log tab; increment footer poll counter   |
| `WatchEvent{Reconnecting}` / `{ReconnectExhausted}`  | Show banner with backoff countdown / Restart CTA            |
| `AccountEvent{Removed, Alias}` for active alias       | Auto-Stop the watcher for that alias; status → Idle         |
| `AccountEvent{Renamed, PrevAlias, Alias}`            | In-place update of the watcher's stored `Alias`             |
| Tab loses focus                                      | Keep subscription alive (sidebar dot still updates); only the Cards/Raw log views detach their bindings |
| App close                                            | `Stop()` for every active watcher; subscriber channels closed |

Multiple subscribers (Watch view + Dashboard recent-events + CLI) read the same event via per-subscriber fan-out channels — no double polling. A slow subscriber **never** blocks the watcher: events drop with `WARN WatchSubscriberSlow`.

---

## 8. Acceptance Snapshot (full criteria in `97-acceptance-criteria.md`)

A merged Watch build is shippable iff:

1. **Heartbeat invariant**: every poll cycle emits one INFO log AND one `PollHeartbeat` event, even with zero new mail (10-cycle test).
2. Sending a real test email produces a Cards card AND a Raw log line within **5 s** end-to-end.
3. Pressing ■ Stop completes IMAP `LOGOUT` + ctx cancel + channel close within **2 s**.
4. Connection drop triggers `WatchStatusReconnecting` and the documented 1/2/5/10/30/60 s backoff ladder.
5. Backoff exhaustion transitions to `WatchStatusError`; subsequent ▶ Start cleanly resumes.
6. Switching active alias while watching shows confirm strip ("Stop watching {OldAlias} first?").
7. Removing an account while it's being watched auto-stops that watcher within 1 s.
8. Multiple subscribers (Watch + Dashboard + CLI) all receive every event; no subscriber blocks the loop.
9. Slow subscriber drops events with `WARN WatchSubscriberSlow`; the watcher itself stays healthy.
10. Cards capped at 200 (oldest dropped); Raw log capped at 2000 lines; Pause + Clear work without affecting underlying subscription.
11. Rule match emits `WatchEvent{RuleMatched}` AND calls `core.Rules.BumpStat` AND, for `Action == OpenUrl`, calls `core.Tools.OpenUrl` for each extracted URL.
12. Zero `interface{}` / `any` in any new code (lint-enforced).
13. **Single-process invariant**: starting a second watcher for an already-watched alias returns `21401 WatchAlreadyStarted` (no double-poll).

---

## 9. Open Questions

None. Confidence: Production-Ready.

The following are explicit **deferrals**, not ambiguities:
- Per-account separate `PollSeconds` → v2 (single global value sufficient for v1).
- Multi-folder watch (other than INBOX) → v2.
- Push / OS notifications → v2.
- CONDSTORE / QRESYNC IMAP extensions → v2.
- Move/delete event detection → v2 (v1 watches new-mail only via `UID > cursor`).
- Multi-process coordination → out of project scope.

---

**End of `05-watch/00-overview.md`**
