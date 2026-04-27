# 06 — Tools — Acceptance Criteria

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Binary, machine-checkable acceptance criteria for the **Tools** feature (overview / backend / frontend). A merged build is shippable iff **every** check passes in CI. Each row maps to a concrete test in `internal/{core,ui/views}/tools_*_test.go` or a benchmark in `*_bench_test.go`.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend: [`./01-backend.md`](./01-backend.md)
- Frontend: [`./02-frontend.md`](./02-frontend.md)
- Error registry: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21750–21769`
- Sibling consumers (must remain consistent): `02-features/03-rules`, `02-features/05-watch`


<!-- sandbox-feasibility-legend v1 -->

## Sandbox feasibility legend

Each criterion below is tagged for the implementing AI so it can pick sandbox-implementable rows first:

| Tag | Meaning | Where it runs |
|---|---|---|
| 🟢 | **headless** — pure Go logic, AST scanner, SQL, registry, lint rule, errtrace, code-quality check | Sandbox: `nix run nixpkgs#go -- test -tags nofyne ./...` |
| 🟡 | **cgo-required** — Fyne canvas / widget render / focus ring / hover / pulse / pixel contrast / screen-reader runtime | Workstation only (CGO + display server) |
| 🔴 | **bench / E2E** — perf gate (`P-*`), benchmark, race detector under UI, multi-process integration | CI infra only |

