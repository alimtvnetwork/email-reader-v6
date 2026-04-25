# 05 — Watch — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **`internal/core` API surface, `internal/watcher` poll-loop architecture, IDLE-or-Poll mode negotiation, heartbeat enforcement, exponential-backoff state machine, fan-out event bus, persistence model, queries, error codes, and logging** for the Watch feature.

This is the contract `internal/ui/views/watch.go` and `internal/cli/watch.go` consume; nothing else may bypass it. In particular, the UI MUST NOT import `internal/watcher`, `internal/mailclient`, `internal/store`, or `internal/eventbus` directly.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.5
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- **Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.4 — heartbeat invariant 🔴**
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21400–21499` (existing: 21401–21405; new in this spec: 21406–21430)
- Cross-feature contracts:
  - Emails: [`../02-emails/01-backend.md`](../02-emails/01-backend.md) — `core.Emails.PersistFromImap` (delegated insert)
  - Rules: [`../03-rules/01-backend.md`](../03-rules/01-backend.md) — `core.Rules.BumpStat` + `Engine.EvaluateAll`
  - Accounts: [`../04-accounts/01-backend.md`](../04-accounts/01-backend.md) — `WatchState` table owner declaration
  - Tools: [`../06-tools/01-backend.md`](../06-tools/01-backend.md) — `core.Tools.OpenUrl` (forward ref; authored later)
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`
- Regression to never repeat: `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`

---

## 1. Service Definition

```go
// Package core — file: internal/core/watch.go
package core

type Watch struct {
    accounts *Accounts             // for account list + AccountEvent subscription
    emails   *Emails               // for PersistFromImap (delegated email insert)
    rules    *Rules                // for BumpStat + Engine.EvaluateAll
    tools    *Tools                // for OpenUrl (rule-action fan-out)
    store    store.Store           // for WatchState row reads/writes (delegated cursor mgmt)
    loop     watcher.LoopFactory   // wraps internal/watcher.New — testable seam
    bus      eventbus.Publisher    // emits WatchEvent via per-subscriber fan-out
    settings *Settings             // for PollSeconds, AutoStartWatch
    clock    Clock

    mu       sync.RWMutex          // guards the runners map
    runners  map[string]*watcher.Loop  // one per active Alias
}

func NewWatch(
    accounts *Accounts,
    emails *Emails,
    rules *Rules,
    tools *Tools,
    s store.Store,
    lf watcher.LoopFactory,
    b eventbus.Publisher,
    settings *Settings,
    c Clock,
) *Watch {
    return &Watch{
        accounts: accounts, emails: emails, rules: rules, tools: tools,
        store: s, loop: lf, bus: b, settings: settings, clock: c,
        runners: make(map[string]*watcher.Loop),
    }
}
```

**Constraints (per `04-coding-standards.md`):**
- All methods take `ctx context.Context` first.
- All methods return `errtrace.Result[T]`.
- No method body > 15 lines.
- No package-level state.
- `interface{}` / `any` banned.
- Constructor receives interfaces (`store.Store`, `watcher.LoopFactory`, `eventbus.Publisher`) — never concretes — for testability.
- The `runners` map is the **single source of truth** for which aliases are being watched. Guarded by `mu` (RWMutex). Never iterated under write-lock to avoid deadlock with `Stop` callbacks.

---

## 2. Public Methods

### 2.1 `Start`

```go
func (w *Watch) Start(ctx context.Context, opts WatchOptions) errtrace.Result[Unit]
```

**Pipeline:**
1. Validate `opts.Alias` non-empty (`21410 WatchAliasRequired`).
2. Look up account via `accounts.Get(ctx, opts.Alias)`; reject `21703 AccountNotFound` (forwarded).
3. Acquire `mu.Lock`; check `runners[opts.Alias]`; if present return `21401 WatchAlreadyStarted`. Release lock as soon as the entry is reserved (placeholder).
4. Decrypt `PasswordB64` → password (held in stack, zeroed on defer).
5. Construct `watcher.Loop` via `loop.New(account, opts, deps)`; store in `runners[opts.Alias]`.
6. Spawn goroutine `go runner.Run(loopCtx)` where `loopCtx` is a child of `ctx` cancellable independently.
7. Publish `WatchEvent{Kind: WatchStart, Alias, Message: "watch started"}` via `bus`.
8. Return `Unit{}`.

