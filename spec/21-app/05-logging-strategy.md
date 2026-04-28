# 05 — Logging Strategy

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **single contract** for log line shape, level, destination, and required fields across `email-read` (CLI + Fyne UI). Every feature spec under `02-features/*` cites a section of this file (e.g. "watcher.poll log line — see §6.4") **and** restates the exact log lines it must emit (per project rule: central + restated).

> **Audit invariant:** Given only the logs from a single run, an operator MUST be able to reconstruct (a) every poll cycle, (b) every email persisted, (c) every rule match, (d) every browser launch, (e) every error with `file:line` chain.

---

## Citation Map

| Topic | Source |
|---|---|
| Three-tier error & envelope shape | `spec/12-consolidated-guidelines/03-error-management.md` |
| Verbose flag rules | `spec/12-consolidated-guidelines/23-generic-cli.md` §Verbose Logging |
| Heartbeat invariant | `.lovable/strictly-avoid.md` ("Do NOT write polling/watch loops that are silent on success") |
| Solved issues that drive this spec | `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`, `04-noisy-watcher-log-output.md`, `08-readable-watcher-logs.md` |
| Wrapper used | `internal/errtrace/errtrace.go` (`Wrap`, `New`, `Format`, `Frames`) |
| Coding standards hooks | [04-coding-standards.md §6 §11](./04-coding-standards.md) |

---

## 1. Log Levels (Closed Set)

Exactly five levels. No others. Stored as PascalCase enum constants in `internal/logger/levels.go`.

| Level | Const | When to use | Default visibility |
|---|---|---|---|
| `DEBUG` | `LevelDebug` | Function entry, intermediate values, IMAP wire details | Hidden unless `--verbose` |
| `INFO` | `LevelInfo` | Successful operations, heartbeats, lifecycle events | Visible |
| `WARN` | `LevelWarn` | Recoverable degradations (rule miss, dedup hit, retry) | Visible |
| `ERROR` | `LevelError` | Failed operation; chain captured via `errtrace.Format` | Visible (always) |
| `FATAL` | `LevelFatal` | Process must exit; only allowed in `cmd/*/main.go` | Visible (always) |

**Promotion rule:** `ERROR` and `FATAL` are NEVER suppressed by any flag. `--quiet` suppresses `INFO` and `DEBUG` only.

---

## 2. Destinations

| Stream | Content | Format |
|---|---|---|
| `stdout` | Primary command output ONLY (table / CSV / JSON of business data) | Per `--format` flag |
| `stderr` | All log lines (every level) + progress bars + interactive prompts | See §3 |
| `data/logs/email-read.log` | Every log line at every level, append-only, rotated daily | JSON Lines (§4) |
| `data/logs/email-read-error.log` | `ERROR` + `FATAL` only, with full `errtrace.Format` chain | JSON Lines (§4) |

### 2.1 Rotation

- File rotation: daily at local midnight, retain 14 days, gzip after rotation. Implemented in `internal/logger/rotate.go`.
- Naming: `email-read.log`, `email-read.2026-04-24.log.gz`, …
- Max single-file size before forced rotation: **20 MiB**.

### 2.2 Stream choice rule (hard)

```
If line is business data       -> stdout
Otherwise                      -> stderr (and file)
```

Forbidden: `fmt.Println` from anywhere outside `internal/cli/*` business-data renderers. Logging from `internal/core/*` MUST use `logger.*`.

---

## 3. Console Format (Human-Readable)

Console (stderr) lines are **single-line, fixed-column, no JSON**. Color via `internal/logger/color.go` (auto-disabled when stderr is not a TTY or `NO_COLOR=1`).

```
LEVEL TIMESTAMP            LOGGER                FIELDS                                  MESSAGE
INFO  2026-04-25T10:14:03Z core.SaveEmail        Alias=ab Uid=12345 DurationMs=42        saved email
WARN  2026-04-25T10:14:03Z rules.Evaluate        Alias=ab RuleId=2 Reason=NoMatch        rule did not match
ERROR 2026-04-25T10:14:03Z core.SaveEmail        Alias=ab Uid=12345 ErrCode=ER-DB-21108 insert email: UNIQUE constraint failed
  at internal/store/store.go:184 (store.InsertEmail)
  at internal/core/emails.go:73 (core.SaveEmail)
  at internal/watcher/watcher.go:142 (watcher.processNew)
```

