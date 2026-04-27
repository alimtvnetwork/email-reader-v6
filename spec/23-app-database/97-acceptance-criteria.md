# 23 — App Database — Acceptance Criteria

**Version:** 1.0.1
**Updated:** 2026-04-27
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Binary, machine-checkable acceptance criteria for `spec/23-app-database/`. Each item maps to ≥ 1 automated test in the file path listed.

---

---

## Sandbox feasibility legend (added Slice #184 — see `mem://workflow/progress-tracker.md`)

A fresh AI picking up an unchecked row should consult the `**Sandbox:**` tag immediately under
each section header to decide whether the row is implementable in the Lovable sandbox or must
be deferred to a workstation/CI runner.

| Tag | Meaning | Implementable in sandbox? |
|---|---|---|
| 🟢 **headless** | Go unit/integration tests, AST scanners, log greps, spec-doc edits. Verified via `nix run nixpkgs#go -- test -tags nofyne ./...`. | **Yes** — preferred sandbox work. |
| 🟡 **cgo-required** | Fyne canvas widget tests, real driver behaviour. Needs cgo + GL/X11. See `mem://workflow/canvas-harness-starter.md` (Slice #180). | **No** — defer to workstation; planned. |
| 🔴 **needs bench / E2E infra** | p95 perf gates (bench infra) or multi-process IMAP+browser E2E. See `mem://workflow/{bench,e2e}-harness-starter.md` (Slices #178/#179). | **No** — defer to CI runner; planned. |
| ⚪ **N/A** | Manual sign-off checklist; no automated test possible. | **No** — human reviewer. |

A section may carry **two** tags when its rows split (e.g. `🟢 + 🔴`); pick the right tag per row by reading the row itself.

## A. Schema Integrity

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| # | Criterion | Test ID |
|---|---|---|
| AC-DB-01 | Fresh `Open()` on a non-existent DB creates exactly the four tables in `01-schema.md` §1 — no more, no fewer. | `Test_Open_FreshSchema` |
| AC-DB-02 | Every column listed in `01-schema.md` §2–§5 exists with the documented type and NULL-ability. | `Test_Schema_ColumnsMatchSpec` (driven by `PRAGMA table_info`) |
| AC-DB-03 | Every index listed in `01-schema.md` exists. No additional indexes exist beyond those listed. | `Test_Schema_IndexesMatchSpec` |
| AC-DB-04 | `UX_Email_Alias_MessageId` is enforced (insert two rows with same `(Alias, MessageId)` → conflict). | `Test_Email_UniqueAliasMessageId` |
| AC-DB-05 | `UX_OpenedUrl_Dedup` is partial (`WHERE Decision='Launched'`); inserting two `Blocked` rows with identical key columns succeeds. | `Test_OpenedUrl_Dedup_PartialIndex` |
| AC-DB-06 | `OpenedUrls.EmailId` is `ON DELETE SET NULL` (delete an `Emails` row → matching `OpenedUrls` rows have `EmailId = NULL`, not deleted). | `Test_OpenedUrl_FkSetNull` |
| AC-DB-07 | `Origin` enum rejects values not in `{Watcher, Manual, Rule}`. | `Test_OpenedUrl_OriginCheck` |
| AC-DB-08 | `Decision` enum rejects values not in `{Launched, Blocked, Skipped, Failed}`. | `Test_OpenedUrl_DecisionCheck` |
| AC-DB-09 | `HasAttachment` only accepts `0` or `1`. | `Test_Email_HasAttachmentCheck` |

## B. PRAGMAs

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| # | Criterion | Test ID |
|---|---|---|
| AC-DB-10 | Every connection from the pool reports `journal_mode=wal`, `synchronous=1` (NORMAL), `foreign_keys=1`, `busy_timeout=5000`. | `Test_Store_PragmaOnEveryConn` |
| AC-DB-11 | Closing then reopening preserves WAL mode (does not silently fall back to DELETE). | `Test_Store_WalPersists` |

## C. Named Queries

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| # | Criterion | Test ID |
|---|---|---|
| AC-DB-20 | Every `Q-*` declared in `02-queries.md` §2 has a matching Go function in `internal/store/queries/`. | `Test_Queries_AllImplemented` (AST + reflect) |
| AC-DB-21 | No SQL string outside `internal/store/queries/` or `internal/store/migrate/`. | `Test_AST_NoStraySql` |
| AC-DB-22 | `Q-EMAIL-UPSERT` returns the same `Id` on second call with the same `(Alias, MessageId)`. | `Test_Q_EmailUpsert_Idempotent` |
| AC-DB-23 | `Q-WATCH-UPSERT` never decreases `LastUid` (even if caller passes a smaller value — `MAX(...)` clause). | `Test_Q_WatchUpsert_Monotonic` |
| AC-DB-24 | `Q-OPEN-DEDUP` returns a row when a `Launched` row exists within `:Since` window. | `Test_Q_OpenDedup_Hit` |
| AC-DB-25 | `Q-OPEN-DEDUP` returns no row when only `Blocked` rows match (partial index excludes them). | `Test_Q_OpenDedup_BlockedNotMatched` |
| AC-DB-26 | `Q-EXPORT-STREAM` is consumed via `*sql.Rows`; the exporter never calls `rows.Next` after `rows.Close`. | `Test_Q_ExportStream_NoBuffering` (race-free run + memory ceiling) |
| AC-DB-27 | `EXPLAIN QUERY PLAN` for every `Q-*` matches the golden snapshot in `02-queries.md` §4. | `Test_Queries_PlanGolden` |
| AC-DB-28 | Every `Q-*` p95 latency on the 100 k-row synthetic dataset meets `02-queries.md` §5 budgets. | `Test_Queries_Perf` |

## D. Migrations

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| # | Criterion | Test ID |
|---|---|---|
| AC-DB-30 | `migrate.Apply` on an empty DB applies all 4 v1.0.0 migrations in order. | `Test_Migrate_FreshApplyAll` |
| AC-DB-31 | `migrate.Apply` on an up-to-date DB is a no-op. | `Test_Migrate_NoOpWhenCurrent` |
| AC-DB-32 | Version gap returns `ER-MIG-22101`. | `Test_Migrate_GapDetected` |
| AC-DB-33 | Tampered checksum returns `ER-MIG-22103`. | `Test_Migrate_ChecksumMismatch` |
| AC-DB-34 | Stored unknown version returns `ER-MIG-22104`. | `Test_Migrate_DowngradeRejected` |
| AC-DB-35 | Mid-tx crash leaves DB at prior version (no partial `_SchemaVersion` row). | `Test_Migrate_CrashSafe` |
| AC-DB-36 | M004 renames legacy `Emails`/`OpenedUrls` when present; no-op otherwise. | `Test_Migrate_LegacyRename` |
| AC-DB-37 | AST scan: only `internal/store/migrate` issues `CREATE`/`ALTER`/`DROP`. | `Test_AST_DdlOnlyInMigrate` |

## E. Maintenance / Retention

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| # | Criterion | Test ID |
|---|---|---|
| AC-DB-40 | `Q-OPEN-PRUNE-LAUNCHED` only deletes `Decision='Launched'` rows older than cutoff. | `Test_Maintenance_PruneLaunchedScope` |
| AC-DB-41 | `Q-OPEN-PRUNE-BLOCKED` only deletes the three blocked-style decisions older than cutoff. | `Test_Maintenance_PruneBlockedScope` |
| AC-DB-42 | Retention `0` disables the corresponding prune entirely (zero DELETE statements observed). | `Test_Maintenance_RetentionZeroDisables` |
| AC-DB-43 | Prune runs in batches of `PruneBatchSize` rows. | `Test_Maintenance_BatchSize` |
| AC-DB-44 | ANALYZE only runs when ≥ 1 000 rows deleted. | `Test_Maintenance_AnalyzeThreshold` |
| AC-DB-45 | VACUUM only runs when ≥ 10 000 rows deleted OR weekly window hit. | `Test_Maintenance_VacuumThreshold` |
| AC-DB-46 | Maintenance defers while Watch poll is in flight (verified via injected `IdleProbe`). | `Test_Maintenance_DefersOnBusy` |
| AC-DB-47 | AST scan: only `internal/store/maintenance` issues `VACUUM`/`ANALYZE`/`PRAGMA wal_checkpoint`. | `Test_AST_MaintenanceOnly` |

## F. Ownership / Anti-Features

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| # | Criterion | Test ID |
|---|---|---|
| AC-DB-50 | AST scan: only `internal/store` (and its sub-packages) imports a SQL driver. | `Test_AST_DriverImportLimit` |
| AC-DB-51 | AST scan: no `*sql.DB`, `*sql.Tx`, or `*sql.Rows` value escapes `internal/store/...` (return types of public methods do not include them). | `Test_AST_NoSqlTypeLeak` |
| AC-DB-52 | AST scan: feature backends (`internal/core/*`) call typed `store.*` methods only. | `Test_AST_CoreUsesStoreOnly` |
| AC-DB-53 | All datetime values in `Emails`, `WatchState`, `OpenedUrls` are stored as RFC 3339 UTC strings (regex on a sample fetch). | `Test_DateTime_FormatUtc` |
| AC-DB-54 | All boolean columns are positive-named (no `Is` / `Has` columns whose `0` value means a positive condition). | `Test_BooleanPositive` |
| AC-DB-55 | `OriginalUrl` value never appears in test logger output above DEBUG. | `Test_LogScan_NoOriginalUrlLeak` |

## G. Performance

**Sandbox:** 🔴 **needs bench infra** — see `mem://workflow/bench-infra-starter.md` (Slice #178).

| # | Criterion | Threshold |
|---|---|---|
| AC-DBP-01 | Cold `Open()` on 100 k-row DB | ≤ 80 ms p95 |
| AC-DBP-02 | Watch per-message tx (`Q-EMAIL-UPSERT`) | ≤ 5 ms p95 |
| AC-DBP-03 | `Q-EMAIL-LIST` (`Limit=50`, alias set) on 100 k rows | ≤ 25 ms p95 |
| AC-DBP-04 | `Q-OPEN-DEDUP` within 10-min window on 100 k OpenedUrls rows | ≤ 2 ms p95 |
| AC-DBP-05 | `Q-EXPORT-STREAM` per-row | ≤ 0.5 ms p95 |
| AC-DBP-06 | Memory used by `ExportCsv` over 100 k rows | ≤ 32 MiB peak (streaming) |

## H. Definition of Done

**Sandbox:** ⚪ **N/A** — manual sign-off checklist; no automated gate.

All AC-DB-*, AC-DBP-* automated tests pass on `linux/amd64`, `darwin/arm64`, `windows/amd64`. `make spec-check` reports zero TODOs in `spec/23-app-database/`. `EXPLAIN QUERY PLAN` golden files committed under `internal/store/queries/testdata/`.
