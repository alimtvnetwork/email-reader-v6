# 06 — Tools — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the `core.Tools` service: API surface, the four sub-tool implementations (`ReadOnce`, `ExportCsv`, `Diagnose`, `OpenUrl`), the URL validation/redaction pipeline, the `OpenedUrl` audit-table writes, the 60 s `Diagnose` cache, the streaming-channel contract, the error registry block `21750–21769`, and the testing contract.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.6
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.6
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21750–21769` (Tools) + wrapped `21600–21604` (exporter), `ER-DB-21105` (OpenedUrl insert), `ER-MAIL-21201..21210` (diagnose probes; aligned to impl in Slice #161), `ER-COR-21704` (path escape)
- Database: `spec/12-consolidated-guidelines/18-database-conventions.md`; canonical `OpenedUrl` schema in `00-overview.md` §4.2

---

## 1. Service Definition

```go
// Package core — file: internal/core/tools.go
package core

type Tools struct {
    accounts   *Accounts            // alias resolution + AccountEvent subscription
    emails     *Emails              // ReadOnce probe-mode delegation
    mailcli    mailclient.Dialer    // factory for one-shot IMAP probes
    exporter   exporter.CsvWriter   // streaming CSV writer
    browser    browser.Launcher     // OS-specific incognito launcher
    store      store.Store          // OpenedUrl writes; Email reads (export)
    paths      paths.Validator      // ER-COR-21704 path-escape guard
    bus        eventbus.Subscriber  // listens to AccountEvent for cache invalidation
    clock      Clock
    log        Logger

    diagCache  sync.Map             // map[string]*diagCacheEntry — alias → cached DiagnosticsReport
    diagMu     keyedMutex           // per-alias serialization for Diagnose
    openMu     keyedMutex           // per-(Alias|Url) serialization for OpenUrl

    cfg        ToolsConfig          // values pulled from config.Settings at construct time
}

type ToolsConfig struct {
    OpenUrlDedupSeconds   int      // default 60
    OpenUrlMaxLengthBytes int      // default 8192
    OpenUrlAllowedSchemes []string // default ["https", "http"]
    AllowLocalhostUrls    bool     // default false
    AllowPrivateIpUrls    bool     // default false
    BrowserOverride       string   // default ""
    DiagnoseCacheSeconds  int      // default 60 (constant in v1; field exists for v2)
}

type diagCacheEntry struct {
    Report   DiagnosticsReport
    StoredAt time.Time
}

func NewTools(
    accounts *Accounts,
    emails *Emails,
    mailcli mailclient.Dialer,
    exporter exporter.CsvWriter,
    browser browser.Launcher,
    store store.Store,
    paths paths.Validator,
    bus eventbus.Subscriber,
    clock Clock,
    log Logger,
    cfg ToolsConfig,
) (*Tools, errtrace.Result[Unit])
```

**Constructor responsibilities** (must remain ≤ 15 LOC body per `04-coding-standards.md` §3):
1. Validate `cfg` (OpenUrlMaxLengthBytes ∈ [256, 65536]; OpenUrlDedupSeconds ∈ [0, 3600]; OpenUrlAllowedSchemes non-empty subset of `{"http","https"}`).
2. Subscribe to `bus` for `AccountEvent{Updated|Removed}` → invalidate `diagCache` entry.
3. Return the constructed `*Tools` plus `errtrace.Ok(Unit{})`.

The constructor must NOT touch IMAP, the browser, or the filesystem.

---

## 2. Public Methods

Every method returns `errtrace.Result[T]` per `04-coding-standards.md` §6 / `03-error-management.md`. No naked `error` returns from this package surface.

### 2.1 `ReadOnce`

```go
func (t *Tools) ReadOnce(
    ctx context.Context,
    spec ReadSpec,
    progress chan<- string,        // streaming log lines; closed by callee on return
) errtrace.Result[ReadResult]
```

Contract:
- Validates `spec` per §6; returns `21750 ToolsInvalidArgument` on failure.
- Resolves `AccountSpec` via `t.accounts.Get(ctx, spec.Alias)`; forwards `21703 AccountNotFound` if absent.
- Dials a one-shot IMAP connection via `t.mailcli.Dial(ctx, account)`; LOGINs; SELECTs INBOX; FETCHes the most recent `spec.Limit` UIDs.
- Streams human-readable progress to `progress` (e.g. `"dial OK"`, `"login OK as work@example.com"`, `"fetched 10/10 UIDs in 412 ms"`).
- **Does NOT** advance the watcher cursor. `WatchState.LastSeenUid` is never written.
- Closes `progress` exactly once before returning (deferred close).
- Honours `ctx.Done()` within 500 ms.

Errors: `21750`, `21751 ToolsReadFetchFailed`, wrapped `ER-MAIL-21201..21210`.

### 2.2 `ExportCsv`

```go
func (t *Tools) ExportCsv(
    ctx context.Context,
    spec ExportSpec,
    progress chan<- ExportProgress,  // closed by callee on return
) errtrace.Result[ExportReport]
```

Contract:
- Validates `spec` per §6; `21750` on bad input.
- `t.paths.Validate(spec.OutPath)` — must be inside the user's documents dir or app data dir; else `ER-COR-21704 ErrCorePathOutsideData`.
- If `!spec.Overwrite && fileExists(spec.OutPath)` → `21753 ToolsExportPathExists`.
- Phase 1 (`Counting`): single `SELECT COUNT(*)` → publish `ExportProgress{Phase: Counting, TotalRows: N}`.
- Phase 2 (`Writing`): stream `SELECT … WHERE Alias = ? AND ReceivedAt >= ? AND ReceivedAt < ? ORDER BY ReceivedAt`; publish `ExportProgress{Phase: Writing, RowsWritten: k, TotalRows: N}` every **256 rows** (rate-limit) and at completion.
- Phase 3 (`Flushing`): `csv.Flush()` + `os.File.Sync()`; publish once.
- Phase 4 (`Done`): publish terminal `ExportProgress{Phase: Done, RowsWritten: N, TotalRows: N}`.
- Closes `progress` exactly once before returning.
- Honours `ctx.Done()` within 500 ms; on cancel, deletes the partial file (best-effort) and returns `21703 AccountNotFound`-style cancelled wrap.

Errors: `21750`, `21753`, wrapped `ER-EXP-21601..21604`, `ER-COR-21704`.

### 2.3 `Diagnose`

```go
type DiagnoseSpec struct {
    Alias string
    Force bool   // bypass cache; invalidate entry on success
}

