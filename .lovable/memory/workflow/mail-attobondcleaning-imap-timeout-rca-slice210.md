---
name: mail.attobondcleaning.store IMAP timeout RCA (Slice #210)
description: External TCP timeout to mail.attobondcleaning.store:993 means server/firewall/DNS provider issue before IMAP login; app must not blame credentials.
type: feature
---
# mail.attobondcleaning.store IMAP timeout RCA — Slice #210

## Symptom

The app shows:

```text
[ER-ACC-22201] test connection failed ... [ER-MAIL-21208] imap dial timed out: dial tcp 103.138.189.68:993: i/o timeout
```

The user's local TCP probe also shows:

```text
nc -zv mail.attobondcleaning.store 993
connectx ... port 993 failed: Operation timed out
```

## Root cause

This is **not an application bug and not a password/authentication failure yet**. TCP to `mail.attobondcleaning.store:993` times out before IMAP LOGIN is reached, so the app cannot authenticate or read mail.

DNS resolves `mail.attobondcleaning.store` to `103.138.189.68`, but the IMAP port is unreachable externally. Most likely causes:

- Dovecot/IMAP service is stopped or not listening on 993/143.
- Hosting firewall/CSF/LFD blocks inbound IMAP ports 993 and/or 143.
- External IMAP is disabled by the hosting provider.
- Cloudflare/mail DNS is proxied instead of DNS-only.
- `mail.attobondcleaning.store` points to the wrong shared-hosting mail server hostname/IP.
- Local ISP/network blocks the port, confirmed by comparing another network/mobile hotspot.

## Solution path

1. Test both IMAP ports from the user's machine:

   ```sh
   nc -zv mail.attobondcleaning.store 993
   nc -zv mail.attobondcleaning.store 143
   ```

2. If `143` works but `993` fails, configure the app as host `mail.attobondcleaning.store`, port `143`, TLS unchecked.
3. If both timeout, ask hosting support to enable Dovecot/IMAP and open external TCP ports `993` and `143`.
4. If DNS is on Cloudflare, ensure the `mail` record is **DNS only**/gray-cloud, not proxied.
5. In cPanel → Email Accounts → Connect Devices, copy the secure IMAP hostname shown by the host; it may be a server hostname rather than `mail.attobondcleaning.store`.
6. Never store or repeat the user's mailbox password in memory/docs/logs. If a password was pasted into chat/screenshots, advise the user to rotate it after debugging.

## Code behavior locked in

- `core.TestAccountConnection` now expands timeout/dial failures with reachability guidance: `nc -zv`, 993/143, Dovecot/firewall, and Cloudflare DNS-only.
- Watch Raw log/card rendering adds the same reachability RCA for wrapped `ER-MAIL-21208`/`ER-MAIL-21201` errors.
- Auth wording remains reserved for `ER-MAIL-21202` (`ErrMailLogin`) only, because that proves IMAP LOGIN was reached.