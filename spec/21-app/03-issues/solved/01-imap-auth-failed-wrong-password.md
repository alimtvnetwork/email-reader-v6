# 01 — IMAP `AUTHENTICATIONFAILED` on every poll (wrong password)

**Status:** solved in v0.10.0
**Severity:** Medium
**Area:** 04-accounts, 05-watch
**Opened:** 2026-04-20
**Resolved:** 2026-04-20
**Spec links:** [../../02-features/04-accounts/](../../02-features/04-accounts/), [../../02-features/05-watch/](../../02-features/05-watch/), [../../06-error-registry.md](../../06-error-registry.md)
**Source:** `.lovable/solved-issues/01-imap-auth-failed-wrong-password.md`

---

## Symptom

Watcher started cleanly:

```
starting watcher (poll=3s, host=...)
```

…but every subsequent poll silently failed. Adding raw IMAP testing showed:

```
a1 NO [AUTHENTICATIONFAILED] Authentication failed
```

returned by the Dovecot server.

## Root cause

The plaintext password the user typed during `email-read add` did not match the actual mailbox password. The Base64 encode/decode round-trip in `internal/config` was working correctly all along — the stored `PasswordB64` decoded to exactly the (incorrect) plaintext that was typed.

Because no helpful error was surfaced to the user beyond `AUTHENTICATIONFAILED`, debugging gravitated (incorrectly) toward the encoding layer.

## Fix

No code fix was required for the underlying bug — it was a user-data error. Two procedural learnings were applied:

1. The `Accounts` feature now documents (in `02-features/04-accounts/02-frontend.md`) that on `Test Connection` failure with code **`ER-ACC-21430`** (`AccountTestFailed`), the UI must instruct the user to verify the password via raw IMAP before assuming the app is at fault.
2. The CLI `doctor` command (introduced later in v0.13.0 — see issue 03) gained a "decoded password rune dump" so users can audit what is actually being sent.

### Verification procedure (now documented for users)

```bash
# 1. Decode the stored password to confirm what's actually being sent:
echo "<PasswordB64 from config.json>" | base64 -d

# 2. Test that exact plaintext via raw IMAP:
openssl s_client -connect <host>:993 -crlf -quiet
# after "* OK ... Dovecot ready."
a1 LOGIN <email> <plaintext-password>
```

If `a1 NO`, the password is wrong; reset it in the hosting provider and re-add the account.

## What NOT to repeat

- **Do not** investigate Base64 encoding/decoding logic when `AUTHENTICATIONFAILED` is reported. The encoding has been verified correct repeatedly; it always round-trips.
- **Do not** assume the username format is wrong without first ruling out the password — Dovecot returns the same `AUTHENTICATIONFAILED` for both.
- **Do** route any future IMAP-auth investigation through the `doctor` rune-dump first (issue 03).

## Iteration count

~6 messages of back-and-forth before the user confirmed the decoded password failed the openssl manual login test.

## Strictly-avoid entry

Recorded in `.lovable/strictly-avoid.md`:

> Never debug Base64 encoding when IMAP returns `AUTHENTICATIONFAILED` — start with `email-read doctor <alias>` and a raw `openssl s_client` LOGIN test.