**Failure paths:**
- `21401 WatchAlreadyStarted` — second Start for same alias.
- `21402 WatchStartLoopFailed` (was `ER-WCH-21401 ErrWatcherStart`) — Loop.New returned error (e.g. mailclient dial failed in eager-validate mode).
- `21703 AccountNotFound` — forwarded from `accounts.Get`.

**Note:** `Start` is non-blocking — it spawns the loop goroutine and returns immediately. Subsequent connection failures appear as `WatchEvent{Kind: AccountConnectError}` + `StatusChanged → Reconnecting`, never as a `Start` return error.

### 2.2 `Stop`

```go
func (w *Watch) Stop(ctx context.Context, alias string) errtrace.Result[Unit]
```

**Pipeline:**
1. Acquire `mu.Lock`; pop `runners[alias]`; release lock. If not found return `21411 WatchNotStarted` (NOT an error if `ctx`-flagged as best-effort — see §2.3 `StopAll`).
2. Publish `WatchEvent{Kind: StatusChanged, Status: Stopping}`.
3. Call `runner.Stop(stopCtx)` with `stopCtx` deadline = **2 s** (per heartbeat invariant: `LOGOUT` must complete fast).
4. On runner exit: publish `WatchEvent{Kind: WatchStop}` and `StatusChanged → Idle`.
5. On `stopCtx` deadline elapsed: log `WARN WatcherShutdownSlow`, force-cancel runner context, publish `21405 ErrWatcherShutdown` envelope, return success anyway (Stop is best-effort: a stuck IMAP server cannot block the user).

### 2.3 `StopAll`

```go
func (w *Watch) StopAll(ctx context.Context) errtrace.Result[Unit]
```

**Behavior:** Snapshots `runners` keys under `mu.RLock`, releases, then calls `Stop` on each in parallel via a `errgroup.Group` with `ctx` as parent. Total deadline = **5 s** regardless of account count. Used by `OnAppQuit`.

### 2.4 `Status`

```go
func (w *Watch) Status(ctx context.Context, alias string) errtrace.Result[WatchState]
```

Returns the current in-memory `WatchState` (per §4.1 of overview). If alias not in `runners`, returns `WatchState{Status: Idle, Alias: alias}` (NOT an error — Idle is a valid state for any alias).

### 2.5 `StatusAll`

```go
func (w *Watch) StatusAll(ctx context.Context) errtrace.Result[[]WatchState]
```

Returns one `WatchState` per known account (joins `accounts.List` with `runners` map). Used by sidebar + dashboard.

### 2.6 `Subscribe`

```go
func (w *Watch) Subscribe(ctx context.Context) errtrace.Result[<-chan WatchEvent]
```

**Behavior:** Registers a new subscriber on `bus`. Returns a buffered channel (cap **256**). When `ctx` is cancelled, subscriber is unregistered + channel closed. Multiple concurrent subscribers receive every event (fan-out via `bus.Publish`). Slow subscribers drop oldest event + emit `WARN WatchSubscriberSlow` (`21404 ErrWatcherEventPublish`).

### 2.7 `RestartOnAccountUpdate` (internal — invoked by `core.Accounts` event subscription)

```go
func (w *Watch) RestartOnAccountUpdate(ctx context.Context, alias string) errtrace.Result[Unit]
```

**Behavior:** Called when `AccountEvent{Updated, Alias}` fires for an actively-watched account (e.g. user changed Host or Password). Performs `Stop` → wait → `Start` with same `WatchOptions`. Idempotent if account is not currently being watched (no-op).

### 2.8 `OnAccountRemoved` (internal — invoked by `core.Accounts` event subscription)

```go
func (w *Watch) OnAccountRemoved(ctx context.Context, alias string) errtrace.Result[Unit]
```

Calls `Stop(alias)` if running. No-op otherwise. Logs `INFO WatcherAutoStoppedDueToAccountRemoval`.

---

## 3. The Poll Loop (`internal/watcher`)

The actual loop lives in `internal/watcher/loop.go`. `core.Watch` owns lifecycle; the loop owns the cycle-by-cycle behavior. This section is the canonical pseudocode.

### 3.1 Loop entry point

