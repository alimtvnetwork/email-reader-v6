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

> **Drift notice (Slice #137 → narrowed by Slice #163 → narrowed by Slice #165).** The table-name singular→plural drift was closed in Slice #163; the index-name drift (`IX_*`/`UX_*` → real production `Ix*` names) was closed in Slice #165: every "Index used" annotation in §3 and every plan-snippet row in §4 now references the real migration-defined names (`IxEmailsAliasUid` m0002, `IxOpenedUrlsAliasOpenedAt` m0006, `IxOpenedUrlsUnique` m0004, `IxOpenedUrlsOpenedAt` m0009, `IxEmailsAliasDeletedAt` m0013, `IxWatchEventsAliasOccurredAt` m0008). The `Q-EMAIL-LIST` (alias-empty) plan-row was rewritten as a `SCAN Emails` because no `(ReceivedAt)` index exists in production; the spec's prior `IX_Email_ReceivedAt` was a phantom. The `Q-WATCH-GET` plan-row was rewritten to `sqlite_autoindex_WatchState_1` because the table's `Alias TEXT PRIMARY KEY` produces a TEXT autoindex, not an INTEGER PK. **What still drifts:** (a) the `OpenedUrls` column-shape — §3.8 / §3.9 still document the aspirational columns (`OriginalUrl`/`Decision`/`LaunchedAt`/`BlockedReason`/`OpenedUrl`) instead of the production columns (`Url`/`OpenedAt`/`IsDeduped`/`IsIncognito`/`TraceId`); (b) the `Q-EMAIL-LIST` / `Q-EXPORT-STREAM` ORDER-BY column — spec uses `ReceivedAt`, production uses `Uid` for the list view and the integer rowid `Id` for the export stream; (c) the `WatchState` column inventory — §3.5 / §3.6 reference `LastMessageId` / `LastPolledAt` / `LastErrorCode` columns that don't exist in m0003 (production has only `Alias`, `LastUid`, `LastSubject`, `LastReceivedAt`, `UpdatedAt`, plus m0014's `ConsecutiveFailures`). (a)–(c) are tracked under the deferred "schema-evolution work, ~12 AC-DB rows" backlog item — closing them is a body-of-query rewrite, not a one-line repath. The convention rule and canonical table names live in `01-schema.md` §1; treat that as the source of truth when this file disagrees.

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
INSERT INTO Emails (
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

List/filter for the Emails view. Pagination via `LIMIT/OFFSET` keyed off the per-alias monotonic `Uid` (a per-alias autoincrement), with `Id` as the deterministic tiebreaker. Production composes the WHERE clause dynamically from `EmailsListInput{Alias, Search, Limit, Offset}` (see `internal/store/queries/queries.go::EmailsList`).

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, BodyText, BodyHtml, ReceivedAt, FilePath, CreatedAt,
       IsRead, DeletedAt
FROM Emails
WHERE (:Alias  = '' OR Alias = :Alias)
  AND (:Search = '' OR (LOWER(Subject) LIKE :SearchLike OR LOWER(FromAddr) LIKE :SearchLike))
ORDER BY Uid DESC, Id DESC
LIMIT :Limit OFFSET :Offset;
```

| Param | Type | Notes |
|---|---|---|
| `:Alias` | string | empty = all aliases |
| `:Search` | string | search term (case-folded) |
| `:SearchLike` | string | `'%'+strings.ToLower(Search)+'%'` |
| `:Limit` | int | required, > 0 (per `02-emails/01-backend.md`) |
| `:Offset` | int | ≥ 0; only emitted when `Limit > 0` |

**Tx:** none (read).
**Index used:** `IxEmailsAliasUid` (m0002) when `:Alias != ''` — the leading `Alias` column drives the seek, and the index's secondary `Uid DESC` order matches the `ORDER BY Uid DESC` directly so SQLite skips the temp B-tree sort. When `:Alias = ''`, the planner falls back to a full `SCAN Emails` and an in-memory sort; that's accepted today because the Emails row count is bounded by the retention policy (§4 of `01-schema.md`). The `IsRead` predicate (driven by sibling query `EmailsCountUnreadAll`) has no dedicated index — counting the ~10² unread tail across all aliases stays under the perf budget. Verified by `internal/store/perf_test.go`.

### 3.3 `Q-EMAIL-GET-BY-ID` — read

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, BodyText, BodyHtml, ReceivedAt, FilePath, HasAttachment, CreatedAt
FROM Emails
WHERE Id = :Id;
```

**Returns:** `Email` (full row, including bodies).

### 3.4 `Q-EMAIL-COUNT-BY-ALIAS` — read

```sql
SELECT Alias, COUNT(*) AS Total, MAX(ReceivedAt) AS NewestReceivedAt
FROM Emails
GROUP BY Alias;
```

**Tx:** none. Used by Dashboard for the per-alias summary card.

### 3.5 `Q-WATCH-GET` — read

```sql
SELECT Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt
FROM WatchState
WHERE Alias = :Alias;
```

**Returns:** zero or one row. Watch backend treats "not found" as `LastUid = 0`. The m0014 `ConsecutiveFailures` counter is **not** projected here — it is read on its own dedicated path by `core.ComputeHealth` via `AccountHealthSelectAll` (see queries.go §AccountHealth). Keeping `Q-WATCH-GET` minimal preserves the per-poll cost ceiling (≤ 1 ms in §5).

### 3.6 `Q-WATCH-UPSERT` — write

```sql
INSERT INTO WatchState (Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt)
VALUES (:Alias, :LastUid, :LastSubject, :LastReceivedAt, <NOW>)
ON CONFLICT (Alias) DO UPDATE SET
    LastUid        = excluded.LastUid,
    LastSubject    = excluded.LastSubject,
    LastReceivedAt = excluded.LastReceivedAt,
    UpdatedAt      = <NOW>;
```

`<NOW>` is the canonical SQLite RFC3339 expression `strftime('%Y-%m-%dT%H:%M:%fZ','now')` injected by `internal/store/datetime.go::sqliteRFC3339NowExpr` so the queries package stays dialect-agnostic.

**Monotonic-`LastUid` invariant** — production does **not** wrap `excluded.LastUid` in `MAX(WatchState.LastUid, excluded.LastUid)`. The Go-side caller (`internal/watcher`) asserts `new >= old` before issuing the UPSERT and emits `ER-WCH-21413` on a regression (per `01-schema.md` §3 invariant W-3 + Slice #156). Pushing the guard into SQL was rejected because (a) it would mask a logic bug rather than surface it as a coded error, and (b) the runtime assertion is already covered by `Test_Watcher_LastUidMonotonic`.

**Companion writes (failure path)** — the m0014 `ConsecutiveFailures` counter has its own paired entry points:
- `WatchStateBumpFailures` — UPSERT that seeds `ConsecutiveFailures = 1` on first-boot connect failure or increments on subsequent errors. Kept separate from `Q-WATCH-UPSERT` so success-path callers don't need to know whether the call site is a poll-outcome boundary.
- `WatchStateResetFailures` — plain UPDATE zeroing the counter on a successful poll.

Both expose the same `nowExpr` parameter shape.

### 3.7 `Q-WATCH-LIST` — read

`Q-WATCH-LIST` is **not** a standalone query in production today — the Dashboard's per-alias status panel reads through the `AccountHealthSelectAll` rollup (see `internal/store/queries/queries.go`), which `LEFT JOIN`s `WatchState` against the union of aliases seen in `WatchEvents ∪ Emails ∪ WatchState`. The minimal projection a future direct caller would use is:

```sql
SELECT Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt
FROM WatchState
ORDER BY Alias ASC;
```

Adding a real implementation requires bumping the inventory in §2 and registering the function in `queries.go`.

### 3.8 `Q-OPEN-DEDUP` — read

```sql
SELECT Id, LaunchedAt
FROM OpenedUrls
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
**Index used:** `IxOpenedUrlsAliasOpenedAt` (m0006) when `:Alias != ''` — the leading `Alias` column plus the `OpenedAt` range bounds the row scan within the dedup window. For typical `OpenUrlDedupWindow ≤ 10 min` this is well under 1 ms. The exact-Url predicate is filtered in-memory after the index seek; the per-row column-shape mismatch between this spec body (`OriginalUrl`/`Decision`/`LaunchedAt`) and the production `OpenedUrls` schema (`Url`/`OpenedAt`/`IsDeduped`) is tracked under the deferred OpenedUrls column-shape reconcile in §1.

### 3.9 `Q-OPEN-INS` — write

```sql
INSERT INTO OpenedUrls (
    EmailId, Alias, RuleName, Origin,
    OriginalUrl, OpenedUrl, Decision, BlockedReason, LaunchedAt
) VALUES (
    :EmailId, :Alias, :RuleName, :Origin,
    :OriginalUrl, :OpenedUrl, :Decision, :BlockedReason, :LaunchedAt
)
RETURNING Id;
```

`:EmailId` is `NULL` when `Origin = 'Manual'`. The unique index `IxOpenedUrlsUnique` (m0004, `OpenedUrls(EmailId, Url)`) fires on duplicate launches that share the same `(EmailId, Url)` pair — production uses `INSERT … ON CONFLICT(EmailId, Url) DO NOTHING` so the second insert reports `RowsAffected = 0` and the caller treats that as a dedup hit (the prior row is the winner). The per-row column-shape mismatch between this spec body (`LaunchedAt`/`Decision`/`BlockedReason`/`OpenedUrl`) and the production `OpenedUrls` schema (`OpenedAt`/`IsDeduped`/`IsIncognito`/`TraceId`/`Url`) is tracked under the deferred OpenedUrls column-shape reconcile in §1.

### 3.10 `Q-OPEN-LIST` — read

```sql
SELECT Id, EmailId, Alias, RuleName, Origin,
       OriginalUrl, OpenedUrl, Decision, BlockedReason,
       LaunchedAt, CreatedAt
FROM OpenedUrls
WHERE (:Alias = '' OR Alias = :Alias)
ORDER BY CreatedAt DESC
LIMIT :Limit;
```

Default `:Limit = 100` per Tools backend.

### 3.11 `Q-EXPORT-COUNT` — read

Pre-flight count for the CSV export progress bar.

```sql
SELECT COUNT(*) AS Total
FROM Emails
WHERE (:Alias = '' OR Alias = :Alias)
  AND (:Since IS NULL OR ReceivedAt >= :Since)
  AND (:Until IS NULL OR ReceivedAt <  :Until);
```

### 3.12 `Q-EXPORT-STREAM` — read (cursor)

Streaming cursor for `ExportCsv`. Driver uses `*sql.Rows` directly; no buffering above the row level.

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, ReceivedAt, FilePath, HasAttachment, CreatedAt
FROM Emails
WHERE (:Alias = '' OR Alias = :Alias)
  AND (:Since IS NULL OR ReceivedAt >= :Since)
  AND (:Until IS NULL OR ReceivedAt <  :Until)
ORDER BY ReceivedAt ASC, Id ASC;
```

**No LIMIT.** The exporter MUST `defer rows.Close()` and MUST NOT buffer all rows — verified by AST scan in `97-acceptance-criteria.md`.

---

## 4. EXPLAIN QUERY PLAN Expectations

Each query has a golden `EXPLAIN QUERY PLAN` snapshot in `internal/store/queries/testdata/<query>.plan`. CI fails if the planner stops using the indexed path (e.g., a future schema change that defeats `IxEmailsAliasUid`). Snapshots are regenerated only via `make refresh-query-plans`.

| Query | Expected plan snippet |
|---|---|
| `Q-EMAIL-LIST` (alias set) | `SEARCH Emails USING INDEX IxEmailsAliasUid (Alias=?)` |
| `Q-EMAIL-LIST` (alias empty) | `SCAN Emails` (no index covers an unfiltered list) |
| `Q-EMAIL-GET-BY-ID` | `SEARCH Emails USING INTEGER PRIMARY KEY (rowid=?)` |
| `Q-EMAIL-COUNT-BY-ALIAS` | `SCAN Emails` + `USE TEMP B-TREE FOR GROUP BY` (acceptable for small N) |
| `Q-WATCH-GET` | `SEARCH WatchState USING INDEX sqlite_autoindex_WatchState_1 (Alias=?)` (TEXT PK → autoindex, not INTEGER PK) |
| `Q-OPEN-DEDUP` | `SEARCH OpenedUrls USING INDEX IxOpenedUrlsAliasOpenedAt (Alias=? AND OpenedAt>?)` |
| `Q-OPEN-LIST` | `SEARCH OpenedUrls USING INDEX IxOpenedUrlsAliasOpenedAt (Alias=?)` (alias filter) or `SCAN OpenedUrls` (no alias) |
| `Q-EXPORT-STREAM` (alias set) | `SEARCH Emails USING INDEX IxEmailsAliasUid (Alias=?)` |

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
