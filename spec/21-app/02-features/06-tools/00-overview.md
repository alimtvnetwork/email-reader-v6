# 06 — Tools — Overview

**Version:** 2.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None
**Surface:** Fyne UI + CLI (every sub-tool has parity)

---

## Purpose

The **Tools** feature is the home of every cross-cutting one-shot operation that does not fit cleanly inside Dashboard / Emails / Rules / Accounts / Watch. It owns the four sub-tools `Read`, `ExportCsv`, `Diagnose`, **and `OpenUrl`** — the last of which is the **single audited path** by which `email-read` is permitted to launch the host browser, regardless of whether the launch was triggered by a rule, by a card click, or by a manual paste-URL invocation. Every URL launch goes through `core.Tools.OpenUrl` and produces exactly one `OpenedUrl` audit row. There is no other code path that may shell-out to a browser.

This feature owns: one-shot synchronous reads (`ReadOnce`), CSV export (`ExportCsv`), connection diagnostics (`Diagnose`), incognito browser launches with audit trail (`OpenUrl`), the 60 s diagnose-result cache, and the sub-tool tabbed UI under the `Tools` sidebar entry.

Cross-references:
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.6
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.6 (Tools log lines)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21600–21699` (exporter), `21750–21799` (Tools-core block, this feature claims `21750–21769`); wrapped `ER-STO-21103` (OpenedUrl insert), `ER-MAIL-21200..21209` (diagnose IMAP probes)
- Sibling consumers of `core.Tools.OpenUrl`: `02-features/03-rules` (rule action), `02-features/05-watch` (card hyperlinks)
- Sibling consumer of `core.Tools.ExportCsv`: `02-features/02-emails` (Export selection menu)
- Guidelines: `spec/12-consolidated-guidelines/13-app.md`, `16-app-design-system-and-ui.md`, `23-generic-cli.md`

---

## 1. Scope

### In scope

1. **`ReadOnce(ctx, ReadSpec)`** — synchronous one-shot fetch for an alias up to `Limit` UIDs above the current cursor; returns full `[]EmailSummary` plus a streaming `<-chan string` log channel for incremental UI render. Does NOT advance the watcher cursor (read-only probe).
2. **`ExportCsv(ctx, ExportSpec)`** — write a CSV of `Email` rows for an alias and date range to a user-chosen path; streams progress via `<-chan ExportProgress`; produces an `ExportReport` with `RowsWritten`, `OutPath`, `DurationMs`.
3. **`Diagnose(ctx, alias)`** — five-step IMAP connectivity check (TCP → TLS → LOGIN → SELECT INBOX → MX lookup); returns `DiagnosticsReport` with per-step result + error code; cached 60 s per alias to prevent login-storms; results stream via `<-chan DiagnosticsStep` for live checklist UI.
4. **`OpenUrl(ctx, raw string)`** — the **only** authorised browser-launch path. Validates URL, applies redaction policy, launches the host browser in **incognito-by-default** mode, and writes exactly **one** `OpenedUrl` audit row (with dedup window). Caller-agnostic: rules, cards, manual paste — all funnel through here.
5. **Audit table `OpenedUrl`** — every `OpenUrl` invocation produces one row (or one dedup-skip log line); schema fully owned by Tools (no other feature writes to it).
6. **60-second `Diagnose` cache** — per-alias, in-memory, scoped to a single `Tools` instance; avoids hammering IMAP servers with repeated diagnostic probes.
7. **Sub-tool tabs UI** — single Tools view with `container.NewAppTabs` for Read / Export / Diagnose / OpenUrl forms, each with a streaming output panel.
8. **CLI parity** — every sub-tool exposes a CLI subcommand: `email-read read`, `email-read export-csv`, `email-read doctor`, `email-read open-url`. Identical behaviour, identical exit codes (per `23-generic-cli.md`).
9. **Cancellation** — every sub-tool honours `ctx.Done()` within 500 ms; UI Run buttons swap to "Cancel" while in flight.
10. **Streaming output** — every sub-tool publishes progress on its result channel; UI never blocks waiting for completion.

### Out of scope

- **Mail send** (SMTP). v1 is read-only; deferred to v2.
- **Multi-alias bulk operations** (`Diagnose all`). Looping is a CLI shell concern; v1 sub-tools take exactly one alias.
- **Background scheduling** (cron-like exports). Deferred to v2.
- **Browser-launch with non-incognito flag** outside Tools. There is no opt-out: every launch is incognito.
- **PDF export.** CSV only in v1.
- **`OpenUrl` content fetch / preview.** Tools never fetches the URL itself — it only spawns the browser. Defense against SSRF.
- **JSON / SQLite-dump export formats.** v2.
- **Custom browser binary selection** beyond the `BrowserOverride` setting. The OS default + override is sufficient.

---

## 2. User Stories

| #  | As a … | I want to …                                                          | So that …                                                       |
|----|--------|----------------------------------------------------------------------|-----------------------------------------------------------------|
| 1  | User   | run a one-shot Read without starting the watcher                     | I can sanity-check IMAP credentials before committing to Watch  |
| 2  | User   | export an alias's mail history to CSV                                | I can analyse it in a spreadsheet or hand it to compliance      |
| 3  | User   | watch the export progress live                                        | I know it isn't hanging on a 50 k-row dataset                   |
| 4  | User   | run Diagnose and see exactly which IMAP step fails                    | I fix the right thing (DNS vs auth vs TLS) without guessing     |
| 5  | User   | re-run Diagnose without the app slamming my IMAP server               | the 60 s cache absorbs my impatient clicking                    |
| 6  | User   | click a link inside an email card and have it open in incognito       | trackers can't correlate my mail with browsing history          |
| 7  | Auditor| query the `OpenedUrl` table to see every URL the app ever opened      | I can reconstruct the action timeline for any incident          |
| 8  | User   | paste a URL into the OpenUrl tab and have it open + audited           | even manual launches go through the same audit pipeline         |
| 9  | User   | run any tool from the CLI with identical behaviour                    | I can script `email-read doctor work` in a health-check cron    |
| 10 | User   | cancel a long-running export with the Cancel button                   | I'm never trapped waiting                                       |
| 11 | Operator| see an incognito-launch failure as a structured error                | I diagnose missing browser binaries on locked-down systems      |

---

## 3. Dependencies

| Dependency             | Why                                                                       |
|------------------------|---------------------------------------------------------------------------|
| `core.Accounts`        | Read alias list for sub-tool dropdowns; resolve `Alias → AccountSpec`     |
| `core.Emails`          | `ExportCsv` reads `Email` rows; `ReadOnce` calls into `PersistFromImap` (in non-mutating probe mode) |
| `internal/mailclient`  | (transitive) IMAP dial / LOGIN / SELECT for `Diagnose` and `ReadOnce`     |
| `internal/exporter`    | (transitive) CSV writer with streaming progress                           |
| `internal/store`       | (transitive) `OpenedUrl` insert + dedup index; `Email` SELECT for export  |
| `internal/browser`     | (transitive) OS-specific incognito launcher (xdg-open / open / start)     |
| `internal/ui/theme`    | Token usage in sub-tool forms + checklist colors                          |
| `internal/cli`         | CLI sibling that calls the same `core.Tools` methods                      |

The view (`internal/ui/views/tools.go`) **must not** import `internal/exporter`, `internal/mailclient`, `internal/store`, or `internal/browser` directly. All access goes through `core.Tools`.

---

## 4. Data Model

All names PascalCase per `04-coding-standards.md` §1.1.

### 4.1 Public types (exposed via `core.Tools`)

```go
// ---------- Read ----------