```go
// File: internal/watcher/loop.go
package watcher

type Loop struct {
    account     core.Account
    opts        core.WatchOptions
    mailClient  mailclient.Client    // per-loop dialer
    deps        Deps                 // emails, rules, tools, store, bus, clock
    state       core.WatchState      // mutable, guarded by stateMu
    stateMu     sync.RWMutex
    cycleCount  int                  // increments per pollOnce or IDLE wakeup
    backoffStep int                  // 0..5, see §6 of overview
    cancelOnce  sync.Once
}

func (l *Loop) Run(ctx context.Context) {
    defer l.cleanup()
    l.transition(WatchStatusStarting)

    for {
        if err := l.connect(ctx); err != nil {
            if l.handleConnectError(ctx, err) { continue }   // backoff or exhausted
            return                                           // ctx cancelled or auth-error terminal
        }
        l.backoffStep = 0
        l.transition(WatchStatusWatching)
        l.publishConnected()

        if l.useIdle() {
            l.runIdleLoop(ctx)    // returns on disconnect or ctx
        } else {
            l.runPollLoop(ctx)    // returns on disconnect or ctx
        }

        // disconnect: try to reconnect via the outer for-loop unless ctx done
        if ctx.Err() != nil { return }
    }
}
```

### 3.2 Poll cycle (the heartbeat-emitting hot path 🔴)

```go
func (l *Loop) pollOnce(ctx context.Context) error {
    cycleStart := l.deps.Clock.Now()
    l.cycleCount++
    traceId := newTraceId()

    log.Debug("watcher.pollOnce.start", "Alias", l.account.Alias, "Cycle", l.cycleCount, "TraceId", traceId)

    stat, newUids, err := l.mailClient.FetchSinceUid(ctx, l.state.LastSeenUid)
    if err != nil { return errtrace.Wrap(err, "Watch.pollOnce") }

    for _, uid := range newUids {
        if err := l.processEmail(ctx, uid, traceId); err != nil {
            log.Warn("watcher.processEmail", "Uid", uid, "ErrCode", "ER-WCH-21403", "ErrMessage", err.Error())
            // continue: one bad email never blocks the cycle
        }
    }

    l.updateLastSeenUid(ctx, stat.UidNext)
    l.emitHeartbeat(ctx, stat, len(newUids), int(l.deps.Clock.Now().Sub(cycleStart).Milliseconds()), traceId)
    return nil
}
```

`emitHeartbeat` (the **invariant enforcer** 🔴):

```go
func (l *Loop) emitHeartbeat(ctx context.Context, stat mailclient.MailboxStat, newCount, durMs int, traceId string) {
    msg := fmt.Sprintf("poll: messages=%d uidNext=%d new=%d", stat.MessagesCount, stat.UidNext, newCount)
    log.Info("watcher.pollOnce", "Alias", l.account.Alias,
        "MessagesCount", stat.MessagesCount, "UidNext", stat.UidNext,
        "LastUid", l.state.LastSeenUid, "NewCount", newCount,
        "DurationMs", durMs, "TraceId", traceId, "Msg", msg)
    l.deps.Bus.Publish(core.WatchEvent{
        Time: l.deps.Clock.Now(), Alias: l.account.Alias,
        Kind: core.WatchEventKindPollHeartbeat, Message: msg,
        PollStat: &core.PollStat{
            MessagesCount: stat.MessagesCount, UidNext: stat.UidNext,
            LastUid: l.state.LastSeenUid, NewCount: newCount, DurationMs: durMs,
        }, TraceId: traceId,
    })
}
```

**Invariant enforcement:** `emitHeartbeat` is called at the END of `pollOnce` regardless of `newCount`. The 10-cycle test (`Watcher_HeartbeatEmittedEveryCycle_EvenWhenIdle`) asserts `len(heartbeats) == 10` after 10 idle cycles.

### 3.3 IDLE loop