See also: [`mem://design/schema-naming-convention.md`](mem://design/schema-naming-convention.md), `.lovable/cicd-issues/03-fyne-canvas-needs-cgo.md`, `.lovable/cicd-issues/05-no-bench-infra.md`.

---

## 1. Functional — Read

| #    | Criterion                                                                                  | Test                                                  |
|------|--------------------------------------------------------------------------------------------|-------------------------------------------------------|
| F-01 | 🟡 `ReadOnce(spec)` returns `[]EmailSummary` of length `min(spec.Limit, available)`.          | `Tools_ReadOnce_Happy_StreamsProgress_NoCursorWrite`  |
| F-02 | 🟡 `ReadOnce` does NOT update `WatchState.LastSeenUid` (read-only probe).                     | `Tools_ReadOnce_Happy_StreamsProgress_NoCursorWrite`  |
| F-03 | 🟡 `Limit` outside `1..500` returns `21750 ToolsInvalidArgument`.                             | `Tools_ReadOnce_LimitOutOfRange_Returns21750`         |
| F-04 | 🟡 Missing alias returns `21703 AccountNotFound` (forwarded).                                 | `Tools_ReadOnce_AccountMissing_Returns21703`          |
| F-05 | 🟡 IMAP fetch failure returns `21751 ToolsReadFetchFailed` wrapping the underlying `ER-MAIL-*`. | `Tools_ReadOnce_FetchFails_Returns21751WithMailWrap` |
| F-06 | 🟡 Progress channel closed exactly once by callee on return (success OR error).               | `Tools_ReadOnce_ProgressChannelClosedExactlyOnce`     |
| F-07 | 🟡 Cancellation propagates within **500 ms**.                                                 | `Tools_ReadOnce_CtxCancel_StopsUnder500ms`            |

## 2. Functional — Export CSV

| #    | Criterion                                                                                  | Test                                                  |
|------|--------------------------------------------------------------------------------------------|-------------------------------------------------------|
| F-10 | 🟡 `ExportCsv` writes one row per matching `Email`, headers row first.                        | `Tools_ExportCsv_Happy_RowCountMatchesQuery`          |
| F-11 | 🟡 Phases emit in order: `Counting` → `Writing` → `Flushing` → `Done`.                        | `Tools_ExportCsv_Happy_StreamsCountWritingFlushingDone` |
| F-12 | 🟡 `Writing` progress publishes every **256 rows**, not per row.                              | `Tools_ExportCsv_ProgressEvery256Rows_NotPerRow`      |
| F-13 | 🟢 `OutPath` outside the user data dir → `ER-COR-21704`.                                      | `Tools_ExportCsv_PathOutsideData_Returns21704`        |
| F-14 | 🟡 `OutPath` exists with `Overwrite=false` → `21753 ToolsExportPathExists`.                   | `Tools_ExportCsv_PathExistsNoOverwrite_Returns21753`  |
| F-15 | 🟢 Disk-full mid-write returns `ER-EXP-21602` wrapped; partial file removed (best-effort).    | `Tools_ExportCsv_DiskFull_Returns21602Wrap_PartialFileRemoved` |
| F-16 | 🟡 Cancellation under 500 ms; partial file removal attempted.                                 | `Tools_ExportCsv_CtxCancel_Under500ms_PartialFileRemovalAttempted` |
| F-17 | 🟡 `ExportReport.RowsWritten` matches the `Counting` phase total on success.                  | `Tools_ExportCsv_FinalCountMatchesPhase1Count`        |
| F-18 | 🟡 UI Browse default path = `~/Documents/email-read-export-{alias}-{ts}.csv`.                 | `ToolsVM_Export_BrowseDefaultsToDocumentsPath`        |

## 3. Functional — Diagnose

| #    | Criterion                                                                                  | Test                                                  |
|------|--------------------------------------------------------------------------------------------|-------------------------------------------------------|
| F-20 | 🟡 Five steps run in order: `DnsLookup` → `TcpConnect` → `TlsHandshake` → `ImapLogin` → `InboxSelect`. | `Tools_Diagnose_Happy_5Steps_AllPass`        |
| F-21 | 🟡 First failure marks all subsequent steps `Skipped` (not `Pending`).                        | `Tools_Diagnose_DnsFails_RemainingMarkedSkipped`      |
| F-22 | 🟡 Cache hit within 60 s returns identical report with `Cached = true`; **zero** IMAP calls.  | `Tools_Diagnose_CacheHit_NoImapCalls`                 |
| F-23 | 🟡 `Force = true` bypasses cache; one IMAP login occurs.                                      | `Tools_Diagnose_Force_BypassesCache`                  |
| F-24 | 🟡 `OnAccountUpdate(alias)` invalidates that alias's cache entry.                             | `Tools_Diagnose_OnAccountUpdate_InvalidatesCacheEntry` |
| F-25 | 🟡 Two concurrent `Diagnose(alias)` calls produce **one** IMAP login (per-alias mutex).       | `Tools_Diagnose_TwoConcurrentSameAlias_OneImapLogin`  |
| F-26 | 🟡 Each step emits one `DiagnosticsStep` with `Running` then `Pass`/`Fail`/`Skipped`.         | `Tools_Diagnose_StepsEmit_Running_Then_Terminal`      |
| F-27 | 🟡 UI pre-seeds 5 `Pending` rows so checklist renders immediately on Run.                     | `ToolsVM_Diag_PreSeeds5PendingRows`                   |

## 4. Functional — OpenUrl (security-critical 🔴)

| #    | Criterion                                                                                  | Test                                                  |
|------|--------------------------------------------------------------------------------------------|-------------------------------------------------------|
| F-30 | 🟡 Empty URL → `21760 OpenUrlEmpty`.                                                          | `OpenUrl_Empty_Returns21760`                          |
| F-31 | 🟡 URL > 8192 bytes → `21761 OpenUrlTooLong`.                                                 | `OpenUrl_TooLong_Returns21761`                        |
| F-32 | 🟡 Malformed URL → `21762 OpenUrlMalformed`.                                                  | `OpenUrl_Malformed_Returns21762`                      |
| F-33 | 🟡 `javascript:` / `file:` / `data:` / `mailto:` → `21763 OpenUrlSchemeForbidden`.            | `OpenUrl_{Javascript,File,Data,Mailto}Scheme_Returns21763` |
| F-34 | 🟡 Localhost URLs → `21764 OpenUrlLocalhostBlocked` unless `Settings.AllowLocalhostUrls`.     | `OpenUrl_Localhost_Returns21764_WhenAllowFalse` + `..._Allowed_WhenSettingTrue` |
| F-35 | 🟡 All RFC1918 ranges (`10/8`, `172.16/12`, `192.168/16`, `169.254/16`) → `21765`.            | `OpenUrl_PrivateIp_Returns21765_AllRfc1918Ranges`     |
| F-36 | 🟡 OTP-like values redacted in `Url`; `OriginalUrl` retains original.                          | `OpenUrl_OtpRedacted_InUrl_OriginalKept`              |
| F-37 | 🟡 `password=` / `pwd=` / `secret=` / `token=` query keys masked to `***` in `Url`; `OriginalUrl` retains. | `OpenUrl_PasswordRedacted_InUrl_OriginalKept` |
| F-38 | 🟡 Userinfo (`https://u:p@host`) stripped; one WARN log; URL fields **omitted** from log.     | `OpenUrl_UserinfoStripped_WarnLogged_NoUrlInLog`      |
| F-39 | 🟡 Dedup hit (same `(Alias, Url)` within 60 s) → no browser launch; audit row with `IsDeduped=1`. | `OpenUrl_DedupHit_NoLaunch_AuditRowWithDeduped1` |
| F-40 | 🟡 Dedup miss → exactly one browser launch + one audit row with `IsDeduped=0`.                | `OpenUrl_DedupMiss_LaunchAndAudit`                    |
| F-41 | 🟡 Two concurrent same-`(Alias,Url)` calls → exactly one launch; second sees `Deduped=true`.  | `OpenUrl_TwoConcurrentSameKey_OneLaunch_OneDeduped`   |
| F-42 | 🟡 Browser binary unavailable → `21766 OpenUrlBrowserUnavailable`.                            | `OpenUrl_BrowserUnavailable_Returns21766`             |
| F-43 | 🟡 Browser launch failure → `21767 OpenUrlLaunchFailed`; **audit row still written**.          | `OpenUrl_LaunchFails_Returns21767_AuditRowStillWritten` |
| F-44 | 🟢 Audit insert failure post-launch → `21768 OpenUrlAuditInsertFailed` wrapping `ER-STO-21103`. | `OpenUrl_AuditInsertFails_LaunchAlreadyHappened_Returns21768Wrap` |
| F-45 | 🟡 `Incognito = true` in **every** `OpenUrlReport`. No code path produces `Incognito = false`. | `OpenUrl_AlwaysIncognito_NoOptOutPathExists` (lint + reflection) |
| F-46 | 🟡 `TraceId` present and identical in INFO log line + persisted `OpenedUrl.TraceId`.          | `OpenUrl_TraceIdPresentInLogAndAuditRow`              |
| F-47 | 🟡 `OriginRule` populates `RuleName`; `OriginCard` populates `EmailId`; `OriginCli`/`OriginManual` may have empty `Alias`. | `OpenUrl_From{Rule,Card,Cli,Manual}_FieldsPopulated` |
| F-48 | 🟡 UI Recent panel has **no** re-open affordance (anti-feature).                              | `ToolsVM_OpenUrl_RecentPanel_HasNoReopenButton`       |
| F-49 | 🟡 UI OpenUrl tab pins `Origin = Manual`; not user-editable.                                  | `ToolsVM_OpenUrl_OriginPinnedManual_NotEditable`      |
| F-50 | 🟡 Friendly toast text exists for every code in `21760..21768` (no raw stack to user).        | `ToolsVM_OpenUrl_FriendlyErrorMapping_AllCodes_21760_to_21768` |

## 5. Audit Trail Invariants 🔴 (single-handedly blocks merge)

| #    | Criterion                                                                                  | Test                                                  |
|------|--------------------------------------------------------------------------------------------|-------------------------------------------------------|
| A-01 | 🟡 **Every** browser launch in the codebase goes through `core.Tools.OpenUrl`. AST scan over `internal/**/*.go` excluding `internal/browser/` and `internal/core/tools.go` finds **zero** `os/exec` invocations whose first arg matches `(xdg-open\|open\|start\|firefox\|chrome\|chromium\|safari\|brave)`. | `Tools_NoOtherFile_ShellsOutToBrowser` |
| A-02 | 🟢 **Every** `OpenedUrl` row insert in the codebase happens inside `internal/core/tools.go`. AST scan over `internal/store/` shows the `INSERT INTO OpenedUrl` SQL is referenced only by a function called from `core.Tools.OpenUrl`. | `Store_OpenedUrlInsert_OnlyCalledByTools` |
| A-03 | 🟡 Audit row written for **launch success**, **launch failure**, AND **dedup skip**. Three distinct test cases exist. | F-40, F-43, F-39 |
| A-04 | 🟡 `OpenedUrl` rows survive `Account.Delete` (no FK cascade).                                 | `Store_OpenedUrl_SurvivesAccountDelete`               |
| A-05 | 🟡 `OpenedUrl` rows survive `Email.Delete` (no FK cascade).                                   | `Store_OpenedUrl_SurvivesEmailDelete`                 |
| A-06 | 🟡 Watch (`02-features/05-watch`) hyperlink clicks route through `core.Tools.OpenUrl` with `Origin=Card`, `EmailId` populated. | `WatchVM_HyperlinkClickGoesViaCoreTools` (in Watch tests; cross-referenced) |
| A-07 | 🟡 Rules engine `Action == OpenUrl` calls `core.Tools.OpenUrl` once per extracted URL with `Origin=Rule`, `RuleName` populated. | `Watcher_RuleMatch_OpenUrl_CallsToolsOpenUrlPerExtractedUrl` (Watch backend; cross-referenced) |
| A-08 | 🟡 CLI `email-read open-url` calls `core.Tools.OpenUrl` with `Origin=Cli`.                    | `Cli_OpenUrl_OriginCli` (CLI test; cross-referenced)  |

## 6. Performance (CI-gated benchmarks)

| #    | Op                                                       | Budget        | Bench                              |
|------|----------------------------------------------------------|---------------|------------------------------------|
| P-01 | 🔴 `ReadOnce` 10 UIDs warm IMAP                             | ≤ 2 s p95     | `BenchmarkReadOnce10`              |
| P-02 | 🔴 `ExportCsv` 10 k rows local FS                           | ≤ 3 s p95     | `BenchmarkExportCsv10k`            |
| P-03 | 🔴 `Diagnose` 5 steps warm DNS                              | ≤ 1.5 s p95   | `BenchmarkDiagnose`                |
| P-04 | 🔴 `Diagnose` cache hit                                     | ≤ 2 ms        | `BenchmarkDiagnoseCacheHit`        |
| P-05 | 🔴 `OpenUrl` dedup-hit (no launch)                          | ≤ 5 ms        | `BenchmarkOpenUrlDedupHit`         |
| P-06 | 🔴 `OpenUrl` validate + redact + launch + audit             | ≤ 80 ms p95   | `BenchmarkOpenUrlLaunch`           |
| P-07 | 🔴 `RecentOpenedUrls` 100 rows                              | ≤ 8 ms        | `BenchmarkRecentOpenedUrls`        |
| P-08 | 🟢 Cancellation latency (any sub-tool)                      | ≤ 500 ms      | `BenchmarkCancelLatency`           |
| P-09 | 🔴 UI `Attach` first paint                                  | ≤ 80 ms       | `BenchmarkToolsAttach`             |
| P-10 | 🟡 UI tab switch                                            | ≤ 8 ms        | `BenchmarkToolsTabSwitch`          |
| P-11 | 🔴 UI memory (`readOutput + openRecent`) after 1 h synthetic | ≤ 1 MiB      | `TestToolsMemoryCeiling`           |

## 7. Code Quality

| #    | Criterion                                                                                  | How verified                       |
|------|--------------------------------------------------------------------------------------------|------------------------------------|
| Q-01 | 🟢 Zero `interface{}` / `any` in `internal/core/tools*.go` and `internal/ui/views/tools.go`.  | `linters/no-empty-interface.sh`    |
| Q-02 | 🟢 Every public method returns `errtrace.Result[T]`.                                          | `linters/result-only.sh`           |
| Q-03 | 🟢 Every function ≤ 15 LOC body.                                                              | `linters/fn-length.sh`             |
| Q-04 | 🟢 No `time.Sleep` in production code (cache TTL via `clock.Since`; cancels via `ctx`).       | `linters/no-time-sleep.sh`         |
| Q-05 | 🟢 No `os/exec`, `net/http`, `database/sql` in `internal/ui/views/tools.go`.                  | `golangci-lint forbidigo`          |
| Q-06 | 🟢 No browser-binary string literals (`xdg-open`, `open`, `start`, etc.) outside `internal/browser/`. | `golangci-lint forbidigo`     |
| Q-07 | 🟢 All identifiers PascalCase.                                                                | `linters/pascalcase.sh`            |
| Q-08 | 🟢 `golangci-lint exhaustive` passes for `diagStatusToken` map and any switch over `OpenUrlOrigin` / `DiagnosticsStepStatus`. | `golangci-lint run`     |
| Q-09 | 🔴 `goleak.VerifyNone(t)` in `TestMain` of `internal/core` and `internal/ui/views`.           | Test code review                   |
| Q-10 | 🟢 No hex color literals in `tools.go` (AST scan).                                             | `ToolsVM_NoHardcodedColors`        |
| Q-11 | 🟢 `Incognito = false` literal forbidden in any source file.                                  | `linters/no-incognito-false.sh`    |

## 8. Security & PII

| #    | Criterion                                                                                  | Test                                              |
|------|--------------------------------------------------------------------------------------------|---------------------------------------------------|
| S-01 | 🟢 `OriginalUrl` is **never** logged at any level (only persisted to DB column).              | `Logging_NeverContainsOriginalUrl`                |
| S-02 | 🟢 `Url` field in any log line is the canonical (post-redacted) URL.                          | `Logging_UrlAlwaysCanonical`                      |
| S-03 | 🟡 `OsError` strings stripped of home paths (`/home/{user}/`, `/Users/{user}/`) and userinfo bytes. | `Logging_OsErrorRedaction`                  |
| S-04 | 🟡 `userinfo` URLs strip credentials; WARN log omits the `Url` field entirely (only `Alias`, `Origin`, `TraceId`). | F-38 |
| S-05 | 🟡 OTP/password redaction tested with both query-string and path variants.                    | F-36, F-37 + `OpenUrl_RedactionPathSegment`        |
| S-06 | 🟡 Per-key mutex prevents concurrent same-key launches (no race-double-spawn).                | F-41                                              |
| S-07 | 🟡 Browser launches are **always** incognito; no code path produces `Incognito = false`.      | F-45 + Q-11                                       |
| S-08 | 🟢 Path-escape on `ExportSpec.OutPath` blocked with `ER-COR-21704`.                           | F-13                                              |

## 9. Logging

| #    | Criterion                                                                                  | Test                                              |
|------|--------------------------------------------------------------------------------------------|---------------------------------------------------|
| L-01 | 🟢 One INFO `OpenUrlLaunched` per successful launch with all fields per `01-backend.md` §9.   | `Logging_OpenUrlLaunched_FullFields`              |
| L-02 | 🟢 One INFO `OpenUrlDeduped` per dedup-skip; no `OpenUrlLaunched` for dedup-skips.            | `Logging_OpenUrlDeduped_NoLaunchedLine`           |
| L-03 | 🟢 One WARN `OpenUrlUserinfoStripped` when userinfo present; no `Url` field in that line.     | F-38                                              |
| L-04 | 🟢 One WARN `OpenUrlAuditInsertFailed` per `21768`; carries `ErrCode=ER-STO-21103`.           | F-44                                              |
| L-05 | 🟢 `TraceId` matches across log line and `OpenedUrl.TraceId` for the same operation.          | F-46                                              |
| L-06 | 🟢 `Diagnose` emits one INFO `ToolsDiagnoseStep` per step with `Detail` (Pass) or `ErrCode` (Fail). | `Logging_DiagnoseStep_PerStep`              |

## 10. Database

| #    | Criterion                                                                                  | Test                                              |
|------|--------------------------------------------------------------------------------------------|---------------------------------------------------|
| D-01 | 🟢 `OpenedUrl` schema matches `01-backend.md` §5 verbatim (column names, types, defaults).    | `Schema_OpenedUrl_MatchesSpec`                    |
| D-02 | 🟢 `IsIncognito` and `IsDeduped` are positive booleans per `18-database-conventions.md` §4.    | D-01 (covers)                                     |
| D-03 | 🟢 `IxOpenedUrlsUnique(Alias, Url, OpenedAt)` exists and is enforced.                         | `Schema_OpenedUrlsUnique_Index`                   |
| D-04 | 🟢 `IxOpenedUrlAliasOpenedAt(Alias, OpenedAt DESC)` exists and used by `Q-OPEN-DEDUP`.        | `Schema_OpenedUrl_DedupIndexUsed` (EXPLAIN check) |
| D-05 | 🟢 No FK from `OpenedUrl.Alias` to `Account.Alias` (intentional; audit survives delete).      | A-04                                              |

## 11. Atomicity & Safety

| #    | Criterion                                                                                  | Test                                              |
|------|--------------------------------------------------------------------------------------------|---------------------------------------------------|
| X-01 | Browser launch + audit insert NOT in single tx; launch first, audit second; documented.    | `Tools_OpenUrl_LaunchBeforeAudit_DocumentedDecision` |
| X-02 | Concurrent same-key calls serialised by `keyedMutex`; one launch, one dedup audit.         | F-41                                              |
| X-03 | Concurrent same-alias `Diagnose` serialised; one IMAP login.                               | F-25                                              |
| X-04 | Cancellation mid-Export removes partial file (best-effort).                                | F-16                                              |
| X-05 | `OnAccountUpdate` is pure cache-delete; no IMAP, no FS, no DB; safe under fan-out lock.    | `Tools_OnAccountUpdate_Idempotent_NoSideEffectsBeyondCache` |
| X-06 | `keyedMutex` memory growth is documented (unbounded in v1); v2 LRU deferred.               | Doc-only check                                    |

## 12. Accessibility

| #    | Criterion                                                                                  | How verified                       |
|------|--------------------------------------------------------------------------------------------|------------------------------------|
| Y-01 | Step status icons accompanied by step name + status text; no colour-only semantics.        | Manual a11y audit + snapshot        |
| Y-02 | Cancel button has aria-label "Cancel running operation".                                   | Snapshot test                      |
| Y-03 | OpenUrl notice card always contains the word "incognito".                                  | AST scan                           |
| Y-04 | Reduced motion disables Diagnose Running spinner (static frame).                           | `ToolsVM_ReducedMotion_NoSpinner`  |

## 13. Sign-off

A merge into `main` requires:

- [ ] CI green: every test in §1–§12 passes; every benchmark within budget.
- [ ] `golangci-lint run` clean (exhaustive, forbidigo, no-empty-interface, fn-length, no-incognito-false).
- [ ] `goleak.VerifyNone(t)` passes in `TestMain` of `internal/core` and `internal/ui/views`.
- [ ] **Audit-trail invariants §5 (A-01, A-02)** green — single-handedly blocks merge.
- [ ] Code owner review by Tools lead.
- [ ] Security lead sign-off on §4 (OpenUrl) + §8 (PII).
- [ ] Spec deltas (if any) merged in the **same** PR.

> Any criterion failing = build is **not shippable**.

---

**End of `06-tools/97-acceptance-criteria.md`**
