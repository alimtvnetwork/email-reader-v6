# Solved — IMAP `AUTHENTICATIONFAILED` on every poll

## Description
Watcher started cleanly (`starting watcher (poll=3s, host=...)`) but every subsequent poll silently failed. After adding raw IMAP testing, `a1 NO [AUTHENTICATIONFAILED] Authentication failed` was returned by the Dovecot server.

## Root Cause
The password the user typed during `email-read add` did not match the actual mailbox password. The CLI's Base64 encode/decode round-trip was working correctly all along — the stored `passwordB64` decoded to exactly the plaintext that the user (incorrectly) typed.

## Steps to Reproduce
1. `email-read add` with a wrong password.
2. `email-read watch <alias>` → silent failures (no progress, no log lines past startup with the OLD logging).

## Solution
1. Decode the stored password to confirm what's actually being sent:
   ```bash
   echo "<passwordB64 from config.json>" | base64 -d
   ```
2. Test that exact plaintext via raw IMAP:
   ```bash
   openssl s_client -connect <host>:993 -crlf -quiet
   # after "* OK ... Dovecot ready."
   a1 LOGIN <email> <plaintext-password>
   ```
3. If `a1 NO`, the password is wrong. Reset it in cPanel (or hosting provider) and ideally use a simple test password (no `=`, `&`, `}`, `$`, spaces) for first-time setup.
4. `email-read remove <alias>` + `email-read add` with the corrected password.

## Iteration Count
~6 messages of back-and-forth before the user confirmed the decoded password failed the openssl manual login test.

## Learning
- The Base64 layer in `internal/config` is **not** an authentication issue — it always round-trips cleanly. When auth fails, it is virtually always a wrong-password issue at the user input layer.
- Always teach users to verify credentials with `openssl s_client` BEFORE blaming the CLI. This is a 30-second test that isolates the variable.

## What NOT to Repeat
- Do not investigate Base64 encoding/decoding logic when `AUTHENTICATIONFAILED` is reported. The encoding has been verified correct repeatedly.
- Do not assume the username format is wrong without first ruling out the password — Dovecot returns the same `AUTHENTICATIONFAILED` for both.