### 3.1 Fixed columns

| Column | Width | Padding |
|---|---|---|
| `LEVEL` | 5 | right-padded space |
| `TIMESTAMP` | 20 (RFC3339 UTC, second-precision) | none |
| `LOGGER` | 22 (truncate with `…`, left-aligned) | right-padded space |
| `FIELDS` | 40 (truncate with `…`) | right-padded space |
| `MESSAGE` | rest of line | none |

### 3.2 Colors (TTY only)

| Level | Color |
|---|---|
| `DEBUG` | Dim grey |
| `INFO` | Default |
| `WARN` | Yellow |
| `ERROR` | Red |
| `FATAL` | Red, bold |

### 3.3 Stack trace rendering

`ERROR` and `FATAL` lines are immediately followed by `errtrace.Format(err)` output (one frame per indented line). This block is inseparable from the log line it belongs to — never interleave.

---

## 4. File Format (JSON Lines)

Every file log entry is one JSON object per line, PascalCase keys (per `04-coding-standards.md §2.2`).

### 4.1 Required fields (every entry)

```json
{
  "Ts": "2026-04-25T10:14:03.221Z",
  "Level": "INFO",
  "Logger": "core.SaveEmail",
  "Pid": 18432,
  "GoroutineId": 42,
  "Op": "SaveEmail",
  "Msg": "saved email"
}
```

| Key | Type | Source |
|---|---|---|
| `Ts` | string (RFC3339 UTC, ms precision) | `time.Now().UTC()` |
| `Level` | string enum | §1 |
| `Logger` | string `<package>.<func>` | call site |
| `Pid` | int | `os.Getpid()` |
| `GoroutineId` | int | parsed from runtime stack (cheap) |
| `Op` | string | logical operation name (PascalCase verb) |
| `Msg` | string | one-sentence human summary, lowercase first letter |

### 4.2 Conditional fields (when present)

| Key | When | Type |
|---|---|---|
| `Alias` | Any operation tied to a configured account | string |
| `Uid` | Any operation tied to one IMAP message | uint32 |
| `EmailId` | Any operation tied to a persisted email row | int64 |
| `RuleId` | Any operation tied to one rule | int64 |
| `Url` | Any operation that opens or considers a URL | string |
| `MessagesCount` | Mailbox state snapshot | int |
| `UidNext` | Mailbox state snapshot | uint32 |
| `DurationMs` | All `INFO` exit lines | int |
| `ErrCode` | All `WARN`/`ERROR`/`FATAL` lines | string (`ER-XXX-NNNNN`) |
| `ErrMsg` | All `WARN`/`ERROR`/`FATAL` lines | string |
| `ErrFrames` | All `ERROR`/`FATAL` lines | array of `{File,Line,Func}` (from `errtrace.Frames`) |

### 4.3 Reserved field names (do not reuse for other purposes)

`Ts`, `Level`, `Logger`, `Pid`, `GoroutineId`, `Op`, `Msg`, `Alias`, `Uid`, `EmailId`, `RuleId`, `Url`, `MessagesCount`, `UidNext`, `DurationMs`, `ErrCode`, `ErrMsg`, `ErrFrames`, `TraceId`, `ParentTraceId`.

---

## 5. Trace IDs

Every top-level operation creates a `TraceId` (16 hex chars from `crypto/rand`). All log lines emitted while that operation is on the stack carry the same `TraceId`. Nested operations carry the parent's `TraceId` plus their own (`ParentTraceId` field on the child's first line is optional).

### 5.1 Trace boundaries (where a new TraceId begins)

