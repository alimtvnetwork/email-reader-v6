# Architecture decisions

## Language & runtime
- **Go 1.22+.** Single static binary, easy distribution to Windows users, strong stdlib for IMAP and CSV.
- **CGO disabled.** Build must work on any host (including the sandbox) without a C compiler.

## SQLite driver
- **`modernc.org/sqlite`** (pure-Go transpilation of SQLite). Trade slightly slower performance for zero-CGO builds.
- Alternative `mattn/go-sqlite3` was rejected because it requires CGO.

## CLI framework
- **Cobra** for command tree (`add`, `list`, `remove`, `rules`, `export-csv`, default `<alias>` watch).
- **Survey** for interactive prompts on `add` (host autodiscovery via `internal/imapdef`).

## Storage layout
- `data/config.json` — all account + rule + watch settings. One file, hand-editable.
- `data/emails.db` — SQLite. Tables: `Emails`, `WatchState`, `OpenedUrls` (PascalCase, intentional, matches spec).
- `email/<alias>/<YYYY-MM-DD>/<uid>.eml` — raw archive on disk for forensic / replay purposes.

## Rule engine
- Regex match against subject + body. On match, extract URLs, dedupe via `OpenedUrls` table (so a re-fetched email doesn't reopen the same link), then spawn Chrome with `--incognito --new-window <url>`.

## Watcher
- Polling (default 3s) instead of IMAP IDLE — simpler, works behind every NAT / proxy, fine for the scale the user runs at.
- Stores last-seen UID per account in `WatchState` so restarts don't reprocess history.
- Default alias = first account in `config.json` when `email-read` is invoked with no argument.

## What was rejected
- IMAP IDLE — unnecessary complexity for a personal-scale tool.
- Embedding the React UI as a control panel — out of scope; CLI is the product.
