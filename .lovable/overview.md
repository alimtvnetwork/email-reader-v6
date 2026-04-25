# email-read — project overview

A Windows-first Go CLI that watches IMAP inboxes, persists every email to SQLite + disk, and auto-opens URLs from matching emails in Chrome incognito based on regex rules.

## Stack
- **Language:** Go 1.22+ (CGO-free build via `modernc.org/sqlite`)
- **CLI framework:** Cobra + Survey (interactive prompts)
- **Storage:** SQLite (`data/emails.db`) + raw `.eml` files on disk (`email/<alias>/<date>/`)
- **Config:** JSON at `data/config.json` with Base64-encoded passwords
- **Bootstrap:** `run.ps1` (PowerShell) — git pull → go build → ensure dirs → idempotent PATH add
- **Target OS:** Windows (primary). Build is host-agnostic thanks to pure-Go SQLite.

## Repo layout
```
cmd/email-read/main.go        # entrypoint, holds Version constant
internal/
  cli/                        # Cobra commands, rules export
  config/                     # config.json load/save, Base64 helpers
  imapdef/                    # built-in IMAP server defaults table
  store/                      # SQLite open + migrations + upserts
  mailclient/                 # IMAP fetch + parse + .eml writer
  rules/                      # regex evaluation
  browser/                    # Chrome incognito launcher + dedup
  watcher/                    # polling loop + Ctrl+C handling
  exporter/                   # CSV export of Emails table
spec/21-app/                  # canonical app spec (App Project Template)
                              #   ├── 00-overview.md, 01-fundamentals.md
                              #   ├── 02-features/   (dashboard, emails, rules, accounts, watch, tools, settings)
                              #   ├── 03-issues/
                              #   └── legacy/        (archived original CLI spec + Fyne plan)
run.ps1                       # one-command Windows bootstrap
README.md                     # install, command reference, sample rules
```

## Current version
`0.8.0` — defined in `cmd/email-read/main.go`.

## Status
All 10 plan steps complete. Ready for end-user run on Windows.

## Note on the React/Vite frontend
The `src/` React app is the default Lovable scaffold and is **not** used by this project. The product is the Go CLI only. Do not invest in the frontend unless explicitly asked.