| Boundary | TraceId source |
|---|---|
| One Cobra command invocation | New TraceId per `Run*` call |
| One Fyne user action (button click, sidebar nav) | New TraceId per dispatcher call |
| One watcher poll cycle | New TraceId per `pollOnce` |
| One incoming IMAP IDLE notification | New TraceId per notification |

### 5.2 Propagation

`internal/logger.WithTrace(ctx, traceId) context.Context` injects the ID. Every `internal/core` function MUST accept a `context.Context` as first param and pass it down. The logger reads `TraceId` and `ParentTraceId` from `ctx`.

---

## 6. Per-Layer Required Log Lines

Every feature spec MUST restate the lines listed in this section verbatim. The list is the minimum — features may add more `DEBUG` lines as helpful, but never fewer.

### 6.1 `internal/core/*` — every exported function

| Hook | Level | Logger | Required fields | Msg pattern |
|---|---|---|---|---|
| Entry | `DEBUG` | `core.<Func>` | All input identifiers (e.g. `Alias`, `Uid`) | `"enter"` |
| Exit (success) | `INFO` | `core.<Func>` | Output identifiers + `DurationMs` | one-sentence verb-led summary |
| Exit (error) | `ERROR` | `core.<Func>` | `ErrCode`, `ErrMsg`, `ErrFrames` + same input identifiers | wrapped error message |

### 6.2 `internal/store/*` — every public method

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| `INSERT` success | `DEBUG` | `Table`, `RowId` | `"inserted row"` |
| `UPDATE`/`DELETE` success | `DEBUG` | `Table`, `RowsAffected` | `"updated rows"` / `"deleted rows"` |
| `SELECT` success | (no log unless `--verbose`) | `Table`, `RowsRead` | `"read rows"` |
| Constraint violation | `WARN` | `Table`, `Constraint`, `ErrCode=ER-DB-*` | `"constraint hit: <name>"` |
| Connection error | `ERROR` | `ErrCode=ER-DB-*`, `ErrFrames` | `"db connection failed"` |

### 6.3 `internal/mailclient/*`

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| Connect attempt | `DEBUG` | `Alias`, `Host`, `Port` | `"dialing imap"` |
| Login success | `INFO` | `Alias`, `User`, `DurationMs` | `"imap login ok"` |
| Login failure | `ERROR` | `Alias`, `User`, `ErrCode=ER-MAIL-21201`, `ErrFrames` | `"imap login failed"` |
| Mailbox SELECT | `DEBUG` | `Alias`, `Mailbox`, `MessagesCount`, `UidNext` | `"selected mailbox"` |
| Fetch UID range | `DEBUG` | `Alias`, `Mailbox`, `UidFrom`, `UidTo`, `Count` | `"fetched uids"` |
| Disconnect | `DEBUG` | `Alias` | `"imap disconnected"` |

### 6.4 `internal/watcher/*` — heartbeat invariant 🔴

**Every poll cycle MUST emit at least one line, even when zero new mail.** Violation = `solved-issues/02-watcher-silent-on-healthy-idle.md` regression.

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| Poll start | `DEBUG` | `Alias`, `TraceId` | `"poll start"` |
| Mailbox stats (always) | `INFO` | `Alias`, `MessagesCount`, `UidNext`, `LastUid`, `NewCount`, `DurationMs` | `"poll: messages=N uidNext=M new=K"` |
| Per new email | `INFO` | `Alias`, `Uid`, `EmailId`, `Subject`, `From` | `"fetched new email"` |
| Backoff after error | `WARN` | `Alias`, `BackoffSeconds`, `ErrCode` | `"backing off"` |
| Cycle error | `ERROR` | `Alias`, `ErrCode`, `ErrFrames` | `"poll cycle failed"` |
| Loop start | `INFO` | `Alias`, `PollSeconds` | `"watch started"` |
| Loop stop (Ctrl+C) | `INFO` | `Alias`, `Reason=Signal` | `"watch stopped"` |