func (t *Tools) Diagnose(
    ctx context.Context,
    spec DiagnoseSpec,
    progress chan<- DiagnosticsStep,  // closed by callee on return; published per step
) errtrace.Result[DiagnosticsReport]
```

Contract:
- Cache check first (unless `spec.Force`): if `entry.StoredAt + 60s > now`, publish each cached step to `progress` (so UI renders identically), set `Cached = true`, return.
- Acquire `t.diagMu.Lock(spec.Alias)` to prevent concurrent IMAP probes for the same alias; release on return.
- Run the 5 steps in order: `DnsLookup` → `TcpConnect` → `TlsHandshake` → `ImapLogin` → `InboxSelect`.
- For each step: publish `DiagnosticsStep{Status: Running}`, then either `Pass` or `Fail`. On `Fail`, mark all subsequent steps `Skipped` (publish them too, with `Skipped` status, so the UI checklist shows the full list).
- Cache the final `DiagnosticsReport` on completion (regardless of `OverallOk`).
- Closes `progress` exactly once before returning.

Errors: `21750`, `21752 ToolsDiagnoseAborted` (only on ctx cancel mid-step), wrapped `ER-MAIL-21201..21210`.

### 2.4 `OpenUrl`

```go
type OpenUrlSpec struct {
    Url      string
    Origin   OpenUrlOrigin    // required; caller declares context
    Alias    string           // "" allowed for OriginManual / OriginCli
    RuleName string           // populated when Origin == Rule
    EmailId  int64            // populated when triggered by an email; 0 otherwise
}

func (t *Tools) OpenUrl(
    ctx context.Context,
    spec OpenUrlSpec,
) errtrace.Result[OpenUrlReport]
```

Contract — the **only** authorised browser-launch path:

1. **Validate** per §5.1 of overview: empty / too-long / malformed / scheme-forbidden / localhost / private-IP — first failure short-circuits with the corresponding `21760..21765` code.
2. **Redact** per §5.2 of overview: produce `(canonicalUrl, originalUrl)`. If userinfo was stripped, emit one WARN log.
3. **Acquire `t.openMu.Lock(spec.Alias + "|" + canonicalUrl)`** to serialise concurrent same-key calls.
4. **Dedup check**: query `SELECT 1 FROM OpenedUrl WHERE Alias = ? AND Url = ? AND OpenedAt > ? LIMIT 1` with `now - DedupWindowSec`.
   - If hit: insert audit row with `IsDeduped = 1`, `BrowserBinary = ""`, return `OpenUrlReport{Deduped: true, …}`.
   - If miss: continue.
5. **Resolve browser binary**: `cfg.BrowserOverride` if set, else `t.browser.DefaultBinary()`. Error = `21766 OpenUrlBrowserUnavailable`.
6. **Launch incognito**: `t.browser.LaunchIncognito(ctx, binary, canonicalUrl)`. Always incognito; no opt-out. Error = `21767 OpenUrlLaunchFailed`.
7. **Audit insert**: one `INSERT INTO OpenedUrl` row with `IsDeduped = 0`, `IsIncognito = 1`, `BrowserBinary = resolved`, `Url = canonical`, `OriginalUrl = original`, populated `Alias/RuleName/EmailId/Origin/TraceId`. Wrapped error → `ER-DB-21105`.
8. **Return** `OpenUrlReport`.

Errors: `21760..21767`, wrapped `ER-DB-21105`.

**Atomicity note**: the launch (step 6) and audit insert (step 7) are NOT in a single transaction (the OS process spawn is not transactional). Order matters: launch FIRST, then audit. If audit insert fails after a successful launch, the WARN log + `ER-DB-21105` is the audit trail (operator can grep logs). This trade-off is documented in §7.

### 2.5 `RecentOpenedUrls` (read-only audit accessor)

```go
type OpenedUrlListSpec struct {
    Alias  string  // "" = all aliases
    Origin OpenUrlOrigin  // "" = all origins
    Limit  int     // 1..1000; default 100
    Before time.Time  // pagination cursor; zero = now
}

