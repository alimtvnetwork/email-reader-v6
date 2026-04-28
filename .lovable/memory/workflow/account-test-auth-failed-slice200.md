---
name: Account Test Connection auth failure RCA (Slice #200)
description: Test Connection showed correct-looking email/password but server returned AUTHENTICATIONFAILED; app now sanitizes test passwords and wraps failures as ER-ACC-22201 with endpoint context.
type: feature
---

# Symptom

In Add account → Test connection, user entered:

- Email: `lovable.admin@attobondcleaning.store`
- Host: `mail.attobondcleaning.store`
- Port: `993`
- TLS: enabled

The UI returned:

`Test failed: [ER-MAIL-21202] imap login: Authentication failed. (Email=... Host=...)`

# Root cause analysis

The app reached the correct IMAP server and got as far as `LOGIN`. That means DNS, port, and TLS were working. The failure came from the mail server rejecting the username/password pair.

Two actionable causes remain:

1. The mailbox password/token is not the password the IMAP server accepts (most common with cPanel/custom-domain mailboxes).
2. Hidden leading/trailing Unicode from copy-paste changed the password bytes before LOGIN.

The code had one real UX/consistency bug: `Save account` sanitized hidden leading/trailing Unicode through `config.SanitizePassword`, but `Test connection` passed `PlainPassword` directly to `mailclient.DialPlain`. So a password that would be cleaned on Save could still fail during Test.

# Fix (Slice #200)

- `core.TestAccountConnection` now sanitizes the password before calling `mailclient.DialPlain`, matching Save/Watch behavior.
- Test failures are wrapped as `ER-ACC-22201 ErrAccountTestFailed` with endpoint context and a user-actionable message: verify the same credentials in webmail or reset the mailbox password.
- Added a regression test that injects zero-width Unicode around a password and asserts Test Connection sends the sanitized password while preserving the wrapped `ER-MAIL-21202` login cause.

# What this does not change

If the same sanitized credentials are rejected by the hosting provider, the app cannot bypass that. The mailbox password must be reset/confirmed in the mail host, then re-tested.