### 6.5 `internal/rules/*`

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| Evaluate begin | `DEBUG` | `Alias`, `EmailId`, `RuleCount` | `"evaluating rules"` |
| Match | `INFO` | `Alias`, `EmailId`, `RuleId`, `RuleName`, `MatchedUrl` | `"rule matched"` |
| No match | `DEBUG` | `Alias`, `EmailId` | `"no rule matched"` |
| Rule disabled | `DEBUG` | `RuleId`, `RuleName` | `"rule skipped: disabled"` |
| Compile error | `ERROR` | `RuleId`, `Pattern`, `ErrCode=ER-RUL-21301`, `ErrFrames` | `"rule regex invalid"` |

### 6.6 `internal/browser/*`

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| Dedup hit | `WARN` | `Alias`, `Url`, `OpenedAt` | `"url already opened — skipping"` |
| Launch attempt | `DEBUG` | `Alias`, `Url`, `ChromePath` | `"launching browser"` |
| Launch ok | `INFO` | `Alias`, `Url`, `DurationMs` | `"browser launched"` |
| Launch failure | `ERROR` | `Alias`, `Url`, `ErrCode=ER-BRW-21501`, `ErrFrames` | `"browser launch failed"` |
| Chrome not found | `ERROR` | `Alias`, `ErrCode=ER-BRW-21502`, `SearchedPaths` | `"chrome not found"` |

### 6.7 `internal/exporter/*`

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| Begin | `INFO` | `Alias`, `OutPath`, `EstimatedRows` | `"export started"` |
| Done | `INFO` | `Alias`, `OutPath`, `RowsWritten`, `DurationMs` | `"export complete"` |
| Failure | `ERROR` | `Alias`, `OutPath`, `ErrCode=ER-EXP-*`, `ErrFrames` | `"export failed"` |

### 6.8 `internal/config/*`

| Hook | Level | Required fields | Msg |
|---|---|---|---|
| Load ok | `INFO` | `Path`, `AccountCount`, `RuleCount` | `"config loaded"` |
| Load failure | `FATAL` | `Path`, `ErrCode=ER-CFG-21001`, `ErrFrames` | `"config load failed"` |
| Save ok | `INFO` | `Path`, `BytesWritten` | `"config saved"` |
| Save failure | `ERROR` | `Path`, `ErrCode=ER-CFG-21002`, `ErrFrames` | `"config save failed"` |
| Hidden Unicode in password | `WARN` | `Alias`, `CodePoint`, `ErrCode=ER-CFG-21003` | `"hidden unicode in password — see solved-issues/03"` |

### 6.9 `internal/cli/*`

| Hook | Level | Fields | Msg |
|---|---|---|---|
| Command begin | `INFO` | `Cmd`, `Args`, `TraceId` | `"command started"` |
| Command end | `INFO` | `Cmd`, `ExitCode`, `DurationMs` | `"command finished"` |

### 6.10 `internal/ui/*` (Fyne)

| Hook | Level | Fields | Msg |
|---|---|---|---|
| App start | `INFO` | `WindowSize`, `LastView` | `"ui started"` |
| Sidebar nav | `DEBUG` | `From`, `To` | `"nav"` |
| Form submit | `INFO` | `Form`, `TraceId` | `"submit"` |
| Form validation fail | `WARN` | `Form`, `Field`, `ErrCode=ER-UI-*` | `"validation failed"` |
| App stop | `INFO` | `Reason` | `"ui stopped"` |

---

## 7. Logger API (`internal/logger`)

```go
package logger

type Level int
const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
    LevelFatal
)

type Field struct {
    Key   string
    Value any
}

// Constructors — single allocation per field, all PascalCase keys.
func F(key string, value any) Field

// Logger is the single facade. One instance per package, named via New("core.SaveEmail").
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(err error, msg string, fields ...Field) // attaches ErrCode + ErrFrames
    Fatal(err error, msg string, fields ...Field) // calls os.Exit(1) after flush
}

// New returns a logger bound to a logical name (LOGGER column).
func New(name string) Logger

// WithTrace returns ctx with the given TraceId; logger reads it on every call.
func WithTrace(ctx context.Context, traceId string) context.Context

// FromContext extracts TraceId/ParentTraceId from ctx.
func FromContext(ctx context.Context) (traceId, parentTraceId string)
```