```go
func (l *Loop) runIdleLoop(ctx context.Context) {
    if err := l.mailClient.IdleStart(ctx); err != nil {
        if errors.Is(err, mailclient.ErrIdleUnsupported) {
            log.Info("watcher.idle.unsupported.fallback", "Alias", l.account.Alias)
            l.opts.Mode = core.WatchModePoll
            l.runPollLoop(ctx)
            return
        }
        // other IDLE error → treat as disconnect
        return
    }
    defer l.mailClient.IdleStop(context.Background())   // best-effort

    // Even in IDLE we must heartbeat: emit one every IdleHeartbeatInterval (default 30s).
    heartbeat := l.deps.Clock.NewTicker(IdleHeartbeatInterval)
    defer heartbeat.Stop()

    for {
        select {
        case <-ctx.Done(): return
        case <-l.mailClient.IdleNotify():
            if err := l.pollOnce(ctx); err != nil { return }   // disconnect or fatal
        case <-heartbeat.C():
            if err := l.pollOnce(ctx); err != nil { return }   // periodic heartbeat
        }
    }
}
```

**`IdleHeartbeatInterval = 30 * time.Second`** — even in IDLE mode (where the server pushes notifications), the watcher MUST emit a heartbeat every 30 s. Silence is the regression.

### 3.4 Poll loop (when IDLE unavailable)

```go
func (l *Loop) runPollLoop(ctx context.Context) {
    interval := time.Duration(l.opts.PollSeconds) * time.Second
    ticker := l.deps.Clock.NewTicker(interval)
    defer ticker.Stop()

    if err := l.pollOnce(ctx); err != nil { return }   // fire one immediately on connect
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C():
            if err := l.pollOnce(ctx); err != nil { return }
        }
    }
}
```

### 3.5 Disconnect / backoff state machine

```go
func (l *Loop) handleConnectError(ctx context.Context, err error) bool {
    code := errCodeOf(err)   // e.g. "ER-MAIL-21201" for auth fail
    if code == "ER-MAIL-21201" {
        // Auth failure is terminal — no backoff, user must fix credentials.
        l.transition(WatchStatusError)
        l.publishError(code, "Authentication failed — check your password in Accounts.")
        return false
    }

    l.transition(WatchStatusReconnecting)
    if l.backoffStep >= len(BackoffLadder) {
        l.transition(WatchStatusError)
        l.deps.Bus.Publish(WatchEvent{Kind: WatchEventKindReconnectExhausted, ...})
        return false
    }
    wait := BackoffLadder[l.backoffStep]
    l.backoffStep++

    l.publishReconnecting(wait, code, err.Error())

    select {
    case <-ctx.Done(): return false
    case <-l.deps.Clock.After(wait): return true
    }
}

var BackoffLadder = []time.Duration{1*time.Second, 2*time.Second, 5*time.Second, 10*time.Second, 30*time.Second, 60*time.Second}
```

### 3.6 Process one new email

```go
func (l *Loop) processEmail(ctx context.Context, uid uint32, traceId string) error {
    msg, err := l.mailClient.FetchOne(ctx, uid)
    if err != nil { return errtrace.Wrap(err, "Watch.processEmail.fetch") }

    summary, err := l.deps.Emails.PersistFromImap(ctx, l.account.Alias, msg)
    if err != nil { return errtrace.Wrap(err, "Watch.processEmail.persist") }

    l.deps.Bus.Publish(WatchEvent{Kind: NewMail, Email: &summary, ...})

    matches := l.deps.Rules.Engine.EvaluateAll(summary)
    for _, m := range matches {
        l.deps.Rules.BumpStat(ctx, m.RuleName, l.deps.Clock.Now())
        l.deps.Bus.Publish(WatchEvent{Kind: RuleMatched, Email: &summary, RuleName: m.RuleName, Urls: m.ExtractedUrls, ...})
        if m.Action == RuleActionOpenUrl {
            for _, u := range m.ExtractedUrls {
                l.deps.Tools.OpenUrl(ctx, u)   // best-effort; logged on failure
            }
        }
    }
    return nil
}
```

`processEmail` errors are **never fatal** to the cycle — the loop logs `WARN ER-WCH-21403` and continues with the next UID. A poison email cannot stop the watcher.

---

## 4. SQL Schema

The `WatchState` table is created in migration `M0003_CreateWatchState`. **Owned by Watch feature** (declared here, referenced by Accounts).

```sql
CREATE TABLE IF NOT EXISTS WatchState (
    Alias            TEXT     PRIMARY KEY,
    LastSeenUid      INTEGER  NOT NULL DEFAULT 0,
    LastConnectedAt  DATETIME,
    LastConnectError TEXT     NOT NULL DEFAULT '',
    UpdatedAt        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS IxWatchStateUpdatedAt ON WatchState(UpdatedAt DESC);
```

