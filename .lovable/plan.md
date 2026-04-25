# Project plan — email-read

Single source of truth for the project roadmap. The original CLI build plan now lives at `spec/21-app/legacy/plan-cli.md` (archived; all 10 steps complete). The Fyne UI work is specified at `spec/21-app/02-features/` per the App Project Template.

## Active

### Immediate next action
- ⏳ Bump `Version` constant in `cmd/email-read/main.go` from `0.8.0` → `0.9.0` (required by project rule; missed during 2026-04-21 debugging session).

### Verification (user-side, in progress)
- ⏳ Rebuild via `.\run.ps1` (or `go build`) to pick up the new verbose poll logging.
- ⏳ Run `email-read watch ab` and confirm a per-poll log block appears every 3s with mailbox stats.
- ⏳ Send a test email from webmail (`https://webmail.attobondcleaning.store`) — guaranteed local delivery. Confirm `messages` count increments and `fetched 1 new message(s)` appears.
- ⏳ Confirm a test email triggers the rule engine and Chrome incognito opens the URL.
- ⏳ Run `email-read export-csv` and verify the file is written under `email-reader-cli/data/`.

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
| 11 | ✅ Verbose per-poll logging in watcher | `MailboxStats` struct + per-step `pollOnce` log lines (2026-04-21 debug session) |
