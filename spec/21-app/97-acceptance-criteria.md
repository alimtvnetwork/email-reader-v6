# Acceptance Criteria — 21-app (Project-Wide)

**Version:** 2.0.0
**Updated:** 2026-04-25
**Status:** Approved — supersedes v1.0.0
**Scope:** Cross-feature, project-wide acceptance gate. The seven per-feature `97-acceptance-criteria.md` files (Dashboard, Emails, Rules, Accounts, Watch, Tools, Settings) plus `spec/23-app-database/97-acceptance-criteria.md` and `spec/24-app-design-system-and-ui/97-acceptance-criteria.md` remain the **definitive per-area gates**. This file consolidates them, adds **cross-feature criteria** that no single feature owns, and defines the merge-to-`main` sign-off ladder.

---

## 0. How to read this file

- **AC-PROJ-XX** rows below are **new**, project-wide criteria that span ≥ 2 features or that test the integrated system end-to-end. They are owned by this file.
- **Per-feature AC tables** (§3) are **roll-ups** of the IDs defined in each feature's own `97-acceptance-criteria.md`. The source file is the source of truth — this file just totals and links.
- A merge into `main` requires **every** AC-PROJ-XX row in §1, **every** per-feature checkbox in §3 (via the linked file), and the §4 sign-off checklist.

ID conventions (final, do not invent new shapes):

| Prefix | Owner area | Numbering |
|---|---|---|
| `AC-PROJ-NN` | This file (cross-feature) | 01–99, contiguous |
| `F-NN` / `H-NN` / `N-NN` / `P-NN` / `S-NN` / `Q-NN` / `L-NN` / `D-NN` / `A-NN` / `M-NN` | Per-feature (Dashboard, Emails, Rules, Accounts, Watch, Tools) — F = Functional, H = Heartbeat, N = Negative, P = Performance, S = Security/PII, Q = Code-Quality, L = Logging, D = Database, A = Accessibility, M = Atomicity/Misc | Local to each feature 97 |
| `AC-SB-NN` / `AC-SF-NN` / `AC-SP-NN` / `AC-SX-NN` | Settings (B = Backend, F = Frontend, P = Performance, X = Cross-cutting) | Per Settings 97 |
| `AC-DB-NN` / `AC-DBP-NN` | Database schema (P suffix = Performance) | Per `spec/23-app-database/97-acceptance-criteria.md` |
| `AC-DS-NN` | Design System & UI shell | Per `spec/24-app-design-system-and-ui/97-acceptance-criteria.md` |

---

## 1. Project-Wide Acceptance Criteria (AC-PROJ)

These criteria are **cross-feature**: each one exercises ≥ 2 feature backends, the database, AND the design system / frontend together. They are NOT duplicated in any per-feature file.

### 1.1 End-to-end mail pipeline (Watch → Emails → Rules → Tools → DB)

| #          | Criterion                                                                                                                                                                                                                                                                                                                                                              | Test                                                                  |
|------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------|
| AC-PROJ-01 | A real new email arriving at the IMAP server is detected by `core.Watch`, persisted by `core.Emails.Insert`, evaluated by `core.Rules.EvaluateAll`, and (on match) opened by `core.Tools.OpenUrl` end-to-end **within 5 s** of `RFC822.Date` (Poll mode @ 3 s). One row each in `Email`, `RuleStat` (Hits++), and `OpenedUrl`. | `Project_NewMail_E2E_Under5s_AllSideEffectsPersisted` (integration)   |
| AC-PROJ-02 | When a rule matches, the **Watch live-log card**, the **Emails detail pane**, and the **Rules stats panel** all update within 1 s of the `RuleMatched` event (subscribers see the same monotonic event ID).                                                                                                                                                            | `Project_RuleMatch_FanOutToThreeViewsUnder1s`                         |
| AC-PROJ-03 | The same URL arriving via two distinct emails is opened **once** (deduped by `OpenedUrl` partial unique index `IxOpenedUrlDedup`); the second emits `WatchEvent{UrlSkipped, Reason="duplicate"}`.                                                                                                                                                                      | `Project_DuplicateUrl_OpenedOnce_SecondReportedAsSkip`                |
| AC-PROJ-04 | Manually re-opening a URL via `core.Emails.OpenUrl` (button in Emails detail) bypasses the dedup index, writes `OpenedUrl{RuleName="manual"}`, and emits the same `UrlOpened` event subscribers see during automated opens.                                                                                                                                            | `Project_ManualOpen_BypassesDedup_RecordsManualOrigin`                |
| AC-PROJ-05 | The `read <alias> <uid>` CLI subcommand and the GUI "Re-open URL" action route through **the same** `core.Emails.OpenUrl` code path; the audit row is byte-identical (`OpenedUrl.Origin = "manual"`, `RuleName = "manual"`).                                                                                                                                           | `Project_CliRead_And_GuiReopen_ProduceIdenticalAudit`                 |

