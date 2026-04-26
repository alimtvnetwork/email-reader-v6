# 04 — Accounts — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **`internal/core` API surface, IMAP test-connection pipeline, persistence model, queries, error codes, and logging** for the Accounts feature.

This is the contract `internal/ui/views/accounts.go` and `internal/cli/accounts.go` consume; nothing else may bypass it. In particular, the UI MUST NOT import `internal/imapdef`, `internal/mailclient`, `internal/config`, or `internal/store` directly.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.4
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21000–21099` (config), `21200–21299` (mailclient), `21700–21799` (core)
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`
- Watch feature: [`../05-watch/01-backend.md`](../05-watch/01-backend.md) (consumes `Account`; emits connection events)

---

## 1. Service Definition

```go
// Package core — file: internal/core/accounts.go
package core

type Accounts struct {
    cfg     config.Loader        // Load/Save config.json (Accounts[])
    store   store.Store          // WatchState table (cascade on rename/remove)
    imap    mailclient.Dialer    // wraps internal/mailclient — testable seam
    suggest imapdef.Suggester    // wraps internal/imapdef.Lookup
    bus     eventbus.Publisher   // emits AccountEvent
    clock   Clock
}

func NewAccounts(
    cfg config.Loader,
    s store.Store,
    d mailclient.Dialer,
    sg imapdef.Suggester,
    b eventbus.Publisher,
    c Clock,
) *Accounts {
    return &Accounts{cfg: cfg, store: s, imap: d, suggest: sg, bus: b, clock: c}
}
```

**Constraints (per `04-coding-standards.md`):**
- All methods take `ctx context.Context` first.
- All methods return `errtrace.Result[T]`.
- No method body > 15 lines.
- No package-level state.
- `interface{}` / `any` banned.
- Constructor receives interfaces — never concretes — for testability.

---

## 2. Public Methods

### 2.1 `List`

```go
func (a *Accounts) List(ctx context.Context) errtrace.Result[[]AccountView]
```

**Behavior:** Loads `config.Accounts[]`, joins with `WatchState` (left join on `Alias`), joins with the most recent in-memory connection-status snapshot from `bus`, sorts by `Order ASC, Alias ASC`. **Strips `PasswordB64` from every returned row.** Single round-trip to disk + one SQL query.

**Budget:** ≤ 15 ms with 50 accounts.

**Errors:**
- `21701 AccountsListConfigLoadFailed`
- `21702 AccountsListWatchStateQueryFailed`

### 2.2 `Get`

```go
func (a *Accounts) Get(ctx context.Context, alias string) errtrace.Result[AccountView]
```

Returns `21703 AccountNotFound` when no account with that `Alias`. Password fields stripped.

### 2.3 `SuggestImap`

```go
func (a *Accounts) SuggestImap(ctx context.Context, emailAddr string) errtrace.Result[ImapDefaults]
```

**Behavior:** Parses `emailAddr` via `net/mail.ParseAddress`, lower-cases the domain, delegates to `imapdef.Suggester.Lookup(domain)`. **Pure read** — no disk, no network.

**Errors:**
- `21704 AccountsSuggestEmailInvalid` (parse failure)
- Returns `ImapDefaults{Source: ImapDefaultsSourceUnknown}` (NOT an error) when domain not in built-in table — UI surfaces a hint, not a failure.

### 2.4 `TestConnection`

```go
func (a *Accounts) TestConnection(ctx context.Context, spec AccountSpec) errtrace.Result[TestConnectionResult]
```

**Behavior:** Validates `spec` (§5), opens an IMAP connection per `(Host, Port, UseTls)`, runs `LOGIN(EmailAddr, Password)`, captures the server greeting + latency, **logs out cleanly**, returns `TestConnectionResult`. **Writes nothing.** Bound by `ctx` deadline; if caller did not set one, an internal **5 s** deadline is applied.

