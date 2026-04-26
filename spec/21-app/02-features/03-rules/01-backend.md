# 03 — Rules — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **`internal/core` API surface, regex-validation pipeline, persistence model, queries, error codes, and logging** for the Rules feature.

This is the contract `internal/ui/views/rules.go` consumes; nothing else may bypass it.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.3
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21300–21399
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`

---

## 1. Service Definition

```go
// Package core — file: internal/core/rules.go
package core

type Rules struct {
    cfg     config.Loader      // Load/Save config.json (Rules[])
    store   store.Store        // RuleStat table
    engine  rules.EngineFactory // wraps internal/rules.New
    clock   Clock
}

func NewRules(cfg config.Loader, s store.Store, e rules.EngineFactory, c Clock) *Rules {
    return &Rules{cfg: cfg, store: s, engine: e, clock: c}
}
```

**Constraints (per `04-coding-standards.md`):**
- All methods take `ctx context.Context` first.
- All methods return `errtrace.Result[T]`.
- No method body > 15 lines.
- No package-level state.
- `interface{}` / `any` banned.

---

## 2. Public Methods

### 2.1 `List`

```go
func (r *Rules) List(ctx context.Context) errtrace.Result[[]RuleWithStat]
```

```go
type RuleWithStat struct {
    Rule
    LastMatchedAt time.Time
    MatchCount    int64
}
```

**Behavior:** Loads `config.Rules[]`, joins with `RuleStat` (left join on `RuleName`), sorts by `Order ASC, Name ASC`. Single round-trip to disk + one SQL query.

**Budget:** ≤ 20 ms with 200 rules.

**Errors:**
- `21301 RulesListConfigLoadFailed`
- `21302 RulesListStatQueryFailed`

### 2.2 `Get`

```go
func (r *Rules) Get(ctx context.Context, name string) errtrace.Result[RuleWithStat]
```

Returns `21303 RuleNotFound` when no rule with that `Name`.

### 2.3 `Create`

```go
func (r *Rules) Create(ctx context.Context, spec RuleSpec) errtrace.Result[Rule]
```

**Pipeline:**
1. Trim `Name`. Reject empty (`21310 RuleNameRequired`).
2. Reject duplicate `Name` (`21311 RuleNameTaken`).
3. Compile each non-empty regex (`21312 RuleInvalidRegex`, with `Field` in error envelope).
4. If `Order == 0`, set to `max(existing.Order) + 10`.
5. Stamp `CreatedAt = UpdatedAt = clock.Now()`.
6. Append to `config.Rules`. `cfg.Save()`.
7. Insert empty `RuleStat` row.
8. Return persisted `Rule`.

### 2.4 `Update`

```go
func (r *Rules) Update(ctx context.Context, name string, spec RuleSpec) errtrace.Result[Rule]
```

**Behavior:** Locate by `name`; if `spec.Name != name`, rejects (`21313 RuleRenameViaSeparateOp`). Re-validates every regex. Stamps `UpdatedAt`. Persists. Does **not** touch `RuleStat`.

### 2.5 `Rename`

```go
func (r *Rules) Rename(ctx context.Context, oldName, newName string) errtrace.Result[Rule]
```

**Behavior:** Atomic: updates `config.Rules[i].Name` + `UPDATE RuleStat SET RuleName = ?` + `UPDATE OpenedUrl SET RuleName = ?` in a single SQLite transaction. Rejects `21314 RuleRenameTargetTaken` if `newName` already exists.

### 2.6 `Delete`

```go
func (r *Rules) Delete(ctx context.Context, name string) errtrace.Result[Unit]
```

**Behavior:** Removes from `config.Rules` + `DELETE FROM RuleStat WHERE RuleName = ?` in one transaction. `OpenedUrl` rows keep `RuleName` for historical accuracy (FK is by `EmailId`, not `RuleName`).

### 2.7 `SetEnabled`

```go
func (r *Rules) SetEnabled(ctx context.Context, name string, enabled bool) errtrace.Result[Unit]
```

**Behavior:** Idempotent. If current state == requested, no write. Otherwise flip + `UpdatedAt`.

### 2.8 `Reorder`

```go
func (r *Rules) Reorder(ctx context.Context, namesInOrder []string) errtrace.Result[Unit]
```

**Behavior:**
1. Validate every `name` exists exactly once in `config.Rules`; otherwise `21320 RuleReorderInvalidSet`.
2. Validate `len(namesInOrder) == len(config.Rules)`; otherwise `21320`.
3. Reassign `Order = (i+1) * 10` for each.
4. `cfg.Save()` (atomic file write — see `spec/12-consolidated-guidelines/06-seedable-config.md`).

Atomicity: `config.Save` writes to a temp file + `os.Rename` → single fsync. No partial state visible.

### 2.9 `DryRun`

```go
func (r *Rules) DryRun(ctx context.Context, spec RuleSpec, sample EmailSample) errtrace.Result[RuleMatch]
```

**Behavior:** Compiles `spec` (no persist), evaluates against `sample`, returns `RuleMatch` with per-field bool flags + `ExtractedUrls` + `DurationMicro`. **Writes nothing** — no `RuleStat` bump, no `OpenedUrl` insert. Read-only.

**Budget:** ≤ 15 ms for a 100 KB `BodyText`.

### 2.10 `BumpStat` (internal — exported only for `internal/watcher`)

```go
func (r *Rules) BumpStat(ctx context.Context, ruleName string, at time.Time) errtrace.Result[Unit]
```

Single `UPSERT INTO RuleStat (RuleName, LastMatchedAt, MatchCount) VALUES (?, ?, 1) ON CONFLICT(RuleName) DO UPDATE SET LastMatchedAt = excluded.LastMatchedAt, MatchCount = MatchCount + 1`. Called by `internal/watcher` per match.

---

## 3. SQL Schema

Migration `M0011_CreateRuleStat`:

```sql
CREATE TABLE IF NOT EXISTS RuleStat (
    RuleName       TEXT     PRIMARY KEY,
    LastMatchedAt  DATETIME,
    MatchCount     INTEGER  NOT NULL DEFAULT 0,
    UpdatedAt      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS IxRuleStatLastMatchedAt ON RuleStat(LastMatchedAt DESC);
```

Migration `M0012_CreateEmailTag` (for `Action == Tag`):

```sql
CREATE TABLE IF NOT EXISTS EmailTag (
    Id        INTEGER PRIMARY KEY AUTOINCREMENT,
    EmailId   INTEGER NOT NULL,
    TagName   TEXT    NOT NULL,
    CreatedAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(EmailId) REFERENCES Email(Id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS IxEmailTagUnique ON EmailTag(EmailId, TagName);
CREATE INDEX IF NOT EXISTS IxEmailTagTag ON EmailTag(TagName);
```

All identifiers PascalCase. Singular table names. Positive booleans. (Per `18-database-conventions.md`.)

---

## 4. Queries

### 4.1 `List` join

```sql
-- Stats for every known rule (rules without a stat row come back via LEFT JOIN in Go)
SELECT RuleName, LastMatchedAt, MatchCount
FROM   RuleStat;
```

The Go layer joins this map onto `config.Rules[]` (in-memory). One SQL call total.

### 4.2 `BumpStat` upsert

```sql
INSERT INTO RuleStat (RuleName, LastMatchedAt, MatchCount, UpdatedAt)
VALUES (?, ?, 1, ?)
ON CONFLICT(RuleName) DO UPDATE SET
    LastMatchedAt = excluded.LastMatchedAt,
    MatchCount    = RuleStat.MatchCount + 1,
    UpdatedAt     = excluded.UpdatedAt;
```

### 4.3 `Rename` transaction

```sql
BEGIN IMMEDIATE;
UPDATE RuleStat   SET RuleName = $New, UpdatedAt = $Now WHERE RuleName = $Old;
UPDATE OpenedUrl  SET RuleName = $New                      WHERE RuleName = $Old;
COMMIT;
```

(`config.json` write happens outside the SQLite tx but inside the same `Rename` method; failure of either triggers rollback of both — see §6.)

### 4.4 `Delete` transaction

```sql
BEGIN IMMEDIATE;
DELETE FROM RuleStat WHERE RuleName = $Name;
COMMIT;
```

`OpenedUrl.RuleName` is preserved for audit history (the rule no longer exists, but its past actions are still attributable).

---

## 5. Validation Rules

| Field         | Rule                                                                            | Error code |
|---------------|---------------------------------------------------------------------------------|------------|
| `Name`        | trimmed length 1–64; chars `[A-Za-z0-9_-]` only                                 | `21310` (empty) / `21315 RuleNameInvalidChars` |
| `Name`        | unique among existing rules                                                     | `21311` (Create) / `21314` (Rename) |
| `FromRegex`   | empty OR compiles via `regexp.Compile`                                          | `21312` with `Field="FromRegex"` |
| `SubjectRegex`| empty OR compiles                                                               | `21312` with `Field="SubjectRegex"` |
| `BodyRegex`   | empty OR compiles                                                               | `21312` with `Field="BodyRegex"` |
| `UrlRegex`    | empty OR compiles; required when `Action == OpenUrl` (else `21316 RuleUrlRegexRequired`) | `21312` |
| `Action`      | one of `OpenUrl` / `MarkRead` / `Tag`                                           | `21317 RuleActionInvalid` |
| `Order`       | `>= 0`; auto-assigned when 0 in Create                                          | `21318 RuleOrderNegative` |

Validation is **synchronous** — no method persists before all checks pass.

---

## 6. Atomicity & Recovery

The `Rename` operation crosses two storage layers (`config.json` + SQLite). Pattern:

```go
1. Compute new config.Rules with the rename applied.
2. BEGIN IMMEDIATE on SQLite; run UPDATEs.
3. Write config.json atomically (temp + rename).
4. If step 3 fails → ROLLBACK SQLite tx, return wrapped 21319 RuleRenameAtomicityFailed.
5. COMMIT SQLite tx.
6. If COMMIT fails → revert config.json from in-memory snapshot taken at step 1; return 21319.
```

The same pattern applies to `Delete` (config remove + SQLite delete) and to `Reorder` if a future revision adds a SQLite-side ordering index.

---

## 7. Error Codes (registry §21300–21399)

| Code  | Name                              | Layer  | Recovery                              |
|-------|-----------------------------------|--------|---------------------------------------|
| 21301 | `RulesListConfigLoadFailed`       | core   | Show error envelope, **Retry**        |
| 21302 | `RulesListStatQueryFailed`        | store  | Show error envelope, **Retry**        |
| 21303 | `RuleNotFound`                    | core   | UI shows "not found" empty state      |
| 21310 | `RuleNameRequired`                | core   | Field error in form                   |
| 21311 | `RuleNameTaken`                   | core   | Field error in form                   |
| 21312 | `RuleInvalidRegex`                | core   | Field error with offending `Field`    |
| 21313 | `RuleRenameViaSeparateOp`         | core   | Caller bug — log WARN                 |
| 21314 | `RuleRenameTargetTaken`           | core   | Field error in rename dialog          |
| 21315 | `RuleNameInvalidChars`            | core   | Field error in form                   |
| 21316 | `RuleUrlRegexRequired`            | core   | Field error when `Action == OpenUrl`  |
| 21317 | `RuleActionInvalid`               | core   | Caller bug — log WARN                 |
| 21318 | `RuleOrderNegative`               | core   | Caller bug — log WARN                 |
| 21319 | `RuleRenameAtomicityFailed`       | core   | Toast: "Rename failed; state restored"|
| 21320 | `RuleReorderInvalidSet`           | core   | Caller bug — log WARN                 |
| 21330 | `RuleCreatePersistFailed`         | core   | Error envelope, **Retry**             |
| 21331 | `RuleUpdatePersistFailed`         | core   | Error envelope, **Retry**             |
| 21332 | `RuleDeletePersistFailed`         | core   | Error envelope, **Retry**             |
| 21333 | `RuleSetEnabledPersistFailed`     | core   | Rollback optimistic flip, error toast |
| 21334 | `RuleReorderPersistFailed`        | core   | Rollback optimistic UI, error toast   |
| 21340 | `RuleDryRunCompileFailed`         | core   | Field error in dry-run dialog         |
| 21341 | `RuleDryRunTimeout`               | core   | Toast: "Dry-run took too long"        |
| 21350 | `RuleStatBumpFailed`              | watcher| WARN log only; never blocks watcher   |

Every error wrapped with `errtrace.Wrap(err, "Rules.<Method>")`. Field-level errors set `errtrace.WithField("Field", "<RegexName>")`.

---

## 8. Logging

Per `05-logging-strategy.md`. PascalCase keys.

| Level | Event                | Fields                                                          |
|-------|----------------------|-----------------------------------------------------------------|
| DEBUG | `RulesList`          | `TraceId`, `DurationMs`, `RuleCount`                            |
| INFO  | `RuleCreated`        | `TraceId`, `RuleName`, `Action`, `Enabled`                      |
| INFO  | `RuleUpdated`        | `TraceId`, `RuleName`, `Action`                                 |
| INFO  | `RuleDeleted`        | `TraceId`, `RuleName`                                           |
| INFO  | `RuleEnabledToggled` | `TraceId`, `RuleName`, `Enabled`                                |
| INFO  | `RulesReordered`     | `TraceId`, `RuleCount`                                          |
| INFO  | `RuleRenamed`        | `TraceId`, `OldName`, `NewName`, `StatRowsUpdated`              |
| DEBUG | `RuleDryRun`         | `TraceId`, `RuleName`, `DurationMicro`, `Matched`, `UrlCount`   |
| WARN  | `RuleStatBumpFailed` | `TraceId`, `RuleName`, `ErrorMessage`                           |
| WARN  | `RulesListSlow`      | `TraceId`, `DurationMs`, `Threshold=20`                         |
| ERROR | `RulesFailed`        | `TraceId`, `Method`, `ErrorCode`, `ErrorMessage`                |

**PII redaction:** `EmailSample.BodyText` and `EmailSample.FromAddr` are **never** logged. `RuleDryRun` logs only structural counters.

---

## 9. Testing Contract

File: `internal/core/rules_test.go`. Target ≥ 90 % coverage.

Required test cases:

1. `List_NoRules_ReturnsEmpty`.
2. `List_TwoHundredRules_Under20ms` — perf gate (skipped under `-short`).
3. `Create_ValidatesAllRegexes`.
4. `Create_DuplicateName_ReturnsErr21311`.
5. `Create_InvalidRegex_ReturnsErr21312_WithField`.
6. `Create_OpenUrlWithoutUrlRegex_ReturnsErr21316`.
7. `Create_AssignsOrderMaxPlus10_WhenZero`.
8. `Update_RejectsRename_ReturnsErr21313`.
9. `Rename_AtomicAcrossConfigAndSqlite` — fault-injects SQLite failure → asserts config.json reverted.
10. `Rename_TargetTaken_ReturnsErr21314`.
11. `Delete_AlsoRemovesRuleStat_KeepsOpenedUrlHistory`.
12. `SetEnabled_Idempotent_NoWriteWhenSameState`.
13. `Reorder_InvalidSet_ReturnsErr21320`.
14. `Reorder_ReassignsBy10s`.
15. `DryRun_OneHundredKBBody_Under15ms` — perf gate.
16. `DryRun_WritesNothing` — asserts zero SQL `EXEC` calls + no config write.
17. `BumpStat_UpsertsCorrectly_FromZero_AndIncrements`.

Fakes:
- `core.FakeConfigLoader` (in-memory).
- `store.NewMemory()`.
- `core.FakeClock`.

---

## 10. Migration from Legacy `AddRule` / `ListRules` / `GetRule`

The current `internal/core/rules.go::AddRule, ListRules, GetRule, …` are **transitional** package-level functions. The migration plan:

1. Add `type Rules struct{}` and `NewRules` (this spec).
2. Re-implement legacy functions as thin wrappers calling `(*Rules).Create / List / Get` against a process-singleton instance.
3. Update CLI dispatch (`internal/cli/rules.go`) to use `*core.Rules` constructor-injected from `cmd/email-read/main.go`.
4. Update UI view to use `*core.Rules`.
5. Delete the package-level functions once both binaries are migrated.

No behavior change is expected — only call shape and added `RuleStat` writes.

---

## 11. Compliance Checklist

- [x] All identifiers PascalCase.
- [x] Methods use `errtrace.Result[T]`.
- [x] Constructor injects interfaces (`config.Loader`, `store.Store`, `rules.EngineFactory`, `Clock`).
- [x] No `any` / `interface{}`.
- [x] No `os.Exit`, no `fmt.Print*`.
- [x] All SQL uses singular PascalCase table names (`RuleStat`, `EmailTag`, `OpenedUrl`).
- [x] Atomic cross-storage operations documented (Rename / Delete).
- [x] Error codes registered in 21300–21399 range.
- [x] PII redaction documented.
- [x] Cites 02-coding, 03-error-management, 06-seedable-config, 18-database-conventions.

---

## N. Symbol Map (AC → Go symbol)

Authoritative bridge between `97-acceptance-criteria.md` IDs and the production Go identifiers an AI implementer must touch. **Status legend:** ✅ shipped on `main` · ⏳ planned · 🧪 test-only · 🟡 partial.

### N.1 Service surface

| AC IDs                    | Go symbol                                                                            | File                                  | Status |
|---------------------------|--------------------------------------------------------------------------------------|---------------------------------------|:------:|
| F-01, T-02                | `core.Rules` + `NewRules(config.Manager, store.Store, Clock) *Rules`                 | `internal/core/rules.go`              |   ⏳   |
| F-01                      | `(*Rules).List(ctx) errtrace.Result[[]RuleView]`                                     | `internal/core/rules.go`              |   🟡   |
| F-04                      | `(*Rules).Get(ctx, name) errtrace.Result[RuleView]`                                  | `internal/core/rules.go`              |   ✅   |
| F-06                      | `(*Rules).Create(ctx, RuleInput) errtrace.Result[RuleView]`                          | `internal/core/rules.go`              |   ✅   |
| F-07, F-08                | `(*Rules).Update(ctx, name, RuleInput) errtrace.Result[RuleView]`                    | `internal/core/rules.go`              |   ⏳   |
| F-09, F-10                | `(*Rules).Rename(ctx, oldName, newName) errtrace.Result[Unit]` *(atomic txn)*        | `internal/core/rules.go`              |   ⏳   |
| F-11                      | `(*Rules).Delete(ctx, name) errtrace.Result[Unit]`                                   | `internal/core/rules.go`              |   ✅   |
| F-12                      | `(*Rules).SetEnabled(ctx, name, enabled bool) errtrace.Result[Unit]`                 | `internal/core/rules.go`              |   ✅   |
| F-13, F-14                | `(*Rules).Reorder(ctx, []string) errtrace.Result[Unit]`                              | `internal/core/rules.go`              |   ⏳   |
| F-15                      | `(*Rules).DryRun(ctx, name, EmailSample) errtrace.Result[DryRunReport]`              | `internal/core/rules.go`              |   ⏳   |
| F-21, L-02                | `(*Rules).MatchAll(ctx, EmailSample) errtrace.Result[[]MatchedRule]`                 | `internal/core/rules.go`              |   ⏳   |
| F-22                      | `(*Rules).BumpStat(ctx, ruleName, hitTime time.Time) errtrace.Result[Unit]`          | `internal/core/rules.go`              |   ⏳   |

### N.2 Projection types

| AC IDs              | Go symbol                                              | File                          | Status |
|---------------------|--------------------------------------------------------|-------------------------------|:------:|
| F-01, F-04          | `core.RuleView`, `core.RuleInput`, `core.RuleAction`   | `internal/core/rules.go`      |   🟡   |
| F-15, F-21          | `core.EmailSample`, `core.MatchedRule`, `core.DryRunReport` | `internal/core/rules.go`  |   ⏳   |
| L-01                | `core.RuleEvent` + `eventbus.Bus[RuleEvent]`           | `internal/core/rule_events.go` |   ⏳   |

### N.3 Store / SQL surface

| AC IDs   | Go symbol / SQL artefact                                                          | File                                  | Status |
|----------|-----------------------------------------------------------------------------------|---------------------------------------|:------:|
| D-01     | Migration `M0011_CreateRuleStat`                                                  | `internal/store/migrate/`             |   ⏳   |
| D-02     | `RuleStat` table (`RuleName PK`, `HitCount`, `LastHitAt`)                         | `internal/store/store.go`             |   ⏳   |
| D-03, F-10 | `Store.RenameRuleAtomic(ctx, oldName, newName) error` *(updates `RuleStat` + `OpenedUrl` in one txn)* | `internal/store/shims.go` |   ⏳   |
| F-11     | `Store.DeleteRuleStat(ctx, name) error` *(preserves `OpenedUrl.RuleName`)*        | `internal/store/shims.go`             |   ⏳   |
| F-22     | `Store.BumpRuleStat(ctx, name, at time.Time) error`                               | `internal/store/shims.go`             |   ⏳   |

### N.4 Errors & logging

| AC IDs        | Go symbol                                                              | File                                    | Status |
|---------------|------------------------------------------------------------------------|-----------------------------------------|:------:|
| E-01..E-06    | Codes 21300..21399 per §8 (e.g. `ErrRuleRenameTargetTaken` = 21314, `ErrRuleReorderSetMismatch` = 21320) | `internal/errtrace/codes_gen.go` | ⏳ |
| G-01..G-06    | `rulesSlog` (`component=rules`) + `FormatRules*` helpers               | `internal/ui/rules_log.go`              |   ⏳   |

### N.5 Test contract

| AC IDs        | Test symbol                                                          | File                                            | Status |
|---------------|----------------------------------------------------------------------|-------------------------------------------------|:------:|
| T-01, T-02    | `Test_Rules_*` (per §6 test list)                                    | `internal/core/rules_test.go`                   |   🟡   |
| T-04          | `Test_Rules_RaceClean`                                               | `internal/core/rules_race_test.go`              |   ⏳   |
| T-05          | `BenchmarkRulesMatchAll_100Rules`                                    | `internal/core/rules_bench_test.go`             |   ⏳   |
| X-01..X-05    | XSS / template-injection guards on rule expressions                  | `internal/core/rules_security_test.go`          |   ⏳   |

---

**End of `03-rules/01-backend.md`**