### 1.2 Account lifecycle propagation (Accounts → Watch → DB → Sidebar)

| #          | Criterion                                                                                                                                                                                                                              | Test                                                            |
|------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| AC-PROJ-06 | `core.Accounts.Add(spec)` succeeds → sidebar account picker, Dashboard count card, and Watch alias dropdown all reflect the new alias **within 1 s** (single `AccountChanged` event observed by all three).                            | `Project_AccountAdd_FansOutToSidebarDashboardWatchUnder1s`      |
| AC-PROJ-07 | `core.Accounts.Remove(alias)` for an alias currently being watched: `core.Watch` auto-stops the runner ≤ 1 s, deletes the matching `WatchState` row, and the sidebar removes the alias **without restart**. No orphaned cursor remains. | `Project_AccountRemove_WhileWatching_AutoStopsAndCleansState`   |
| AC-PROJ-08 | Editing an account password via Settings → Accounts re-encodes via `config.SanitizePassword`, restarts the runner via `RestartOnAccountUpdate`, and **preserves `LastSeenUid`** across the restart (no re-scan of mailbox).            | `Project_AccountPwdEdit_RestartsRunner_PreservesCursor`         |

### 1.3 Settings round-trip (Settings → Watch → Browser → UI)

| #          | Criterion                                                                                                                                                                                                                                            | Test                                                          |
|------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------|
| AC-PROJ-09 | Changing `Watch.PollSeconds` from 3 → 10 in Settings is honored on the **next poll cycle** without restart (no extra IMAP `LOGOUT`); a `SettingsEvent{PollSeconds=10}` is emitted and observed by the watcher loop.                                  | `Project_PollSecondsChange_LiveAppliedNoLogout`               |
| AC-PROJ-10 | Toggling theme (light ↔ dark) in Settings updates **every mounted view** within one Fyne tick (≤ 16 ms) using only design-system tokens — no view does its own `theme.NewXxxColor`.                                                                  | `Project_ThemeToggle_AllViewsRespondInOneTick`                |
| AC-PROJ-11 | Setting `Browser.ChromePath` to a custom binary in Settings causes the next `core.Tools.OpenUrl` invocation to launch THAT binary; the **Watch startup banner** shows the new path on the next runner start.                                         | `Project_BrowserOverride_LaunchedByTools_ShownByWatchBanner`  |

### 1.4 Logging & error contract (cross-cutting)