type ReadSpec struct {
    Alias string  // required; must exist in core.Accounts.List
    Limit int     // 1..500; default 10; rejected outside range with 21750
}

type ReadResult struct {
    Alias       string
    Emails      []EmailSummary  // fetched; same shape as Emails feature
    StartedAt   time.Time
    DurationMs  int
    UidsScanned int
}

// ---------- Export ----------

type ExportSpec struct {
    Alias    string     // required
    From     time.Time  // inclusive; UTC
    To       time.Time  // exclusive; UTC; must be > From
    OutPath  string     // absolute; must be inside the user's documents dir or app data dir (validated via core.paths)
    Overwrite bool       // default false; if false and OutPath exists, returns 21753
}

type ExportProgress struct {
    RowsWritten int
    TotalRows   int      // 0 until SELECT COUNT(*) completes (≤ 200 ms in)
    Phase       ExportPhase
}

type ExportPhase string
const (
    ExportPhaseCounting ExportPhase = "Counting"
    ExportPhaseWriting  ExportPhase = "Writing"
    ExportPhaseFlushing ExportPhase = "Flushing"
    ExportPhaseDone     ExportPhase = "Done"
)

type ExportReport struct {
    OutPath     string
    RowsWritten int
    DurationMs  int
    StartedAt   time.Time
    FinishedAt  time.Time
}

