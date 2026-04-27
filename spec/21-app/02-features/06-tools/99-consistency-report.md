# 06 — Tools — Consistency Report

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Verifies that the four files of the Tools feature (`00-overview.md`, `01-backend.md`, `02-frontend.md`, `97-acceptance-criteria.md`) are internally consistent AND consistent with project-wide conventions in `spec/12-consolidated-guidelines/` and `spec/21-app/04..07`. **Also** resolves the forward-references from sibling features (`03-rules`, `05-watch`) that depend on `core.Tools.OpenUrl`.

Files checked:
- [`./00-overview.md`](./00-overview.md), [`./01-backend.md`](./01-backend.md), [`./02-frontend.md`](./02-frontend.md), [`./97-acceptance-criteria.md`](./97-acceptance-criteria.md)

External anchors:
- [`../../04-coding-standards.md`](../../04-coding-standards.md), [`../../05-logging-strategy.md`](../../05-logging-strategy.md), [`../../06-error-registry.md`](../../06-error-registry.md), [`../../07-architecture.md`](../../07-architecture.md)
- `spec/12-consolidated-guidelines/02-coding-guidelines.md`, `03-error-management.md`, `13-app.md`, `16-app-design-system-and-ui.md`, `18-database-conventions.md`, `23-generic-cli.md`
- Sibling feature specs: `02-features/03-rules`, `02-features/05-watch`

---

## 1. Naming Consistency

| Term                       | Overview        | Backend         | Frontend        | Acceptance      | Verdict |
|----------------------------|-----------------|-----------------|-----------------|-----------------|---------|
| `core.Tools`               | ✅ throughout   | ✅ §1            | ✅ §1.1         | ✅              | ✅ Same |
| `ReadSpec` / `ReadResult`  | ✅ §4.1         | ✅ §2.1          | ✅ §1 (form)    | ✅ §1           | ✅ |
| `ExportSpec` / `ExportProgress` / `ExportPhase` / `ExportReport` | ✅ §4.1 | ✅ §2.2 | ✅ §1 (form) | ✅ §2 | ✅ |
| `ExportPhase` 4 values (`Counting/Writing/Flushing/Done`) | ✅ §4.1 | ✅ §2.2 | ✅ §2.3 (label) | ✅ F-11 | ✅ |
| `DiagnoseSpec` / `DiagnosticsReport` / `DiagnosticsStep` / `DiagnosticsStepName` (5) / `DiagnosticsStepStatus` (5) | ✅ §4.1 | ✅ §2.3 / §4 | ✅ §1 / §3.2 | ✅ §3 | ✅ |
| `OpenUrlSpec` / `OpenUrlReport` / `OpenUrlOrigin` (4 values) | ✅ §4.1 | ✅ §2.4 | ✅ §1 / §2.5 | ✅ §4 | ✅ |
| `OpenedUrlRow` (DB shape)  | ✅ §4.2 (schema) | ✅ §5 (DDL)     | ✅ §2.5 (Recent panel binding) | ✅ §10 | ✅ |
| `OpenUrlOrigin` enum: `Rule`, `Card`, `Manual`, `Cli` | ✅ §4.1 | ✅ §2.4 | ✅ §2.5 (pinned `Manual`) | ✅ F-47 | ✅ |
| `DedupWindowSec` (60 s default) | ✅ §5.3   | ✅ §2.4 / §6.1   | (mentioned in §4.4 toast) | ✅ F-39 | ✅ |
| `OpenUrlMaxLengthBytes` (8192) | ✅ §4.3      | ✅ §1 cfg / §3.1 | ✅ §2.5 (validation) | ✅ F-31 | ✅ |
| `OpenUrlAllowedSchemes` (`["https","http"]`) | ✅ §4.3 | ✅ §1 cfg / §3.1 | ✅ §4.4.1 toast | ✅ F-33 | ✅ |
| Buffer caps (`ToolsReadOutputCap=2000`, `ToolsOpenRecentCap=100`, `ToolsDiagStepCount=5`) | — | — | ✅ §1.2 | ✅ F-06, P-11 | ✅ |
| `ToolsReadOutputCap == WatchRawLogCap == 2000` | — | — | ✅ §1.2 (cross-feature anchor) | — | ✅ Cross-checked vs `05-watch/02-frontend.md` §1.2 |

**Result:** ✅ No naming drift across the four files; cross-feature anchor (`ToolsReadOutputCap == WatchRawLogCap`) holds.

---

## 2. Cross-Reference Integrity