All identifiers PascalCase. Singular table name. Positive booleans (none here). (Per `18-database-conventions.md`.)

---

## 5. Queries

### 5.1 Read cursor at connect

```sql
SELECT LastSeenUid FROM WatchState WHERE Alias = ?;
```

### 5.2 Update cursor after every poll

```sql
UPDATE WatchState
SET    LastSeenUid = ?, LastConnectedAt = ?, LastConnectError = '', UpdatedAt = ?
WHERE  Alias = ?;
```

### 5.3 Record connect error (during reconnect)

```sql
UPDATE WatchState
SET    LastConnectError = ?, UpdatedAt = ?
WHERE  Alias = ?;
```

`Email` table writes are **delegated** to `core.Emails.PersistFromImap` — the watcher does not issue raw `INSERT INTO Email` statements. `RuleStat` writes are delegated to `core.Rules.BumpStat`. `OpenedUrl` writes are delegated to `core.Tools.OpenUrl`.

---

## 6. Validation Rules

| Field        | Rule                                                                            | Error code |
|--------------|---------------------------------------------------------------------------------|------------|
| `Alias`      | non-empty; must exist in `core.Accounts.List`                                   | `21410 WatchAliasRequired` / `21703 AccountNotFound` (forwarded) |
| `PollSeconds`| 1..60                                                                            | `21412 WatchPollSecondsOutOfRange` |
| `Mode`       | one of `Auto` / `Idle` / `Poll`                                                  | `21413 WatchModeInvalid` |
| Concurrent Start | refused if alias already in `runners`                                        | `21401 WatchAlreadyStarted` |

Validation is **synchronous** in `Start`. Network validation happens inside the loop (failure surfaces as `WatchEvent{AccountConnectError}`, never as a `Start` return error).

---

## 7. Atomicity & Safety

The Watch feature has **no cross-storage atomic operations** — `WatchState` is the only table it writes, and writes are single-row UPDATEs.

However, three concurrency invariants MUST hold:

| # | Invariant                                                                   | Mechanism                                       |
|---|------------------------------------------------------------------------------|------------------------------------------------|
| 1 | At most one `*Loop` per `Alias` exists in `runners` at any time              | `mu` RWMutex around `runners` map mutation     |
| 2 | `Stop` is non-blocking from the user's perspective even if IMAP hangs        | 2 s deadline + force-cancel runner context     |
| 3 | A panic in any subscriber's goroutine cannot crash the watcher loop          | `bus.Publish` recovers per-subscriber panics + emits `ERROR WatcherSubscriberPanic` |

**Goroutine accounting:** every `Start` spawns exactly one goroutine; every `Stop` (or `ctx` cancel) joins it. The test `Watcher_NoGoroutineLeakAfter100StartStop` asserts `runtime.NumGoroutine` returns to baseline ± 2 after 100 cycles.

---

## 8. Error Codes (registry §21400–21499)

| Code  | Name                              | Layer    | Recovery                                        |
|-------|-----------------------------------|----------|-------------------------------------------------|
| 21401 | `WatchAlreadyStarted`             | core     | UI shows toast "Already watching {Alias}"       |
| 21402 | `WatcherStartLoopFailed` *(was `ErrWatcherStart`)* | core | Show error envelope; user retries Start         |
| 21403 | `WatcherProcessEmail`             | watcher  | WARN log only; cycle continues (poison-email guard) |
| 21404 | `WatcherEventPublish`             | watcher  | WARN log only; subscriber gets dropped event    |
| 21405 | `WatcherShutdown`                 | watcher  | WARN log only; force-cancel runner; Stop returns success |
| 21406 | `WatcherCycleFailed` *(was `ErrWatcherPollCycle`)* | watcher | Trigger backoff state machine        |
| 21410 | `WatchAliasRequired`              | core     | Caller bug — log WARN                           |
| 21411 | `WatchNotStarted`                 | core     | Idempotent Stop; return success                 |
| 21412 | `WatchPollSecondsOutOfRange`      | core     | Caller bug — log WARN                           |
| 21413 | `WatchModeInvalid`                | core     | Caller bug — log WARN                           |
| 21414 | `WatcherIdleStartFailed`          | watcher  | Auto-fallback to Poll mode + INFO log           |
| 21415 | `WatcherIdleNotifyTimeout`        | watcher  | Treat as disconnect; backoff                    |
| 21416 | `WatcherCursorReadFailed`         | watcher  | ERROR log; loop exits to backoff                |
| 21417 | `WatcherCursorUpdateFailed`       | watcher  | WARN log; cycle continues (cursor recovers next cycle) |
| 21418 | `WatcherReconnectExhausted`       | watcher  | Status → Error; user must Start again           |
| 21419 | `WatcherAuthFailedTerminal`       | watcher  | Status → Error; toast "Auth failed for {Alias}" |
| 21420 | `WatcherSubscriberSlow`           | bus      | WARN log; drop oldest event for that subscriber |
| 21421 | `WatcherSubscriberPanic`          | bus      | ERROR log; subscriber removed; loop unaffected  |
| 21422 | `WatcherShutdownSlow`             | watcher  | WARN log; force-cancel after 2 s deadline       |
| 21423 | `WatcherHeartbeatMissed`          | core     | ERROR log + auto-restart loop (defensive — should never fire if invariant holds) |

