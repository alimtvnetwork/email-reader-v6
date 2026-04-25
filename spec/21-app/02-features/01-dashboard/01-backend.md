# 01 — Dashboard — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **`internal/core` API surface, queries, and projections** that power the Dashboard view. This is the contract `internal/ui/dashboard` consumes; nothing else may bypass it.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.1
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21100–21199
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`

---

## 1. Service Definition

```go
// Package core — file: internal/core/dashboard.go
package core

type Dashboard struct {
    store store.Store
    clock Clock
}

func NewDashboard(s store.Store, c Clock) *Dashboard {
    return &Dashboard{store: s, clock: c}
}
```

**Constraints (per `04-coding-standards.md`):**
- Constructor takes interfaces, not concrete types.
- Zero package-level state.
- All methods take `ctx context.Context` as the first parameter.
- All methods return `errtrace.Result[T]`.
- No method body exceeds 15 lines (helpers extracted as needed).

---

## 2. Public Methods

### 2.1 `Summary`

```go
func (d *Dashboard) Summary(ctx context.Context) errtrace.Result[DashboardSummary]
```

**Behavior:** Returns aggregate counts in a single call. Composes 4 store queries (§3.1). Total p95 budget: **40 ms** at 100 000 emails / 10 accounts.

**Errors:** wraps any store error with code `21101` (`DashboardSummaryFailed`).

### 2.2 `RecentActivity`

```go
func (d *Dashboard) RecentActivity(ctx context.Context, limit int) errtrace.Result[[]ActivityRow]
```

**Behavior:** Returns the most recent `limit` rows from the `WatchEvent` log (max 200, min 1). Sorted by `OccurredAt DESC`. If `limit < 1` → `21102 DashboardInvalidLimit`. If `limit > 200` → silently clamped to 200.

### 2.3 `AccountHealth`

```go
func (d *Dashboard) AccountHealth(ctx context.Context) errtrace.Result[[]AccountHealthRow]
```

**Behavior:** Joins `WatchState` × `Email` (count) × in-memory `Accounts[]` from config. One row per configured alias, even if `WatchState` has no row yet (returns `LastPollAt = zero`, `Health = "Warning"`).

---

## 3. SQL Queries

All queries use **PascalCase** identifiers (per `18-database-conventions.md`). Schema lives in `internal/store/store.go`.

### 3.1 `Summary` composite

```sql
-- Q1: Email totals
SELECT COUNT(*)                                  AS EmailsTotal,
       COALESCE(SUM(CASE WHEN IsRead = 0 THEN 1 ELSE 0 END), 0) AS UnreadTotal
FROM   Email;

-- Q2: Latest poll across all accounts
SELECT MAX(LastPollAt) AS LastPollAt FROM WatchState;

