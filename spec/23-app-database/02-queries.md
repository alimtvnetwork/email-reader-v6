# 02 — Queries

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Every named query (`Q-*`) in the app, with its SQL, parameters, projection, transactional context, and the feature backend that calls it. New named queries MUST be added here before being implemented in `internal/store`.

Cross-references:
- Schema: [`./01-schema.md`](./01-schema.md)
- Tools backend: [`../21-app/02-features/06-tools/01-backend.md`](../21-app/02-features/06-tools/01-backend.md)
- Watch backend: [`../21-app/02-features/05-watch/01-backend.md`](../21-app/02-features/05-watch/01-backend.md)
- Emails backend: [`../21-app/02-features/02-emails/01-backend.md`](../21-app/02-features/02-emails/01-backend.md)
- Dashboard backend: [`../21-app/02-features/01-dashboard/01-backend.md`](../21-app/02-features/01-dashboard/01-backend.md)

> **Drift notice (Slice #137 → narrowed by Slice #163).** The table-name singular→plural drift was closed in Slice #163: every query body and projection in §3 + every plan-snapshot row in §4 now uses the locked plural names (`Emails`, `OpenedUrls`, `WatchEvents`) per `01-schema.md` §1, with `WatchState` (singleton-per-key) and `_SchemaVersion` (bookkeeping) staying singular. **What still drifts:** the spec's `IX_Email_*` / `IX_OpenedUrl_*` / `UX_OpenedUrl_Dedup` index references describe a Pascal-Case-with-underscores naming scheme that doesn't match production (`IxEmailsAliasUid`, `IxOpenedUrlsAliasOpenedAt`, `IxOpenedUrlsUnique`, etc. — see `internal/store/migrate/m0002..m0013`), AND some of the spec's expected indexes don't exist at all (e.g. no `(Alias, ReceivedAt)` index for `Q-EMAIL-LIST`). Both are tracked under the deferred "schema-evolution work, ~12 AC-DB rows" backlog item — the index-name fix needs a coordinated decision between (a) renaming impl indexes to match spec, or (b) editing every index reference here AND every plan-snapshot expectation in lockstep. The convention rule and canonical table names live in `01-schema.md` §1; treat that as the source of truth when this file disagrees.

---

## 1. Convention

| Item | Rule |
|---|---|
| Naming | `Q-<DOMAIN>-<VERB>` upper-kebab. Examples: `Q-EMAIL-UPSERT`, `Q-OPEN-DEDUP`. |
| Owning file | `internal/store/queries/<domain>.go` — exactly one Go function per `Q-*`. |
| Parameter style | Named: `:Alias`, `:Uid`, etc. (`modernc.org/sqlite` supports `?NNN` and `:name`). |
| Projection | One Go struct per `Q-*` returning rows; never `map[string]any`. |
| Tx | Listed per-query below. Default = autocommit. Multi-statement = explicit `BEGIN IMMEDIATE`. |
| Logging | Each call site logs `query=Q-…` at DEBUG with `duration_ms` (per `05-logging-strategy.md` §6.x). |

---

## 2. Inventory

| Query | Domain | Read/Write | Caller |
|---|---|---|---|
| `Q-EMAIL-UPSERT` | Email | W | Watch backend §4 (per new message) |
| `Q-EMAIL-LIST` | Email | R | Emails backend (list/filter view) |
| `Q-EMAIL-GET-BY-ID` | Email | R | Emails backend (detail view) |
| `Q-EMAIL-COUNT-BY-ALIAS` | Email | R | Dashboard backend (totals panel) |
| `Q-WATCH-GET` | WatchState | R | Watch backend §3 (poll start) |
| `Q-WATCH-UPSERT` | WatchState | W | Watch backend §6 (poll end, success or failure) |
| `Q-WATCH-LIST` | WatchState | R | Dashboard backend (per-alias status) |
| `Q-OPEN-DEDUP` | OpenedUrl | R | Tools backend §3.4 step 4 (dedup check) |
| `Q-OPEN-INS` | OpenedUrl | W | Tools backend §3.4 step 7 (audit insert) |
| `Q-OPEN-LIST` | OpenedUrl | R | Tools backend `RecentOpenedUrls` |
| `Q-EXPORT-COUNT` | Email | R | Tools backend `ExportCsv` (pre-flight count) |
| `Q-EXPORT-STREAM` | Email | R (stream) | Tools backend `ExportCsv` (cursor) |

Total: 12 named queries. Adding one requires updating §3 below AND the consistency report.

---

## 3. Definitions

### 3.1 `Q-EMAIL-UPSERT` — write

Idempotent insert of an IMAP message. Conflict on `(Alias, MessageId)` is the **expected** path on re-fetch (e.g., after a crash mid-cycle).

```sql
INSERT INTO Email (
    Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
    Subject, BodyText, BodyHtml, ReceivedAt, FilePath, HasAttachment
) VALUES (
    :Alias, :MessageId, :Uid, :FromAddr, :ToAddr, :CcAddr,
    :Subject, :BodyText, :BodyHtml, :ReceivedAt, :FilePath, :HasAttachment
)
ON CONFLICT (Alias, MessageId) DO UPDATE SET
    Uid           = excluded.Uid,
    FromAddr      = excluded.FromAddr,
    ToAddr        = excluded.ToAddr,
    CcAddr        = excluded.CcAddr,
    Subject       = excluded.Subject,
    BodyText      = excluded.BodyText,
    BodyHtml      = excluded.BodyHtml,
    ReceivedAt    = excluded.ReceivedAt,
    FilePath      = excluded.FilePath,
    HasAttachment = excluded.HasAttachment
RETURNING Id;
```

| Param | Type | Notes |
|---|---|---|
| `:Alias` | string | non-empty |
| `:MessageId` | string | RFC 822 or synthesized |
| `:Uid` | int64 | ≥ 1 |
| `:FromAddr`/`:ToAddr`/`:CcAddr` | string | UTF-8, decoded |
| `:Subject` | string | decoded, no MIME tokens |
| `:BodyText`/`:BodyHtml` | string | may be empty |
| `:ReceivedAt` | RFC 3339 string (UTC) | required |
| `:FilePath` | string | absolute, validated by `paths.Validator` |
| `:HasAttachment` | int (0/1) | required |

**Returns:** `Id` (int64).
**Tx:** part of the per-message tx in Watch backend §4 (so the `.eml` write and Email row commit atomically).

### 3.2 `Q-EMAIL-LIST` — read

List/filter for the Emails view. Pagination via keyset on `(ReceivedAt, Id)`.

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, ReceivedAt, FilePath, HasAttachment, CreatedAt
FROM Email
WHERE (:Alias = '' OR Alias = :Alias)
  AND (:Q     = '' OR (Subject LIKE :QLike OR FromAddr LIKE :QLike))
  AND (:Since IS NULL OR ReceivedAt >= :Since)
  AND (:CursorReceivedAt IS NULL
       OR ReceivedAt < :CursorReceivedAt
       OR (ReceivedAt = :CursorReceivedAt AND Id < :CursorId))
ORDER BY ReceivedAt DESC, Id DESC
LIMIT :Limit;
```

| Param | Type | Notes |
|---|---|---|
| `:Alias` | string | empty = all |
| `:Q` | string | search term |
| `:QLike` | string | `'%'+escapeLike(Q)+'%'` |
| `:Since` | RFC 3339 / NULL | optional lower bound |
| `:CursorReceivedAt` | RFC 3339 / NULL | keyset cursor |
| `:CursorId` | int64 / NULL | keyset cursor |
| `:Limit` | int | 1..200 (per `02-emails/01-backend.md`) |

**Tx:** none (read).
**Index used:** `IX_Email_Alias_Received` when `:Alias != ''`, else `IX_Email_ReceivedAt`. Verified by `EXPLAIN QUERY PLAN` test.

### 3.3 `Q-EMAIL-GET-BY-ID` — read

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, BodyText, BodyHtml, ReceivedAt, FilePath, HasAttachment, CreatedAt
FROM Email
WHERE Id = :Id;
```

**Returns:** `Email` (full row, including bodies).

### 3.4 `Q-EMAIL-COUNT-BY-ALIAS` — read

```sql
SELECT Alias, COUNT(*) AS Total, MAX(ReceivedAt) AS NewestReceivedAt
FROM Email
GROUP BY Alias;
```

**Tx:** none. Used by Dashboard for the per-alias summary card.

### 3.5 `Q-WATCH-GET` — read

```sql
SELECT Alias, LastUid, LastMessageId, LastSubject, LastReceivedAt,
       LastPolledAt, LastErrorCode, UpdatedAt
FROM WatchState
WHERE Alias = :Alias;
```

**Returns:** zero or one row. Watch backend treats "not found" as `LastUid = 0`.

### 3.6 `Q-WATCH-UPSERT` — write

```sql
INSERT INTO WatchState (
    Alias, LastUid, LastMessageId, LastSubject, LastReceivedAt,
    LastPolledAt, LastErrorCode, UpdatedAt
) VALUES (
    :Alias, :LastUid, :LastMessageId, :LastSubject, :LastReceivedAt,
    :LastPolledAt, :LastErrorCode, :UpdatedAt
)
ON CONFLICT (Alias) DO UPDATE SET
    LastUid        = MAX(WatchState.LastUid, excluded.LastUid),  -- monotonic
    LastMessageId  = excluded.LastMessageId,
    LastSubject    = excluded.LastSubject,
    LastReceivedAt = excluded.LastReceivedAt,
    LastPolledAt   = excluded.LastPolledAt,
    LastErrorCode  = excluded.LastErrorCode,
    UpdatedAt      = excluded.UpdatedAt;
```

The `MAX(...)` clause enforces invariant W-3 from `01-schema.md` §3 at the SQL level — a stale-write race cannot rewind `LastUid`.

### 3.7 `Q-WATCH-LIST` — read

```sql
SELECT Alias, LastUid, LastSubject, LastReceivedAt,
       LastPolledAt, LastErrorCode, UpdatedAt
FROM WatchState
ORDER BY Alias ASC;
```

### 3.8 `Q-OPEN-DEDUP` — read

```sql
SELECT Id, LaunchedAt
FROM OpenedUrl
WHERE Alias       = :Alias
  AND OriginalUrl = :OriginalUrl
  AND Decision    = 'Launched'
  AND LaunchedAt  >= :Since
ORDER BY LaunchedAt DESC
LIMIT 1;
```

| Param | Type | Notes |
|---|---|---|
| `:Alias` | string | dedup is per-alias, not global |
| `:OriginalUrl` | string | exact match (no normalization) |
| `:Since` | RFC 3339 | `now - OpenUrlDedupWindow` (Tools backend §3.4) |

**Returns:** zero or one row. Caller uses presence to short-circuit launch.
**Index used:** falls back to `IX_OpenedUrl_Alias_Created` then row scan within window. For typical `OpenUrlDedupWindow ≤ 10 min` this is well under 1 ms.

### 3.9 `Q-OPEN-INS` — write

```sql
INSERT INTO OpenedUrl (
    EmailId, Alias, RuleName, Origin,
    OriginalUrl, OpenedUrl, Decision, BlockedReason, LaunchedAt
) VALUES (
    :EmailId, :Alias, :RuleName, :Origin,
    :OriginalUrl, :OpenedUrl, :Decision, :BlockedReason, :LaunchedAt
)
RETURNING Id;
```

`:EmailId` is `NULL` when `Origin = 'Manual'`. The partial unique index `UX_OpenedUrl_Dedup` may fire on duplicate launches within the same millisecond — caller catches `SQLITE_CONSTRAINT_UNIQUE` and treats as success (the prior row is the winner).

### 3.10 `Q-OPEN-LIST` — read

```sql
SELECT Id, EmailId, Alias, RuleName, Origin,
       OriginalUrl, OpenedUrl, Decision, BlockedReason,
       LaunchedAt, CreatedAt
FROM OpenedUrl
WHERE (:Alias = '' OR Alias = :Alias)
ORDER BY CreatedAt DESC
LIMIT :Limit;
```

Default `:Limit = 100` per Tools backend.

### 3.11 `Q-EXPORT-COUNT` — read

Pre-flight count for the CSV export progress bar.

```sql
SELECT COUNT(*) AS Total
FROM Email
WHERE (:Alias = '' OR Alias = :Alias)
  AND (:Since IS NULL OR ReceivedAt >= :Since)
  AND (:Until IS NULL OR ReceivedAt <  :Until);
```

### 3.12 `Q-EXPORT-STREAM` — read (cursor)

Streaming cursor for `ExportCsv`. Driver uses `*sql.Rows` directly; no buffering above the row level.

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, ReceivedAt, FilePath, HasAttachment, CreatedAt
FROM Email
WHERE (:Alias = '' OR Alias = :Alias)
  AND (:Since IS NULL OR ReceivedAt >= :Since)
  AND (:Until IS NULL OR ReceivedAt <  :Until)
ORDER BY ReceivedAt ASC, Id ASC;
```

**No LIMIT.** The exporter MUST `defer rows.Close()` and MUST NOT buffer all rows — verified by AST scan in `97-acceptance-criteria.md`.

---

## 4. EXPLAIN QUERY PLAN Expectations

Each query has a golden `EXPLAIN QUERY PLAN` snapshot in `internal/store/queries/testdata/<query>.plan`. CI fails if the planner stops using the indexed path (e.g., a future schema change that defeats `IX_Email_Alias_Received`). Snapshots are regenerated only via `make refresh-query-plans`.

| Query | Expected plan snippet |
|---|---|
| `Q-EMAIL-LIST` (alias set) | `SEARCH Email USING INDEX IX_Email_Alias_Received` |
| `Q-EMAIL-LIST` (alias empty) | `SEARCH Email USING INDEX IX_Email_ReceivedAt` |
| `Q-EMAIL-GET-BY-ID` | `SEARCH Email USING INTEGER PRIMARY KEY (rowid=?)` |
| `Q-EMAIL-COUNT-BY-ALIAS` | `SCAN Email` + `USE TEMP B-TREE FOR GROUP BY` (acceptable for small N) |
| `Q-WATCH-GET` | `SEARCH WatchState USING INTEGER PRIMARY KEY` |
| `Q-OPEN-DEDUP` | `SEARCH OpenedUrl USING INDEX IX_OpenedUrl_Alias_Created` |
| `Q-OPEN-LIST` | `SEARCH OpenedUrl USING INDEX IX_OpenedUrl_Alias_Created` |
| `Q-EXPORT-STREAM` (alias set) | `SEARCH Email USING INDEX IX_Email_Alias_Received` |

---

## 5. Performance Budgets

| Query | p95 | Notes |
|---|---|---|
| `Q-EMAIL-UPSERT` | ≤ 5 ms | inside Watch per-message tx |
| `Q-EMAIL-LIST` (`Limit=50`) | ≤ 25 ms | with 100 k rows |
| `Q-EMAIL-GET-BY-ID` | ≤ 2 ms | |
| `Q-WATCH-GET` | ≤ 1 ms | |
| `Q-WATCH-UPSERT` | ≤ 3 ms | |
| `Q-OPEN-DEDUP` | ≤ 2 ms | within 10-min window |
| `Q-OPEN-INS` | ≤ 5 ms | |
| `Q-EXPORT-STREAM` (per row) | ≤ 0.5 ms | streaming, not batch |

Budgets are enforced by `internal/store/perf_test.go` against a synthetic 100 k-row dataset.
