# 02 — Emails — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **`internal/core` API surface, queries, indexes, error codes, and logging** for the Emails feature. The contract `internal/ui/views/emails.go` consumes; nothing else may bypass it.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.2
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21200–21299
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`

---

## 1. Service Definition

```go
// Package core — file: internal/core/emails.go
package core

type Emails struct {
    store store.Store
    watch *Watch                // for PollOnce delegation
    rules *Rules                // for MatchedRules projection
    clock Clock
}

func NewEmails(s store.Store, w *Watch, r *Rules, c Clock) *Emails {
    return &Emails{store: s, watch: w, rules: r, clock: c}
}
```

**Constraints (per `04-coding-standards.md`):**
- All methods take `ctx context.Context` first.
- All methods return `errtrace.Result[T]`.
- No method body > 15 lines (extract helpers).
- No package-level state.
- `interface{}` / `any` banned.

---

## 2. Public Methods

### 2.1 `List`

```go
func (e *Emails) List(ctx context.Context, q EmailQuery) errtrace.Result[EmailPage]
```

**Behavior:** Returns a single page of `EmailSummary`. Validates `q` (§4), executes one count + one row query (§3.1), then enriches each row with `MatchedRules` from `e.rules.MatchAll(ctx, EmailSample)`.

**Budget:** p95 ≤ 60 ms with 100 000 rows + 3-char `Search`.

**Errors:**
- `21201 EmailsListInvalidQuery` — caller bug (e.g., `Limit < 1`).
- `21202 EmailsListQueryFailed` — store error.
- `21203 EmailsListRulesEnrichFailed` — non-fatal; row returned with `MatchedRules = nil` and WARN logged.

### 2.2 `Get`

```go
func (e *Emails) Get(ctx context.Context, alias string, uid uint32) errtrace.Result[EmailDetail]
```

**Behavior:** Fetches one row by `(Alias, Uid)`. Returns `21210 EmailNotFound` if no row. Does **not** mark as read — that is an explicit `MarkRead` call.

### 2.3 `MarkRead`

```go
func (e *Emails) MarkRead(ctx context.Context, alias string, uids []uint32, read bool) errtrace.Result[Unit]
```

**Behavior:** Single `UPDATE Email SET IsRead = ? WHERE Alias = ? AND Uid IN (...)`. Idempotent (re-issuing same op is a no-op). Empty `uids` → returns `Unit{}` immediately (no SQL).

**Budget:** ≤ 150 ms for 500 UIDs.

**Errors:**
- `21220 EmailsMarkReadFailed` — store error.
- `21221 EmailsMarkReadTooMany` — `len(uids) > 1000` (caller bug).

### 2.4 `Delete` (soft-delete)

```go
func (e *Emails) Delete(ctx context.Context, alias string, uids []uint32) errtrace.Result[DeleteReceipt]
```

**Behavior:** Sets `DeletedAt = now()` for matching rows. Returns a `DeleteReceipt{AffectedIds []int64, At time.Time}` so the UI can implement undo.

```go
type DeleteReceipt struct {
    AffectedIds []int64
    DeletedAt   time.Time
}