**Errors (returned inside `TestConnectionResult.ErrorCode`, NOT as `Result.Err`):**
- `ER-MAIL-21200 ErrMailDial`
- `ER-MAIL-21201 ErrMailLoginFailed`
- `ER-MAIL-21207 ErrMailTLSHandshake`
- `ER-MAIL-21208 ErrMailTimeout`

`Result.Err` is non-nil only for **programmer errors** (nil deps, invalid context). Network/credential errors are domain-data, not exceptions.

### 2.5 `Add`

```go
func (a *Accounts) Add(ctx context.Context, spec AccountSpec) errtrace.Result[AccountView]
```

**Pipeline:**
1. Trim `Alias`. Validate per §5 (`21710..21716`).
2. Reject duplicate `Alias` (`21712 AccountAliasTaken`) or duplicate `EmailAddr` (`21713 AccountEmailTaken`).
3. If `Host == ""` or `Port == 0`, run `SuggestImap`; reject `21714 AccountHostUnresolved` if `Source == Unknown` AND user provided no manual `Host`.
4. Run `TestConnection`; if `!Ok`, return `21720 AccountAddTestConnectionFailed` with the underlying `ErrorCode` in the envelope.
5. Encode `Password → PasswordB64`. **Plaintext password discarded immediately** — never held beyond this stack frame.
6. If `Order == 0`, set to `max(existing.Order) + 10`.
7. Stamp `CreatedAt = UpdatedAt = clock.Now()`.
8. **Atomic two-storage commit** (see §6): append to `config.Accounts` + `INSERT INTO WatchState (Alias, LastSeenUid=0, …)` in one logical op.
9. Publish `AccountEvent{Kind: Added, Alias}`.
10. Return persisted `AccountView` (password-stripped).

### 2.6 `Update`

```go
func (a *Accounts) Update(ctx context.Context, alias string, spec AccountSpec) errtrace.Result[AccountView]
```

**Behavior:** Locate by `alias`; if `spec.Alias != alias`, reject `21715 AccountRenameViaSeparateOp`. Re-validate every field. If `Password == ""`, **preserve existing `PasswordB64`** (edit-only-host flow). If any of `Host`/`Port`/`UseTls`/`Password` changed, re-run `TestConnection`; on failure return `21721 AccountUpdateTestConnectionFailed`. Stamp `UpdatedAt`. Persist. Publish `AccountEvent{Kind: Updated}`. Does **not** touch `WatchState`.

### 2.7 `Rename`

```go
func (a *Accounts) Rename(ctx context.Context, oldAlias, newAlias string) errtrace.Result[AccountView]
```

**Behavior:** Atomic across two stores (see §6): `UPDATE WatchState SET Alias = $New WHERE Alias = $Old` + patch `config.Accounts[i].Alias`. Rejects `21716 AccountRenameTargetTaken` if `newAlias` already exists. **Preserves `LastSeenUid`** — this is the whole point of having Rename instead of Remove+Add. Publishes `AccountEvent{Kind: Renamed, Alias: newAlias, PrevAlias: oldAlias}`.

### 2.8 `Remove`

```go
func (a *Accounts) Remove(ctx context.Context, alias string) errtrace.Result[Unit]
```

**Behavior:** Atomic across two stores (see §6):
1. `BEGIN IMMEDIATE` on SQLite.
2. `DELETE FROM WatchState WHERE Alias = ?` (capture the deleted row for rollback).
3. Remove entry from in-memory `config.Accounts`.
4. `cfg.Save()` (atomic temp+rename).
5. `COMMIT` SQLite.
6. On failure of step 4 or 5: rollback SQLite, restore in-memory slice, return `21730 AccountRemoveAtomicityFailed`.

`Email` rows are **NOT** cascaded — historical email data outlives the account by design. (FK on `Email.Alias` is `ON DELETE SET NULL` per the schema in `spec/23-app-database/`.)

Publishes `AccountEvent{Kind: Removed, Alias}`.

### 2.9 `SetOrder`

