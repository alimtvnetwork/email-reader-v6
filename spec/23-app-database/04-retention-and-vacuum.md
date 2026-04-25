# 04 — Retention and Vacuum

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines how long each table's rows live, when they are pruned, and when SQLite's `VACUUM` and `ANALYZE` run. Owned by `internal/store/maintenance`. All defaults are tunable from `config.json` `Maintenance.*` (see §5).

Cross-references:
- Schema: [`./01-schema.md`](./01-schema.md)
- Settings backend (config slice): [`../21-app/02-features/07-settings/01-backend.md`](../21-app/02-features/07-settings/01-backend.md)

---

## 1. Per-Table Retention

| Table | Default lifetime | Trigger | Rationale |
|---|---|---|---|
| `Email` | **infinite** (never auto-pruned) | manual via Tools (future) | `.eml` files on disk are the user's archive; deleting metadata silently is unacceptable. |
| `WatchState` | per-alias rows live as long as the alias exists in `config.json` | When an alias is removed, its `WatchState` row is deleted in the same `Accounts.RemoveAccount` tx. | Avoid orphan high-water marks resurrecting deleted aliases. |
| `OpenedUrl` (`Decision='Launched'`) | **365 days** | nightly maintenance job | Audit value diminishes; dedup window is minutes. |
| `OpenedUrl` (`Decision IN ('Blocked','Failed','Skipped')`) | **90 days** | nightly maintenance job | Security-relevant but lower volume; 90 d is enough for incident review. |
| `SchemaMigration` | **infinite** | never | Migration ledger. |

Retention is **opt-out**: setting the corresponding `Maintenance.*Days = 0` disables that prune.

---

## 2. Maintenance Schedule

The maintenance loop runs in `internal/store/maintenance` as a single goroutine started by `internal/store.Open`. Cadence:

| Job | Cadence | Operation |
|---|---|---|
| Prune `OpenedUrl` (Launched) | Every 24 h, on first idle window after 03:00 local | `Q-OPEN-PRUNE-LAUNCHED` |
| Prune `OpenedUrl` (Blocked/Failed/Skipped) | Same window | `Q-OPEN-PRUNE-BLOCKED` |
| `ANALYZE` | After any prune that deletes ≥ 1 000 rows | `ANALYZE;` |
| `PRAGMA wal_checkpoint(TRUNCATE)` | Every 6 h | reduce WAL file size |
| `VACUUM` | After a prune that deletes ≥ 10 000 rows OR weekly on Sunday 03:00 local | reclaim file space |

"Idle window" = no Watch poll in flight AND no Tools operation streaming. The maintenance loop yields to active work; if no idle window appears within 6 h, the job is **deferred** and retried next cycle (logged at INFO `event=maintenance_deferred`).

---

## 3. Prune Queries

These join the named-query inventory in `02-queries.md` (added here because they belong to the maintenance domain):

### `Q-OPEN-PRUNE-LAUNCHED`

```sql
DELETE FROM OpenedUrl
WHERE Decision = 'Launched'
  AND CreatedAt < :Cutoff;
```

`:Cutoff = now - Maintenance.LaunchedRetentionDays`.

### `Q-OPEN-PRUNE-BLOCKED`

```sql
DELETE FROM OpenedUrl
WHERE Decision IN ('Blocked','Failed','Skipped')
  AND CreatedAt < :Cutoff;
```

`:Cutoff = now - Maintenance.BlockedRetentionDays`.

Both queries are bounded — they MUST run in batches of `:Batch = 5000` rows (`LIMIT 5000` + loop) to keep individual transactions short and let the busy-timeout protect interactive queries. Verified by `Test_Maintenance_BatchSize`.

---

## 4. VACUUM Semantics

- `VACUUM` requires no other readers/writers; the maintenance loop coordinates by holding the same idle-window guard as prune.
- `VACUUM` is skipped if free-list size is < 5 % of total page count (cheap probe via `PRAGMA freelist_count` / `PRAGMA page_count`).
- Forced manual vacuum is exposed as a Tools sub-action in a future spec; not in v1.

---

## 5. Configurable Knobs

Owned by Settings; serialized under `Maintenance` in `config.json` (extends the slice in `02-features/07-settings/01-backend.md` §4):

```json
{
  "Maintenance": {
    "LaunchedRetentionDays": 365,
    "BlockedRetentionDays": 90,
    "WeeklyVacuumOn": "Sunday",
    "WeeklyVacuumHourLocal": 3,
    "WalCheckpointHours": 6,
    "PruneBatchSize": 5000
  }
}
```

Validation:

| Field | Range | Error code |
|---|---|---|
| `LaunchedRetentionDays` | 0..3650 | `ER-SET-21778` (treated as Settings validation) |
| `BlockedRetentionDays`  | 0..3650 | `ER-SET-21778` |
| `WeeklyVacuumOn` | weekday name | `ER-SET-21778` |
| `WeeklyVacuumHourLocal` | 0..23 | `ER-SET-21778` |
| `WalCheckpointHours` | 1..168 | `ER-SET-21778` |
| `PruneBatchSize` | 100..50000 | `ER-SET-21778` |

(Settings backend §6 will gain a row for each of these in a future MINOR bump; tracked under issues.)

---

## 6. Logging

Per maintenance run the loop emits at INFO:

```
component=maintenance event=run_start
component=maintenance event=prune table=OpenedUrl decision=Launched deleted=1234 batches=1 duration_ms=42
component=maintenance event=prune table=OpenedUrl decision=Blocked  deleted=15   batches=1 duration_ms=3
component=maintenance event=analyze duration_ms=18
component=maintenance event=wal_checkpoint mode=TRUNCATE pages=42 duration_ms=6
component=maintenance event=vacuum duration_ms=480 reclaimed_bytes=2097152
component=maintenance event=run_end total_duration_ms=560
```

---

## 7. Testing Contract

Tests live in `internal/store/maintenance/maintenance_test.go`. 12 required cases:

1. Prune-launched deletes only `Decision='Launched'` rows older than cutoff.
2. Prune-blocked deletes only the three blocked-style decisions older than cutoff.
3. Prune respects `PruneBatchSize` (delete in chunks; verified via row-count probe between batches).
4. `LaunchedRetentionDays = 0` disables that prune entirely (no DELETE issued).
5. `BlockedRetentionDays = 0` likewise.
6. ANALYZE runs only when ≥ 1 000 rows deleted.
7. VACUUM runs only when ≥ 10 000 rows deleted OR weekly window hit.
8. VACUUM skipped when free-list < 5 % of pages.
9. WAL checkpoint runs every 6 h with `TRUNCATE` mode.
10. Maintenance defers when a Watch poll is in flight (uses an injected `IdleProbe`).
11. Concurrent maintenance × 2 (test scaffolding only) — second call is a no-op.
12. AST scan: only `internal/store/maintenance` issues `VACUUM`, `ANALYZE`, or `PRAGMA wal_checkpoint`.