func (t *Tools) RecentOpenedUrls(
    ctx context.Context,
    spec OpenedUrlListSpec,
) errtrace.Result[[]OpenedUrlRow]
```

Used by Rules' hit-history view and by Tools' own OpenUrl tab "Recent" panel. Pure SELECT; no side effects.

### 2.6 `OnAccountUpdate` (internal — invoked by AccountEvent subscription)

```go
func (t *Tools) OnAccountUpdate(ctx context.Context, alias string) errtrace.Result[Unit]
```

- Invalidates the diagnose cache entry for `alias`.
- Idempotent; never blocks; ≤ 1 ms.
- Called from the `bus` subscriber goroutine; must NOT call back into `bus`.

---

## 3. URL Validation & Redaction Implementation

### 3.1 Validation pipeline

```go
func (t *Tools) validateUrl(raw string) (parsed *url.URL, err error) {
    if raw == "" {
        return nil, apperror.New(ToolsErr.OpenUrlEmpty, "url is empty")
    }
    if len(raw) > t.cfg.OpenUrlMaxLengthBytes {
        return nil, apperror.New(ToolsErr.OpenUrlTooLong,
            fmt.Sprintf("url length %d exceeds max %d", len(raw), t.cfg.OpenUrlMaxLengthBytes))
    }
    u, perr := url.Parse(raw)
    if perr != nil || u.Host == "" {
        return nil, apperror.New(ToolsErr.OpenUrlMalformed, "url is malformed")
    }
    if !t.schemeAllowed(u.Scheme) {
        return nil, apperror.New(ToolsErr.OpenUrlSchemeForbidden,
            fmt.Sprintf("scheme %q not allowed", u.Scheme))
    }
    if !t.cfg.AllowLocalhostUrls && isLoopback(u.Hostname()) {
        return nil, apperror.New(ToolsErr.OpenUrlLocalhostBlocked, "localhost urls are blocked")
    }
    if !t.cfg.AllowPrivateIpUrls && isPrivateIp(u.Hostname()) {
        return nil, apperror.New(ToolsErr.OpenUrlPrivateIpBlocked, "private-ip urls are blocked")
    }
    return u, nil
}
```

Helpers (`isLoopback`, `isPrivateIp`, `schemeAllowed`) live in `internal/core/tools_url.go` and are pure functions covered by table-driven tests.

### 3.2 Redaction pipeline

```go
func (t *Tools) redactUrl(u *url.URL) (canonical, original string, hadUserinfo bool) {
    original = u.String()
    if u.User != nil {
        hadUserinfo = true
        u.User = nil
    }
    q := u.Query()
    for _, k := range secretQueryKeys {        // []string{"password","pwd","secret","token","otp","code"}
        if q.Has(k) {
            q.Set(k, "***")
        }
    }
    u.RawQuery = q.Encode()
    canonical = u.String()
    return
}
```

OTP-style redaction inside path segments uses regex `\b\d{4,8}\b` only when the URL path also matches `(?i)/(otp|code|verify|confirm|magic)/`. False-positive risk is low for non-OTP numeric paths (e.g. `/posts/12345` is unaffected).

### 3.3 Per-key mutex

```go
type keyedMutex struct {
    m sync.Map  // key string → *sync.Mutex
}

func (k *keyedMutex) Lock(key string) {
    mu, _ := k.m.LoadOrStore(key, &sync.Mutex{})
    mu.(*sync.Mutex).Lock()
}

func (k *keyedMutex) Unlock(key string) {
    mu, ok := k.m.Load(key)
    if ok { mu.(*sync.Mutex).Unlock() }
}
```

Memory note: keys accumulate forever (one entry per unique `(Alias, Url)`). Acceptable in practice (≤ 10 k unique URLs/year); a v2 LRU eviction is deferred.

---

## 4. Diagnose Implementation

### 4.1 Step runner

```go
func (t *Tools) runDiagnose(ctx context.Context, alias string, progress chan<- DiagnosticsStep) DiagnosticsReport {
    started := t.clock.Now()
    steps := []DiagnosticsStepName{
        DiagnosticsStepDnsLookup,
        DiagnosticsStepTcpConnect,
        DiagnosticsStepTlsHandshake,
        DiagnosticsStepImapLogin,
        DiagnosticsStepInboxSelect,
    }
    out := make([]DiagnosticsStep, 0, len(steps))
    failed := false
    for i, name := range steps {
        if failed {
            step := DiagnosticsStep{Name: name, Status: DiagnosticsStepSkipped}
            out = append(out, step); progress <- step
            continue
        }
        progress <- DiagnosticsStep{Name: name, Status: DiagnosticsStepRunning}
        step := t.runDiagnoseStep(ctx, alias, name, i)
        out = append(out, step); progress <- step
        if step.Status == DiagnosticsStepFail { failed = true }
    }
    return DiagnosticsReport{
        Alias: alias, StartedAt: started, DurationMs: int(t.clock.Since(started).Milliseconds()),
        Steps: out, Cached: false, OverallOk: !failed,
    }
}
```

Per-step implementations live in `internal/core/tools_diagnose.go`; each is ≤ 15 LOC body, returns one `DiagnosticsStep` with `Detail` populated on success and `Err` populated on failure.

### 4.2 Cache contract

```go
func (t *Tools) cacheGet(alias string) (*DiagnosticsReport, bool) {
    raw, ok := t.diagCache.Load(alias)
    if !ok { return nil, false }
    e := raw.(*diagCacheEntry)
    if t.clock.Since(e.StoredAt) > time.Duration(t.cfg.DiagnoseCacheSeconds)*time.Second {
        t.diagCache.Delete(alias)
        return nil, false
    }
    cp := e.Report; cp.Cached = true
    return &cp, true
}