```go
func (a *Accounts) SetOrder(ctx context.Context, aliasesInOrder []string) errtrace.Result[Unit]
```

**Behavior:**
1. Validate every `alias` exists exactly once in `config.Accounts`; otherwise `21717 AccountReorderInvalidSet`.
2. Validate `len(aliasesInOrder) == len(config.Accounts)`; otherwise `21717`.
3. Reassign `Order = (i+1) * 10` for each.
4. `cfg.Save()` (single fsync via temp+rename).
5. Publish `AccountEvent{Kind: Reordered}`.

Atomicity: `config.Save` writes to a temp file + `os.Rename` → single fsync. No partial state visible. No SQLite involvement (Order is config-only).

### 2.10 `Subscribe` (UI live channel)

```go
func (a *Accounts) Subscribe(ctx context.Context) errtrace.Result[<-chan AccountEvent]
```

Returns a buffered channel (cap `64`) closed when `ctx` is cancelled. Drops oldest event on overflow + emits `WARN AccountEventOverflow`. The UI uses this for sub-second sidebar refresh; the watcher uses it to reconcile its account list.

---

## 3. SQL Schema

The `WatchState` table is **owned by the Watch feature** but written-to by Accounts on Add/Rename/Remove. Migration `M0003_CreateWatchState` (defined in Watch backend spec) is the source of truth. Excerpt for context:

```sql
CREATE TABLE IF NOT EXISTS WatchState (
    Alias            TEXT     PRIMARY KEY,
    LastSeenUid      INTEGER  NOT NULL DEFAULT 0,
    LastConnectedAt  DATETIME,
    LastConnectError TEXT     NOT NULL DEFAULT '',
    UpdatedAt        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS IxWatchStateUpdatedAt ON WatchState(UpdatedAt DESC);
```

Accounts feature adds **no new tables**. All identifiers PascalCase. Singular table names. Positive booleans. (Per `18-database-conventions.md`.)

---

## 4. Queries

### 4.1 `List` join

```sql
-- Watch-state for every known account; accounts without a row fall back to defaults via LEFT JOIN in Go.
SELECT Alias, LastSeenUid, LastConnectedAt, LastConnectError
FROM   WatchState;
```

The Go layer joins this map onto `config.Accounts[]` (in-memory). One SQL call total. The connection-status (`IsConnected` derived field) comes from the `bus`'s last-known snapshot — no extra query.

### 4.2 `Add` insert

```sql
INSERT INTO WatchState (Alias, LastSeenUid, LastConnectedAt, LastConnectError, UpdatedAt)
VALUES (?, 0, NULL, '', ?);
```

### 4.3 `Rename` transaction

```sql
BEGIN IMMEDIATE;
UPDATE WatchState SET Alias = $New, UpdatedAt = $Now WHERE Alias = $Old;
COMMIT;
```

(`config.json` write happens outside the SQLite tx but inside the same `Rename` method; failure of either triggers rollback of both — see §6.)

### 4.4 `Remove` transaction

```sql
BEGIN IMMEDIATE;
DELETE FROM WatchState WHERE Alias = $Alias;
COMMIT;
```

`Email.Alias` rows for this account remain (FK is `ON DELETE SET NULL` — see `spec/23-app-database/`). This is intentional: deleting an account does not erase your archive.

---

## 5. Validation Rules

