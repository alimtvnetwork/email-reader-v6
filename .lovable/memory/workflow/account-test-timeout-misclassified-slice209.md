---
name: Account Test Connection timeout misclassified as login rejection (Slice #209)
description: ER-ACC-22201 is only the Account Test wrapper; inspect wrapped ER-MAIL cause. Timeout/dial failures must say endpoint unreachable, not password rejected.
type: feature
---

# Symptom

Add/Edit Account → Test connection showed:

```text
[ER-ACC-22201] test connection failed — IMAP server rejected the login ...
[ER-MAIL-21208] imap dial timed out: dial tcp 103.138.189.68:993: i/o timeout
```

# Root cause analysis

`ER-ACC-22201` is the account-level wrapper for any Test Connection failure. The real cause is the wrapped mail error.

Here, the wrapped cause was `ER-MAIL-21208 imap dial timed out`, meaning the TCP/TLS endpoint was unreachable or tarpitted before IMAP LOGIN. The server did **not** reject the password in this attempt because LOGIN was never reached.

The app bug was in `core.wrapTestConnectionError`: it used one hardcoded message for all test failures — “IMAP server rejected the login” — even when the underlying cause was a network timeout/dial failure.

# Fix / future rule

- Only show the “server rejected login / verify password” message when the wrapped cause is `ER-MAIL-21202` (`ErrMailLogin`).
- For `ER-MAIL-21208` (`ErrMailTimeout`) or `ER-MAIL-21201` (`ErrMailDial`), show endpoint reachability guidance: verify host, port, TLS, firewall, DNS, and mail-server availability.
- When debugging future Test Connection reports, do not diagnose from `ER-ACC-22201` alone. Always inspect the wrapped `ER-MAIL-*` code.