| #          | Criterion                                                                                                                                                                                                                                                                | Test                                                            |
|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| AC-PROJ-12 | Every error returned to the UI carries an `ER-XXX-NNNNN` code defined in `06-error-registry.md`; the UI **never** renders a raw Go error message (always `ToastFromError(err)` mapping). Verified by AST scan of `internal/ui/views/`.                                   | `Project_NoRawErrorsLeakToUi` (AST scan)                        |
| AC-PROJ-13 | Every `errtrace.Wrap` boundary in `internal/{watcher,mailclient,rules,store,browser}` carries a `file:line` frame; verified by parsing the structured log fixture for at least one frame line per propagated error.                                                      | `Project_ErrtraceFramesPresentOnAllBoundaries`                  |
| AC-PROJ-14 | The watcher's **heartbeat invariant** (one INFO line per cycle in verbose, one heartbeat per ≤ 3 min in quiet) is observed for ≥ 60 s of CI runtime in both modes. No regression of solved issue 02 or 04.                                                               | `Project_HeartbeatInvariantHoldsInBothModes`                    |
| AC-PROJ-15 | The **always-emitted event class** (startup banner + per-mail rule trace + URL open + errors) is present in **quiet mode** for every mail processed in CI fixture; no regression of solved issue 05.                                                                     | `Project_AlwaysEmittedEventsPresentInQuietMode`                 |

### 1.5 Layering & ownership (architectural invariants)

| #          | Criterion                                                                                                                                                                                                                                                                                                          | Test                                                |
|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------|
| AC-PROJ-16 | Only `internal/store` imports any SQL driver (`database/sql`, `modernc.org/sqlite`, `mattn/go-sqlite3`). AST scan returns zero matches in any other package. (See `AC-DB-50`.)                                                                                                                                     | `Project_OnlyStoreImportsSqlDrivers` (AST scan)     |
| AC-PROJ-17 | Only `internal/store/migrate` executes DDL statements. AST scan finds no `CREATE TABLE`/`ALTER TABLE`/`DROP` literals outside that package or its `*.sql` migration files. (See `AC-DB-51`.)                                                                                                                       | `Project_OnlyMigratePackageRunsDdl` (AST scan)      |
| AC-PROJ-18 | Only `internal/ui` and `cmd/email-read-ui` import `fyne.io/fyne/v2`. AST scan returns zero matches in `internal/{core,store,watcher,rules,mailclient,browser,config,errtrace}`.                                                                                                                                    | `Project_OnlyUiPackagesImportFyne` (AST scan)       |
| AC-PROJ-19 | Every public `core.*` method returns `errtrace.Result[T]` (or `errtrace.Result[Unit]` for void). Zero raw `(T, error)` return tuples in `internal/core/*.go`. (See `spec/12-consolidated-guidelines/03-error-management.md`.)                                                                                       | `Project_CoreApiUsesResultEnvelope` (AST scan)      |
| AC-PROJ-20 | Every Go file under `internal/` and `cmd/` has functions ≤ 15 statements long (per `spec/12-consolidated-guidelines/02-coding-guidelines.md` §3). Verified by `linter-scripts/check-fn-length.sh` in CI.                                                                                                            | `Project_FnLengthLinter_GreenInCi`                  |

### 1.6 Database & retention (system-wide)

| #          | Criterion                                                                                                                                                                                                                                              | Test                                                  |
|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------|
| AC-PROJ-21 | A fresh install (no `data/email-read.db`) bootstraps `SchemaMigration` and runs `M001`–`M00N` in order; failure aborts the app with `ER-STO-21100` and writes no half-applied state. (Cross-checks `AC-DB-30`–`AC-DB-37`.)                             | `Project_FreshBootstrap_RunsAllMigrations_OrAborts`   |
| AC-PROJ-22 | Retention sweep deletes `OpenedUrl` rows older than the configured TTL (365 d / 90 d) and triggers `VACUUM`/`ANALYZE` per the threshold rules in `spec/23-app-database/04-retention-and-vacuum.md`. Idempotent across reruns.                          | `Project_RetentionSweep_DeletesAndVacuumsCorrectly`   |

### 1.7 Issue regression coverage (every solved issue has a test)

Each row pins one entry from `spec/21-app/03-issues/solved/` to a regression test that re-fails if the underlying bug returns.