| Field        | Rule                                                                                   | Error code |
|--------------|----------------------------------------------------------------------------------------|------------|
| `Alias`      | trimmed length 1–32; chars `[A-Za-z0-9_-]` only                                        | `21710 AccountAliasRequired` (empty) / `21711 AccountAliasInvalidChars` |
| `Alias`      | unique among existing accounts                                                         | `21712` (Add) / `21716` (Rename) |
| `EmailAddr`  | parses via `net/mail.ParseAddress`; lowercase domain stored                            | `21718 AccountEmailInvalid` |
| `EmailAddr`  | unique among existing accounts                                                         | `21713` (Add) |
| `Host`       | RFC 1123 hostname; non-empty after `SuggestImap` or manual entry                       | `21714 AccountHostUnresolved` |
| `Port`       | 1..65535; non-zero after `SuggestImap` or manual entry                                 | `21719 AccountPortInvalid` |
| `Password`   | non-empty; ≤ 1024 bytes; rejected if any C0 control char or zero-width Unicode         | `ER-CFG-21003 ErrConfigHiddenUnicodePwd` |
| `(EmailAddr, Host)` pair | must succeed `LOGIN` before persistence                                    | `21720` (Add) / `21721` (Update) wrapping `ER-MAIL-21201` |
| `Order`      | `>= 0`; auto-assigned when 0 in Add                                                    | `21722 AccountOrderNegative` |

Validation is **synchronous** — no method persists before all checks pass. Network validation (`TestConnection`) runs before any disk write.

---

## 6. Atomicity & Recovery

Three operations cross two storage layers (`config.json` + SQLite `WatchState`). Pattern, mirroring Rules `21319`:

### 6.1 `Add` (insert in both)

```go
1. snapshot := deepCopy(config.Accounts)         // for rollback
2. testRes := TestConnection(spec)               // pure read, no side-effect
3. if !testRes.Ok → return 21720 with envelope.
4. config.Accounts = append(config.Accounts, newAccount)
5. BEGIN IMMEDIATE on SQLite.
6. INSERT INTO WatchState (...).                 // step A
7. cfg.Save()  (temp + rename, single fsync).    // step B
8. If step 7 fails → ROLLBACK SQLite; config.Accounts = snapshot; return 21731 AccountAddAtomicityFailed.
9. COMMIT SQLite.
10. If COMMIT fails → revert config.json from snapshot (write+rename); return 21731.
11. bus.Publish(AccountEvent{Added})
```

### 6.2 `Remove` (delete in both)

```go
1. snapshot := deepCopy(config.Accounts)
2. deletedWS := SELECT * FROM WatchState WHERE Alias=?   // for rollback
3. config.Accounts = removeByAlias(config.Accounts, alias)
4. BEGIN IMMEDIATE.
5. DELETE FROM WatchState WHERE Alias=?
6. cfg.Save()
7. If step 6 fails → ROLLBACK; config.Accounts = snapshot; return 21730 AccountRemoveAtomicityFailed.
8. COMMIT.
9. If COMMIT fails → revert config.json from snapshot; INSERT deletedWS back via fresh tx; return 21730.
10. bus.Publish(AccountEvent{Removed})
```

### 6.3 `Rename` (update in both)

```go
1. snapshot := deepCopy(config.Accounts)
2. config.Accounts[i].Alias = newAlias
3. BEGIN IMMEDIATE.
4. UPDATE WatchState SET Alias=newAlias WHERE Alias=oldAlias.
5. cfg.Save()
6. If step 5 fails → ROLLBACK; config.Accounts = snapshot; return 21732 AccountRenameAtomicityFailed.
7. COMMIT.
8. If COMMIT fails → revert config.json; UPDATE WatchState SET Alias=oldAlias; return 21732.
9. bus.Publish(AccountEvent{Renamed, PrevAlias: oldAlias})
```

`Update` (Host/Port/UseTls/Password) is config-only — no atomicity concern. `SetOrder` is config-only.

---

## 7. Error Codes (registry §21000–21099, §21200–21299, §21700–21799)

