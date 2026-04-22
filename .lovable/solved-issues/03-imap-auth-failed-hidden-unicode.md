# Solved — IMAP `AUTHENTICATIONFAILED` caused by hidden Unicode in stored password

## Description
`email-read watch admin` failed every poll with `imap login ...: Authentication failed.`
But the same plaintext password worked when typed manually via `openssl s_client` raw IMAP login.

## Root Cause
The user pasted the password from a chat/markdown source. The rendered chat surrounds inline
code with **U+2060 WORD JOINER** (and a leading/trailing ASCII space). Those invisible runes
were captured by `add-quick --password '...'` and Base64-encoded into config.json verbatim.

When the watcher decoded and sent the password, the literal bytes (`\u2060 ZPb*sz=d!cEE_Wgc \u2060`)
did NOT match the server-side password (`ZPb*sz=d!cEE_Wgc`). Server returned `AUTHENTICATIONFAILED`.

Manual `openssl` LOGIN succeeded because the user typed the password fresh (no zero-width chars).

## Detection
```bash
python3 -c "pw='\u2060 ZPb*sz=d!cEE_Wgc \u2060'; [print(f'U+{ord(c):04X} {c!r}') for c in pw]"
```
Showed leading/trailing `U+2060` runes wrapping the visible password.

## Solution (v0.13.0)
1. `config.SanitizePassword(s)` strips leading/trailing whitespace, Unicode `Cf` (format) runes,
   and common no-break spaces (NBSP, NNBSP, IDEO SP).
2. `EncodePassword` and `DecodePassword` both call it — defense in depth, fixes accounts that
   were stored before the fix existed (next watch will use the cleaned form).
3. `add-quick` warns if sanitization removed any chars, telling the user how many were stripped.
4. New `email-read doctor [alias]` command dumps the rune-by-rune contents of stored passwords
   so users can audit what's truly being sent to IMAP.

## What NOT to Repeat
- Do NOT investigate Base64 logic when `AUTHENTICATIONFAILED` is reported — the encoding
  always round-trips correctly.
- Do NOT trust `[System.Text.Encoding]::UTF8.GetString(...)` output in PowerShell as proof
  the password is "clean" — many terminals render U+2060/U+200B as nothing.
- Always check stored password length vs visible length (use `email-read doctor`).
