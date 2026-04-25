# Feature 04 — Accounts

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI + CLI (`add/list/remove`)

---

## Purpose

Manage IMAP accounts. Sidebar account picker is populated from this list. Adding a new account auto-detects IMAP server settings from the email domain.

## User stories

- I see a table of accounts: alias, email, host, port, TLS, last-seen UID.
- I click "+ Add account" → inline form: alias, email, password, host (auto-suggest), port (auto-suggest), TLS toggle. Submit validates by attempting an IMAP LOGIN.
- I click "Remove" on a row → inline confirm strip.
- I see which account the sidebar is currently filtering by (highlighted row).

## Layout

```
┌─ Accounts ───────────────────────────────────────┐
│ [+ Add account]                                  │
│ ┌─────────────────────────────────────────────┐  │
│ │ alias   email          host           uid    │  │
│ │─────────────────────────────────────────────│  │
│ │ work    me@co.com      outlook.…      4123   │  │
│ │ atto    lov@atto…      mail.atto…     117    │  │
│ └─────────────────────────────────────────────┘  │
│                                                   │
│  [add form appears here when adding]             │
└───────────────────────────────────────────────────┘
```

## Backend (core API)

`internal/core/accounts.go`:

```go
func ListAccounts(ctx context.Context) ([]Account, error)
func SuggestImap(email string) ImapDefaults              // wraps internal/imapdef
func AddAccount(ctx context.Context, a Account, password string) error  // tries LOGIN before save
func RemoveAccount(ctx context.Context, alias string) error
```

`AddAccount` MUST:
1. Run `SuggestImap` if host is empty.
2. Open an IMAP connection and LOGIN — return the typed error to the UI on failure.
3. Only on successful LOGIN: persist Base64 password to `config.json`.

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-A1 | Typing an email in the form auto-fills host/port/TLS via `imapdef` lookup. |
| AC-A2 | Submitting wrong password shows the IMAP error inline; nothing written to disk. |
| AC-A3 | Removing an account also clears its rows from `WatchState` (consistent with CLI behavior). |
| AC-A4 | Sidebar account picker reflects add/remove within 1s. |
| AC-A5 | Password field is masked; never echoed to logs (verify `errtrace` redaction). |
