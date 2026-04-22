---
name: IMAP auth failed root causes
description: Two known causes of AUTHENTICATIONFAILED — wrong password (issue 01) or hidden Unicode in stored password (issue 03). Always rule out password byte-level integrity BEFORE blaming code paths.
type: feature
---
When IMAP returns `AUTHENTICATIONFAILED`:

1. Verify password byte-level integrity: `email-read doctor <alias>` (added in v0.13.0).
   - Look for runes outside the printable ASCII range, especially U+2060 (WORD JOINER),
     U+200B (ZERO-WIDTH SPACE), U+FEFF (BOM), U+00A0 (NBSP).
   - These come from copy-pasting passwords out of chat/markdown.
2. If sanitized form differs from raw stored form, re-add via `add-quick` (sanitization is
   automatic on encode now), or just re-run watch — DecodePassword sanitizes too.
3. If password is byte-clean and IMAP still rejects, do the openssl s_client manual LOGIN
   test (see solved-issues/01) — the password is genuinely wrong server-side.

Base64 encoding/decoding is NOT the problem. It always round-trips correctly.