func (t *Tools) cachePut(alias string, r DiagnosticsReport) {
    t.diagCache.Store(alias, &diagCacheEntry{Report: r, StoredAt: t.clock.Now()})
}

func (t *Tools) cacheInvalidate(alias string) { t.diagCache.Delete(alias) }
```

`OnAccountUpdate` calls `cacheInvalidate`; `Diagnose` calls `cacheGet` first and `cachePut` last.

---

## 5. SQL Schema

Canonical schema lives in `00-overview.md` §4.2. The backend file is the single place where writing queries are defined.

```sql
-- (Reproduced for read clarity; spec/23-app-database/ is the single source of truth post-Task #32.)
CREATE TABLE OpenedUrl (
    Id              INTEGER PRIMARY KEY AUTOINCREMENT,
    Alias           TEXT    NOT NULL,
    Url             TEXT    NOT NULL,
    OriginalUrl     TEXT    NOT NULL,
    Origin          TEXT    NOT NULL,
    RuleName        TEXT    NOT NULL DEFAULT '',
    EmailId         INTEGER NOT NULL DEFAULT 0,
    OpenedAt        DATETIME NOT NULL,
    BrowserBinary   TEXT    NOT NULL,
    IsIncognito     INTEGER NOT NULL DEFAULT 1,
    IsDeduped       INTEGER NOT NULL DEFAULT 0,
    TraceId         TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX        IxOpenedUrlAliasOpenedAt ON OpenedUrl(Alias, OpenedAt DESC);
CREATE INDEX        IxOpenedUrlOrigin        ON OpenedUrl(Origin, OpenedAt DESC);
CREATE UNIQUE INDEX IxOpenedUrlsUnique       ON OpenedUrl(Alias, Url, OpenedAt);
```

Cascade rules: NONE. `Alias` is a logical reference, not a FK, so audit history survives `Account.Delete` (per overview §4.2 / acceptance §1 #9).

---

## 6. Queries

### 6.1 OpenUrl dedup probe

```sql
-- Q-OPEN-DEDUP
SELECT 1
FROM OpenedUrl
WHERE Alias    = :alias
  AND Url      = :url
  AND OpenedAt > :since
ORDER BY OpenedAt DESC
LIMIT 1;
```

### 6.2 OpenUrl insert

```sql
-- Q-OPEN-INS
INSERT INTO OpenedUrl
  (Alias, Url, OriginalUrl, Origin, RuleName, EmailId, OpenedAt,
   BrowserBinary, IsIncognito, IsDeduped, TraceId)
VALUES
  (:alias, :url, :originalUrl, :origin, :ruleName, :emailId, :openedAt,
   :browserBinary, 1, :isDeduped, :traceId);
```

### 6.3 RecentOpenedUrls

```sql
-- Q-OPEN-LIST
SELECT Id, Alias, Url, OriginalUrl, Origin, RuleName, EmailId,
       OpenedAt, BrowserBinary, IsIncognito, IsDeduped, TraceId
FROM OpenedUrl
WHERE (:alias  = '' OR Alias  = :alias)
  AND (:origin = '' OR Origin = :origin)
  AND OpenedAt < :before
ORDER BY OpenedAt DESC
LIMIT :limit;
```

### 6.4 ExportCsv count

```sql
-- Q-EXPORT-COUNT
SELECT COUNT(*) FROM Email
WHERE Alias = :alias AND ReceivedAt >= :from AND ReceivedAt < :to;
```

### 6.5 ExportCsv stream

```sql
-- Q-EXPORT-STREAM (cursor; LIMIT/OFFSET disallowed for streaming — use prepared cursor + Rows.Next)
SELECT Id, Alias, FromAddr, Subject, SnippetText, ReceivedAt
FROM Email
WHERE Alias = :alias AND ReceivedAt >= :from AND ReceivedAt < :to
ORDER BY ReceivedAt ASC, Id ASC;
```

---

## 7. Atomicity & Safety

| Concern                                   | Decision |
|-------------------------------------------|----------|
| Browser launch + audit insert in one tx?  | **No** — OS process spawn is not transactional. Order = launch first, audit second. If audit fails post-launch, log `ER-DB-21105` WARN; the WARN itself is the audit trail (greppable). Accepted trade-off; alternative (audit-first, launch-second) would orphan rows on launch failure, which is **worse** for forensic completeness. |
| Concurrent same-`(Alias,Url)` open        | Serialised by `keyedMutex.Lock(alias+"|"+url)`; second call sees dedup hit. |
| Concurrent same-alias `Diagnose`          | Serialised by `keyedMutex.Lock(alias)`; second call gets cached result (after first completes) or shares the in-flight result via cache write at end. |
| Cancellation mid-Export                   | `csv.Flush` + `os.Remove(spec.OutPath)` (best-effort); partial file removal is NOT guaranteed (race vs. fsync) but is attempted. |
| Cancellation mid-Diagnose                 | Current step is interrupted via ctx; remaining steps marked `Skipped`; partial report cached as `OverallOk: false`. |
| `OnAccountUpdate` re-entrancy             | Pure cache delete; no IMAP, no FS, no DB. Safe under fan-out lock. |
| `keyedMutex` memory growth                | Bounded by unique `(Alias|Url)` set; v1 accepts unbounded growth. Documented for v2. |
| Path-escape on `ExportSpec.OutPath`       | `t.paths.Validate` enforced before file open; `ER-COR-21704` on escape. |

---

## 8. Error Codes (registry §21750–21769 — Tools block)

| Code  | Name                              | Layer    | Recovery hint                                   |
|-------|-----------------------------------|----------|-------------------------------------------------|
| 21750 | `ToolsInvalidArgument`            | core     | Caller bug — log WARN; UI rejects form          |
| 21751 | `ToolsReadFetchFailed`            | core     | Wrap `ER-MAIL-*`; UI shows underlying mail code |
| 21752 | `ToolsDiagnoseAborted`            | core     | ctx cancelled; UI marks remaining steps Skipped |
| 21753 | `ToolsExportPathExists`           | core     | UI prompt: overwrite? or pick new path          |
| 21754 | `ToolsExportCancelled`            | core     | ctx cancelled; partial file removed (best-effort) |
| 21755 | `ToolsCacheCorrupted`             | core     | Defensive — cache evicted; full re-run forced   |
| 21760 | `OpenUrlEmpty`                    | core     | Caller bug — log WARN                           |
| 21761 | `OpenUrlTooLong`                  | core     | Reject; UI toast "URL too long"                 |
| 21762 | `OpenUrlMalformed`                | core     | Reject; UI toast "URL malformed"                |
| 21763 | `OpenUrlSchemeForbidden`          | core     | Reject; UI toast "scheme `{x}` is not allowed"  |
| 21764 | `OpenUrlLocalhostBlocked`         | core     | UI toast + Settings hint                        |
| 21765 | `OpenUrlPrivateIpBlocked`         | core     | UI toast + Settings hint                        |
| 21766 | `OpenUrlBrowserUnavailable`       | core     | UI toast "no browser found"; suggest `BrowserOverride` |
| 21767 | `OpenUrlLaunchFailed`             | core     | UI toast with OS error                          |
| 21768 | `OpenUrlAuditInsertFailed`        | store-wrap | Wrap `ER-DB-21105`; WARN log; launch already happened |
| 21769 | `ToolsBackgroundLeakDetected`     | core     | Defensive — sub-tool left a goroutine alive past Detach; ERROR + force-cancel |

Wrapped (referenced, not owned by Tools):
- `ER-EXP-21601..21604` — exporter file I/O
- `ER-MAIL-21201..21210` — mail probes for ReadOnce + Diagnose
- `ER-DB-21105 ErrDbInsertUrl` — audit-insert failure
- `ER-COR-21704 ErrCorePathOutsideData` — export path escape
- `ER-COR-21703 ErrCoreContextCancelled` — ctx cancellation surface

---

## 9. Logging

Per `05-logging-strategy.md` §6.6.

| Level | Event                          | Fields                                                                                  |
|-------|--------------------------------|-----------------------------------------------------------------------------------------|
| INFO  | `ToolsReadStarted`             | `Alias`, `Limit`, `TraceId`                                                             |
| INFO  | `ToolsReadCompleted`           | `Alias`, `UidsFetched`, `DurationMs`, `TraceId`                                         |
| INFO  | `ToolsExportStarted`           | `Alias`, `From`, `To`, `OutPath`, `TraceId`                                             |
| INFO  | `ToolsExportPhase`             | `Alias`, `Phase`, `RowsWritten`, `TotalRows`, `TraceId` (one per phase, not per row)    |
| INFO  | `ToolsExportCompleted`         | `Alias`, `RowsWritten`, `OutPath`, `DurationMs`, `TraceId`                              |
| INFO  | `ToolsDiagnoseStarted`         | `Alias`, `Force`, `Cached`, `TraceId`                                                   |
| INFO  | `ToolsDiagnoseStep`            | `Alias`, `Step`, `Status`, `DurationMs`, `Detail`, `TraceId` (one per step)             |
| INFO  | `ToolsDiagnoseCompleted`       | `Alias`, `OverallOk`, `DurationMs`, `Cached`, `TraceId`                                 |
| INFO  | `OpenUrlLaunched`              | `Alias`, `Url` (redacted), `Origin`, `RuleName`, `EmailId`, `BrowserBinary`, `TraceId`  |
| INFO  | `OpenUrlDeduped`               | `Alias`, `Url`, `Origin`, `WindowSecs`, `TraceId`                                       |
| WARN  | `OpenUrlUserinfoStripped`      | `Alias`, `Origin`, `TraceId` (URL fields **omitted** — userinfo is sensitive)           |
| WARN  | `OpenUrlAuditInsertFailed`     | `Alias`, `Url`, `ErrCode=ER-DB-21105`, `TraceId`                                       |
| ERROR | `OpenUrlBrowserUnavailable`    | `BrowserOverride`, `TraceId`                                                            |
| ERROR | `OpenUrlLaunchFailed`          | `Alias`, `Url`, `BrowserBinary`, `OsError` (redacted), `TraceId`                        |
| ERROR | `ToolsBackgroundLeakDetected`  | `SubTool`, `TraceId`                                                                    |

**Redaction invariants** (asserted by `Logging_NeverContainsSecret`):
- `Url` field in any log line is the **canonical (post-redacted)** URL — never `OriginalUrl`.
- `OriginalUrl` is **never** logged (only persisted to the DB column).
- `OsError` strings stripped of paths matching `/home/{user}/`, `/Users/{user}/`, env-var values, and the stripped userinfo bytes.

---

## 10. Performance Budgets

| Operation                                                       | Budget       | Bench                          |
|-----------------------------------------------------------------|--------------|--------------------------------|
| `ReadOnce` end-to-end (10 UIDs, warm IMAP)                      | ≤ **2 s** p95 | `BenchmarkReadOnce10`         |
| `ExportCsv` 10 k rows to local FS                               | ≤ **3 s** p95 | `BenchmarkExportCsv10k`       |
| `Diagnose` end-to-end (5 steps, warm DNS)                       | ≤ **1.5 s** p95 | `BenchmarkDiagnose`         |
| `Diagnose` cache hit                                            | ≤ **2 ms**   | `BenchmarkDiagnoseCacheHit`    |
| `OpenUrl` (validate + redact + dedup hit, no launch)            | ≤ **5 ms**   | `BenchmarkOpenUrlDedupHit`     |
| `OpenUrl` (validate + redact + launch + audit insert)           | ≤ **80 ms** p95 | `BenchmarkOpenUrlLaunch`    |
| `RecentOpenedUrls` 100 rows                                     | ≤ **8 ms**   | `BenchmarkRecentOpenedUrls`    |
| Cancellation latency (any sub-tool)                             | ≤ **500 ms** | `BenchmarkCancelLatency`       |

---

## 11. Testing Contract

All tests in `internal/core/tools_*_test.go`. Use fakes from `internal/core/fakes_test.go` (`FakeAccounts`, `FakeMailDialer`, `FakeBrowser`, `FakeExporter`, `FakeStore`, `FakePaths`).

### 11.1 Required test cases (sub-tools)

1. `Tools_ReadOnce_Happy_StreamsProgress_NoCursorWrite`
2. `Tools_ReadOnce_LimitOutOfRange_Returns21750`
3. `Tools_ReadOnce_AccountMissing_Returns21703`
4. `Tools_ReadOnce_FetchFails_Returns21751WithMailWrap`
5. `Tools_ReadOnce_CtxCancel_StopsUnder500ms`
6. `Tools_ExportCsv_Happy_StreamsCountWritingFlushingDone`
7. `Tools_ExportCsv_PathOutsideData_Returns21704`
8. `Tools_ExportCsv_PathExistsNoOverwrite_Returns21753`
9. `Tools_ExportCsv_DiskFull_Returns21602Wrap_PartialFileRemoved`
10. `Tools_ExportCsv_CtxCancel_Under500ms_PartialFileRemovalAttempted`
11. `Tools_ExportCsv_ProgressEvery256Rows_NotPerRow`
12. `Tools_Diagnose_Happy_5Steps_AllPass`
13. `Tools_Diagnose_DnsFails_RemainingMarkedSkipped`
14. `Tools_Diagnose_AuthFails_InboxStepSkipped`
15. `Tools_Diagnose_CacheHit_NoImapCalls`
16. `Tools_Diagnose_Force_BypassesCache`
17. `Tools_Diagnose_OnAccountUpdate_InvalidatesCacheEntry`
18. `Tools_Diagnose_TwoConcurrentSameAlias_OneImapLogin`

### 11.2 Required test cases (`OpenUrl` — security-critical)

19. `OpenUrl_Empty_Returns21760`
20. `OpenUrl_TooLong_Returns21761`
21. `OpenUrl_Malformed_Returns21762`
22. `OpenUrl_JavascriptScheme_Returns21763`
23. `OpenUrl_FileScheme_Returns21763`
24. `OpenUrl_DataScheme_Returns21763`
25. `OpenUrl_MailtoScheme_Returns21763`
26. `OpenUrl_Localhost_Returns21764_WhenAllowFalse`
27. `OpenUrl_Localhost_Allowed_WhenSettingTrue`
28. `OpenUrl_PrivateIp_Returns21765_AllRfc1918Ranges`
29. `OpenUrl_OtpRedacted_InUrl_OriginalKept`
30. `OpenUrl_PasswordRedacted_InUrl_OriginalKept`
31. `OpenUrl_UserinfoStripped_WarnLogged_NoUrlInLog`
32. `OpenUrl_DedupHit_NoLaunch_AuditRowWithDeduped1`
33. `OpenUrl_DedupMiss_LaunchAndAudit`
34. `OpenUrl_TwoConcurrentSameKey_OneLaunch_OneDeduped`
35. `OpenUrl_BrowserUnavailable_Returns21766`
36. `OpenUrl_LaunchFails_Returns21767_AuditRowStillWritten` (forensic completeness)
37. `OpenUrl_AuditInsertFails_LaunchAlreadyHappened_Returns21768Wrap`
38. `OpenUrl_AlwaysIncognito_NoOptOutPathExists` (lint + reflection test)
39. `OpenUrl_TraceIdPresentInLogAndAuditRow`
40. `OpenUrl_FromRule_OriginRule_RuleNamePopulated`
41. `OpenUrl_FromCard_OriginCard_EmailIdPopulated`
42. `OpenUrl_Cli_OriginCli_AliasMayBeEmpty`

### 11.3 Required test cases (cross-cutting)

43. `Tools_RecentOpenedUrls_FilterByAlias`
44. `Tools_RecentOpenedUrls_FilterByOrigin`
45. `Tools_RecentOpenedUrls_PaginationWithBefore`
46. `Tools_NoOtherFile_ShellsOutToBrowser` — AST scan over `internal/**/*.go` excluding `internal/browser/` and `internal/core/tools.go` for `os/exec` invocations whose first arg matches `(xdg-open|open|start|firefox|chrome|chromium|safari|brave)`.
47. `Tools_NoEmptyInterface_InPackage` — lint enforcement (`linters/no-empty-interface.sh internal/core/tools*.go`).
48. `Tools_AllResultReturns_NoBareError` — AST scan.
49. `Tools_OnAccountUpdate_Idempotent_NoSideEffectsBeyondCache`

### 11.4 Required fakes

```go
// internal/core/fakes_test.go (additions)
type FakeBrowser struct {
    LaunchedUrls   []string
    LaunchErr      error
    DefaultBin     string
    UnavailableErr error
}
type FakeExporter struct {
    Rows           []ExportRow
    WriteErr       error
    FlushErr       error
}
type FakePaths struct {
    AllowedPrefixes []string
}
```

All fakes implement the same interfaces consumed by `Tools` (no `interface{}`).

---

## 12. Compliance Checklist

- [x] PascalCase identifiers throughout (§1, §2, §4 types/funcs).
- [x] No `any` / `interface{}` in `internal/core/tools*.go` (lint Q-01).
- [x] Every public method returns `errtrace.Result[T]` (§2).
- [x] Every function ≤ 15 LOC body (constructor + handlers split into helpers).
- [x] `core.Tools` is the **only** caller of `internal/browser` (test #46).
- [x] No `time.Sleep` — cache TTL via `t.clock.Since`; cancellations via `ctx`.
- [x] `OpenedUrl` table written **only** by Tools (verified by AST scan over `internal/store/`).
- [x] `Incognito = true` always; lint blocks `Incognito = false` literals (test #38).
- [x] All log lines redact `OriginalUrl`; only canonical `Url` appears (§9).
- [x] `Diagnose` cache invalidated on `AccountEvent{Updated|Removed}` (§2.6).
- [x] Path-escape guard on `ExportSpec.OutPath` (`ER-COR-21704`).
- [x] All 20 error codes in §8 registered in `06-error-registry.md` block `21750–21769`.
- [x] Streaming channels closed exactly once by callee (deferred close in every method).
- [x] Performance budgets §10 enforced by named benchmarks.

---

## N. Symbol Map (AC → Go symbol)

Authoritative bridge between the tools `97-acceptance-criteria.md` table and the production Go identifiers an AI implementer must touch. AC tables already name the Go test functions in their right column; this map covers the **service surface, types, and store shims** an implementer needs to satisfy them. **Status legend:** ✅ shipped on `main` · ⏳ planned · 🧪 test-only · 🟡 partial.

### N.1 Service surface (`core.Tools`)

| AC IDs                | Go symbol                                                                            | File                                   | Status |
|-----------------------|--------------------------------------------------------------------------------------|----------------------------------------|:------:|
| F-01..F-07            | `core.Tools` + `NewTools(...) *Tools`                                                | `internal/core/tools.go`               |   ✅   |
| F-10..F-18            | `(*Tools).OpenUrl(ctx, OpenUrlSpec) errtrace.Result[OpenUrlReport]`                  | `internal/core/tools.go`               |   ✅   |
| F-20..F-27            | `(*Tools).RecentOpenedUrls(ctx, OpenedUrlListSpec) errtrace.Result[[]OpenedUrlRow]`  | `internal/core/tools_diagnose.go`      |   ✅   |
| F-30..F-39            | `(*Tools).ExportCsv(ctx, ExportSpec, progress chan<- ExportProgress) errtrace.Result[ExportReport]` | `internal/core/tools_export.go` |   ✅   |
| F-40..F-50            | `(*Tools).Diagnose(ctx, DiagnoseSpec, emit func(DiagnoseEvent)) errtrace.Result[DiagnosticsReport]` | `internal/core/tools_diagnose.go` |   ✅   |
| F-40..F-50            | `(*Tools).CachedDiagnose(...)` *(memoised wrapper)*                                  | `internal/core/tools_diagnose.go`      |   ✅   |
| L-01..L-06            | `(*Tools).WatchAccountEvents(ctx) (stop func())` *(invalidator)*                     | `internal/core/tools_invalidate.go`    |   ✅   |
| F-01..F-07            | `(*Tools).ReadOnce(ctx, ReadSpec, progress chan<- string) errtrace.Result[ReadResult]` | `internal/core/tools_read.go`        |   ✅   |

### N.2 Projection / spec types

| AC IDs              | Go symbol                                                  | File                                | Status |
|---------------------|------------------------------------------------------------|-------------------------------------|:------:|
| F-10..F-18          | `core.OpenUrlSpec`, `core.OpenUrlReport`                   | `internal/core/tools.go`            |   ✅   |
| F-20..F-27          | `core.OpenedUrlListSpec`, `core.OpenedUrlRow`              | `internal/core/tools_diagnose.go`   |   ✅   |
| F-30..F-39          | `core.ExportSpec`, `core.ExportProgress`, `core.ExportReport` | `internal/core/tools_export.go` |   ✅   |
| F-40..F-50          | `core.DiagnoseSpec`, `core.DiagnoseEvent`, `core.DiagnosticsReport` | `internal/core/tools_diagnose.go` | ✅ |
| F-01..F-07          | `core.ReadSpec`, `core.ReadResult`                         | `internal/core/tools_read.go`       |   ✅   |

### N.3 Store / SQL surface

| AC IDs            | Go symbol / SQL artefact                                                          | File                                  | Status |
|-------------------|-----------------------------------------------------------------------------------|---------------------------------------|:------:|
| F-10..F-18, D-01  | `OpenedUrl` table + Delta #1 PascalCase columns + `TraceId`                       | `internal/store/store.go`             |   ✅   |
| F-30..F-39        | `Store.QueryEmailExportRows(ctx, EmailExportFilter) (RowsScanner, error)`         | `internal/store/shims.go`             |   ✅   |
| F-30..F-39        | `Store.CountEmailsFiltered(ctx, EmailExportFilter) (int, error)`                  | `internal/store/shims.go`             |   ✅   |
| F-20..F-27        | `Store.QueryOpenedUrls(ctx, OpenedUrlListFilter) ([]OpenedUrlRow, error)`         | `internal/store/shims.go`             |   ✅   |
| D-01..D-05, Y-01..Y-04 | `store.PruneOpenedUrlsBeforeBatched(ctx, cutoff, batchSize)`                | `internal/store/vacuum.go`            |   ✅   |
| Y-01..Y-04        | `store.Vacuum`, `WalCheckpointTruncate`, `Analyze`, `ShouldAnalyze`               | `internal/store/vacuum.go`            |   ✅   |

### N.4 Redaction & PII

| AC IDs        | Go symbol                                                              | File                                    | Status |
|---------------|------------------------------------------------------------------------|-----------------------------------------|:------:|
| X-01..X-06, S-01..S-08 | `core.redactUrl`, `core.redactCredentials` *(used by every log path)* | `internal/core/tools_redact.go`     |   ✅   |
| X-01..X-06    | `Test_LogScan_NoOriginalUrlLeak` *(slog buffering guard)*              | `internal/core/tools_log_scan_test.go`  |   ✅   |

### N.5 Errors & logging

| AC IDs        | Go symbol                                                              | File                                    | Status |
|---------------|------------------------------------------------------------------------|-----------------------------------------|:------:|
| E-01..E-08    | Codes 21750..21769 per §8 (e.g. `ErrToolsOpenUrlBlockedHost` = 21755)  | `internal/errtrace/codes_gen.go`        |   ⏳   |
| L-01..L-06    | `toolsSlog` (`component=tools`) + `FormatTools*` helpers               | `internal/ui/tools_log.go`              |   ⏳   |

### N.6 Test contract (named tests already declared in AC tables)

| AC IDs           | Test symbol                                                           | File                                            | Status |
|------------------|-----------------------------------------------------------------------|-------------------------------------------------|:------:|
| F-10..F-18       | `TestCF_T1_*`, `TestCF_T2_*`, `TestCF_T3_*`, `TestCF_T4_*`            | `internal/core/cf_acceptance_*.go`              |   ✅   |
| F-20..F-27, L-01..L-06 | `TestCF_AT1_*`, `TestCF_AT2_*`, `TestCF_AT3_*`                  | `internal/core/cf_acceptance_tools_invalidate_test.go` | ✅ |
| F-30..F-39       | `TestExportCsv_*` (per §11 test list)                                 | `internal/core/tools_export_test.go`            |   ✅   |
| F-40..F-50       | `TestDiagnose_*`, `TestCachedDiagnose_*`                              | `internal/core/tools_diagnose_test.go`          |   ✅   |
| P-01..P-11       | `BenchmarkExportCsv_100k`, `BenchmarkOpenUrl_Audit`                   | `internal/core/tools_bench_test.go`             |   ⏳   |
| Q-01..Q-11       | `TestAST_CoreUsesStoreOnly` *(AC-DB-52)*                              | `internal/store/ast_core_uses_store_only_test.go` | ✅   |

---

**End of `06-tools/01-backend.md`**
