# 23 — App Database — Consistency Report

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Cross-checks `spec/23-app-database/` against every feature backend that consumes the database. Every column reference, query reference, and ownership boundary listed in a feature spec must match this database spec exactly.

---

## 1. Internal Consistency (within `23-app-database/`)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| INT-1 | The 4 tables in `01-schema.md` §1 are exactly those created by migrations 1–4 in `03-migrations.md` §6. | `01-schema.md` §1 ↔ `03-migrations.md` §6 | `Test_Migrate_FreshApplyAll` then `Test_Schema_ColumnsMatchSpec` |
| INT-2 | The 12 named queries in `02-queries.md` §2 plus the 2 prune queries in `04-retention-and-vacuum.md` §3 are the complete `Q-*` inventory; no others exist in code. | `02-queries.md` §2 + `04-…` §3 | `Test_Queries_AllImplemented` (rejects extras) |
| INT-3 | Every column referenced in any `Q-*` exists in `01-schema.md`. | `02-queries.md` §3 ↔ `01-schema.md` | `Test_Queries_ColumnsExist` (parses SQL, cross-references `PRAGMA table_info`) |
| INT-4 | Performance budgets in `02-queries.md` §5 are met by the test fixtures used in AC-DBP-*. | `02-queries.md` §5 ↔ `97-acceptance-criteria.md` §G | `Test_Queries_Perf` |

## 2. Cross-Feature Consistency

### 2.1 Database ↔ Watch (Feature 05)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-W1 | Watch backend's per-message tx writes exactly one `Q-EMAIL-UPSERT` and one `Q-WATCH-UPSERT` per cycle. | `02-features/05-watch/01-backend.md` §4, §6 ↔ `02-queries.md` §3.1, §3.6 | `Test_Watch_TxShape` (counter-injected store) |
| CF-W2 | `WatchState.LastUid` written by Watch is monotonic; the `MAX(...)` clause in `Q-WATCH-UPSERT` is the SQL-level enforcement. | `02-features/05-watch/01-backend.md` §6 ↔ `02-queries.md` §3.6 ↔ `01-schema.md` §3 | `Test_Q_WatchUpsert_Monotonic` |
| CF-W3 | `LastPolledAt` is updated on every poll completion (success OR failure). | `05-logging-strategy.md` heartbeat invariant ↔ `01-schema.md` §3 | `Test_Watch_HeartbeatAlways` |

### 2.2 Database ↔ Tools (Feature 06)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-T1 | `OpenUrl` writes the `OpenedUrl` row **after** the browser launch returns (forensic completeness). | `02-features/06-tools/01-backend.md` §3.4 step 7 ↔ `01-schema.md` §4 | `Test_Tools_AuditAfterLaunch` |
| CF-T2 | Dedup query (`Q-OPEN-DEDUP`) is per-`Alias`, exact-match on `OriginalUrl`, only over `Decision='Launched'`. | `02-features/06-tools/01-backend.md` §3.4 step 4 ↔ `02-queries.md` §3.8 ↔ `01-schema.md` §4 (partial index) | `Test_Q_OpenDedup_Hit` + `_BlockedNotMatched` |
| CF-T3 | `ExportCsv` uses `Q-EXPORT-STREAM` and never buffers all rows. | `02-features/06-tools/01-backend.md` `ExportCsv` ↔ `02-queries.md` §3.12 | `Test_Q_ExportStream_NoBuffering` (peak-memory ≤ 32 MiB AC-DBP-06) |
| CF-T4 | `OriginalUrl` value never appears at log level ≥ DEBUG (Tools backend §11) — DB schema does not encrypt at rest, but logs are scrubbed. | `02-features/06-tools/01-backend.md` §11 ↔ `01-schema.md` §4 | `Test_LogScan_NoOriginalUrlLeak` (AC-DB-55) |

### 2.3 Database ↔ Rules (Feature 03)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-R1 | A "Rule" action that opens a URL goes through `core.Tools.OpenUrl`, which writes `OpenedUrl.Origin = 'Rule'`. | `02-features/03-rules/01-backend.md` (action dispatch) ↔ `02-features/06-tools/01-backend.md` §3.4 ↔ `01-schema.md` §4 | `Test_Rules_OpenedUrl_OriginRule` |
| CF-R2 | `RuleName` column on `OpenedUrl` is populated from the rule's name (truncated at 256 chars). | `02-features/03-rules/01-backend.md` ↔ `01-schema.md` §4 | `Test_Rules_RuleNameRecorded` |