Wrapped underlying errors (surfaced inside the envelope, not as the top-level code):

| Wrapped code | Source             | When                                              |
|--------------|--------------------|---------------------------------------------------|
| `ER-MAIL-21200 ErrMailDial`         | mailclient | TCP dial fails on connect/reconnect              |
| `ER-MAIL-21201 ErrMailLoginFailed`  | mailclient | LOGIN rejected → `21419` (terminal)              |
| `ER-MAIL-21202 ErrMailSelectMailbox` | mailclient | SELECT INBOX fails                              |
| `ER-MAIL-21203 ErrMailFetchUid`     | mailclient | FETCH UID fails → `21403` per-email or `21406` cycle |
| `ER-MAIL-21206 ErrMailLogout`       | mailclient | LOGOUT during Stop fails → `21405`              |
| `ER-MAIL-21208 ErrMailTimeout`      | mailclient | Any IMAP op exceeds deadline                     |
| `ER-MAIL-21209 ErrMailIdleUnsupported` | mailclient | IDLE start fails → auto-fallback (`21414` INFO) |
| `ER-STO-21104 ErrStoreUpdateWatchState` | store  | Cursor update fails → `21417`                    |

Every error wrapped with `errtrace.Wrap(err, "Watch.<Method>")` or `errtrace.Wrap(err, "watcher.<Phase>")`.

---

## 9. Logging

Per `05-logging-strategy.md` §6.4. PascalCase keys.

| Level | Event                          | Fields                                                                           |
|-------|--------------------------------|----------------------------------------------------------------------------------|
| INFO  | `WatchStart`                   | `Alias`, `Mode`, `PollSeconds`, `TraceId`                                        |
| INFO  | `WatchStop`                    | `Alias`, `DurationMs`, `CyclesCompleted`, `TraceId`                              |
| INFO  | `WatcherConnected`             | `Alias`, `Host`, `Port`, `UseTls`, `IdleSupported`, `LatencyMs`, `TraceId`       |
| DEBUG | `WatcherPollStart`             | `Alias`, `Cycle`, `TraceId`                                                      |
| **INFO** 🔴 | `WatcherPoll` *(heartbeat)* | `Alias`, `MessagesCount`, `UidNext`, `LastUid`, `NewCount`, `DurationMs`, `TraceId`, `Msg="poll: messages=N uidNext=M new=K"` |
| INFO  | `WatcherIdleEntered`           | `Alias`, `IdleHeartbeatInterval`, `TraceId`                                      |
| INFO  | `WatcherIdleNotify`            | `Alias`, `TraceId`                                                               |
| INFO  | `WatcherIdleUnsupportedFallback` | `Alias`, `TraceId`                                                             |
| INFO  | `WatcherStatusChanged`         | `Alias`, `From`, `To`, `Reason?`, `TraceId`                                      |
| WARN  | `WatcherReconnecting`          | `Alias`, `BackoffStep`, `WaitSeconds`, `LastErrorCode`, `TraceId`                |
| ERROR | `WatcherReconnectExhausted`    | `Alias`, `LastErrorCode`, `TotalRetries`, `TraceId`                              |
| ERROR | `WatcherAuthFailedTerminal`    | `Alias`, `ErrorCode=ER-MAIL-21201`, `TraceId`                                    |
| WARN  | `WatcherProcessEmail`          | `Alias`, `Uid`, `ErrorCode`, `ErrorMessage`, `TraceId`                           |
| WARN  | `WatcherCursorUpdateFailed`    | `Alias`, `LastSeenUid`, `ErrorMessage`, `TraceId`                                |
| ERROR | `WatcherCycleFailed`           | `Alias`, `Cycle`, `ErrorCode`, `ErrFrames`, `TraceId`                            |
| WARN  | `WatcherEventPublish`          | `EventKind`, `Alias`, `SubscriberId`, `DroppedCount`                             |
| ERROR | `WatcherSubscriberPanic`       | `EventKind`, `SubscriberId`, `PanicMessage`, `Stack`                             |
| WARN  | `WatcherShutdownSlow`          | `Alias`, `DurationMs`, `Threshold=2000`                                          |
| INFO  | `WatcherAutoStoppedDueToAccountRemoval` | `Alias`, `TraceId`                                                      |
| ERROR | `WatcherHeartbeatMissed`       | `Alias`, `LastHeartbeatAgo`, `Cycle`, `TraceId`                                  |

