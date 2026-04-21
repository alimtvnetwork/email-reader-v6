# Project plan — email-read

Single source of truth for the project roadmap. The detailed step-by-step build plan also lives in `spec/21-golang-email-reader/plan.md` (kept in sync).

## Active

_All initial build tasks complete. Awaiting user verification on Windows._

### Verification (user-side, awaiting)
- ⏳ Run `.\run.ps1` locally, confirm build succeeds and `email-read.exe` lands in `email-reader-cli/`.
- ⏳ Reopen terminal and confirm `email-read` is on PATH.
- ⏳ Run `email-read add` to seed an account, then `email-read` to start the watch loop.
- ⏳ Confirm a test email triggers the rule engine and Chrome incognito opens the URL.
- ⏳ Run `email-read export-csv` and verify the file is written to CWD `./data/`.

## Completed

| # | Step | Deliverable |
|---|---|---|
| 1 | ✅ Scaffold Go module + repo layout | `go.mod`, `cmd/email-read/main.go`, `internal/` skeleton |
| 2 | ✅ Config layer | `internal/config` — load/save, Base64 password helpers |
| 3 | ✅ IMAP defaults + `add`/`list`/`remove` | `internal/imapdef` + Cobra commands w/ Survey prompts |
| 4 | ✅ SQLite store + migrations | `internal/store` — `Emails`, `WatchState`, `OpenedUrls` tables |
| 5 | ✅ IMAP mail client | `internal/mailclient` — connect, fetch new UIDs, write `.eml` |
| 6 | ✅ Rules engine + Chrome launcher | `internal/rules` regex + `internal/browser` Chrome incognito w/ dedup |
| 7 | ✅ Watch loop + `email-read <alias>` | `internal/watcher` polling, Ctrl+C, default first alias |
| 8 | ✅ `rules list/enable/disable` + `export-csv` | `internal/exporter` writes `./data/export-<ts>.csv` |
| 9 | ✅ `run.ps1` bootstrap | git pull → go build → ensure dirs → idempotent user PATH add |
| 10 | ✅ README | Windows install via `run.ps1`, app-password notes, command reference, sample rules JSON |