// ---------- Diagnose ----------

type DiagnosticsReport struct {
    Alias        string
    StartedAt    time.Time
    DurationMs   int
    Steps        []DiagnosticsStep   // in execution order; len == 5 on success, ≤ 5 on early-fail
    Cached       bool                // true if served from 60 s cache
    OverallOk    bool                // every step Status == Pass
}

type DiagnosticsStep struct {
    Name      DiagnosticsStepName
    Status    DiagnosticsStepStatus
    DurationMs int
    Detail    string                 // human-readable (e.g. "messages=1842 uidNext=9012")
    Err       *DiagnosticsErr        // populated when Status == Fail
}

type DiagnosticsStepName string
const (
    DiagnosticsStepDnsLookup     DiagnosticsStepName = "DnsLookup"      // MX/A resolution
    DiagnosticsStepTcpConnect    DiagnosticsStepName = "TcpConnect"
    DiagnosticsStepTlsHandshake  DiagnosticsStepName = "TlsHandshake"
    DiagnosticsStepImapLogin     DiagnosticsStepName = "ImapLogin"
    DiagnosticsStepInboxSelect   DiagnosticsStepName = "InboxSelect"    // FETCH messages count + UIDNEXT
)

type DiagnosticsStepStatus string
const (
    DiagnosticsStepPending DiagnosticsStepStatus = "Pending"   // not yet run (UI placeholder)
    DiagnosticsStepRunning DiagnosticsStepStatus = "Running"
    DiagnosticsStepPass    DiagnosticsStepStatus = "Pass"
    DiagnosticsStepFail    DiagnosticsStepStatus = "Fail"
    DiagnosticsStepSkipped DiagnosticsStepStatus = "Skipped"   // earlier step failed; this step never attempted
)

type DiagnosticsErr struct {
    Code    string  // e.g. "ER-MAIL-21201"
    Message string  // user-facing, redacted
}

// ---------- OpenUrl ----------

type OpenUrlReport struct {
    Url            string             // canonical (post-normalization) URL actually handed to the browser
    Origin         OpenUrlOrigin      // who triggered this open
    OpenedAt       time.Time
    BrowserBinary  string             // resolved binary path (e.g. "/usr/bin/firefox")
    Incognito      bool               // ALWAYS true in v1; field exists for forward-compat audit
    Deduped        bool               // true if a matching OpenedUrl row was found within DedupWindow → no second open
    DedupWindowSec int                // 60 (default; from config.Settings.OpenUrlDedupSeconds)
}