**PII redaction (enforced by `errtrace` allow-list):**
- Email body, subject, and from-address are **never** in watcher logs (they are in `core.Emails` logs only, and even there only via `EmailSummary.SnippetText` which is pre-redacted).
- Watcher logs only structural counts and UIDs, never email content.
- Password is never logged (forwarded redaction from Accounts).
- The redaction allow-list test (`internal/errtrace/redaction_test.go`) MUST include a case asserting that running 5 watcher cycles with a fixture mailbox containing "secret-token-XYZ" in a subject produces zero log lines containing "secret-token-XYZ".

---

## 10. Performance Budgets

| Operation                                        | Budget     | Source                                              |
|--------------------------------------------------|------------|-----------------------------------------------------|
| `pollOnce` end-to-end (no new mail, 1k mailbox)  | ≤ 200 ms   | mailclient FETCH (UIDs only) + heartbeat emission   |
| `pollOnce` per-new-email overhead                | ≤ 50 ms    | FETCH BODY + persist + rule eval                    |
| `Stop` end-to-end                                | ≤ 2000 ms  | LOGOUT + ctx cancel + channel close                 |
| `Subscribe` channel cap                          | 256 events | per-subscriber; oldest dropped on overflow          |
| Event publish to all subscribers (10 subs, 1KB event) | ≤ 1 ms | fan-out via per-subscriber buffered chan            |
| `IdleHeartbeatInterval`                          | 30 s       | mandatory heartbeat even in IDLE                    |
| Poll-mode default interval                       | 3 s        | `config.Settings.PollSeconds`                       |
| Backoff ladder total (6 steps exhausted)         | 108 s      | 1+2+5+10+30+60                                      |

---

## 11. Testing Contract

File: `internal/core/watch_test.go` + `internal/watcher/loop_test.go`. Target ≥ 90 % coverage across both.

### 11.1 Required test cases (core)

1. `Start_ValidAlias_RegistersRunner_PublishesWatchStart`.
2. `Start_AlreadyStarted_ReturnsErr21401`.
3. `Start_UnknownAlias_ReturnsForwardedErr21703`.
4. `Stop_NotStarted_ReturnsErr21411_TreatedAsSuccessByStopAll`.
5. `Stop_RunnerHangs_ForceCancelAfter2s_LogsWarn21422`.
6. `StopAll_TenAccounts_CompletesWithin5s`.
7. `Status_NotStarted_ReturnsIdle_NoError`.
8. `StatusAll_JoinsAccountsWithRunners`.
9. `Subscribe_DeliversAllEventsToAllSubscribers`.
10. `Subscribe_SlowSubscriber_DropsOldest_LogsWarn21420`.
11. `Subscribe_SubscriberPanic_RemovedFromBus_LoopUnaffected`.
12. `Subscribe_CtxCancel_ChannelClosed_NoLeak`.
13. `RestartOnAccountUpdate_StopAndStart_PreservesLastSeenUid`.
14. `OnAccountRemoved_AutoStops_LogsInfo`.

### 11.2 Required test cases (loop)

