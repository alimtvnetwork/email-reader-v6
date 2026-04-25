# Issue Index — 21-app

**Version:** 2.0.0
**Updated:** 2026-04-25
**Status:** All historical issues from `.lovable/solved-issues/` migrated.

---

## Overview

Tracked issues, bugs, and investigations specific to the `email-read` app (CLI + Fyne UI). Cross-cutting issues (spec hygiene, coding standards) belong in `../../22-app-issues/` instead.

Each issue is a single Markdown file at `solved/{NN}-{kebab-name}.md` or `pending/{NN}-{kebab-name}.md`. Frontmatter is plain Markdown (no YAML), with bold key-value pairs near the top of the file.

---

## Solved Issues

| # | Issue | Severity | Resolved | Area | Spec links |
|---|-------|----------|----------|------|------------|
| 01 | [IMAP `AUTHENTICATIONFAILED` — wrong password](./solved/01-imap-auth-failed-wrong-password.md) | Medium | v0.10.0 | Accounts / Watch | [04-accounts](../02-features/04-accounts/), [05-watch](../02-features/05-watch/) |
| 02 | [Watcher silent on healthy idle](./solved/02-watcher-silent-on-healthy-idle.md) | High | v0.11.0 | Watch / Logging | [05-watch](../02-features/05-watch/), [05-logging-strategy](../05-logging-strategy.md) |
| 03 | [IMAP auth failed — hidden Unicode in password](./solved/03-imap-auth-failed-hidden-unicode.md) | High | v0.13.0 | Accounts / Tools | [04-accounts](../02-features/04-accounts/), [06-tools](../02-features/06-tools/) |
| 04 | [Watcher log too noisy to read](./solved/04-noisy-watcher-log-output.md) | Medium | v0.14.0 | Watch / Logging | [05-watch](../02-features/05-watch/), [05-logging-strategy](../05-logging-strategy.md) |
| 05 | [Rule match silent — no browser open, no reason](./solved/05-rule-match-silent-no-browser-open.md) | High | v0.15.0 | Rules / Watch | [03-rules](../02-features/03-rules/), [05-watch](../02-features/05-watch/) |
| 06 | [Confusing `.eml` filenames + URL re-open required `watch`](./solved/06-readable-filenames-and-read-command.md) | Medium | v0.16.0 | Emails / Watch | [02-emails](../02-features/02-emails/), [05-watch](../02-features/05-watch/) |
| 07 | [Zero rules blocked incognito open even with browser ready](./solved/07-zero-rules-default-seed.md) | High | v0.17.0 | Rules | [03-rules](../02-features/03-rules/) |
| 08 | [Watcher logs ambiguous and visually heavy](./solved/08-readable-watcher-logs.md) | Low | v0.18.0 | Watch / Design | [05-watch](../02-features/05-watch/), [24-app-design-system-and-ui](../../24-app-design-system-and-ui/) |

**Severity legend:** High = silently breaks user-visible flow · Medium = correct behavior but bad UX · Low = polish / readability.

---

## Pending Issues

_None._

The `pending/` directory is intentionally empty. When a new issue is filed, create `pending/{NN}-{kebab-name}.md` and add a row here. When resolved, move the file to `solved/`, update its **Status** line, and re-link from this index.

---

## When to add an issue here

- Reproducible bug in either binary (CLI or GUI).
- Regression after a feature change.
- A design question that surfaced during implementation and needs a written decision.
- A workflow / DX problem the user explicitly called out (e.g. "this is confusing").

For solved issues that informed hard rules, also add a one-liner to `.lovable/strictly-avoid.md`.

---

## File template

```md
# {NN} — {short title}

**Status:** solved in v{X.Y.Z} · or · pending
**Severity:** High | Medium | Low
**Area:** {feature-folder-slug(s)}
**Opened:** YYYY-MM-DD
**Resolved:** YYYY-MM-DD (omit if pending)
**Spec links:** ../../02-features/{slug}/, ../../{spec-folder}/

## Symptom
…

## Root cause
…

## Fix
…

## What NOT to repeat
…
```

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Feature Index | [../02-features/00-overview.md](../02-features/00-overview.md) |
| App Issues (root-level) | [../../22-app-issues/00-overview.md](../../22-app-issues/00-overview.md) |
| Logging Strategy | [../05-logging-strategy.md](../05-logging-strategy.md) |
| Error Registry | [../06-error-registry.md](../06-error-registry.md) |
| Solved-issues archive (raw) | [../../../.lovable/solved-issues/](../../../.lovable/solved-issues/) |
