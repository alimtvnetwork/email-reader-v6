# 05 — New mail arrives but URL never opens in incognito (silent failure)

**Status:** solved in v0.15.0
**Date:** 2026-04-22 (Asia/Kuala_Lumpur)

## Symptom
User sent a test email containing a `https://lovable.dev/auth/action?...`
verification URL. Watcher correctly logged:
```
[admin] ✉ 1 new message(s)
    uid=12  from=abdullah.mahin.rasia@gmail.com  subj="Re: Check"
```
…and then nothing. No browser launched, no error printed. User quote:
"It works, but when it receives an email, it cannot open it in incognito
mode and verify it. why its fail also log this"

## Root cause
Two compounding issues:

1. **Silent rule evaluation.** The watcher logged
   `matched 0 rule URL(s)` only when `--verbose` was on (v0.14 quiet-mode
   refactor). In quiet mode the rule-engine outcome was completely hidden,
   so users had no way to tell whether 0 rules existed, a rule's
   `subjectRegex` rejected the message, or the URL pattern was too strict.
2. **No startup readiness banner.** The watcher never said "I have N rules
   loaded" or "browser resolved at /usr/bin/chrome" at startup, so the user
   couldn't tell the difference between "no rules configured" and "rules
   exist but didn't match this email."

## Fix (v0.15.0)

### internal/rules/rules.go
- New `EvaluateWithTrace(m) ([]Match, []RuleTrace)` returns a per-rule
  explanation (`fromRegex did not match…`, `urlRegex matched 0 URLs in
  body (regex too strict?)`, `no urlRegex configured`, etc.).
- Existing `Evaluate(m)` kept as thin wrapper for backward compat.
- New `RuleCount()` so the watcher can warn at startup when 0 rules exist.

### internal/watcher/watcher.go — startup banner
Always logs (in both modes):
```
[admin] N enabled rule(s) loaded
[admin] browser ready: /Applications/Google Chrome.app/.../Google Chrome (incognito flag="--incognito")
```
…or, when misconfigured:
```
[admin] ⚠ 0 enabled rules loaded — incoming mail will be saved but no URLs will be opened. Add a rule in data/config.json (rules[].enabled=true with a urlRegex).
[admin] ⚠ browser not resolved yet:
error: no Chrome/Chromium-family browser found; set config.browser.chromePath or EMAIL_READ_CHROME
```

### internal/watcher/watcher.go — per-mail rule trace (always logged)
For every new message, the watcher now prints one line per rule:
```
    rules: ✗ "verify-links" → fromRegex did not match From: "Abdullah Al Mahin" <abdullah.mahin.rasia@gmail.com>
    rules: ✓ "verify-links" → 1 url(s)
    → opening in incognito: https://lovable.dev/auth/action?... (rule=verify-links)
    ✓ launched
```
And on failure:
```
    ✗ browser launch failed for https://...:
    error: launch /usr/bin/chrome: exec: "chrome": executable file not found in $PATH
      at internal/browser/browser.go:62 (browser.Launcher.Open)
```

### Why both modes show this
This is information the user needs *every time mail arrives* — without it,
the system looks broken even when it's working as configured. Per-poll noise
(dialing, watch-state-load) stays gated behind `--verbose`; per-mail
evaluation does not.

## Most likely cause for THIS user's case
Their `data/config.json` either:
- has zero rules with `enabled: true`, OR
- has a `fromRegex` like `^.*@lovable\.dev$` that rejected the gmail
  sender, OR
- has a `urlRegex` like `https://lovable\.dev/auth/.*&apiKey=` that the
  body's *quoted-printable encoded* text breaks (the body contains
  `=3D` instead of `=`, and line-wrapping splits the URL).

The new trace will tell them exactly which condition rejected the message.

## How to verify
```powershell
.\run.ps1
email-read watch admin
# send a test email
# you should see one of:
#   rules: ✓ "name" → N url(s)   → opening in incognito: …   ✓ launched
#   rules: ✗ "name" → <specific reason>
#   ⚠ 0 enabled rules loaded …
```

## Quoted-printable note
If the trace shows `urlRegex matched 0 URLs in body (regex too strict?)`,
the cause is almost certainly that the email body is quoted-printable
encoded (`=3D` for `=`, `=\n` soft-wraps) and the regex is matching against
the encoded form. The mailclient should be decoding QP — if it isn't,
that's a separate bug in `internal/mailclient` to file.