type OpenUrlOrigin string
const (
    OpenUrlOriginRule    OpenUrlOrigin = "Rule"     // triggered by core.Rules.Engine (Action == OpenUrl)
    OpenUrlOriginCard    OpenUrlOrigin = "Card"     // triggered by Watch / Emails card click
    OpenUrlOriginManual  OpenUrlOrigin = "Manual"   // user pasted into Tools → OpenUrl tab
    OpenUrlOriginCli     OpenUrlOrigin = "Cli"      // `email-read open-url` subcommand
)
```

### 4.2 OpenedUrl audit table (canonical schema)

Owned **exclusively** by Tools. No other feature writes to this table.

```sql
CREATE TABLE OpenedUrl (
    Id              INTEGER PRIMARY KEY AUTOINCREMENT,
    Alias           TEXT    NOT NULL,                 -- "" for OriginManual / OriginCli without alias
    Url             TEXT    NOT NULL,                 -- canonical, post-redaction
    OriginalUrl     TEXT    NOT NULL,                 -- pre-redaction, pre-normalization (for audit completeness)
    Origin          TEXT    NOT NULL,                 -- one of OpenUrlOrigin enum values
    RuleName        TEXT    NOT NULL DEFAULT '',      -- populated when Origin == 'Rule'
    EmailId         INTEGER NOT NULL DEFAULT 0,       -- populated when triggered by an email; 0 otherwise
    OpenedAt        DATETIME NOT NULL,
    BrowserBinary   TEXT    NOT NULL,
    IsIncognito     INTEGER NOT NULL DEFAULT 1,       -- positive boolean per 18-database-conventions §4
    IsDeduped       INTEGER NOT NULL DEFAULT 0,       -- 1 if this row records a dedup-skip (no actual launch)
    TraceId         TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IxOpenedUrlAliasOpenedAt ON OpenedUrl(Alias, OpenedAt DESC);
CREATE INDEX IxOpenedUrlOrigin        ON OpenedUrl(Origin, OpenedAt DESC);
CREATE UNIQUE INDEX IxOpenedUrlsUnique ON OpenedUrl(Alias, Url, OpenedAt);  -- collision-free per (alias, url, ts)
```

Cross-feature notes:
- `Rules` reads `OpenedUrl` for its hit-history view but **never** writes (per `02-features/03-rules` X-02).
- `Accounts.Delete` does NOT cascade to `OpenedUrl` — audit history must survive account removal (`OpenedUrl.Alias` is intentionally not a FK to `Account.Alias`).
- `Emails.Delete` does NOT cascade to `OpenedUrl` for the same reason.
- The schema is **owned** by `spec/23-app-database/` (Task #32) but the canonical definition lives here in §4.2.

### 4.3 Default values

```go
ReadSpec{Limit: 10}
ExportSpec{Overwrite: false}
// OpenUrl defaults read from config.Settings:
//   OpenUrlDedupSeconds:        60
//   BrowserOverride:            ""    (use OS default)
//   OpenUrlAllowedSchemes:      ["https", "http"]   // mailto / file / javascript blocked
//   OpenUrlMaxLengthBytes:      8192
```

### 4.4 Persistence shape

The Tools feature persists exactly two things:

1. **`OpenedUrl` SQLite rows** (one per `OpenUrl` invocation, including dedup skips).
2. **Nothing else.** `Diagnose` cache is in-memory only (lost on app restart — intentional).

`ExportCsv` writes a CSV file at `OutPath`; that file lives entirely on disk under the user's chosen directory. Tools does NOT track exports in the database.

---

## 5. URL Validation & Redaction Policy 🔴

`OpenUrl` is the only browser-launch path; its validation must be airtight. This is the **security-critical** subsystem of Tools.

### 5.1 Validation pipeline (in order; first failure short-circuits)

| Step | Check                                                        | On failure |
|------|--------------------------------------------------------------|------------|
| 1    | URL is non-empty                                             | `21760 OpenUrlEmpty` |
| 2    | URL length ≤ `OpenUrlMaxLengthBytes` (8192 default)          | `21761 OpenUrlTooLong` |
| 3    | Parses cleanly via `net/url.Parse`                           | `21762 OpenUrlMalformed` |
| 4    | Scheme ∈ `OpenUrlAllowedSchemes` (default `https`/`http`)    | `21763 OpenUrlSchemeForbidden` (catches `javascript:`, `file:`, `data:`, `mailto:`) |
| 5    | Host is non-empty                                            | `21762 OpenUrlMalformed` |
| 6    | Host is not a literal IP loopback (`127.0.0.1`, `::1`, `localhost`) UNLESS `Settings.AllowLocalhostUrls == true` (default false) | `21764 OpenUrlLocalhostBlocked` |
| 7    | Host does not match the SSRF block list (`169.254.0.0/16`, `10.0.0.0/8`, `192.168.0.0/16`, `172.16.0.0/12`) UNLESS `Settings.AllowPrivateIpUrls == true` | `21765 OpenUrlPrivateIpBlocked` |

### 5.2 Redaction (applied **after** validation, **before** persist + browser launch)

| Pattern                                       | Replacement                                         |
|-----------------------------------------------|------------------------------------------------------|
| Query/path segments matching OTP regex (`\b\d{4,8}\b` next to `code=`, `otp=`, `token=`) | redacted to `***` in the **logged + persisted `Url` column**; original kept in `OriginalUrl` |
| `password=` / `pwd=` / `secret=` query keys   | value replaced with `***` in `Url`; original in `OriginalUrl` |
| URL fragment (`#…`)                           | preserved verbatim (browser strips before sending; not a leak) |
| `userinfo` (`https://user:pass@…`)            | stripped from `Url`; original in `OriginalUrl`; user warned via WARN log |

The split between `Url` and `OriginalUrl` is deliberate: dashboards, logs, and screen-shareable views all use `Url` (safe); audit forensics has access to `OriginalUrl` for completeness.

### 5.3 Dedup window

- For `(Alias, Url)` pairs where a row exists in `OpenedUrl` with `OpenedAt > now - DedupWindowSec`, `OpenUrl` does NOT spawn the browser.
- It still writes a row with `IsDeduped = 1` so the audit trail records the *attempt*.
- Returns success with `OpenUrlReport.Deduped = true` so the caller knows nothing happened.

### 5.4 Concurrency invariant

Two simultaneous `OpenUrl` calls for the same `(Alias, Url)` are serialised by a per-key mutex; only one browser launch happens; the second sees `Deduped = true` (assuming it lands within the window).

---

## 6. Diagnose 60-Second Cache

| Property             | Value                                                                  |
|----------------------|------------------------------------------------------------------------|
| Scope                | Per `Tools` instance, per `Alias`                                      |
| Storage              | In-memory `sync.Map[string, cacheEntry]`                               |
| TTL                  | 60 s (constant; not user-configurable in v1)                           |
| Eviction             | Lazy on read (compare `OpenedAt + 60s < now`)                          |
| Bypass               | `DiagnoseSpec{Force: true}` — UI "Re-run" button after manual edit     |
| Concurrency          | `sync.Mutex` per alias prevents two concurrent diagnose runs hitting IMAP |
| Reset                | Cleared on `core.Accounts.Update(alias)` (creds may have changed)      |

Cache rationale: a user impatient-clicking "Diagnose" 5× should not produce 5 IMAP LOGINs. The 60 s window is long enough to absorb impatience but short enough to be useful for "I just fixed my password, retry" — which is exactly what `Force: true` is for.

---

## 7. Refresh & Live-Update

| Trigger                                              | Action                                                     |
|------------------------------------------------------|------------------------------------------------------------|
| Tab opened                                           | Read alias options from `core.Accounts.List` once          |
| `AccountEvent{Added/Removed/Renamed}`                | Refresh alias dropdowns in all four sub-tool tabs          |
| `AccountEvent{Updated, Alias}`                       | Invalidate that alias's diagnose cache entry               |
| Sub-tool Run pressed                                 | Spawn goroutine; switch button to Cancel; bind streaming channel to output panel |
| Cancel pressed                                       | Cancel ctx; goroutine drains channel + closes              |
| Tool completes                                       | Render terminal report; switch button back to Run          |

The Tools view subscribes to `core.Accounts.Subscribe()` (lightweight; only updates dropdowns + cache). It does NOT subscribe to `core.Watch` events.

---

## 8. Acceptance Snapshot (full criteria in `97-acceptance-criteria.md`)

A merged Tools build is shippable iff:

1. **`OpenUrl` audit invariant**: every browser launch (rule, card, manual, CLI) produces exactly one `OpenedUrl` row; verified by reflection over all `core.Tools.OpenUrl` callers.
2. **Validation**: `javascript:`, `file:`, `data:`, `mailto:` URLs are blocked with `21763`; private IPs blocked with `21765` unless explicitly enabled.
3. **Redaction**: passwords + OTPs in URLs are masked in logs + persisted `Url` column; `OriginalUrl` retains pre-redaction value.
4. **Dedup**: identical `(Alias, Url)` within 60 s spawns browser **once**; second call returns `Deduped = true` and writes `IsDeduped = 1` audit row.
5. **Diagnose cache**: 5 sequential diagnose calls within 60 s = 1 IMAP LOGIN.
6. **Streaming**: every sub-tool publishes progress on its channel before completion; UI verified to render mid-stream.
7. **Cancellation**: cancelling any tool stops it within **500 ms**.
8. **CLI parity**: each sub-tool exposes a CLI subcommand with identical behaviour and exit codes.
9. **Audit survives deletion**: deleting an `Account` or `Email` does NOT delete its `OpenedUrl` rows.
10. **Incognito always**: `Incognito = true` in every `OpenUrlReport`; lint forbids `Incognito = false` literals.
11. **Single browser-launch path**: AST scan asserts no other file in the repo calls `os/exec` with browser binaries (`xdg-open`, `open`, `start`, `firefox`, `chrome`, etc.).
12. **Zero `interface{}` / `any`** in any new code (lint-enforced).
13. **Diagnose checklist UI**: 5 step rows render in order; first failure marks subsequent steps `Skipped` (not `Pending`).

---

## 9. Open Questions

None. Confidence: Production-Ready.

Explicit **deferrals** (not ambiguities):
- SMTP send → v2.
- PDF / JSON / SQLite-dump exports → v2.
- Background scheduled exports → v2.
- Browser launch with non-incognito flag → v2 (would require explicit user opt-in per launch + a new audit field).
- Custom redaction rules per user → v2 (current rules are hard-coded best practice).
- IPv6 SSRF block list → v2 (v1 covers IPv4 RFC1918 + loopback).
- Multi-alias Diagnose / Export → v2.

---

**End of `06-tools/00-overview.md`**
