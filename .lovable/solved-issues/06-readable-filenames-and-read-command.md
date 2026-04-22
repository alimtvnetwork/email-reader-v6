# 06 — Confusing .eml filenames + URL-open only worked from `watch`

**Status:** solved in v0.16.0
**Date:** 2026-04-22 (Asia/Kuala_Lumpur)

## Symptom (user, verbatim)
> "after receiving the email, it create folders like this its file naming so
> confusing add time as a prefix and also use proper readable name. On the
> other hand, after receiving the email, it can read it properly. I told you
> that if you receive any kind of link, you have to open it in incognito,
> but your code can't do it, and only the watch command does this action.
> If you find any link in the mail body, you can delay 10s to check the
> next email."

Two real problems:
1. **Filenames were Gmail Message-IDs** — e.g.
   `CAP8r7W1mfzHBmcybeHK5Dxk8U5Ez6jmU-krVgBfEmVtQZY_vXA@mail.gmail.com.eml`.
   Unsortable, unreadable, can't tell at a glance who/when/what.
2. **Only `watch` opened URLs in incognito.** No way to manually re-trigger
   the verification flow for a saved email; if you closed the tab too fast
   you had to wait for new mail to arrive.
3. **No throttling.** When 3 verification emails arrived back-to-back, the
   browser got hammered and the first page hadn't even loaded before the
   third URL appeared.

## Fix (v0.16.0)

### 1. Readable filenames — `internal/mailclient/mailclient.go`
New format:
```
email/<alias>/<YYYY-MM-DD>/HH.MM.SS__<from>__<subject>__uid<N>.eml
```

Example:
```
email/admin/2026-04-22/19.17.14__abdullah-mahin-rasia-gmail-com__re-check__uid12.eml
```

- `HH.MM.SS` from `ReceivedAt` → files sort chronologically inside the day.
- `from` extracted from `<addr@host>` and lowercased+hyphenated:
  `"Abdullah Al Mahin" <abdullah.mahin.rasia@gmail.com>` →
  `abdullah-mahin-rasia-gmail-com`.
- `subject` lowercased+hyphenated: `Re: Check` → `re-check`.
- `__uidN` suffix → guaranteed unique even when two emails share
  second + sender + subject.
- New helpers: `sanitizeReadable()` and `extractEmailAddr()`.

### 2. New `read` command — `internal/cli/read.go`
```
email-read read <alias> <uid>
```
Loads the saved email from SQLite, runs the same `rules.EvaluateWithTrace`
the watcher uses, and opens matched URLs in incognito with the same
per-rule `✓/✗` trace. Bypasses the OpenedUrls dedup so the user can
re-trigger expired verification links without resetting the DB.

Supporting change: `internal/store/store.go` — new
`GetEmailByUid(ctx, alias, uid)` helper.

### 3. 10-second cooldown — `internal/watcher/watcher.go`
Two cooldowns, both honoring ctx (Ctrl+C interrupts cleanly):
- **Between URL-bearing messages in the same batch** — when 3 new emails
  arrive at once, after launching URLs from message #1 the watcher logs
  `⏳ waiting 10s before processing next message in batch…` then proceeds.
- **Before next poll cycle** — after a poll that opened any URL, the
  watcher logs `⏳ opened URL(s) — waiting 10s before next poll…` and
  delays the next IMAP fetch by 10s.

Idle polls (no URLs opened) are NOT throttled — keeps responsiveness when
the inbox is quiet. Implemented via a tiny `sleepCtx(ctx, d)` helper.

### 4. Error tracing
Every new error path uses `errtrace.Wrap` / `errtrace.Wrapf` so failures
print file:line frames (per the user preference recorded in
`mem://preferences/01-error-stack-traces`). E.g. a write failure now shows:
```
error: write eml /Users/.../19.17.14__...__uid12.eml: permission denied
  at internal/mailclient/mailclient.go:336 (mailclient.SaveRaw)
  at internal/watcher/watcher.go:228 (watcher.pollOnce)
```

### 5. Version bump
`cmd/email-read/main.go`: `0.15.0` → `0.16.0`.

## How to verify
```powershell
.\run.ps1
email-read watch admin
# send an email containing https://lovable.dev/auth/action?...
# expected log:
#   ✉ 1 new message(s)
#       uid=13  from=abdullah.mahin.rasia@gmail.com  subj="Re: Verify"
#       saved → .../19.30.42__abdullah-mahin-rasia-gmail-com__re-verify__uid13.eml
#       rules: ✓ "verify-links" → 1 url(s)
#       → opening in incognito: https://lovable.dev/...
#       ✓ launched
#   [admin] ⏳ opened URL(s) — waiting 10s before next poll…

# Then re-trigger manually:
email-read read admin 13
# expected: same trace, same incognito launch — useful when the link
# expired or you closed the tab.
```

## Tests
```
ok  internal/browser
ok  internal/config
ok  internal/errtrace
ok  internal/exporter
ok  internal/imapdef
ok  internal/mailclient
ok  internal/rules
ok  internal/store
ok  internal/watcher
```
All green; no behavioral regressions.

## Remaining edge cases (intentionally deferred)
- Existing files with the OLD message-id naming are NOT renamed — would
  require a one-shot migration we don't want to run silently. Old files
  still work; new files use the new scheme.
- The `read` command does NOT decode quoted-printable bodies on the fly —
  it uses whatever was stored at fetch time. If the user reports
  `urlRegex matched 0 URLs`, that points at `internal/mailclient/parseRaw`
  not decoding QP transfer encoding — separate bug, separate PR.