15. **`Watcher_HeartbeatEmittedEveryCycle_EvenWhenIdle`** 🔴 — runs 10 cycles against fake mailbox with zero new mail; asserts `len(heartbeats) == 10` AND `len(infoLogsWithMsgPoll) == 10`. **THE invariant test.**
16. `Watcher_PollMode_FiresImmediatelyOnConnect_ThenAtInterval`.
17. `Watcher_IdleMode_HeartbeatEvery30s_EvenWithoutNotify` — fake clock advances 30s; assert heartbeat emitted.
18. `Watcher_IdleUnsupported_FallsBackToPoll_LogsInfo`.
19. `Watcher_NewMail_PersistsViaCoreEmails_PublishesNewMailEvent`.
20. `Watcher_RuleMatch_BumpsStat_PublishesRuleMatchedEvent`.
21. `Watcher_RuleMatch_OpenUrl_CallsToolsOpenUrlPerExtractedUrl`.
22. `Watcher_PoisonEmail_LogsWarn21403_CycleContinues_NextEmailProcessed`.
23. `Watcher_ConnectError_TriggersBackoffLadder_1_2_5_10_30_60` — fake clock; assert each step waited.
24. `Watcher_BackoffReset_OnFirstSuccessfulPollAfterReconnect`.
25. `Watcher_BackoffExhaustion_TransitionsToError_PublishesReconnectExhausted`.
26. `Watcher_AuthFail_Terminal_NoBackoff_TransitionsToError_LogsErr21419`.
27. `Watcher_StatusChanged_PublishedOnEveryTransition_WithFromAndTo`.
28. `Watcher_NoGoroutineLeakAfter100StartStop` — `runtime.NumGoroutine` baseline ± 2.
29. `Watcher_CursorUpdateFailed_LogsWarn21417_CycleContinues`.
30. `Watcher_RedactionTest_NoEmailContentInLogs` — fixture with "secret-token-XYZ" in subject; 5 cycles; assert zero log lines contain that string.

Fakes:
- `core.FakeAccounts`, `core.FakeEmails`, `core.FakeRules`, `core.FakeTools`.
- `mailclient.FakeClient` (scriptable `FetchSinceUid` / `IdleNotify` / `LOGIN` outcomes).
- `eventbus.NewMemory()`.
- `core.FakeClock` (manual tick advance).
- `store.NewMemory()`.

---

## 12. Compliance Checklist

- [x] All identifiers PascalCase.
- [x] Methods use `errtrace.Result[T]`.
- [x] Constructor injects interfaces (8 deps via `NewWatch`).
- [x] No `any` / `interface{}`.
- [x] No `os.Exit`, no `fmt.Print*`.
- [x] All SQL uses singular PascalCase table names (`WatchState`).
- [x] **Heartbeat invariant 🔴** — `emitHeartbeat` called at end of every `pollOnce` regardless of `NewCount`; `IdleHeartbeatInterval = 30s` even in IDLE mode; enforced by named test #15.
- [x] Backoff ladder is `[1s, 2s, 5s, 10s, 30s, 60s]`, exhaustion → `WatchStatusError`.
- [x] Auth failure (`ER-MAIL-21201`) is **terminal** — never re-tries with backoff; user must fix credentials.
- [x] `Stop` is best-effort — 2 s deadline, force-cancel on overrun.
- [x] Watcher does NOT insert into `Email` directly (delegates to `core.Emails.PersistFromImap`).
- [x] Watcher does NOT update `RuleStat` directly (delegates to `core.Rules.BumpStat`).
- [x] Watcher does NOT insert into `OpenedUrl` directly (delegates to `core.Tools.OpenUrl`).
- [x] Subscriber panic recovery prevents loop crash (test #11).
- [x] No goroutine leak after Start/Stop cycles (test #28).
- [x] Error codes registered in 21400–21499 range; wrapped from 21000–21099, 21100–21199, 21200–21299, 21700–21799.
- [x] PII redaction documented + enforced by named test #30.
- [x] `WatchState` migration declared as canonically owned by Watch (referenced by Accounts).
- [x] Cites 02-coding, 03-error-management, **05-logging-strategy §6.4 🔴**, 18-database-conventions, solved-issues/02-watcher-silent-on-healthy-idle.md.

---

**End of `05-watch/01-backend.md`**
