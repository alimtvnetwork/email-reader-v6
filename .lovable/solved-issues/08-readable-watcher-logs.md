# 08 — Watcher logs were ambiguous and heavy to read

**Status:** solved in v0.18.0
**Date:** 2026-04-22 (Asia/Kuala_Lumpur)

## Symptom
After v0.17 the watcher worked end-to-end (mail received → URL opened in
incognito) but the log was hard to parse: every line was prefixed with
`2026/04/22 20:17:00`, new-mail blocks ran together with no visual
separation, very long Lovable verify URLs blew the line width, and the
absolute Chrome path repeated on every startup.

User quote: "the logs are looking so bad it ambiguous & havy so make it
understandable nice".

## Fix (v0.18.0)
1. **Compact timestamps**: switched `log.New(..., log.LstdFlags)` → `log.New(..., 0)`
   in `internal/cli/cli.go` and `internal/cli/read.go`. Watcher now prepends
   its own `HH:MM:SS` (`ts()` helper) only on event lines, not on indented
   detail lines.
2. **Box-drawn startup banner** in `watcher.Run` with one labelled row per
   field (account / server / poll / rules / browser / mode). Replaces the
   previous 4-line wall.
3. **Event blocks** for new mail: blank line, `HH:MM:SS  ✉ [alias] new mail · uid=N`
   header, then indented `from / subject / saved / rules / → open / ✓ launched`
   lines. Easy to scan, easy to grep (`grep ✉` for arrivals, `grep ✗` for
   errors, `grep →` for opens).
4. **URL truncation** via `truncURL()` (>90 chars → `…`). Full URL is still
   recorded in SQLite + `.eml`; only the log line is shortened.
5. **Browser path shortening** via `shortPath()` — startup banner shows just
   `Google Chrome` instead of the full `/Applications/.../MacOS/Google Chrome`.
6. **Indented multi-line errors**: `errtrace.Format(err)` is split on `\n`
   and each frame prefixed with 8 spaces so the error chain visually belongs
   to its event block, not the next one.

## Files
- `internal/watcher/watcher.go` — full rewrite of the log surface
- `internal/cli/cli.go` — logger flag set to `0`
- `internal/cli/read.go` — same
- `cmd/email-read/main.go` — version → `0.18.0`

## Verify
```bash
.\run.ps1
email-read watch admin
```
Expected: a clean banner box, then silence until mail arrives, then a
6–8 line block per email with truncated URLs and an indented `✓ launched`.
