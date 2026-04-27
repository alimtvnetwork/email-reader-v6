# 03 — IMAP `AUTHENTICATIONFAILED` caused by hidden Unicode in stored password

**Status:** solved in v0.13.0
**Severity:** High
**Area:** 04-accounts, 06-tools
**Opened:** 2026-04-21
**Resolved:** 2026-04-21
**Spec links:** [../../02-features/04-accounts/](../../02-features/04-accounts/), [../../02-features/06-tools/](../../02-features/06-tools/), [../../06-error-registry.md](../../06-error-registry.md)
**Source:** `.lovable/solved-issues/03-imap-auth-failed-hidden-unicode.md`

---

## Symptom

`email-read watch admin` failed every poll with `imap login ...: Authentication failed.` But the same plaintext password worked when typed manually via `openssl s_client` raw IMAP login.

## Root cause

The user pasted the password from a chat / Markdown source. The rendered chat surrounded inline code with **U+2060 WORD JOINER** (and a leading/trailing ASCII space). Those invisible runes were captured by `add-quick --password '...'` and Base64-encoded into `config.json` verbatim.

When the watcher decoded and sent the password, the literal bytes (`\u2060 ZPb*sz=d!cEE_Wgc \u2060`) did NOT match the server-side password (`ZPb*sz=d!cEE_Wgc`). The server returned `AUTHENTICATIONFAILED`.

Manual `openssl` LOGIN succeeded because the user typed the password fresh (no zero-width chars).

### Detection one-liner

```bash
python3 -c "pw='\u2060 ZPb*sz=d!cEE_Wgc \u2060'; [print(f'U+{ord(c):04X} {c!r}') for c in pw]"
```

…showed leading/trailing `U+2060` runes wrapping the visible password.

## Fix (v0.13.0)

1. **`config.SanitizePassword(s)`** — strips leading/trailing whitespace, Unicode `Cf` (format) runes, and common no-break spaces (NBSP, NNBSP, IDEO SP).
2. **`EncodePassword`** and **`DecodePassword`** both call it — defense in depth, fixes accounts that were stored before the fix existed (next watch will use the cleaned form).
3. **`add-quick`** warns if sanitization removed any chars, telling the user how many were stripped.
4. **New `email-read doctor [alias]`** command dumps the rune-by-rune contents of stored passwords so users can audit what's truly being sent to IMAP. Now part of the **Tools** feature (`02-features/06-tools/`).

## Spec encoding

- `spec/21-app/02-features/04-accounts/01-backend.md` §Password Sanitization — formalises the sanitization invariant.
- `spec/21-app/02-features/06-tools/02-frontend.md` §Doctor card — UI surface for the rune-dump audit.
- `spec/21-app/06-error-registry.md` — error code `ER-ACC-22202` (`ErrAccountPasswordSanitized`) is a non-fatal warning emitted at sanitization time. *(Slice #158: renumbered from `ER-ACC-21431` — original number fell in the Watcher block `21400–21499`. The Accounts feature now owns its own block `22200–22299` per `spec/21-app/04-coding-standards.md` §5.4.)*

## What NOT to repeat

- **Do not** investigate Base64 logic when `AUTHENTICATIONFAILED` is reported — the encoding always round-trips correctly (see also issue 01).
- **Do not** trust `[System.Text.Encoding]::UTF8.GetString(...)` output in PowerShell as proof the password is "clean" — many terminals render U+2060 / U+200B as nothing.
- **Do** check stored password length vs visible length. The Tools → Doctor card now does this automatically.

## Strictly-avoid entry

Recorded in `.lovable/strictly-avoid.md`:

> Always sanitize free-text credentials at the boundary (`SanitizePassword`); do not rely on the user copy-paste being clean.
