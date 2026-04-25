# Implementation Plan — `email-read`

Each step is one atomic chunk. After finishing a step I stop and wait for **"next"**.

| # | Step | Deliverable | Status |
|---|---|---|---|
| 1 | Scaffold Go module + repo layout | `go.mod`, `cmd/email-read/main.go` printing version, folder skeleton under `internal/`, `.gitignore` for `email-reader-cli/` build output | ✅ done |
| 2 | Config layer | `internal/config` — load/save `data/config.json`, Base64 password helpers, account + rule + watch structs | ✅ done |
| 3 | IMAP defaults + `add` / `list` / `remove` commands | `internal/imapdef` lookup table, Cobra commands wired with survey prompts (interactive `add`) | ✅ done |
| 4 | SQLite store + migrations | `internal/store` — open DB at `data/emails.db`, create `Emails`, `WatchState`, `OpenedUrls` (PascalCase), upsert helpers | ✅ done |
| 5 | IMAP mail client | `internal/mailclient` — connect, fetch new UIDs, parse to struct, save raw `.eml` to `email/<alias>/<date>/` | ✅ done |
| 6 | Rules engine + Chrome launcher | `internal/rules` regex evaluation; `internal/browser` Chrome incognito spawn with dedup via `OpenedUrls` | ✅ done |
| 7 | Watch loop + `email-read <alias>` | `internal/watcher` polling every N sec, graceful Ctrl+C shutdown, default-to-first-alias when no arg | ✅ done |
| 8 | `rules list/enable/disable` + `export-csv` | `internal/exporter` writing `./data/export-<ts>.csv` from CWD | ✅ done |
| 9 | `run.ps1` bootstrap | `git pull` → `go build` to `email-reader-cli/email-read.exe` → ensure `data/` + `email/` → add to user PATH (idempotent) | ✅ done |
| 10 | README | Install (run `run.ps1`), Gmail/Outlook app-password note, command reference, sample rules JSON | ✅ done |

## Open notes
- Build is Windows-targeted; `modernc.org/sqlite` keeps it CGO-free so `go build` works on any host.
- I will not run `go build` from the sandbox (no Go toolchain assumed); the user runs `run.ps1` locally.
- After each step I'll list what's done and what remains, then wait for **"next"**.