func (e *Emails) Undelete(ctx context.Context, ids []int64) errtrace.Result[Unit]
```

**Errors:**
- `21230 EmailsDeleteFailed`
- `21231 EmailsUndeleteFailed`
- `21232 EmailsDeleteTooMany` — `len(uids) > 1000`.

### 2.5 `Refresh`

```go
func (e *Emails) Refresh(ctx context.Context, alias string) errtrace.Result[RefreshReport]
```

**Behavior:** Delegates to `e.watch.PollOnce(ctx, alias)`. Does **not** open IMAP directly. Returns `RefreshReport` with new/updated counts and timing.

**Errors:**
- `21240 EmailsRefreshAliasUnknown` — alias not in config.
- `21241 EmailsRefreshFailed` — wraps watcher error.

### 2.6 `Counts`

```go
func (e *Emails) Counts(ctx context.Context, alias string) errtrace.Result[EmailCounts]
```

```go
type EmailCounts struct {
    Total     int
    Unread    int
    Deleted   int
}
```

Used by the toolbar badges and the Dashboard `AccountHealthRow`.

---

## 3. SQL Queries

All identifiers PascalCase. All queries use explicit column lists (no `SELECT *`).

### 3.1 `List` queries

```sql
-- Q1: Count
SELECT COUNT(*)
FROM   Email
WHERE  ($Alias = '' OR Alias = $Alias)
  AND  ($Search = '' OR Subject LIKE '%'||$Search||'%'
                     OR FromAddr LIKE '%'||$Search||'%'
                     OR BodyText LIKE '%'||$Search||'%')
  AND  ($OnlyUnread = 0 OR IsRead = 0)
  AND  ($IncludeDeleted = 1 OR DeletedAt IS NULL)
  AND  ($SinceAt IS NULL OR ReceivedAt >= $SinceAt)
  AND  ($UntilAt IS NULL OR ReceivedAt <  $UntilAt);

-- Q2: Page (sort key chosen by SortBy enum)
SELECT Id, Alias, Uid, FromAddr, Subject,
       SUBSTR(BodyText, 1, 140) AS Snippet,
       ReceivedAt, IsRead,
       CASE WHEN DeletedAt IS NULL THEN 0 ELSE 1 END AS IsDeleted,
       0 AS HasAttachment
FROM   Email
WHERE  -- (same WHERE as Q1)
ORDER  BY {SortKeyExpr}
LIMIT  $Limit OFFSET $Offset;
```

`SortKeyExpr` map:
| EmailSortKey       | Expression                |
|--------------------|---------------------------|
| `ReceivedAtDesc`   | `ReceivedAt DESC, Id DESC` (default) |
| `ReceivedAtAsc`    | `ReceivedAt ASC, Id ASC`  |
| `SubjectAsc`       | `Subject COLLATE NOCASE ASC, Id ASC` |

### 3.2 Required schema additions

Migration `M0010_AddEmailFlags`:

```sql
ALTER TABLE Email ADD COLUMN IsRead     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE Email ADD COLUMN DeletedAt  DATETIME;
CREATE INDEX IF NOT EXISTS IxEmailAliasReceived ON Email(Alias, ReceivedAt DESC);
CREATE INDEX IF NOT EXISTS IxEmailAliasIsRead   ON Email(Alias, IsRead);
CREATE INDEX IF NOT EXISTS IxEmailDeletedAt     ON Email(DeletedAt);
```

(Per `18-database-conventions.md` §4: positive boolean `IsRead`, never `IsUnread`.)

### 3.3 `MarkRead`

```sql
UPDATE Email
SET    IsRead = $Read
WHERE  Alias = $Alias
  AND  Uid IN ($Uid1, $Uid2, ...);
```

UIDs spliced as bound parameters in batches of 999 (SQLite `SQLITE_MAX_VARIABLE_NUMBER`). Single transaction wraps all batches.

### 3.4 `Delete` / `Undelete`

```sql
-- Delete
UPDATE Email
SET    DeletedAt = $Now
WHERE  Alias = $Alias AND Uid IN (...) AND DeletedAt IS NULL
RETURNING Id;

-- Undelete
UPDATE Email
SET    DeletedAt = NULL
WHERE  Id IN (...);
```

### 3.5 `Counts`

```sql
SELECT
  COUNT(*)                                             AS Total,
  COALESCE(SUM(CASE WHEN IsRead = 0 THEN 1 ELSE 0 END), 0) AS Unread,
  COALESCE(SUM(CASE WHEN DeletedAt IS NULL THEN 0 ELSE 1 END), 0) AS Deleted