| #          | Solved issue                                                | Regression test                                                |
|------------|-------------------------------------------------------------|----------------------------------------------------------------|
| AC-PROJ-23 | 01 — IMAP wrong password                                    | `Regress_Issue01_TestConnectionSurfacesEr21430`                |
| AC-PROJ-24 | 02 — Watcher silent on healthy idle                         | `Regress_Issue02_HeartbeatInQuietModeAtLeastOncePer3min`       |
| AC-PROJ-25 | 03 — Hidden Unicode in password                             | `Regress_Issue03_SanitizePasswordStripsCfRunes`                |
| AC-PROJ-26 | 04 — Noisy watcher log                                      | `Regress_Issue04_QuietModeUnderNLinesPerMinIdle`               |
| AC-PROJ-27 | 05 — Silent rule failure                                    | `Regress_Issue05_PerMailRuleTraceAlwaysEmitted`                |
| AC-PROJ-28 | 06 — Confusing `.eml` filenames + `read` command            | `Regress_Issue06_FilenameSchemeAndReadCmdParity`               |
| AC-PROJ-29 | 07 — Zero-rule deadlock                                     | `Regress_Issue07_AutoSeedDefaultRuleOnEmpty`                   |
| AC-PROJ-30 | 08 — Heavy/ambiguous logs                                   | `Regress_Issue08_NoDoubleTimestampsAndUrlTruncated`            |

### 1.8 Documentation & spec hygiene (CI-gated)