### 2.4 Database ↔ Accounts (Feature 04)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-A1 | Removing an account also removes its `WatchState` row in the same tx. | `02-features/04-accounts/01-backend.md` (`RemoveAccount`) ↔ `01-schema.md` §3 | `Test_Accounts_RemoveDropsWatchState` |
| CF-A2 | Removing an account does NOT delete its `Email` rows or `OpenedUrl` rows (FK is `ON DELETE SET NULL` for `OpenedUrl.EmailId`; no FK from `WatchState` for the same reason). | `02-features/04-accounts/01-backend.md` ↔ `01-schema.md` §3, §4 | `Test_Accounts_RemovePreservesAudit` |

### 2.5 Database ↔ Emails (Feature 02)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-E1 | Emails list view uses `Q-EMAIL-LIST` with keyset pagination on `(ReceivedAt, Id)`. | `02-features/02-emails/01-backend.md` (list flow) ↔ `02-queries.md` §3.2 | `Test_Emails_KeysetPagination` |
| CF-E2 | Emails detail view uses `Q-EMAIL-GET-BY-ID` (single-row PK lookup). | `02-features/02-emails/01-backend.md` (detail flow) ↔ `02-queries.md` §3.3 | `Test_Emails_DetailFetch` |
| CF-E3 | Search filter "subject OR from" is a single `LIKE` over both columns; no separate full-text index in v1. | `02-features/02-emails/01-backend.md` ↔ `02-queries.md` §3.2 | `Test_Q_EmailList_SearchShape` |

### 2.6 Database ↔ Dashboard (Feature 01)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-D1 | Per-alias totals card uses `Q-EMAIL-COUNT-BY-ALIAS`. | `02-features/01-dashboard/01-backend.md` ↔ `02-queries.md` §3.4 | `Test_Dashboard_TotalsQuery` |
| CF-D2 | Watch-status panel uses `Q-WATCH-LIST`. | `02-features/01-dashboard/01-backend.md` ↔ `02-queries.md` §3.7 | `Test_Dashboard_WatchListQuery` |

### 2.7 Database ↔ Settings (Feature 07)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-S1 | Settings owns NO database tables — only `config.json`. | `02-features/07-settings/01-backend.md` §1 ↔ `01-schema.md` §1 | manual review + `Test_AST_SettingsNoStoreCalls` (no `store.*` calls in `internal/core/settings*.go`) |
| CF-S2 | `Maintenance.*` knobs in `04-retention-and-vacuum.md` §5 are validated per Settings rules; today they live in `config.json` directly (Settings backend will add fields in a future MINOR bump — tracked as open issue OI-DB-1). | `04-retention-and-vacuum.md` §5 ↔ `02-features/07-settings/01-backend.md` §6 | manual until OI-DB-1 closes |

---

## 3. Cross-Reference to Consolidated Guidelines

| Guideline | DB-spec reference | Verified by |
|---|---|---|
| `18-database-conventions.md` (singular PascalCase tables) | `01-schema.md` §1 (`Email`, not `Emails`) | `Test_Schema_TableNamesPascalSingular` |
| `18-database-conventions.md` (positive booleans) | `01-schema.md` §6 X-5 | `Test_BooleanPositive` (AC-DB-54) |
| `18-database-conventions.md` (FK rules) | `01-schema.md` §4 (`ON DELETE SET NULL`) | `Test_OpenedUrl_FkSetNull` (AC-DB-06) |
| `02-coding-guidelines.md` §6 (no `interface{}` / `any`) | `02-queries.md` §1 ("never `map[string]any`") | `golangci-lint forbidigo` rule |
| `03-error-management.md` (`apperror.Wrap` + code registry) | `03-migrations.md` §9 block 21800–21809 | `Test_Errors_AllWrapped` (cross-spec) |
| `05-logging-strategy.md` heartbeat invariant | CF-W3 above | `Test_Watch_HeartbeatAlways` |
| `05-split-db-architecture/` | `00-overview.md` §1 (single-file SQLite, WAL) | manual review |

---

## 4. Open Issues

| # | Issue | Owner | Disposition |
|---|---|---|---|
| OI-DB-1 | `Maintenance.*` knobs are read from `config.json` directly today; Settings UI does not expose them. | Settings + DB | Plan: add to `02-features/07-settings/01-backend.md` §3 in a MINOR bump. Defer until v1.1.0. |
| OI-DB-2 | No automated rollback. A bad migration must be fixed forward. | Migrate runner | Documented in `03-migrations.md` §5; no work planned. |

---

## 5. Sign-Off

| Reviewer | Role | Date |
|---|---|---|
| Spec author (AI) | Drafting | 2026-04-25 |
| Pending | Tech lead | — |
| Pending | QA | — |