### 7.1 `Error()` semantics

`Error(err, msg, fields...)` MUST:

1. Read `ErrCode` from `err` if `err` carries one (via project's coded-error helper) — else write `ErrCode=ER-UNKNOWN`.
2. Call `errtrace.Frames(err)` and serialize to `ErrFrames`.
3. Render `errtrace.Format(err)` immediately after the console line.

### 7.2 Initialization order (from §Initialization Order in `03-error-management.md`)

```
1. Load config           -> may need INFO/FATAL before logger fully ready -> stderr-only fallback
2. Ensure data/logs dir  -> required for file sink
3. Initialize logger     -> attach file sinks + console
4. Initialize store      -> first INFO line through full logger
5. Initialize services   -> watcher, ui, etc.
```

Until step 3 completes, only `cmd/*/main.go` may write directly to stderr.

---

## 8. Flag Behavior

| Flag | Effect on console | Effect on files |
|---|---|---|
| (none) | `INFO`+ visible | All levels written |
| `-v`/`--verbose` | `DEBUG`+ visible | unchanged |
| `--debug` | `DEBUG`+ visible + per-IMAP-wire frames | unchanged |
| `--quiet` | `WARN`+ visible (suppresses `INFO`/`DEBUG`) | unchanged |
| `--no-color` | Disables ANSI colors | n/a |
| `--log-format=json` | Console emits JSON Lines instead of fixed-column | unchanged |

**Conflict rule:** `--quiet` + `--verbose` → CLI exits with code 2 and message `"--quiet and --verbose are mutually exclusive"`.

---

## 9. Performance Budget

| Operation | Budget |
|---|---|
| Single console log line | < 50 µs |
| Single file log line (buffered) | < 30 µs |
| `errtrace.Format` for 10-frame chain | < 200 µs |
| Logger init (file open + dir create) | < 20 ms |
| Daily rotation | < 100 ms |

If a future change exceeds these, add an issue to `spec/21-app/03-issues/` rather than relax the budget silently.

---

## 10. Forbidden Patterns

| Pattern | Why |
|---|---|
| `fmt.Println` / `fmt.Printf` outside `internal/cli/<cmd>.go` business renderers | Bypasses logger |
| `log.Println` (stdlib) anywhere | Bypasses logger |
| `panic` outside startup | Skips graceful flush |
| `os.Exit` outside `cmd/*/main.go` | Skips graceful flush |
| Logging secrets (passwords, tokens) | Use `***redacted***` |
| Logging full email body | PII; log only `Subject` + `From` |
| Silent `if err != nil { return err }` in core layer | Must wrap + log |
| Heartbeat suppression in watcher idle | 🔴 hard ban (`solved-issues/02`) |
| Mixing JSON keys casing (e.g. `alias`/`Alias` in same line) | Breaks log parsers |

---

## 11. PII & Secrets Redaction

| Field | Treatment |
|---|---|
| IMAP password | Never logged; even in `--debug`. Replaced with `***redacted***` |
| OAuth tokens | Same as above |
| Email body / HTML | Never logged. Only `Subject`, `From`, `To` (truncated to 120 chars) |
| URLs containing `?token=`, `?key=`, `?password=` query params | Token value redacted, query key kept |
| Local file paths under `$HOME` | Logged as-is (not PII for desktop app) |

Implementation: `internal/logger/redact.go` exports `Redact(s string) string` and is applied to every string field whose key is in the redaction list (`Password`, `Token`, `Authorization`, `Cookie`, `Body`).

---

## 12. Test Coverage for Logging

Every feature spec's acceptance criteria MUST include a test that asserts:

1. The required `INFO` exit line is present with exact `Logger`, `Op`, and required fields.
2. The required `ERROR` line is present on the failure path with correct `ErrCode` and at least 2 `ErrFrames`.
3. (For watcher only) heartbeat present on every poll cycle, including idle.

Helper: `internal/logger/logtest.NewCapture()` returns an in-memory sink usable in tests.

---

## 13. Worked Example — End-to-End for One Watcher Cycle (Idle)

Console (stderr):

```
INFO  2026-04-25T10:14:00Z watcher.Loop          Alias=ab PollSeconds=3                   watch started
INFO  2026-04-25T10:14:03Z watcher.pollOnce      Alias=ab MessagesCount=42 UidNext=12346 LastUid=12345 NewCount=0 DurationMs=187 poll: messages=42 uidNext=12346 new=0
INFO  2026-04-25T10:14:06Z watcher.pollOnce      Alias=ab MessagesCount=42 UidNext=12346 LastUid=12345 NewCount=0 DurationMs=174 poll: messages=42 uidNext=12346 new=0
```

File (`data/logs/email-read.log`):

```jsonl
{"Ts":"2026-04-25T10:14:00.000Z","Level":"INFO","Logger":"watcher.Loop","Pid":18432,"GoroutineId":42,"Op":"WatchStart","Alias":"ab","PollSeconds":3,"TraceId":"a1b2c3d4e5f60718","Msg":"watch started"}
{"Ts":"2026-04-25T10:14:03.187Z","Level":"INFO","Logger":"watcher.pollOnce","Pid":18432,"GoroutineId":43,"Op":"Poll","Alias":"ab","MessagesCount":42,"UidNext":12346,"LastUid":12345,"NewCount":0,"DurationMs":187,"TraceId":"f0e1d2c3b4a59687","ParentTraceId":"a1b2c3d4e5f60718","Msg":"poll: messages=42 uidNext=12346 new=0"}
```

---

## 14. Worked Example — End-to-End for One Failed IMAP Login

Console (stderr):

```
ERROR 2026-04-25T10:14:00Z mailclient.Login      Alias=ab User=foo@bar.com ErrCode=ER-MAIL-21201 imap login failed: AUTHENTICATIONFAILED
  at internal/mailclient/mailclient.go:184 (mailclient.Login)
  at internal/core/diagnose.go:55 (core.Diagnose)
  at internal/cli/cli.go:218 (cli.runDiagnose)
ERROR 2026-04-25T10:14:00Z core.Diagnose         Alias=ab ErrCode=ER-MAIL-21201             diagnose failed
  at internal/core/diagnose.go:55 (core.Diagnose)
  at internal/cli/cli.go:218 (cli.runDiagnose)
```

File (`data/logs/email-read-error.log`):

```jsonl
{"Ts":"2026-04-25T10:14:00.221Z","Level":"ERROR","Logger":"mailclient.Login","Pid":18432,"GoroutineId":1,"Op":"ImapLogin","Alias":"ab","User":"foo@bar.com","ErrCode":"ER-MAIL-21201","ErrMsg":"imap login failed: AUTHENTICATIONFAILED","ErrFrames":[{"File":"internal/mailclient/mailclient.go","Line":184,"Func":"mailclient.Login"},{"File":"internal/core/diagnose.go","Line":55,"Func":"core.Diagnose"}],"TraceId":"…","Msg":"imap login failed"}
```

---

## 15. Cross-References

| Reference | Location |
|---|---|
| Coding standards (logging hooks) | [04-coding-standards.md §6](./04-coding-standards.md) |
| Error registry (every `ErrCode`) | [06-error-registry.md](./06-error-registry.md) |
| Architecture (which package logs what) | [07-architecture.md](./07-architecture.md) |
| Watcher feature (cites §6.4 + restates lines) | [02-features/05-watch/00-overview.md](./02-features/05-watch/00-overview.md) |
| `internal/errtrace` source | `internal/errtrace/errtrace.go` |
| Heartbeat invariant root cause | `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md` |
| Noisy log fix | `.lovable/solved-issues/04-noisy-watcher-log-output.md` |
| Readable watcher logs | `.lovable/solved-issues/08-readable-watcher-logs.md` |
