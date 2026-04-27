# 04 — Accounts — Overview

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **Accounts** feature is the credential-and-connection surface of `email-read`. It owns the lifecycle of every IMAP account the app knows about: discovery (auto-suggesting host/port/TLS from the email domain), validation (proving credentials work via a real `LOGIN` round-trip *before* persisting anything), persistence (`config.json` `Accounts[]` with Base64-obfuscated password), removal (cascading cleanup of `WatchState`), and visibility (sidebar account-picker is fed from this list).

This feature does **not** poll mail, evaluate rules, or open URLs — it only manages the set of accounts other features iterate over.

Cross-references:
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.4
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21000–21099` (config) + `21200–21299` (mailclient)
- Sibling features that consume accounts: `02-emails`, `05-watch`, `01-dashboard` (sidebar picker)
- Guidelines: `spec/12-consolidated-guidelines/13-app.md`, `16-app-design-system-and-ui.md`

---

## 1. Scope

### In scope
1. List all configured accounts in user-defined `Order` (display + sidebar feed).
2. Add an account through a guided form: `Alias`, `EmailAddr`, `Password`, optional `Host`, `Port`, `UseTls`.
3. **Auto-suggest** `Host`/`Port`/`UseTls` from the email domain via `internal/imapdef` whenever the form's `Host` field is empty.
4. **Test-connection on save**: open a real IMAP TCP/TLS dial + `LOGIN` *before* writing to disk; reject with the typed error on failure.
5. Remove an account by `Alias`, with a cascading delete of its `WatchState` rows in the same logical operation.
6. Rename an account `Alias` (carries `WatchState` rows by FK update — single transaction).
7. Edit an account: change `Host`, `Port`, `UseTls`, or `Password` (each edit re-runs the test-connection).
8. Sidebar picker stays in sync: add/remove/rename reflected within **1 s** via the live event channel.
9. Mask password input in UI; ensure password is never echoed to logs (relies on `errtrace` redaction allow-list).
10. Per-account read-only telemetry: `LastSeenUid`, `LastConnectedAt`, `LastConnectError` (sourced from `WatchState` + last `core.Watch` event).

### Out of scope
- OAuth2 / XOAUTH2. Deferred to v2 (`Account.AuthMethod` field reserved).
- Multi-folder selection per account (always `INBOX` for now).
- App-password generation flows (Gmail / Apple guidance is a static help link, not a wizard).
- Server-side rule push, IDLE-only mode, or per-account polling intervals (global interval lives in Settings).
- Importing accounts from `.mbox` / Thunderbird / `mutt` config files.

---

## 2. User Stories

| #  | As a … | I want to …                                                          | So that …                                                  |
|----|--------|----------------------------------------------------------------------|------------------------------------------------------------|
| 1  | User   | see every account I've added with its alias, address, host, last UID | I can audit what the watcher is monitoring                 |
| 2  | User   | type only my email and have host/port/TLS auto-fill                  | I don't need to know IMAP server names                     |
| 3  | User   | be told *immediately* if my password is wrong                        | I never end up with a saved-but-broken account             |
| 4  | User   | rename an alias without losing watch-state                           | My existing UID cursor is not reset                        |
| 5  | User   | remove an account in one click (with confirm)                        | I can clean up old addresses                               |
| 6  | User   | see which account the sidebar is filtering by                        | I always know the scope of what I'm reading                |
| 7  | User   | edit just the password (e.g. after rotation)                         | I don't have to re-enter host/port                         |
| 8  | User   | see the last connection error inline                                 | I can diagnose a watcher outage without opening logs       |
| 9  | User   | reorder accounts                                                     | The sidebar picker shows them in my preferred priority     |

---

## 3. Dependencies

| Dependency             | Why                                                                  |
|------------------------|----------------------------------------------------------------------|
| `core.Accounts`        | All CRUD + test-connection                                           |
| `internal/imapdef`     | (transitive) host/port/TLS suggestion from email domain              |
| `internal/mailclient`  | (transitive) IMAP dial + `LOGIN` for the test-connection step        |
| `internal/config`      | (transitive) reads/writes `config.json` `Accounts[]`                 |
| `internal/store`       | (transitive) `WatchState` cascade on remove / cascade on rename      |
| `core.Sidebar`         | Subscribes to `AccountEvent.Kind ∈ {Added, Removed, Renamed}`        |
| `core.Watch`           | Reads account list at startup; emits `LastConnectError` events       |
| `internal/ui/theme`    | Tokens for connected/disconnected badges, danger (delete confirm)    |

The view **must not** import `internal/imapdef`, `internal/mailclient`, `internal/config`, or `internal/store` directly. All access goes through `core.Accounts`.

---

## 4. Data Model

All names PascalCase (per `04-coding-standards.md` §1.1).

### 4.1 Core types

```go
type Account struct {
    Alias          string        // unique, case-sensitive identifier (1..32 chars, [A-Za-z0-9_-])
    EmailAddr      string        // RFC 5321 mailbox; unique across accounts
    Host           string        // IMAP host, e.g. "imap.gmail.com"
    Port           int           // 993 (TLS) or 143 (STARTTLS); 1..65535
    UseTls         bool          // true = implicit TLS on Port; false = STARTTLS
    PasswordB64    string        // Base64(password); never logged; never returned by List
    Order          int           // ascending; ties broken by Alias asc
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type AccountSpec struct {        // input shape for Add / Update — Password is plaintext
    Alias       string
    EmailAddr   string
    Host        string           // empty triggers SuggestImap
    Port        int              // 0 triggers SuggestImap
    UseTls      bool
    Password    string           // plaintext; encoded by core before persistence
    Order       int              // 0 = append (max+10)
}

type AccountView struct {        // safe-for-UI projection — Password fields stripped
    Alias            string
    EmailAddr        string
    Host             string
    Port             int
    UseTls           bool
    Order            int
    LastSeenUid      uint32        // joined from WatchState
    LastConnectedAt  time.Time     // joined from WatchState
    LastConnectError string        // joined from last WatchEvent.LastConnectError
    IsConnected      bool          // derived: LastConnectError == "" && LastConnectedAt within 2× poll interval
}

type ImapDefaults struct {        // returned by SuggestImap
    Host    string
    Port    int
    UseTls  bool
    Source  ImapDefaultsSource    // PascalCase enum
}

type ImapDefaultsSource string
const (
    ImapDefaultsSourceBuiltin ImapDefaultsSource = "Builtin"  // hard-coded table (gmail, outlook, …)
    ImapDefaultsSourceMxLookup ImapDefaultsSource = "MxLookup" // reserved; not used in v1
    ImapDefaultsSourceUnknown  ImapDefaultsSource = "Unknown"  // domain not recognised; UI shows manual fields
)

type TestConnectionResult struct {
    Ok          bool
    ServerGreeting string         // first IMAP server line, for diagnostics
    LatencyMs   int
    ErrorCode   string            // e.g. "ER-MAIL-21201" when Ok == false
    ErrorMsg    string            // user-facing, redacted
}

type AccountEvent struct {
    Kind       AccountEventKind
    Alias      string             // current alias (post-rename)
    PrevAlias  string             // populated only when Kind == AccountEventKindRenamed
    OccurredAt time.Time
}

type AccountEventKind string
const (
    AccountEventKindAdded    AccountEventKind = "Added"
    AccountEventKindRemoved  AccountEventKind = "Removed"
    AccountEventKindRenamed  AccountEventKind = "Renamed"
    AccountEventKindUpdated  AccountEventKind = "Updated"   // Host/Port/UseTls/Password change
    AccountEventKindReordered AccountEventKind = "Reordered"
)
```

### 4.2 Validation rules

| Field        | Rule                                                                                  | Error code on violation        |
|--------------|---------------------------------------------------------------------------------------|--------------------------------|
| `Alias`      | 1..32 chars, regex `^[A-Za-z0-9_-]+$`, unique                                         | `ER-CFG-21005` (duplicate) / `ER-COR-21701` (format) |
| `EmailAddr`  | Parses via `net/mail.ParseAddress`; lowercase domain stored                           | `ER-COR-21702`                 |
| `Host`       | RFC 1123 hostname; non-empty after `SuggestImap`                                      | `ER-COR-21703`                 |
| `Port`       | 1..65535; non-zero after `SuggestImap`                                                | `ER-COR-21704`                 |
| `Password`   | Non-empty; ≤ 1024 bytes; rejected if any C0 control char (security: hidden Unicode)   | `ER-CFG-21003`                 |
| Pair `(EmailAddr, Host)` | Must succeed `LOGIN` before persistence                                  | `ER-MAIL-21202` (login) / `ER-MAIL-21201` (dial) / `ER-MAIL-21207` (TLS) |

Validation runs **synchronously** in the `core.Accounts.Add`/`Update` call before any disk write. The test-connection step is wrapped in a context with a **5 s** deadline.

### 4.3 Default values

```go
AccountSpec{
    Order:  max(existing.Order)+10,   // append, gaps for easy reordering
    UseTls: true,                     // safer default
    Port:   0,                        // 0 → SuggestImap fills in 993 (TLS) or 143 (STARTTLS)
}
```

### 4.4 Persistence shape

`config.json` (excerpt):
```json
{
  "Accounts": [
    {
      "Alias": "work",
      "EmailAddr": "me@co.com",
      "Host": "outlook.office365.com",
      "Port": 993,
      "UseTls": true,
      "PasswordB64": "c2VjcmV0",
      "Order": 10,
      "CreatedAt": "2026-04-25T09:00:00Z",
      "UpdatedAt": "2026-04-25T09:00:00Z"
    }
  ]
}
```

`WatchState` rows (one per `Alias`) are owned by the Watch feature but **cascaded** by Accounts on rename/remove — see §5.

---

## 5. Lifecycle Coordination (cross-storage atomicity)

`Accounts` straddles two storage tiers (`config.json` for credentials, SQLite `WatchState` for cursors). The same atomic pattern Rules uses (Code `21319` rollback) applies here:

| Operation | Step 1                                  | Step 2                                                | Rollback if Step 2 fails               |
|-----------|-----------------------------------------|-------------------------------------------------------|----------------------------------------|
| `Add`     | Test-connection (read-only)             | Append to `config.json` and `INSERT WatchState` (single SQLite tx) | Restore in-memory `Accounts[]` slice; no disk side-effect occurred yet |
| `Remove`  | `DELETE FROM WatchState WHERE Alias=?`  | Remove entry from `config.json`                       | `INSERT` the deleted `WatchState` row back; surface `ER-COR-21710` |
| `Rename`  | `UPDATE WatchState SET Alias=NewAlias`  | Patch `config.json` `Accounts[*].Alias`               | `UPDATE WatchState SET Alias=OldAlias`; surface `ER-COR-21711` |
| `Update`  | Test-connection with new credentials    | Patch the in-memory `Account`; persist `config.json`  | Re-load `config.json` from disk; surface `ER-COR-21712` |
| `Reorder` | Patch in-memory `Order`                  | `config.json` write (single fsync)                    | Re-load from disk                      |

All five operations emit exactly one `AccountEvent` on success and **zero** events on failure.

---

## 6. Refresh & Live-Update

| Trigger                                                    | Action                                                  |
|------------------------------------------------------------|---------------------------------------------------------|
| Tab opened                                                 | `core.Accounts.List` once                               |
| Account added / removed / renamed / updated / reordered    | Optimistic UI update; server-confirmed re-list on success |
| `WatchEvent.Kind == AccountConnected` for visible row      | Flip `IsConnected` badge to green; clear `LastConnectError` |
| `WatchEvent.Kind == AccountConnectError` for visible row   | Flip `IsConnected` badge to red; populate `LastConnectError` |
| Sidebar picker change (cross-feature)                       | Highlight the matching row (no list refresh)            |
| Tab loses focus                                            | Unsubscribe from `WatchEvent` channel                   |

The view never opens an IMAP connection itself — only `core.Accounts.TestConnection` and `core.Watch` do. The view's **TestConnection** path is read-only and writes nothing to `config.json` or SQLite.

---

## 7. Acceptance Snapshot (full criteria in `97-acceptance-criteria.md`)

A merged Accounts build is shippable iff:

1. `List` returns ≤ **15 ms** with 50 accounts (joined with `WatchState`).
2. `Add` with a wrong password surfaces `ER-MAIL-21201` inline within ≤ **5 s** and writes **nothing** to `config.json` or SQLite (verified by file mtime + DB row count).
3. Auto-suggest fills `Host`/`Port`/`UseTls` for the top 10 providers (gmail, outlook, yahoo, icloud, fastmail, gmx, protonmail-bridge, zoho, aol, hostinger) using `internal/imapdef`'s built-in table.
4. `Remove` cascades the `WatchState` row in the same logical op — verified by `SELECT COUNT(*) FROM WatchState WHERE Alias=?` returning 0 within the same call.
5. `Rename` preserves `LastSeenUid` (FK update, not delete-and-reinsert).
6. Password is masked in UI (`widget.Entry.Password = true`) and never appears in any structured log line — enforced by `errtrace` redaction allow-list test.
7. Sidebar picker reflects add/remove/rename within **1 s** via the live event channel (no polling).
8. Editing only `Password` does **not** trigger `Host`/`Port` re-suggestion (form preserves user-edited fields).
9. Concurrent `Add` + `Reorder` cannot interleave to break `Order` uniqueness invariant ("ascending integers, no duplicates within an account list").
10. Zero `interface{}` / `any` in any new code (lint-enforced).

---

## 8. Open Questions

None. Confidence: Production-Ready.

The following are explicit **deferrals**, not ambiguities:
- OAuth2 / XOAUTH2 → v2 (`AuthMethod` field reserved).
- Per-account poll interval → v2 (global interval in Settings is sufficient for v1).
- MX-record-based `SuggestImap` fallback → v2 (`ImapDefaultsSourceMxLookup` enum reserved).

---

**End of `04-accounts/00-overview.md`**