| Code  | Name                                | Layer    | Recovery                                      |
|-------|-------------------------------------|----------|-----------------------------------------------|
| 21701 | `AccountsListConfigLoadFailed`      | core     | Show error envelope, **Retry**                |
| 21702 | `AccountsListWatchStateQueryFailed` | core     | Show error envelope, **Retry**                |
| 21703 | `AccountNotFound`                   | core     | UI shows "not found" empty state              |
| 21704 | `AccountsSuggestEmailInvalid`       | core     | Field error in form                           |
| 21710 | `AccountAliasRequired`              | core     | Field error in form                           |
| 21711 | `AccountAliasInvalidChars`          | core     | Field error in form                           |
| 21712 | `AccountAliasTaken`                 | core     | Field error in form                           |
| 21713 | `AccountEmailTaken`                 | core     | Field error in form                           |
| 21714 | `AccountHostUnresolved`             | core     | Reveal manual Host/Port fields                |
| 21715 | `AccountRenameViaSeparateOp`        | core     | Caller bug — log WARN                         |
| 21716 | `AccountRenameTargetTaken`          | core     | Field error in rename dialog                  |
| 21717 | `AccountReorderInvalidSet`          | core     | Caller bug — log WARN                         |
| 21718 | `AccountEmailInvalid`               | core     | Field error in form                           |
| 21719 | `AccountPortInvalid`                | core     | Field error in form                           |
| 21720 | `AccountAddTestConnectionFailed`    | core     | Inline error under Password field             |
| 21721 | `AccountUpdateTestConnectionFailed` | core     | Inline error under changed field              |
| 21722 | `AccountOrderNegative`              | core     | Caller bug — log WARN                         |
| 21730 | `AccountRemoveAtomicityFailed`      | core     | Toast: "Remove failed; state restored"        |
| 21731 | `AccountAddAtomicityFailed`         | core     | Toast: "Add failed; state restored"           |
| 21732 | `AccountRenameAtomicityFailed`      | core     | Toast: "Rename failed; state restored"        |
| 21740 | `AccountAddPersistFailed`           | core     | Error envelope, **Retry**                     |
| 21741 | `AccountUpdatePersistFailed`        | core     | Error envelope, **Retry**                     |
| 21742 | `AccountRemovePersistFailed`        | core     | Error envelope, **Retry**                     |
| 21743 | `AccountReorderPersistFailed`       | core     | Rollback optimistic UI, error toast           |

Wrapped underlying errors (surfaced inside the envelope, not as the top-level code):

| Wrapped code | Source             | When                                         |
|--------------|--------------------|----------------------------------------------|
| `ER-MAIL-21200 ErrMailDial`         | mailclient | TCP dial fails (DNS, refused, unreachable)   |
| `ER-MAIL-21201 ErrMailLoginFailed`  | mailclient | IMAP `LOGIN` rejected by server              |
| `ER-MAIL-21207 ErrMailTLSHandshake` | mailclient | TLS negotiation fails                        |
| `ER-MAIL-21208 ErrMailTimeout`      | mailclient | 5 s deadline elapsed                         |
| `ER-CFG-21002 ErrConfigSaveFailed`  | config     | `cfg.Save` fails (disk full, permissions)    |
| `ER-CFG-21003 ErrConfigHiddenUnicodePwd` | config | Password contains C0 / zero-width chars    |
| `ER-CFG-21005 ErrConfigAliasDuplicate`   | config | (Should be caught earlier by `21712`)      |

Every error wrapped with `errtrace.Wrap(err, "Accounts.<Method>")`. Field-level errors set `errtrace.WithField("Field", "<FieldName>")`.

---

## 8. Logging

Per `05-logging-strategy.md`. PascalCase keys.

