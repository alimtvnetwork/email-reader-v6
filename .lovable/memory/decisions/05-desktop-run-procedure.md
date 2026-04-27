# Desktop-run procedure for `email-read`

Documented 2026-04-27 in response to "can I run this on my desktop?" question.

## Summary

`email-read` is a Go application with two binaries:
- `cmd/email-read/` — CLI (Cobra commands: `add`, `list`, `watch`, `read`, `export-csv`, `doctor`).
- `cmd/email-read-ui/` — Fyne desktop app (sidebar + dashboard + emails + rules + accounts + watch + tools + settings).

Both share `internal/core/*` services and `internal/store` (SQLite via modernc.org/sqlite — pure Go, no cgo for the CLI; the Fyne UI does need cgo + GL).

## Prerequisites

| Tool | Version | URL |
|---|---|---|
| Go | 1.22+ | https://go.dev/dl/ |
| Git | any | https://git-scm.com/downloads |
| Google Chrome | any modern | system default; needed by `internal/browser` for incognito launch |
| **macOS / Linux only:** Xcode CLI tools / X11+GL headers | — | needed for the Fyne UI binary (cgo) |

## Procedure (Windows)

```powershell
# 1. Clone and bootstrap
git clone <repo-url>
cd email-read

# 2. Run the bootstrap script (git pull → go build → ensure dirs → PATH add)
.\run.ps1

# 3. Add an IMAP account (interactive prompts)
email-read add

# 4. Sanity-check the account
email-read doctor <alias>

# 5a. CLI watcher (verbose per-poll logging)
email-read watch <alias>

# 5b. OR launch the desktop UI
.\cmd\email-read-ui\email-read-ui.exe
```

## Procedure (macOS / Linux)

```bash
# 1. Clone and build the CLI (no cgo required for CLI)
git clone <repo-url>
cd email-read
go build -tags nofyne ./cmd/email-read

# 2. Build the UI (requires cgo + GL — nofyne tag NOT applied)
go build ./cmd/email-read-ui

# 3. Add account, then watch
./email-read add
./email-read doctor <alias>
./email-read watch <alias>
# OR
./email-read-ui
```

## Verification checklist (smoke test the user must run)

After launching the UI binary:

- [ ] Settings view renders, theme toggle (light/dark) propagates within one tick.
- [ ] Density toggle (compact/normal/comfortable) re-flows the sidebar.
- [ ] All 4 maintenance knob rows visible: `OpenUrlsRetentionDays`, `WeeklyVacuumOn`/`HourLocal`, `WalCheckpointHours`, `PruneBatchSize`.
- [ ] Watch tab Start/Stop works; live event log shows the 10 EventKind translations.
- [ ] Dashboard 5-tile counter row increments on `EventNewMail`.
- [ ] Recent opens tab shows rows from `OpenedUrls` with correct PascalCase columns.
- [ ] Retention-days field round-trips through Save (re-open Settings, value persists).
- [ ] All four canonical maintenance log lines appear at their cadences (`event=prune`, `event=analyze`, `event=wal_checkpoint`, `event=vacuum`).
- [ ] Flaky-network simulation: disconnect briefly; `⏳ [alias] backing off after N consecutive error(s): next poll in …` line appears.

## Why this matters for AC coverage

The "App boot smoke test" is the single deferred user-side verification
that closes several behaviour-but-not-AST acceptance rows. Until the
user runs the binary on real hardware, those rows stay in the
`coverageGapAllowlist` even if the underlying code is shipped.

## Sandbox limitation

The Lovable sandbox cannot run the Fyne UI (no display server). Use
`-tags nofyne` for any sandbox `go test` / `go vet` invocation; the
production-code paths under `internal/ui/` are still type-checked, just
not exercised under a real canvas. AC rows requiring canvas inspection
are deferred to **Slice #118e — Fyne canvas harness**.
