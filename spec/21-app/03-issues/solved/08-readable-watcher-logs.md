# 08 — Watcher logs were ambiguous and visually heavy

**Status:** solved in v0.18.0
**Severity:** Low
**Area:** 05-watch, 24-design-system
**Opened:** 2026-04-22
**Resolved:** 2026-04-22
**Spec links:** [../../02-features/05-watch/](../../02-features/05-watch/), [../../05-logging-strategy.md](../../05-logging-strategy.md), [../../../24-app-design-system-and-ui/](../../../24-app-design-system-and-ui/)
**Source:** `.lovable/solved-issues/08-readable-watcher-logs.md`

---

## Symptom

After v0.17 the watcher worked end-to-end (mail received → URL opened in incognito) but the log was hard to parse:

- every line was prefixed with `2026/04/22 20:17:00` (Go's default log timestamp)
- new-mail blocks ran together with no visual separation
- very long Lovable verify URLs blew the line width
- the absolute Chrome path repeated on every startup banner

> **User quote:** "the logs are looking so bad it ambiguous & havy so make it understandable nice".

## Root cause

The CLI's `log.Logger` was created with `log.LstdFlags`, prepending a date+time on every line. Combined with the watcher's own `HH:MM:SS` prefix on event lines (added in v0.14), every event line had two timestamps. Indented detail lines also got the date prefix, making it visually impossible to tell a child line from a sibling event. URLs were never truncated; banners always printed absolute paths.

This is the same conceptual problem as issue 04 (noise) but at the *visual* layer rather than the *event-rate* layer.

## Fix (v0.18.0)

### 1. Compact timestamps

`internal/cli/cli.go` and `internal/cli/read.go`: switched `log.New(..., log.LstdFlags)` → `log.New(..., 0)`. The watcher now prepends its own `HH:MM:SS` (`ts()` helper) only on event lines, not on indented detail lines.

### 2. Box-drawn startup banner

`watcher.Run` writes one labelled row per field (account / server / poll / rules / browser / mode). Replaces the previous 4-line wall.

### 3. Event blocks for new mail

For every new message:

- a blank line
- `HH:MM:SS  ✉ [alias] new mail · uid=N` header
- indented `from / subject / saved / rules / → open / ✓ launched` lines

Easy to scan; easy to grep:

- `grep ✉` for arrivals
- `grep ✗` for errors
- `grep →` for opens

### 4. URL truncation

`truncURL()` shortens any URL > 90 chars to `…`. Full URL is still recorded in SQLite + `.eml`; only the log line is shortened.

### 5. Browser path shortening

`shortPath()` — startup banner shows just `Google Chrome` instead of the full `/Applications/.../MacOS/Google Chrome`.

### 6. Indented multi-line errors

`errtrace.Format(err)` is split on `\n` and each frame prefixed with 8 spaces so the error chain visually belongs to its event block, not the next one.

### Files

- `internal/watcher/watcher.go` — full rewrite of the log surface
- `internal/cli/cli.go` — logger flag set to `0`
- `internal/cli/read.go` — same
- `cmd/email-read/main.go` — version → `0.18.0`

## Spec encoding

- `spec/21-app/05-logging-strategy.md` §Visual format — formalises the rules: single `HH:MM:SS` per event line; indented children carry no timestamp; URL > 90 → truncate with `…`; long paths shortened to basename in banners; `errtrace.Format` lines indented 8 spaces.
- `spec/21-app/02-features/05-watch/02-frontend.md` §Live log card — the GUI inherits the same visual rules: `ColorWatchDot{Ok|Warn|Err}` design tokens map 1:1 to the CLI's `✓` / `⚠` / `✗` glyphs, and the URL truncation helper is shared between CLI and GUI.
- `spec/24-app-design-system-and-ui/04-components.md` §Live-log row — the dot-glyph-line tri-component is a first-class design-system widget. **Closes Watch OI-1** (the `ColorWatchDot*` tokens referenced from the Watch frontend now exist in the design-system spec).

## What NOT to repeat

- **Do not** use Go's `log.LstdFlags` for user-facing CLI output. It double-stamps and clutters. Always use `log.New(out, "", 0)` and let the application format its own timestamps.
- **Do not** print absolute filesystem paths in user-facing banners; show basenames or shortened forms. Full paths belong in error traces and verbose mode.
- **Do not** print full URLs in event logs when they can exceed terminal width. Truncate to ~90 chars in display; keep the canonical form in storage.
- **Do** ensure indented detail lines carry no timestamp prefix — visual hierarchy depends on this.

## Cross-references

- **Builds on:** issue 04 (noise reduction) and issue 05 (always-on per-mail trace).
- **Closes:** Watch feature open issue **OI-1** (`ColorWatchDot*` design tokens).