FROM Email
WHERE ($Alias = '' OR Alias = $Alias);
```

---

## 4. Validation Rules

`EmailQuery` is rejected with `21201 EmailsListInvalidQuery` when:
- `Limit < 1` or `Limit > 200`.
- `Offset < 0`.
- `SinceAt` and `UntilAt` both set and `SinceAt >= UntilAt`.
- `SortBy` is not a recognized `EmailSortKey` constant.

`Search` is **not** validated — empty string disables the predicate; other strings are passed through bound parameters (no SQL injection surface).

---

## 5. Error Codes (registry §21200–21299)

| Code  | Name                              | Layer  | Recovery                              |
|-------|-----------------------------------|--------|---------------------------------------|
| 21201 | `EmailsListInvalidQuery`          | core   | Caller bug — log WARN, return safely  |
| 21202 | `EmailsListQueryFailed`           | store  | Show error envelope, **Retry**        |
| 21203 | `EmailsListRulesEnrichFailed`     | core   | Non-fatal — log WARN, continue        |
| 21210 | `EmailNotFound`                   | core   | UI shows "not found" empty state      |
| 21211 | `EmailGetQueryFailed`             | store  | Error envelope, **Retry**             |
| 21220 | `EmailsMarkReadFailed`            | store  | Rollback optimistic UI, error toast   |
| 21221 | `EmailsMarkReadTooMany`           | core   | Caller bug — log WARN                 |
| 21230 | `EmailsDeleteFailed`              | store  | Rollback optimistic UI, error toast   |
| 21231 | `EmailsUndeleteFailed`            | store  | Error toast, undo unavailable         |
| 21232 | `EmailsDeleteTooMany`             | core   | Caller bug — log WARN                 |
| 21240 | `EmailsRefreshAliasUnknown`       | core   | Toast "alias not configured"          |
| 21241 | `EmailsRefreshFailed`             | core   | Error envelope, **Retry**             |
| 21250 | `EmailsCountsQueryFailed`         | store  | Toolbar badges hidden, log ERROR      |

Every error wrapped with `errtrace.Wrap(err, "Emails.<Method>")` at the boundary. `Alias` and `Uid` (when known) appended via `errtrace.WithFields`.

---

## 6. Logging

Per `05-logging-strategy.md`. PascalCase keys.

| Level | Event                  | Fields                                                                 |
|-------|------------------------|------------------------------------------------------------------------|
| DEBUG | `EmailsList`           | `TraceId`, `DurationMs`, `Alias`, `Search`, `Limit`, `Offset`, `Total` |
| DEBUG | `EmailsGet`            | `TraceId`, `DurationMs`, `Alias`, `Uid`                                |
| INFO  | `EmailsMarkRead`       | `TraceId`, `Alias`, `UidCount`, `Read`                                 |
| INFO  | `EmailsDelete`         | `TraceId`, `Alias`, `UidCount`, `AffectedCount`                        |
| INFO  | `EmailsRefreshStarted` | `TraceId`, `Alias`                                                     |
| INFO  | `EmailsRefreshFinished`| `TraceId`, `Alias`, `NewCount`, `UpdatedCount`, `DurationMs`           |
| WARN  | `EmailsListSlow`       | `TraceId`, `DurationMs`, `Threshold=60`                                |
| ERROR | `EmailsFailed`         | `TraceId`, `Method`, `ErrorCode`, `ErrorMessage`                       |

**PII redaction:** `BodyText`, `BodyHtml`, full `FromAddr`/`ToAddr` are **never** logged. `Subject` is truncated to 80 chars and only logged at DEBUG.

---

## 7. Testing Contract

File: `internal/core/emails_test.go`. Target ≥ 90 % coverage.

Required test cases:

1. `List_EmptyStore_ReturnsEmptyPage`.
2. `List_HundredKEmails_3CharSearch_Under60ms` — perf gate (skipped under `-short`).
3. `List_OnlyUnread_FiltersCorrectly`.
4. `List_IncludeDeletedFalse_HidesSoftDeleted`.
5. `List_InvalidLimit_ReturnsErr21201`.
6. `List_RulesEnrichFails_RowsStillReturned_WithWarn`.
7. `Get_KnownUid_ReturnsDetail`.
8. `Get_UnknownUid_ReturnsErr21210`.
9. `MarkRead_500Uids_Under150ms` — perf gate.
10. `MarkRead_EmptyUids_NoSql` — asserts zero `EXEC` calls via fake.
11. `MarkRead_Idempotent` — re-issuing same op affects 0 rows the second time.
12. `Delete_ReturnsAffectedIds_UndeleteRestores`.
13. `Delete_TooMany_ReturnsErr21232`.
14. `Refresh_UnknownAlias_ReturnsErr21240`.
15. `Refresh_DelegatesToWatchPollOnce` — fake watcher records call.
16. `Counts_MatchesDirectSql`.

Fakes:
- `store.NewMemory()`.
- `core.FakeWatch` with controllable `PollOnce` result.
- `core.FakeRules.MatchAll` returning fixed tag list.

---

## 8. Compliance Checklist

- [x] All identifiers PascalCase (Go, SQL, JSON, log keys).
- [x] Methods use `errtrace.Result[T]`.
- [x] Constructor injects interfaces.
- [x] No `any` / `interface{}`.
- [x] No `os.Exit`, `fmt.Print*`.
- [x] All SQL uses singular PascalCase table names.
- [x] Positive booleans (`IsRead`, not `Unread`).
- [x] No `SELECT *`.
- [x] Error codes registered in 21200–21299 range.
- [x] PII redaction documented.
- [x] Cites 02-coding, 03-error-management, 18-database-conventions.

---

## 13. Symbol Map (AC → Go symbol)

Authoritative bridge between `97-acceptance-criteria.md` IDs and the production Go identifiers an AI implementer must touch. **Status legend:** ✅ shipped on `main` · ⏳ planned · 🧪 test-only · 🟡 partial (top-level helpers exist; `Emails` service refactor pending).

### 13.1 Service surface

| AC IDs                          | Go symbol                                                                                       | File                                            | Status |
|---------------------------------|-------------------------------------------------------------------------------------------------|-------------------------------------------------|:------:|
| F-01, T-02                      | `core.Emails` + `NewEmails(store.Store, *Watch, *Rules, Clock) *Emails`                         | `internal/core/emails.go`                       |   ⏳   |
| F-01, F-04, F-16, P-01, P-08    | `(*Emails).List(ctx, EmailQuery) errtrace.Result[EmailPage]`                                    | `internal/core/emails.go`                       |   ⏳   |
| F-07, F-18, P-04                | `(*Emails).Get(ctx, alias, uid) errtrace.Result[EmailDetail]`                                   | `internal/core/emails.go`                       |   🟡   |
| F-11, F-19, P-05, T-06          | `(*Emails).MarkRead(ctx, alias, uids, read bool) errtrace.Result[Unit]`                         | `internal/core/emails.go`                       |   ⏳   |
| F-12, F-13                      | `(*Emails).Delete(ctx, alias, uids) errtrace.Result[DeleteReceipt]`                             | `internal/core/emails.go`                       |   ⏳   |
| F-13                            | `(*Emails).Undelete(ctx, ids) errtrace.Result[Unit]`                                            | `internal/core/emails.go`                       |   ⏳   |
| F-14                            | `(*Emails).Refresh(ctx, alias) errtrace.Result[RefreshReport]`                                  | `internal/core/emails.go`                       |   ⏳   |
| F-17                            | `(*Emails).Counts(ctx, alias) errtrace.Result[EmailCounts]`                                     | `internal/core/emails.go`                       |   ⏳   |
| —                               | `core.ListEmails`, `core.GetEmail`, `core.CountEmails` *(legacy free funcs)*                    | `internal/core/emails.go`                       |   ✅   |

### 13.2 Projection types

| AC IDs              | Go symbol                                                  | File                          | Status |
|---------------------|------------------------------------------------------------|-------------------------------|:------:|
| F-01, F-04          | `core.EmailQuery`, `core.EmailPage`, `core.EmailSortKey`   | `internal/core/emails.go`     |   ⏳   |
| F-07                | `core.EmailDetail`                                         | `internal/core/emails.go`     |   ✅   |
| F-13                | `core.DeleteReceipt`                                       | `internal/core/emails.go`     |   ⏳   |
| F-17                | `core.EmailCounts`                                         | `internal/core/emails.go`     |   ⏳   |

### 13.3 Store / SQL surface

| AC IDs           | Go symbol / SQL artefact                                                          | File                                  | Status |
|------------------|-----------------------------------------------------------------------------------|---------------------------------------|:------:|
| D-01             | Migration `M0010_AddEmailFlags`                                                   | `internal/store/migrate/`             |   ⏳   |
| D-02             | Indexes `IxEmailAliasReceived`, `IxEmailAliasIsRead`, `IxEmailDeletedAt`          | `internal/store/store.go`             |   ⏳   |
| F-01, F-04, P-01, D-06 | `Store.QueryEmailPage(ctx, EmailListFilter) ([]EmailRow, error)` + filter shape | `internal/store/shims.go`           |   ⏳   |
| F-17             | `Store.CountEmailsFiltered(ctx, EmailExportFilter) (int, error)`                  | `internal/store/shims.go`             |   ✅   |
| F-11, P-05, D-05 | `Store.SetEmailRead(ctx, alias, uids []uint32, read bool) (int64, error)` *(≤999 batches)* | `internal/store/shims.go`     |   ⏳   |
| F-12, F-13       | `Store.SoftDeleteEmails`, `Store.UndeleteEmails`                                  | `internal/store/shims.go`             |   ⏳   |
| D-04             | Positive-boolean enforcement test `Test_BooleanPositive`                          | `internal/store/boolean_positive_test.go` |   ✅   |

### 13.4 Live-update surface

| AC IDs        | Go symbol                                                                | File                                    | Status |
|---------------|--------------------------------------------------------------------------|-----------------------------------------|:------:|
| L-01, L-02    | `core.WatchEvent` (`Kind=EmailStored`) consumed via `eventbus.Bus[WatchEvent]` | `internal/core/watch.go`           |   ✅   |
| L-03, L-05    | `BridgeWatcherBus` lifecycle hook                                        | `internal/core/watch_bridge.go`         |   ✅   |

### 13.5 Errors & logging

| AC IDs        | Go symbol                                                              | File                                    | Status |
|---------------|------------------------------------------------------------------------|-----------------------------------------|:------:|
| E-01, E-06    | `errtrace.ErrEmailsListInvalidQuery` (21201), `…QueryFailed` (21202)   | `internal/errtrace/codes_gen.go`        |   ⏳   |
| E-02          | `errtrace.ErrEmailNotFound` (21210)                                    | `internal/errtrace/codes_gen.go`        |   ⏳   |
| E-03          | `errtrace.ErrEmailsMarkReadFailed` (21220)                             | `internal/errtrace/codes_gen.go`        |   ⏳   |
| E-04          | `errtrace.ErrEmailsDeleteFailed` (21230) + `…UndeleteFailed` (21231)   | `internal/errtrace/codes_gen.go`        |   ⏳   |
| G-01..G-06    | `emailsSlog` (`component=emails`) + `FormatEmails*` helpers            | `internal/ui/emails_log.go`             |   ⏳   |

### 13.6 Test contract

| AC IDs        | Test symbol                                                          | File                                            | Status |
|---------------|----------------------------------------------------------------------|-------------------------------------------------|:------:|
| T-01, T-02    | `Test_Emails_List_*`, `_Get_*`, `_MarkRead_*`, `_Delete_*` (16 cases per §7) | `internal/core/emails_test.go`           |   🟡   |
| T-04          | `Test_Emails_RaceClean`                                              | `internal/core/emails_race_test.go`             |   ⏳   |
| T-05          | `BenchmarkEmailsList_100k_3CharSearch`, `BenchmarkEmailsMarkRead_500` | `internal/core/emails_bench_test.go`           |   ⏳   |
| T-06          | `Test_Emails_MarkRead_Idempotent`                                    | `internal/core/emails_test.go`                  |   ⏳   |
| G-05, G-06    | `Test_LogScan_NoOriginalUrlLeak` *(extends to email PII)*            | `internal/core/tools_log_scan_test.go`          |   ✅   |

---

**End of `02-emails/01-backend.md`**