| Level | Event                       | Fields                                                                  |
|-------|-----------------------------|-------------------------------------------------------------------------|
| DEBUG | `AccountsList`              | `TraceId`, `DurationMs`, `AccountCount`                                 |
| INFO  | `AccountAdded`              | `TraceId`, `Alias`, `Host`, `Port`, `UseTls`, `TestLatencyMs`           |
| INFO  | `AccountUpdated`            | `TraceId`, `Alias`, `ChangedFields[]`, `TestLatencyMs?`                 |
| INFO  | `AccountRemoved`            | `TraceId`, `Alias`, `WatchStateRowsDeleted`                             |
| INFO  | `AccountRenamed`            | `TraceId`, `OldAlias`, `NewAlias`, `WatchStateRowsUpdated`              |
| INFO  | `AccountsReordered`         | `TraceId`, `AccountCount`                                               |
| DEBUG | `AccountSuggestImap`        | `TraceId`, `Domain`, `Source`, `Host?`, `Port?`                         |
| DEBUG | `AccountTestConnection`     | `TraceId`, `Alias?`, `Host`, `Port`, `UseTls`, `Ok`, `LatencyMs`, `ErrorCode?` |
| WARN  | `AccountEventOverflow`      | `TraceId`, `Subscriber`, `DroppedCount`                                 |
| WARN  | `AccountsListSlow`          | `TraceId`, `DurationMs`, `Threshold=15`                                 |
| ERROR | `AccountsFailed`            | `TraceId`, `Method`, `ErrorCode`, `ErrorMessage`                        |

