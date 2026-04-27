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

> **Drift notice (Slice #137 → narrowed by Slice #163 → narrowed by Slice #165 → closed by Slice #167).** The table-name singular→plural drift was closed in Slice #163; the index-name drift (`IX_*`/`UX_*` → real production `Ix*` names) was closed in Slice #165: every "Index used" annotation in §3 and every plan-snippet row in §4 references the real migration-defined names (`IxEmailsAliasUid` m0002, `IxOpenedUrlsAliasOpenedAt` m0006, `IxOpenedUrlsUnique` m0004, `IxOpenedUrlsOpenedAt` m0009, `IxEmailsAliasDeletedAt` m0013, `IxWatchEventsAliasOccurredAt` m0008). The `Q-EMAIL-LIST` (alias-empty) plan-row was rewritten as a `SCAN Emails` because no `(ReceivedAt)` index exists in production; the spec's prior `IX_Email_ReceivedAt` was a phantom. The `Q-WATCH-GET` plan-row was rewritten to `sqlite_autoindex_WatchState_1` because the table's `Alias TEXT PRIMARY KEY` produces a TEXT autoindex, not an INTEGER PK. **Slice #167** closed the remaining body-of-query drift: (a) `Q-OPEN-DEDUP` is now the production `SELECT COUNT(1) … WHERE EmailId=? AND Url=?` (was: aspirational `Alias`/`OriginalUrl`/`Decision='Launched'`/`LaunchedAt` projection); (b) `Q-OPEN-INS` now lists production columns (`Url`/`OriginalUrl`/`IsDeduped`/`IsIncognito`/`TraceId`) with the real `ON CONFLICT(EmailId, Url) DO NOTHING` clause (was: aspirational `OpenedUrl`/`Decision`/`BlockedReason`/`LaunchedAt` + `RETURNING Id`); (c) `Q-OPEN-LIST` now has the production `OpenedAt`-keyset shape with `:Before`/`:Origin` filters (was: aspirational `CreatedAt` ordering); (d) `Q-EMAIL-LIST` now uses `ORDER BY Uid DESC, Id DESC` with `:Limit/:Offset` (was: keyset on `(ReceivedAt, Id)`); (e) `Q-EXPORT-STREAM` now uses `ORDER BY Id ASC` and lists `BodyText`/`BodyHtml`, drops `HasAttachment` (was: `ORDER BY ReceivedAt ASC, Id ASC` over a phantom `(ReceivedAt)` index); (f) `Q-WATCH-GET`/`Q-WATCH-UPSERT` now project only the m0003+m0014 columns (`Alias`/`LastUid`/`LastSubject`/`LastReceivedAt`/`UpdatedAt`/`ConsecutiveFailures`), with the spec'd `MAX(WatchState.LastUid, excluded.LastUid)` SQL guard reframed as a Go-side runtime assertion (`ER-WCH-21413`) — phantom columns `LastMessageId`/`LastPolledAt`/`LastErrorCode` removed; `WatchStateBumpFailures` / `WatchStateResetFailures` documented as paired m0014 entry points; (g) `Q-WATCH-LIST` reframed as the `AccountHealthSelectAll` rollup (no standalone production query). The convention rule and canonical table names live in `01-schema.md` §1; this file now matches production at `internal/store/queries/queries.go` HEAD. **Decision rationale (per user rule "best one that doesn't hurt the other code and also best for the application"):** chose Path B (rewrite spec to match impl) over Path A (ship inventing migrations) — Path A would have meant inventing a `Decision`/`BlockedReason`/`LaunchedAt`/`OpenedUrl` quartet on `OpenedUrls` plus a `LastMessageId`/`LastPolledAt`/`LastErrorCode` triple on `WatchState`, none of which the runtime needs. Zero impl/runtime churn — pure spec-text edit.

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

Existence check for `(EmailId, Url)` against the production `IxOpenedUrlsUnique` index. Returns a count (0 or 1) — the caller (`internal/store/store.go::HasOpenedUrl`) treats `> 0` as "already recorded" and short-circuits the launch. Production query is `queries.HasOpenedUrl`.

```sql
SELECT COUNT(1)
FROM OpenedUrls
WHERE EmailId = :EmailId
  AND Url     = :Url;
```

| Param | Type | Notes |
|---|---|---|
| `:EmailId` | int64 | the parent email's `Emails.Id` |
| `:Url` | string | exact match on the canonicalized URL (the `Url` column already holds the post-redaction value the caller will pass to the browser) |

**Returns:** `int` (0 or 1). Manual launches (no parent email) bypass this query — they go straight to `Q-OPEN-INS` because `EmailId` would be NULL and the `(EmailId, Url)` unique constraint cannot match a NULL.

**Tx:** none (read).
**Index used:** `IxOpenedUrlsUnique` (m0004, `OpenedUrls(EmailId, Url)`) — both columns are equality-bound so SQLite seeks directly to the matching row. Cost is independent of `OpenUrlDedupWindow`. Sub-millisecond at any realistic table size.

> **Per-alias / time-windowed dedup variant.** The Tools-backend dedup-window concept (`OpenUrlDedupWindow`, "no double-launch within N minutes") is not a separate query — it is enforced at write time by `Q-OPEN-INS`'s `ON CONFLICT(EmailId, Url) DO NOTHING`. A future per-alias-anywhere variant (i.e. dedup across emails) would need a new index on `(Alias, OpenedAt)` and a new query slot; tracked under the deferred Tools-backend Delta-#2 work, not this file.

### 3.9 `Q-OPEN-INS` — write

Append the audit row for one URL launch (or blocked decision). On `(EmailId, Url)` conflict the row is left untouched and `RowsAffected = 0` — the caller treats that as a dedup hit (the prior row is the winner). Production query is `queries.OpenedUrlInsert`.

```sql
INSERT INTO OpenedUrls (
    EmailId, RuleName, Url, Alias, Origin,
    OriginalUrl, IsDeduped, IsIncognito, TraceId
) VALUES (
    :EmailId, :RuleName, :Url, :Alias, :Origin,
    :OriginalUrl, :IsDeduped, :IsIncognito, :TraceId
)
ON CONFLICT (EmailId, Url) DO NOTHING;
```

| Param | Type | Notes |
|---|---|---|
| `:EmailId` | int64 | parent email; NULL is **not** supported by m0004's NOT NULL — Manual launches use a sentinel email row (Tools backend §3.4) |
| `:RuleName` | string | empty unless `Origin = 'Rule'` |
| `:Url` | string | post-redaction (stage 1) URL — what was actually launched |
| `:Alias` | string | account context (always populated, even for Manual) |
| `:Origin` | string | `Watcher` / `Manual` / `Rule` (Tools backend §3.4 enum) |
| `:OriginalUrl` | string | URL **as found** in the email body, including tracking params. Never logged above DEBUG (Tools backend §11). |
| `:IsDeduped` | int (0/1) | `1` when the caller short-circuited via `Q-OPEN-DEDUP` and is recording the audit-only ghost row |
| `:IsIncognito` | int (0/1) | `1` when the launch used the browser's incognito profile (Settings → Tools) |
| `:TraceId` | string | request-scoped trace ID for correlation with logs |

**Returns:** `RowsAffected` (1 = inserted, 0 = dedup hit).
**Tx:** none (single statement). Insert MUST run **after** the browser launch returns (forensic completeness — Tools backend §3.4 step 7).
**Note on `Decision` / `BlockedReason` / `LaunchedAt` / `OpenedUrl`:** these aspirational columns from earlier drafts are **not** in the production schema. Block/skip/fail outcomes are encoded by **omitting** the row entirely — the audit ledger records launches only. The Tools backend logs blocked decisions to the structured-log stream (`ER-TLS-2176X`) instead of the DB.

### 3.10 `Q-OPEN-LIST` — read

Reverse-chronological page of audit rows for the Tools "Recent opened URLs" view. Production query is `queries.OpenedUrlsList` and uses keyset pagination on `OpenedAt` (the `:Before` cursor).

```sql
SELECT Id, EmailId, Alias, RuleName, Origin, Url,
       OriginalUrl, IsDeduped, IsIncognito, TraceId, OpenedAt
FROM OpenedUrls
WHERE OpenedAt < :Before
  AND (:Alias  = '' OR Alias  = :Alias)
  AND (:Origin = '' OR Origin = :Origin)
ORDER BY OpenedAt DESC, Id DESC
LIMIT :Limit;
```

| Param | Type | Notes |
|---|---|---|
| `:Before` | RFC 3339 | required keyset cursor; first page passes `time.Now()` |
| `:Alias` | string | empty = all aliases |
| `:Origin` | string | empty = all origins; otherwise `Watcher` / `Manual` / `Rule` |
| `:Limit` | int | required, > 0 (default 100 per Tools backend) |

**Tx:** none (read).
**Index used:** `IxOpenedUrlsAliasOpenedAt` (m0006) when `:Alias != ''` — leading `Alias` equality + secondary `OpenedAt DESC` matches `ORDER BY OpenedAt DESC` directly. When `:Alias = ''`, falls back to `IxOpenedUrlsOpenedAt` (m0009) for the `OpenedAt < :Before` range. The `:Origin` predicate is filtered in-memory after the index seek.

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

Streaming cursor for `ExportCsv`. Driver uses `*sql.Rows` directly; no buffering above the row level. Production query is `queries.EmailExport`.

```sql
SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
       Subject, BodyText, BodyHtml, ReceivedAt, FilePath, CreatedAt
FROM Emails
WHERE (:Alias = '' OR Alias = :Alias)
  AND (:Since IS NULL OR ReceivedAt >= :Since)
  AND (:Until IS NULL OR ReceivedAt <  :Until)
ORDER BY Id ASC;
```

**No LIMIT.** The exporter MUST `defer rows.Close()` and MUST NOT buffer all rows — verified by AST scan in `97-acceptance-criteria.md`.

**Why `ORDER BY Id ASC` (not `ReceivedAt`)** — `Emails.Id` is the rowid alias and gives a stable insertion-order traversal that the planner can serve straight off the table without a temp B-tree sort, even when `:Alias = ''`. `ReceivedAt` ordering would force a sort on the unfiltered case (no `(ReceivedAt)` index exists in production — see §1 drift notice and `internal/store/queries/no_phantom_index_test.go`). The export is consumed as a CSV stream where chronological-vs-insertion order is interchangeable for the user.

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
| `Q-OPEN-DEDUP` | `SEARCH OpenedUrls USING INDEX IxOpenedUrlsUnique (EmailId=? AND Url=?)` (covering equality probe per Slice #167) |
| `Q-OPEN-LIST` | `SEARCH OpenedUrls USING INDEX IxOpenedUrlsAliasOpenedAt (Alias=? AND OpenedAt<?)` (alias filter) or `SEARCH OpenedUrls USING INDEX IxOpenedUrlsOpenedAt (OpenedAt<?)` (no alias, m0009) |
| `Q-EXPORT-STREAM` (alias set) | `SEARCH Emails USING INDEX IxEmailsAliasUid (Alias=?)` |
| `Q-EXPORT-STREAM` (alias empty) | `SCAN Emails` (`ORDER BY Id ASC` served straight off the rowid; no temp B-tree sort) |

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
