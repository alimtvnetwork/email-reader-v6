# 05 — New mail arrives but URL never opens in incognito (silent failure)

**Status:** solved in v0.15.0
**Severity:** High
**Area:** 03-rules, 05-watch
**Opened:** 2026-04-22
**Resolved:** 2026-04-22
**Spec links:** [../../02-features/03-rules/](../../02-features/03-rules/), [../../02-features/05-watch/](../../02-features/05-watch/), [../../06-error-registry.md](../../06-error-registry.md)
**Source:** `.lovable/solved-issues/05-rule-match-silent-no-browser-open.md`

---

## Symptom

User sent a test email containing a `https://lovable.dev/auth/action?...` verification URL. Watcher correctly logged:

```
[admin] ✉ 1 new message(s)
    uid=12  from=abdullah.mahin.rasia@gmail.com  subj="Re: Check"
```

…and then nothing. No browser launched, no error printed.

> **User quote:** "It works, but when it receives an email, it cannot open it in incognito mode and verify it. why its fail also log this"

## Root cause

Two compounding issues:

1. **Silent rule evaluation.** The watcher logged `matched 0 rule URL(s)` only when `--verbose` was on (v0.14 quiet-mode refactor — see issue 04). In quiet mode the rule-engine outcome was completely hidden, so users had no way to tell whether 0 rules existed, a rule's `SubjectRegex` rejected the message, or the URL pattern was too strict.
2. **No startup readiness banner.** The watcher never said "I have N rules loaded" or "browser resolved at /usr/bin/chrome" at startup, so the user couldn't tell the difference between "no rules configured" and "rules exist but didn't match this email".

## Fix (v0.15.0)

### `internal/rules/rules.go`

- New `EvaluateWithTrace(m) ([]Match, []RuleTrace)` returns a per-rule explanation: `fromRegex did not match…`, `urlRegex matched 0 URLs in body (regex too strict?)`, `no urlRegex configured`, etc.
- Existing `Evaluate(m)` kept as thin wrapper for backward compat.
- New `RuleCount()` so the watcher can warn at startup when 0 rules exist.

### `internal/watcher/watcher.go` — startup banner

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

### `internal/watcher/watcher.go` — per-mail rule trace (always logged)

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

This is information the user needs **every time mail arrives** — without it, the system looks broken even when working as configured. Per-poll noise (dialing, watch-state-load) stays gated behind `--verbose`; per-mail evaluation does not.

## Spec encoding

- `spec/21-app/02-features/03-rules/01-backend.md` §`EvaluateWithTrace` — formalises the trace contract and the `RuleTrace` struct.
- `spec/21-app/02-features/05-watch/01-backend.md` §Startup banner / §Per-mail trace — both are listed as **always-emitted** events (not gated by verbosity).
- `spec/21-app/02-features/05-watch/02-frontend.md` §Rule trace card — the GUI live-log surfaces each `RuleTrace` row inline with the email card so users see ✓/✗ next to the rule name.
- `spec/21-app/05-logging-strategy.md` §Always-emitted events — adds rule-trace and startup-readiness to the always-on list.

## Why this happened (process learning)

The fix for issue 04 (noise reduction) silenced rule-evaluation output along with the per-poll diagnostics, because the watcher had no notion of *event severity* — only "verbose" or "not". The classification triplet introduced here (always / quiet-only / verbose-only) is what made it possible to fix this without re-introducing the noise of issue 04.

## What NOT to repeat

- **Do not** silence rule-engine outcomes in quiet mode. Per-mail decisions are events, not noise.
- **Do not** evaluate rules without a startup readiness check (rule count > 0, browser resolvable). Surface those at startup, not at first-mail-arrival.

## Most likely cause for THIS user's case

Their `data/config.json` either:

- had zero rules with `Enabled: true`, OR
- had a `FromRegex` like `^.*@lovable\.dev$` that rejected the gmail sender, OR
- had a `UrlRegex` like `https://lovable\.dev/auth/.*&apiKey=` that the body's *quoted-printable encoded* text breaks (the body contains `=3D` instead of `=`, and line-wrapping splits the URL).

The new trace tells them exactly which condition rejected the message.