**PII redaction (enforced by `errtrace` allow-list):**
- `Password` and `PasswordB64` are **never** logged. Not in any field, not in any error message, not in any wrapped envelope. The redaction allow-list test (`internal/errtrace/redaction_test.go`) MUST include a case that constructs an `Account` with `Password = "secret"` and asserts the rendered log line never contains `"secret"`.
- `EmailAddr` IS logged (operationally necessary; not considered PII for this app's threat model).
- `ServerGreeting` IS logged but truncated to **256 bytes** to avoid log bloat from chatty servers.

---

## 9. Testing Contract

File: `internal/core/accounts_test.go`. Target ≥ 90 % coverage.

Required test cases:

1. `List_NoAccounts_ReturnsEmpty`.
2. `List_FiftyAccounts_Under15ms` — perf gate (skipped under `-short`).
3. `List_StripsPasswordB64` — asserts no returned struct contains a non-empty password field.
4. `SuggestImap_KnownDomain_ReturnsBuiltin` — table test for top 10 providers.
5. `SuggestImap_UnknownDomain_ReturnsSourceUnknown_NoError`.
6. `SuggestImap_InvalidEmail_ReturnsErr21704`.
7. `TestConnection_Success_PopulatesGreetingAndLatency`.
8. `TestConnection_WrongPassword_ReturnsResultWithErrorCode_NotErr` — asserts `Result.Err == nil` AND `TestConnectionResult.ErrorCode == "ER-MAIL-21201"`.
9. `TestConnection_RespectsContextDeadline` — fault-injects slow dialer, expects `ER-MAIL-21208` within 5 s ± 100 ms.
10. `TestConnection_NeverPersists` — asserts zero `cfg.Save` calls AND zero SQL `EXEC` calls.
11. `Add_RejectsDuplicateAlias_Err21712`.
12. `Add_RejectsDuplicateEmail_Err21713`.
13. `Add_AutoSuggestsHostWhenEmpty`.
14. `Add_TestConnectionFails_NoDiskWrite_Err21720` — fault-injects login failure; asserts `config.json` mtime unchanged AND `WatchState` row count unchanged.
15. `Add_AssignsOrderMaxPlus10_WhenZero`.
16. `Add_AtomicAcrossConfigAndSqlite` — fault-injects `cfg.Save` failure; asserts SQLite `WatchState` insert rolled back AND `config.Accounts` reverted.
17. `Update_EmptyPassword_PreservesExisting`.
18. `Update_RejectsRename_Err21715`.
19. `Update_OnlyHostChanged_RerunsTestConnection`.
20. `Update_OnlyOrderChanged_DoesNotRerunTestConnection`.
21. `Rename_PreservesLastSeenUid` — asserts `WatchState.LastSeenUid` of the renamed alias equals the pre-rename value.
22. `Rename_TargetTaken_Err21716`.
23. `Rename_AtomicAcrossConfigAndSqlite` — fault-injects `cfg.Save`; asserts both stores reverted.
24. `Remove_AlsoDeletesWatchStateRow`.
25. `Remove_DoesNotCascadeEmailRows` — asserts `Email.Alias` set to `NULL` per FK rule.
26. `Remove_AtomicAcrossConfigAndSqlite` — fault-injects `cfg.Save`; asserts `WatchState` row reinserted.
27. `SetOrder_InvalidSet_Err21717`.
28. `SetOrder_ReassignsBy10s`.
29. `Subscribe_DeliversAddRemoveRename_InOrder`.
30. `Subscribe_OverflowDropsOldest_LogsWarn`.
31. `PasswordRedaction_NeverAppearsInLogs` — constructs Account with `Password="HuntER2"`, replays every method, scans captured log buffer for the substring; MUST be zero hits.

Fakes:
- `core.FakeConfigLoader` (in-memory).
- `store.NewMemory()`.
- `mailclient.FakeDialer` (scriptable Login outcome + greeting + latency).
- `imapdef.FakeSuggester` (table-driven).
- `eventbus.NewMemory()`.
- `core.FakeClock`.

---

## 10. Migration from Legacy `AddAccount` / `ListAccounts` / `RemoveAccount`

The current `internal/core/accounts.go` exposes package-level `AddAccount`, `ListAccounts`, `RemoveAccount` functions. Migration plan:

1. Add `type Accounts struct{}` and `NewAccounts` (this spec).
2. Re-implement legacy functions as thin wrappers calling `(*Accounts).Add / List / Remove` against a process-singleton instance.
3. Update CLI dispatch (`internal/cli/accounts.go`) to use `*core.Accounts` constructor-injected from `cmd/email-read/main.go`.
4. Update UI view (`internal/ui/views/accounts.go`) to use `*core.Accounts`.
5. Delete the package-level functions once both binaries are migrated.
6. Add `Update`, `Rename`, `SetOrder`, `Subscribe`, `TestConnection`, `SuggestImap` (these did not exist in legacy form).

**Behavioral deltas vs legacy:**
- Legacy `AddAccount` did NOT cascade-insert `WatchState` — the watcher lazily created the row on first poll. New behavior creates it eagerly inside the same atomic op.
- Legacy `RemoveAccount` did NOT delete the `WatchState` row — it leaked. New behavior deletes it.
- Legacy had no rename: users had to `Remove + Add`, losing `LastSeenUid` (which re-flooded the inbox on next poll). New `Rename` preserves the cursor.

---

## 11. Compliance Checklist

- [x] All identifiers PascalCase.
- [x] Methods use `errtrace.Result[T]`.
- [x] Constructor injects interfaces (`config.Loader`, `store.Store`, `mailclient.Dialer`, `imapdef.Suggester`, `eventbus.Publisher`, `Clock`).
- [x] No `any` / `interface{}`.
- [x] No `os.Exit`, no `fmt.Print*`.
- [x] All SQL uses singular PascalCase table names (`WatchState`).
- [x] Atomic cross-storage operations documented (Add / Remove / Rename).
- [x] Error codes registered in 21700–21799 range; wrapped codes from 21000–21099 (config) and 21200–21299 (mailclient).
- [x] PII redaction documented + enforced by named test (`PasswordRedaction_NeverAppearsInLogs`).
- [x] `View` projection strips `PasswordB64` from every UI-bound return.
- [x] `TestConnection` is read-only (zero `cfg.Save` and zero SQL `EXEC` — verified by named test).
- [x] Cites 02-coding, 03-error-management, 06-seedable-config, 18-database-conventions.

---

## N. Symbol Map (AC → Go symbol)

Authoritative bridge between `97-acceptance-criteria.md` IDs and the production Go identifiers an AI implementer must touch. **Status legend:** ✅ shipped on `main` · ⏳ planned · 🧪 test-only · 🟡 partial.

### N.1 Service surface

| AC IDs               | Go symbol                                                                            | File                                  | Status |
|----------------------|--------------------------------------------------------------------------------------|---------------------------------------|:------:|
| F-01, T-02           | `core.Accounts` + `NewAccounts(config.Manager, store.Store, Clock) *Accounts`        | `internal/core/accounts.go`           |   🟡   |
| F-02                 | `(*Accounts).List(ctx) errtrace.Result[[]AccountView]` *(strips PasswordB64)*        | `internal/core/accounts.go`           |   🟡   |
| F-04                 | `(*Accounts).Get(ctx, alias) errtrace.Result[AccountView]`                           | `internal/core/accounts.go`           |   ✅   |
| F-06, F-07           | `(*Accounts).Add(ctx, AccountInput) errtrace.Result[AddAccountResult]`               | `internal/core/accounts.go`           |   ✅   |
| F-08                 | `(*Accounts).Update(ctx, alias, AccountInput) errtrace.Result[AccountView]`          | `internal/core/account_update.go`     |   ✅   |
| F-11                 | `(*Accounts).Remove(ctx, alias) errtrace.Result[Unit]`                               | `internal/core/accounts.go`           |   ✅   |
| F-13, F-14, F-15     | `(*Accounts).TestConnection(ctx, alias) errtrace.Result[TestConnReport]` *(read-only)* | `internal/core/test_connection.go`  |   ✅   |
| L-01..L-09           | `core.AccountEvent` (`Added` / `Updated` / `Removed`) + `core.SubscribeAccountEvents` | `internal/core/account_events.go`    |   ✅   |

### N.2 Projection types

| AC IDs              | Go symbol                                              | File                          | Status |
|---------------------|--------------------------------------------------------|-------------------------------|:------:|
| F-02, F-04          | `core.AccountView`                                     | `internal/core/accounts.go`   |   🟡   |
| F-06                | `core.AccountInput`, `core.AddAccountResult`           | `internal/core/accounts.go`   |   ✅   |
| F-13                | `core.TestConnReport`                                  | `internal/core/test_connection.go` |   ✅ |

### N.3 Config / persistence

| AC IDs        | Go symbol                                                              | File                                    | Status |
|---------------|------------------------------------------------------------------------|-----------------------------------------|:------:|
| F-06, F-11, S-04 | `config.WithWriteLock(func(*Config) error) error` *(atomic mutate)*  | `internal/config/config.go`             |   ✅   |
| S-01..S-07    | `core.cleanPassword`, `core.validateAndSanitize` *(PII guards)*        | `internal/core/accounts.go`             |   ✅   |

### N.4 Errors & logging

| AC IDs        | Go symbol                                                              | File                                    | Status |
|---------------|------------------------------------------------------------------------|-----------------------------------------|:------:|
| E-01..E-10    | Codes 21500..21599 per §8 (e.g. `ErrAccountAliasTaken`, `ErrAccountNotFound`, `ErrTestConnectionAuthFailed`) | `internal/errtrace/codes_gen.go` | ⏳ |
| G-01..G-08    | `accountsSlog` (`component=accounts`) + `FormatAccount*` helpers       | `internal/ui/accounts_log.go`           |   ⏳   |
| S-05, X-01..X-07 | Password redaction enforced by `Test_LogScan_NoOriginalUrlLeak` pattern | `internal/core/accounts_log_scan_test.go` | ⏳ |

### N.5 Test contract

| AC IDs        | Test symbol                                                          | File                                            | Status |
|---------------|----------------------------------------------------------------------|-------------------------------------------------|:------:|
| T-01, T-02    | `TestAccounts_*` (full §7 test list)                                 | `internal/core/accounts_test.go`                |   🟡   |
| T-04          | `TestCF_A1`, `TestCF_A2_RaceFree`                                    | `internal/core/cf_acceptance_*.go`              |   ✅   |
| T-05          | `BenchmarkAccountsList_50Accounts`                                   | `internal/core/accounts_bench_test.go`          |   ⏳   |
| T-08          | `TestAccounts_TestConnection_ReadOnly`                               | `internal/core/test_connection_test.go`         |   ✅   |

---

**End of `04-accounts/01-backend.md`**