| #          | Criterion                                                                                                                                                                                                                                                          | Test                                                   |
|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|
| AC-PROJ-31 | Every error code referenced in any spec file (`ER-XXX-NNNNN`) is defined in `spec/21-app/06-error-registry.md`. Zero dangling references.                                                                                                                          | `Project_AllErrorRefsResolveInRegistry` (spec linter)  |
| AC-PROJ-32 | Every named query referenced in any backend spec (`Q-XXX-XXX`) is defined in `spec/23-app-database/02-queries.md`.                                                                                                                                                 | `Project_AllQueryRefsResolveInDbQueries` (spec linter) |
| AC-PROJ-33 | Every `mem://`, `./`, `../` link in any spec file resolves to an existing file. Verified by `linter-scripts/check-internal-links.sh`.                                                                                                                              | `Project_NoBrokenSpecLinks_GreenInCi`                  |
| AC-PROJ-34 | Every feature folder under `02-features/` contains exactly the five files: `00-overview.md`, `01-backend.md`, `02-frontend.md`, `97-acceptance-criteria.md`, `99-consistency-report.md`. No extras, no missing.                                                    | `Project_FeatureFolderShapeIsUniform`                  |
| AC-PROJ-35 | All open issues (`OI-N`) referenced in any feature 99-report are resolved (✅ Closed) before merge. Currently: Watch OI-1 (closed by Task #33) and Watch OI-2 (closed by Tasks #27–30) — both ✅.                                                                   | `Project_NoOpenOiReferencesAtMergeTime`                |

**Total project-wide criteria: 35.**

---

## 2. Total acceptance-criteria count (project-wide)

| Source                                                                  | Count | Style                  |
|-------------------------------------------------------------------------|------:|------------------------|
| **AC-PROJ-XX** (this file, §1)                                          |    35 | Cross-feature gate     |
| Dashboard — `02-features/01-dashboard/97-acceptance-criteria.md`        |    50 | F/N/P/Q/L/D/A/S local IDs |
| Emails — `02-features/02-emails/97-acceptance-criteria.md`              |    73 | F/N/P/Q/L/D/A/S local IDs |
| Rules — `02-features/03-rules/97-acceptance-criteria.md`                |    83 | F/N/P/Q/L/D/A/S local IDs |
| Accounts — `02-features/04-accounts/97-acceptance-criteria.md`          |   108 | F/N/P/Q/L/D/A/S local IDs |
| Watch — `02-features/05-watch/97-acceptance-criteria.md`                |    96 | F/H/N/P/Q/L/D/A/S/M local IDs |
| Tools — `02-features/06-tools/97-acceptance-criteria.md`                |   104 | F/N/P/Q/L/D/A/S local IDs |
| Settings — `02-features/07-settings/97-acceptance-criteria.md`          |    56 | AC-SB / AC-SF / AC-SP / AC-SX |
| Database — `spec/23-app-database/97-acceptance-criteria.md`             |    48 | AC-DB / AC-DBP         |
| Design System & UI — `spec/24-app-design-system-and-ui/97-acceptance-criteria.md` | 47 | AC-DS                  |
| **Project-wide total**                                                  | **700** | (sum)               |

> Counts sourced via `grep -ohE "^\s*\|\s*[A-Z]+-[0-9]+\s*\||^\s*-\s*\[ \]\s*\*\*[A-Z]+-[0-9]+" <file> \| wc -l` on 2026-04-25.

---

## 3. Per-feature roll-up (links to source files)

Each row links to the **single source of truth** for that area's acceptance criteria. Treat this as an index — do not duplicate criteria here.

### 3.1 Features (`spec/21-app/02-features/`)

| Feature   | AC source                                                                  | Headline gates                                                                                                                |
|-----------|----------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------|
| Dashboard | [`01-dashboard/97-acceptance-criteria.md`](./02-features/01-dashboard/97-acceptance-criteria.md) | Cold-open ≤ 100 ms on 10k-email DB; counts auto-refresh 5 s; no Start when zero accounts.                       |
| Emails    | [`02-emails/97-acceptance-criteria.md`](./02-features/02-emails/97-acceptance-criteria.md)       | 50k-row virtualized scroll; debounced search ≤ 200 ms; readable filename scheme (issue 06).                      |
| Rules     | [`03-rules/97-acceptance-criteria.md`](./02-features/03-rules/97-acceptance-criteria.md)         | Auto-seed default rule on empty (issue 07); rename atomic across `RuleStat` + `OpenedUrl`; dry-run writes nothing. |
| Accounts  | [`04-accounts/97-acceptance-criteria.md`](./02-features/04-accounts/97-acceptance-criteria.md)   | `SanitizePassword` boundary (issue 03); `Q-EMAIL-DELETE-BY-ALIAS` cascade on remove; password never logged.       |
| Watch     | [`05-watch/97-acceptance-criteria.md`](./02-features/05-watch/97-acceptance-criteria.md)         | 🔴 **Heartbeat invariant** (issue 02); always-emitted per-mail trace (issue 05); 2 s `LOGOUT` p95.                 |
| Tools     | [`06-tools/97-acceptance-criteria.md`](./02-features/06-tools/97-acceptance-criteria.md)         | Streaming output; doctor rune-dump (issue 03); diagnose halts on first failure; cancel button.                    |
| Settings  | [`07-settings/97-acceptance-criteria.md`](./02-features/07-settings/97-acceptance-criteria.md)   | Atomic `Save` (tmp + fsync + rename); live `PollSeconds` reload; theme switch with no restart.                    |

### 3.2 Cross-cutting

| Area                | AC source                                                                                          | Headline gates                                                                                            |
|---------------------|----------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------|
| Database            | [`spec/23-app-database/97-acceptance-criteria.md`](../23-app-database/97-acceptance-criteria.md)   | Singular PascalCase tables; checksum-verified migrations; `EXPLAIN QUERY PLAN` budgets per `Q-*`.         |
| Design System & UI  | [`spec/24-app-design-system-and-ui/97-acceptance-criteria.md`](../24-app-design-system-and-ui/97-acceptance-criteria.md) | Tokens-only colours (no `theme.NewXxxColor`); WCAG AA contrast matrix; `ColorWatchDot*` defined (closes Watch OI-1). |

---

## 4. Merge-to-`main` sign-off ladder

A PR merges to `main` iff **all** of the following are true. Each row is a hard gate; partial credit does not exist.

### 4.1 Automated gates (CI)

- [ ] All AC-PROJ-XX rows in §1 pass (35/35).
- [ ] Every per-feature `97-acceptance-criteria.md` passes (700/700 total inc. AC-PROJ).
- [ ] `linter-scripts/check-internal-links.sh` returns zero broken links (AC-PROJ-33).
- [ ] `linter-scripts/check-fn-length.sh` returns zero violations (AC-PROJ-20).
- [ ] AST scans (AC-PROJ-16/17/18/19) return zero violations.
- [ ] `go test ./...` is green on Linux, macOS, Windows (matrix).
- [ ] `go vet ./...` and `staticcheck ./...` are green.
- [ ] No solved-issue regression test in §1.7 has been disabled or `t.Skip()`'d.

### 4.2 Manual / spec gates

- [ ] No open issue (`OI-N`) in any feature `99-consistency-report.md` is unresolved (AC-PROJ-35).
- [ ] No file under `spec/21-app/03-issues/pending/` exists (or every entry has a `Status:` line and a tracking PR link).
- [ ] Every error code introduced in this PR is registered in `06-error-registry.md` with `code | name | layer | message-template | recovery`.
- [ ] Every named query introduced in this PR is registered in `spec/23-app-database/02-queries.md` with `id | sql | budget | EXPLAIN QUERY PLAN`.
- [ ] If a feature's UI changed, its `02-frontend.md` widget tree is updated and the corresponding design-system tokens (if new) are registered in `spec/24-app-design-system-and-ui/01-tokens.md`.

### 4.3 Owner sign-off

- [ ] **Watch lead** signs off on the heartbeat invariant 🔴 (AC-PROJ-14, plus Watch H-01).
- [ ] **Database lead** signs off on schema/query changes (AC-PROJ-21–22, plus AC-DB-XX).
- [ ] **Design system lead** signs off on token additions/changes (AC-PROJ-10, plus AC-DS-XX).
- [ ] **Security lead** signs off on any change touching `internal/config` password handling, `internal/browser`, or `internal/store` migrations.
- [ ] **Product lead** signs off on any change to user-facing copy in CLI banners or GUI toasts.

---

## 5. What is intentionally NOT here

To prevent drift between this file and the per-feature 97s, the following are **out of scope** for AC-PROJ-XX and live only in their feature files:

- Single-feature widget-tree assertions (e.g. "the Rules form has 6 fields").
- Single-feature error codes (e.g. `ER-WCH-21401` belongs in Watch's 97 + the registry, not here).
- Single-feature performance budgets (e.g. "Dashboard cold-open ≤ 100 ms" is a Dashboard P-NN row).
- Per-table DDL details (live in `spec/23-app-database/01-schema.md` + AC-DB-XX).
- Per-token hex values (live in `spec/24-app-design-system-and-ui/01-tokens.md` + AC-DS-XX).

If a future criterion would touch ≥ 2 features OR has no obvious single owner, add it as **AC-PROJ-36** (next contiguous ID) and link it from each feature's 99-report.

---

## 6. Cross-references

| Reference                  | Location                                                              |
|----------------------------|-----------------------------------------------------------------------|
| Project consistency report | [`./99-consistency-report.md`](./99-consistency-report.md)            |
| Coding standards           | [`./04-coding-standards.md`](./04-coding-standards.md)                |
| Logging strategy           | [`./05-logging-strategy.md`](./05-logging-strategy.md)                |
| Error registry             | [`./06-error-registry.md`](./06-error-registry.md)                    |
| Architecture               | [`./07-architecture.md`](./07-architecture.md)                        |
| Issues index               | [`./03-issues/00-overview.md`](./03-issues/00-overview.md)            |
| Database spec              | [`../23-app-database/`](../23-app-database/)                          |
| Design system spec         | [`../24-app-design-system-and-ui/`](../24-app-design-system-and-ui/)  |

---

## 7. Validation history

| Date       | Version | Action                                                                                                                                                                       |
|------------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 2026-04-25 | 1.0.0   | Initial 35-row prose stub authored alongside `00-overview.md` scaffolding (Task #06).                                                                                        |
| 2026-04-25 | 2.0.0   | **Full rewrite** (Task #35). Added 35 AC-PROJ rows, per-feature roll-up index of 700 total criteria, ID-prefix taxonomy, sign-off ladder, and explicit out-of-scope §5.       |