| Reference                                                  | From                | To                                  | Verdict |
|------------------------------------------------------------|---------------------|-------------------------------------|---------|
| Overview §3 — `core.Accounts` dep                          | overview            | `04-accounts/01-backend.md`         | ✅ Methods exist (`Get`, `List`, `Subscribe`) |
| Overview §3 — `core.Emails` dep (export SELECT)            | overview            | `02-emails/01-backend.md`           | ✅ Schema available |
| Overview §4.2 — `OpenedUrl` schema                         | overview            | `01-backend.md` §5 + `spec/23-app-database/01-schema.md` §4 | ✅ Closed (OI-6 2026-04-27) — canonical home is `spec/23-app-database/01-schema.md` §4 (table `OpenedUrls` after Slice #137 pluralisation); backend §5 still mirrors for in-feature reading. |
| Backend §2.4 — `core.Tools.OpenUrl` signature              | backend             | architecture §4.6                   | ✅ Match: `OpenUrl(ctx, raw string) errtrace.Result[Unit]` (overview/backend use richer `OpenUrlSpec`+`OpenUrlReport` — see §3 below) |
| Backend §8 — error codes 21750–21769                       | backend             | `internal/errtrace/codes.yaml` (`Tools` block) | ✅ Closed (OI-2 2026-04-27) — range uniquely allocated to the `Tools` block in `codes.yaml`; `ER-TLS` prefix is the historical Tools abbreviation, NOT TLS network protocol (clarifying comment added in-tree). |
| Backend §8 — wraps `ER-EXP-*`, `ER-MAIL-*`, `ER-STO-21103`, `ER-COR-21703/21704` | backend | `06-error-registry.md` | ✅ All wrapped codes pre-existing |
| Frontend §3 — design tokens (`ColorDiagStep*`, `ColorOpenUrl*`) | frontend         | `spec/24-app-design-system-and-ui/01-tokens.md` §2.9 + §2.10 | ✅ Closed (OI-1 2026-04-27) — DiagStep dots registered in §2.9, OpenUrl badges in §2.10; §2.11 totals bumped 39 → 47. |
| Frontend §1.2 — `WatchRawLogCap` anchor                    | frontend            | `05-watch/02-frontend.md` §1.2      | ✅ Both equal 2000 |
| Acceptance A-01 — AST scan over `internal/**/*.go`         | acceptance          | `linters/no-other-browser-launch.sh` + `linters/no-incognito-false.sh` | ✅ Closed (OI-5) — both lint scripts present in-tree; reserved test name `Tools_NoOtherFile_ShellsOutToBrowser` is the AST companion still owned by the lint toolchain. |
| Acceptance A-04/A-05 — survives delete                     | acceptance          | `internal/store/` Account/Email delete impls | ✅ No FK by design (overview §4.2) |
| Acceptance A-06 — Watch hyperlink path                     | acceptance          | `05-watch/97-acceptance-criteria.md` U-10 | ✅ Cross-referenced; same test name |
| Acceptance A-07 — Rules engine path                        | acceptance          | `05-watch/01-backend.md` §3.6 + `03-rules/97-acceptance-criteria.md` F-22 | ✅ Both files describe the same path |
| Acceptance A-08 — CLI path                                 | acceptance          | `internal/cli/` (Task #28-CLI parity)| ✅ CLI subcommand `email-read open-url` declared in overview §1 #8 |

**Result:** ✅ All references resolve — every previous ⚠ forward-ref closed by OI-1/OI-2/OI-5/OI-6 on 2026-04-27.

---

## 3. Method Signature Reconciliation (architecture §4.6 vs Tools §2)

`07-architecture.md` §4.6 declares:
```go
func (t *Tools) OpenUrl(ctx context.Context, raw string) errtrace.Result[Unit]
```

`01-backend.md` §2.4 declares:
```go
func (t *Tools) OpenUrl(ctx context.Context, spec OpenUrlSpec) errtrace.Result[OpenUrlReport]
```

**Reconciliation:** The richer `OpenUrlSpec`/`OpenUrlReport` API supersedes the architecture stub. The architecture file's §4.6 will be updated in Task #35 (final consistency pass) to match. This is a deliberate evolution — the stub didn't capture `Origin`/`RuleName`/`EmailId`/dedup, all of which are required by the audit-invariant chain (A-03..A-07).

| Architecture stub               | Tools §2.4 (canonical)                | Action |
|---------------------------------|---------------------------------------|--------|
| `OpenUrl(ctx, raw string) Result[Unit]` | `OpenUrl(ctx, spec OpenUrlSpec) Result[OpenUrlReport]` | Update arch §4.6 in Task #35 |
| `Export(ctx, ExportSpec) Result[ExportReport]` | `ExportCsv(ctx, ExportSpec, progress chan<-ExportProgress) Result[ExportReport]` | Update arch §4.6: rename `Export` → `ExportCsv`; add streaming channel |
| `Diagnose(ctx) Result[DiagnosticsReport]` | `Diagnose(ctx, DiagnoseSpec, progress chan<- DiagnosticsStep) Result[DiagnosticsReport]` | Update arch §4.6: take `DiagnoseSpec`; add streaming channel |
| (missing) `ReadOnce`            | `ReadOnce(ctx, ReadSpec, progress chan<- string) Result[ReadResult]` | Add to arch §4.6 |
| (missing) `RecentOpenedUrls`    | `RecentOpenedUrls(ctx, OpenedUrlListSpec) Result[[]OpenedUrlRow]` | Add to arch §4.6 |
| (missing) `OnAccountUpdate`     | `OnAccountUpdate(ctx, alias) Result[Unit]` (internal)                | Add to arch §4.6 |

**Verdict:** ✅ Closed (OI-4 2026-04-27) — `spec/21-app/07-architecture.md` §4.6 was updated in Slice #135 to list all 5 public Tools methods (`ReadOnce`, `ExportCsv`, `Diagnose`, `OpenUrl`, `RecentOpenedUrls`) plus the internal `OnAccountUpdate`. Tools spec remains the source of truth; architecture is now in lockstep.

---

## 4. Error Code Consistency

Tools owns block `21750–21769`. Distribution:

| Code  | Name                          | Backend §8 | Frontend toast (§4.4.1)         | Acceptance §1–§4 |
|-------|-------------------------------|------------|----------------------------------|------------------|
| 21750 | ToolsInvalidArgument          | ✅         | (caller bug; logged WARN; UI rejects form) | F-03 |
| 21751 | ToolsReadFetchFailed          | ✅         | (wrapped mail code surfaced)     | F-05 |
| 21752 | ToolsDiagnoseAborted          | ✅         | (UI marks remaining Skipped)     | (cancel path)    |
| 21753 | ToolsExportPathExists         | ✅         | "Path exists. Overwrite?"        | F-14             |
| 21754 | ToolsExportCancelled          | ✅         | (silent on Cancel)               | F-16             |
| 21755 | ToolsCacheCorrupted           | ✅         | (defensive; ERROR + cache evict) | —                |
| 21760 | OpenUrlEmpty                  | ✅         | "URL is empty."                  | F-30             |
| 21761 | OpenUrlTooLong                | ✅         | "URL is too long…"               | F-31             |
| 21762 | OpenUrlMalformed              | ✅         | "URL is malformed."              | F-32             |
| 21763 | OpenUrlSchemeForbidden        | ✅         | "Scheme `{x}` is not allowed…"   | F-33             |
| 21764 | OpenUrlLocalhostBlocked       | ✅         | "Localhost URLs are blocked…"    | F-34             |
| 21765 | OpenUrlPrivateIpBlocked       | ✅         | "Private-IP URLs are blocked…"   | F-35             |
| 21766 | OpenUrlBrowserUnavailable     | ✅         | "No browser found…"              | F-42             |
| 21767 | OpenUrlLaunchFailed           | ✅         | "Browser launch failed…"         | F-43             |
| 21768 | OpenUrlAuditInsertFailed      | ✅         | "Browser opened, but audit insert failed…" | F-44 |
| 21769 | ToolsBackgroundLeakDetected   | ✅         | (defensive ERROR; force-cancel)  | (lint enforces)  |

**Result:** ✅ 15 codes drafted in 21750–21769 block. All 9 user-facing codes have friendly toast text (F-50). Defensive codes (21755, 21769) intentionally not surfaced.

---

## 5. Performance Budget Consistency

| Budget                                       | Overview     | Backend       | Frontend       | Acceptance   | Match? |
|----------------------------------------------|--------------|---------------|----------------|--------------|--------|
| Cancellation ≤ 500 ms                        | ✅ §1 #9     | ✅ §10        | ✅ §5.2        | ✅ F-07, P-08 | ✅ |
| `Diagnose` cache hit ≤ 2 ms                  | —            | ✅ §10        | —              | ✅ P-04      | ✅ |
| `OpenUrl` dedup-hit ≤ 5 ms                   | —            | ✅ §10        | —              | ✅ P-05      | ✅ |
| `OpenUrl` launch ≤ 80 ms p95                 | —            | ✅ §10        | —              | ✅ P-06      | ✅ |
| `ReadOnce` ≤ 2 s p95                         | —            | ✅ §10        | —              | ✅ P-01      | ✅ |
| `ExportCsv` 10 k rows ≤ 3 s p95              | —            | ✅ §10        | —              | ✅ P-02      | ✅ |
| `Diagnose` ≤ 1.5 s p95                       | —            | ✅ §10        | —              | ✅ P-03      | ✅ |
| UI Attach ≤ 80 ms                            | —            | —             | ✅ §7          | ✅ P-09      | ✅ |
| UI tab switch ≤ 8 ms                         | —            | —             | ✅ §7          | ✅ P-10      | ✅ |
| UI memory ≤ 1 MiB after 1 h synthetic        | —            | —             | ✅ §7          | ✅ P-11      | ✅ |
| Progress publish every 256 rows (Export)     | —            | ✅ §2.2       | (UI consumes; rate-limit honoured) | ✅ F-12 | ✅ |

**Result:** ✅ Every budget mirrored across at least one spec layer + acceptance test. No "spec-only" budgets without a gate.

---

## 6. Architectural Compliance

| Rule (`07-architecture.md`)                                                          | Verdict |
|--------------------------------------------------------------------------------------|---------|
| `internal/ui/views/*` may NOT import `internal/exporter`, `internal/mailclient`, `internal/store`, `internal/browser` directly | ✅ Frontend §10 explicit ban + lint |
| `core.Tools` is the only public surface for tool functionality                       | ✅ All callers route through `core.Tools` |
| Long-running ops own their goroutines; views attach drainer goroutines               | ✅ Frontend §5.3 — 5 patterns, leak-tested |
| `internal/browser` is the only file that may invoke `os/exec` for browser binaries   | ✅ Acceptance A-01 + Q-06 lint (`xdg-open` etc.) |
| `internal/store` `INSERT INTO OpenedUrl` only callable from `core.Tools.OpenUrl`     | ✅ Acceptance A-02 |

**Result:** ✅ No architectural violations.

---

## 7. Coding Standards Compliance

Per `04-coding-standards.md` and `02-coding-guidelines.md`:

| Rule                                                            | Verdict |
|-----------------------------------------------------------------|---------|
| §1.1 PascalCase for ALL identifiers                              | ✅ Q-07 lint |
| §3 Functions ≤ 15 LOC body                                      | ✅ Constructor + handlers split into helpers; Q-03 lint |
| §5 Strict typing — no `any`                                     | ✅ Q-01 lint; `binding.Untyped` wrapped in typed accessors |
| §6 No bare `error` returns from public API                      | ✅ Q-02 lint; every public method returns `errtrace.Result[T]` |
| `apperror.Wrap` for cross-layer errors                          | ✅ Backend §8 (every wrap shown) |
| No `time.Sleep` in production                                   | ✅ Q-04 lint; cache TTL via `clock.Since`; cancels via ctx |
| No `panic` / `log.Fatal`                                        | ✅ All errors via Result + apperror |
| Forbidden imports in `views/`                                   | ✅ Q-05, Q-06 lint |
| `Incognito = false` literal forbidden                           | ✅ Q-11 lint (custom: `linters/no-incognito-false.sh`) |
| Browser-binary string literals forbidden outside `internal/browser/` | ✅ Q-06 lint |

**Result:** ✅ Full compliance.

---

## 8. Logging Compliance

Per `05-logging-strategy.md` §6.6:

| Rule                                                                   | Verdict |
|------------------------------------------------------------------------|---------|
| Every sub-tool emits Started/Completed log lines                       | ✅ Backend §9 |
| Every error log carries `TraceId`                                      | ✅ Backend §9 + L-05 |
| `OriginalUrl` never logged                                             | ✅ S-01 + L-01 (only `Url` canonical) |
| `OsError` redacted of home paths and userinfo                          | ✅ S-03 |
| `OpenUrlUserinfoStripped` WARN omits `Url` field entirely              | ✅ S-04 / F-38 |
| Diagnose emits one INFO per step (not per probe attempt)               | ✅ Backend §9 + L-06 |
| Export `Phase` log line is one-per-phase, not one-per-row              | ✅ Backend §9 + F-12 |

**Result:** ✅ All redaction and rate-limit invariants captured by tests.

---

## 9. Database Compliance

Per `18-database-conventions.md`:

| Rule                                                       | Verdict |
|------------------------------------------------------------|---------|
| Singular PascalCase table name                             | ✅ `OpenedUrl` (singular) |
| Positive boolean column names                              | ✅ `IsIncognito`, `IsDeduped` |
| FK rules explicit (or explicitly absent + justified)       | ✅ `Alias` is intentionally NOT a FK to `Account.Alias` (audit survives delete; D-05) |
| Single UPDATE / INSERT per logical operation               | ✅ One `INSERT INTO OpenedUrl` per `OpenUrl` invocation (success, failure, AND dedup-skip) |
| Indexes documented and exercised                           | ✅ D-03, D-04 (EXPLAIN check) |

**Result:** ✅ Schema conforms; the no-FK decision is explicit and documented (overview §4.2 + acceptance D-05 + A-04/A-05).

---

## 10. Security & PII Compliance

| Rule                                                       | Verdict |
|------------------------------------------------------------|---------|
| `OriginalUrl` never logged                                 | ✅ S-01 |
| `Url` in logs always canonical (post-redaction)            | ✅ S-02 |
| `OsError` paths/userinfo stripped                          | ✅ S-03 |
| Userinfo URL → strip + WARN with no URL field              | ✅ S-04 |
| OTP/password/token redaction (query + path variants)        | ✅ S-05 |
| Per-key mutex prevents concurrent same-key launches        | ✅ S-06 + F-41 |
| Always incognito; no opt-out path                          | ✅ S-07 + F-45 + Q-11 |
| Path-escape on export blocked                              | ✅ S-08 + F-13 |
| Single browser-launch path                                 | ✅ A-01 + Q-06 |
| Single audit-insert path                                   | ✅ A-02 |

**Result:** ✅ Defense-in-depth at six layers: validation → redaction → mutex → incognito-always → single-launch-path → single-audit-path.

---

## 11. Atomicity & Safety

| Concern                                                                     | Verdict |
|-----------------------------------------------------------------------------|---------|
| Launch + audit NOT in single tx; documented decision                        | ✅ X-01 |
| Concurrent same-key OpenUrl serialised                                      | ✅ X-02 / F-41 |
| Concurrent same-alias Diagnose serialised                                   | ✅ X-03 / F-25 |
| Cancellation removes partial Export file                                    | ✅ X-04 / F-16 |
| `OnAccountUpdate` is pure cache-delete                                      | ✅ X-05 |
| `keyedMutex` unbounded growth documented                                    | ✅ X-06 (v2 LRU deferred) |

**Result:** ✅ Every concurrent-or-failure path has an acceptance test + documented trade-off.

---

## 12. Cross-Feature Consistency

The `core.Tools.OpenUrl` API is consumed by **three** sibling features. Each must align:

| Consumer  | Caller code path                                                         | Origin value | Verified |
|-----------|--------------------------------------------------------------------------|--------------|----------|
| Watch     | `internal/watcher/loop.go::processEmail` → rule action OpenUrl           | `Rule`       | A-07; cross-test in `05-watch/97` F-12 |
| Watch UI  | `internal/ui/views/watch.go::cardRow` hyperlink click                    | `Card`       | A-06; cross-test in `05-watch/97` U-10 |
| Rules UI  | `internal/ui/views/rules.go::testRule` "open in browser" preview button   | `Manual` (with `RuleName`) | Cross-test in `03-rules/97` (must add: `RulesVM_TestRule_OpenUrl_OriginManualWithRuleName`) |
| Tools UI  | `internal/ui/views/tools.go::handleOpenRun` (manual paste)                | `Manual`     | F-49 + F-50 |
| CLI       | `internal/cli/cmd_open_url.go`                                            | `Cli`        | A-08      |

**Action item OI-3 (new):** Add cross-test `RulesVM_TestRule_OpenUrl_OriginManualWithRuleName` to `03-rules/97-acceptance-criteria.md`. Resolves in Task #35 (final consistency pass) — non-blocking since Rules already routes through `core.Tools.OpenUrl` per its existing F-22.

| Forward-references resolved by this feature                                                  | Status |
|----------------------------------------------------------------------------------------------|--------|
| `05-watch/99-consistency-report.md` OI-2 (Tools `OpenUrl` signature)                          | ✅ Resolved by `01-backend.md` §2.4 |
| `02-features/03-rules` `Action == OpenUrl` extracted-URL handling                             | ✅ Confirmed: `core.Tools.OpenUrl` called once per URL with `Origin=Rule` |
| `02-features/02-emails` future "Export selection" menu                                        | ✅ Via `core.Tools.ExportCsv` (signature locked) |

**Result:** ✅ All inbound forward-references resolved. One new outbound action item (OI-3) for Rules cross-test.

---

## 13. Single-Browser-Launch Invariant 🔴 — Triple-Layer Verification

Because audit completeness is the security promise of this feature, the single-launch invariant must be enforceable at three independent layers:

| Layer       | Mechanism                                                              | Acceptance |
|-------------|------------------------------------------------------------------------|------------|
| Lint        | `forbidigo` blocks browser-binary string literals (`xdg-open`, `open`, `start`, `firefox`, `chrome`, `chromium`, `safari`, `brave`) outside `internal/browser/` | Q-06 |
| AST         | `Tools_NoOtherFile_ShellsOutToBrowser` scans all `.go` files for `os/exec` calls whose first arg matches the browser-binary regex; only `internal/browser/` and `internal/core/tools.go` permitted | A-01 |
| Runtime     | `core.Tools.OpenUrl` is the only function in the codebase that calls `t.browser.LaunchIncognito`; `Store_OpenedUrlInsert_OnlyCalledByTools` guarantees the audit row is paired | A-02 |

**Result:** ✅ Three independent guards. If any single layer regresses, at least two others fail loudly. The build cannot ship with an un-audited browser launch.

---

## 14. Open Issues

| #    | Issue                                                                                  | Owner          | Resolves in task | Status |
|------|----------------------------------------------------------------------------------------|----------------|------------------|--------|
| OI-1 | `ColorDiagStep*` / `ColorOpenUrl*` design tokens referenced in frontend §3 but not yet registered in `spec/24-app-design-system-and-ui/` | Design system  | Task #33 | ✅ **Closed** 2026-04-27 — tokens registered in `spec/24-app-design-system-and-ui/01-tokens.md` §2.9 (DiagStep dots) + §2.10 (OpenUrl badges); §2.11 totals bumped 39 → 47. |
| OI-2 | Error registry `21750–21769` block to be added to `06-error-registry.md`               | Errors         | Task #35 | ✅ **Closed** 2026-04-27 — range is uniquely allocated to the `Tools` block in `internal/errtrace/codes.yaml`; clarifying comment added that `ER-TLS` prefix is a historical Tools abbreviation, NOT the TLS network protocol (TLS-handshake errors live under `Mail`, e.g. `ErrMailTLSHandshake`). No collision, no rework. |
| OI-3 | Add cross-test `RulesVM_TestRule_OpenUrl_OriginManualWithRuleName` to `03-rules/97`    | Rules          | Task #35 | ✅ **Closed** 2026-04-27 — added as `F-23` in `spec/21-app/02-features/03-rules/97-acceptance-criteria.md`. |
| OI-4 | Update `07-architecture.md` §4.6 to match the richer Tools API (5 method signatures + 1 internal) | Architecture   | Task #35 | ✅ Closed 2026-04-27 — `spec/21-app/07-architecture.md` §4.6 now lists `ReadOnce`, `ExportCsv`, `Diagnose`, `OpenUrl`, `RecentOpenedUrls` + internal `OnAccountUpdate` |
| OI-5 | New lint script `linters/no-other-browser-launch.sh` + `linters/no-incognito-false.sh` | Lint toolchain | Task #34/#35 | ✅ Closed — both scripts present at `linters/no-other-browser-launch.sh` and `linters/no-incognito-false.sh` |
| OI-6 | Canonical `OpenedUrl` schema home moves from this spec to `spec/23-app-database/`      | DB schema      | Task #32 | ✅ Closed — canonical `OpenedUrl` table defined in `spec/23-app-database/01-schema.md` §4 |

All six closed (2026-04-27). No open issues remain in the Tools spec.

---

## 15. Sign-off

A merge into `main` for the Tools feature requires:

- [ ] Every checkbox in §1–§13 is ✅ (none ⚠ except OI-1..OI-6 which are scheduled).
- [ ] CI green for every test in `97-acceptance-criteria.md`.
- [ ] OI-1..OI-6 closed when corresponding tasks merge.
- [ ] Code owner review by Tools lead.
- [ ] Security lead sign-off on §10 + the OpenUrl block (acceptance §4).
- [ ] Product lead sign-off on the single-browser-launch invariant 🔴 (since it is THE audit promise).

**Reviewed by:** _________________________   **Date:** ____________

---

**End of `06-tools/99-consistency-report.md`**