-- Q3 + Q4: Account/Rule counts come from in-memory config (zero SQL).
```

`Email.IsRead` is a positive boolean (0 = unread, 1 = read), per `18-database-conventions.md` §4 (positive booleans only). If the column is absent, migration `M0007_AddEmailIsRead` adds it with default `0`.

### 3.2 `RecentActivity`

```sql
SELECT OccurredAt, Alias, Kind, Message, ErrorCode
FROM   WatchEvent
ORDER  BY OccurredAt DESC
LIMIT  ?;
```

Requires migration `M0008_CreateWatchEvent`:

```sql
CREATE TABLE IF NOT EXISTS WatchEvent (
    Id          INTEGER PRIMARY KEY AUTOINCREMENT,
    OccurredAt  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Alias       TEXT     NOT NULL,
    Kind        TEXT     NOT NULL,        -- ActivityKind enum (PascalCase)
    Message     TEXT     NOT NULL DEFAULT '',
    ErrorCode   INTEGER  NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS IxWatchEventOccurredAt ON WatchEvent(OccurredAt DESC);
CREATE INDEX IF NOT EXISTS IxWatchEventAlias       ON WatchEvent(Alias, OccurredAt DESC);
```

Retention: `internal/watcher` trims to **last 10 000 rows** on every successful poll (`DELETE FROM WatchEvent WHERE Id < (SELECT MAX(Id) - 10000 FROM WatchEvent)`).

### 3.3 `AccountHealth`

```sql
SELECT  ws.Alias,
        ws.LastPollAt,
        ws.LastErrorAt,
        ws.ConsecutiveFailures,
        COALESCE(ec.Cnt, 0)    AS EmailsStored,
        COALESCE(uc.Cnt, 0)    AS UnreadCount
FROM    WatchState ws
LEFT JOIN (SELECT Alias, COUNT(*) AS Cnt FROM Email GROUP BY Alias)                       ec ON ec.Alias = ws.Alias
LEFT JOIN (SELECT Alias, COUNT(*) AS Cnt FROM Email WHERE IsRead = 0 GROUP BY Alias)      uc ON uc.Alias = ws.Alias;
```

Requires migration `M0009_AddWatchStateHealth`:

```sql
ALTER TABLE WatchState ADD COLUMN LastErrorAt          DATETIME;
ALTER TABLE WatchState ADD COLUMN ConsecutiveFailures  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE WatchState ADD COLUMN LastPollAt           DATETIME;
```

`Health` is computed in Go (not SQL):
```go
func ComputeHealth(row AccountHealthRow, now time.Time) HealthLevel {
    switch {
    case row.ConsecutiveFailures >= 3:                          return "Error"
    case now.Sub(row.LastPollAt) > 15*time.Minute:              return "Warning"
    case row.LastErrorAt.After(row.LastPollAt):                 return "Warning"
    default:                                                    return "Healthy"
    }
}
```

---

## 4. Error Codes (registry §21100–21199)

| Code  | Name                          | Layer  | Recovery                              |
|-------|-------------------------------|--------|---------------------------------------|
| 21100 | `DashboardConfigLoadFailed`   | core   | Show error envelope, offer **Retry**  |
| 21101 | `DashboardSummaryFailed`      | core   | Show error envelope, offer **Retry**  |
| 21102 | `DashboardInvalidLimit`       | core   | Caller bug — log WARN, return safely  |
| 21103 | `DashboardActivityQueryFailed`| store  | Show error envelope, offer **Retry**  |
| 21104 | `DashboardHealthQueryFailed`  | store  | Show error envelope, offer **Retry**  |
| 21105 | `DashboardClockUnavailable`   | core   | Use `time.Now()` fallback, log WARN   |

Every error is wrapped with `errtrace.Wrap(err, "Dashboard.<Method>")` at the boundary.

---

## 5. Logging

Per `05-logging-strategy.md`. The Dashboard service emits these log lines:

| Level | Event                | Fields                                                          |
|-------|----------------------|-----------------------------------------------------------------|
| DEBUG | `DashboardSummary`   | `TraceId`, `DurationMs`, `EmailsTotal`, `Accounts`              |
| WARN  | `DashboardSlow`      | `TraceId`, `DurationMs`, `Threshold=100`                        |
| ERROR | `DashboardFailed`    | `TraceId`, `ErrorCode`, `ErrorMessage`, `Method`                |

Slow-call threshold: any method >100 ms emits `DashboardSlow`.

---

## 6. Testing Contract

Located in `internal/core/dashboard_test.go`. Targets ≥ 90 % coverage.

Required test cases:

1. `Summary_EmptyStore_ReturnsZeros` — fresh in-memory store → all counts 0.
2. `Summary_TenAccountsHundredKEmails_Under100ms` — perf gate (`testing.Short()` skips).
3. `RecentActivity_LimitClampedTo200` — `limit=999` returns ≤ 200 rows.
4. `RecentActivity_NegativeLimit_ReturnsErr21102`.
5. `AccountHealth_NoWatchStateRow_ReturnsWarning` — alias in config but missing in `WatchState`.
6. `AccountHealth_ThreeConsecutiveFailures_ReturnsError`.
7. `Summary_StoreError_WrappedWith21101`.
8. `ComputeHealth_MatrixTable` — table-driven, all 4 branches.

Fakes used:
- `store.NewMemory()` — pure-Go in-memory SQLite.
- `core.FakeClock{T: ...}` — for deterministic time-based health checks.

---

## 7. Migration from Legacy `LoadDashboardStats`

The current `internal/core/dashboard.go::LoadDashboardStats` is a **transitional** function. The migration plan:

1. Add `type Dashboard struct{}` and `NewDashboard` (this spec).
2. Re-implement `LoadDashboardStats` as `(*Dashboard).legacySummary` — internal, deprecated, kept until CLI `status` subcommand lands.
3. Update `internal/ui/dashboard` to depend on `*core.Dashboard` (constructor-injected from `cmd/email-read-ui/main.go`).
4. Delete `LoadDashboardStats` once the UI cutover ships and the CLI `status` command uses `(*Dashboard).Summary` directly.

No behavior change is expected — only the call shape.

---

## 8. Compliance Checklist

- [x] All identifiers PascalCase (Go, SQL, JSON).
- [x] Methods use `errtrace.Result[T]`.
- [x] Constructor injects interfaces (`store.Store`, `Clock`).
- [x] No `any` / `interface{}`.
- [x] No `os.Exit`, no `fmt.Print*`.
- [x] All SQL uses singular PascalCase table names (`Email`, `WatchState`, `WatchEvent`).
- [x] Error codes registered in 21100–21199 range.
- [x] Cites 02-coding, 03-error-management, 18-database-conventions from `spec/12-consolidated-guidelines/`.

---

**End of `01-dashboard/01-backend.md`**
