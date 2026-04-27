# Project plan — email-read

Single source of truth for the project roadmap. The original CLI build plan now lives at `spec/21-app/legacy/plan-cli.md` (archived; all 10 steps complete). The Fyne UI work is specified at `spec/21-app/02-features/` per the App Project Template.

For per-slice implementation history and the canonical "% done" signal,
see `.lovable/memory/workflow/01-status.md` and `mem://workflow/progress-tracker`.

## Active

### Phase 3 — AC coverage rollout (current)

The original roadmap is complete. We are now closing the residual
allowlist of acceptance-criteria rows that don't yet have a citing test.
The allowlist shrinks monotonically (enforced by `Test_AC_CoverageAudit`).

- ⏳ **Slice #132 — AC-DS AST gaps (~4 rows)** — next slice; AST scanners for widget-usage limits, animation duration ceilings, hard-coded color guard.
- ⏳ **AC-DS long-tail headless (~18 rows)** — remaining headless AC-DS rows after #132.
- ⏳ **Spec cleanup → AC-PROJ-31/33** — grow `06-error-registry.md` (~39 missing ER codes); fix ~33 broken cross-tree spec links.
- ⏳ **AC-PROJ-35** — flip OI-1..OI-6 in `spec/21-app/02-features/06-tools/99-consistency-report.md` to ✅ Closed.

### Deferred (require infra not in this sandbox)

- 🚫 **Slice #118e — Fyne canvas harness (~50 rows)** — needs cgo/X11/GL workstation. Unblocks AC-SF (21), AC-DS canvas (~22), AC-SX-06 frontend (1).
- 🚫 **Bucket #9 — Bench/race/goleak infra (~11 rows)** — unblocks AC-DBP (6) + AC-SP (5).
- 🚫 **AC-DB schema-evolution (~12 rows)** — table rename, enum CHECKs, FK SET NULL, gap/checksum/downgrade detection.
- 🚫 **AC-PROJ E2E harness (~13 rows)** — multi-process integration scenarios.

### Verification (user-side, in progress)

- ⏳ Bump `Version` constant in `cmd/email-read/main.go` from `0.8.0` → `0.9.0` (carry-over from 2026-04-21 debugging).
- ⏳ Rebuild via `.\run.ps1` (or `go build`) to pick up verbose poll logging.
- ⏳ Run `email-read watch ab` and confirm per-poll log block appears every 3s.
- ⏳ Send test email; confirm `messages` count increments and `fetched 1 new message(s)` appears.
- ⏳ Confirm test email triggers rule engine and Chrome incognito opens URL.
- ⏳ Run `email-read export-csv` and verify file under `email-reader-cli/data/`.
- ⏳ **App boot smoke test** — full desktop run validating Settings/Watch/Dashboard/Recent-opens/maintenance-log lines/backoff line. (Procedure documented in `.lovable/memory/decisions/05-desktop-run-procedure.md`.)

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
| 11 | ✅ Verbose per-poll logging in watcher | `MailboxStats` struct + per-step `pollOnce` log lines (2026-04-21) |
| 12 | ✅ **Phase 1+2 — Fyne UI implementation** (Slices #1-#41) | All 7 features: dashboard, emails, rules, accounts, watch, tools, settings |
| 13 | ✅ **Maintenance jobs** (Slices #22-#33) | Daily retention prune, ANALYZE-after-N-deletes, weekly VACUUM, 6-hourly WAL checkpoint, settings UI knobs, `event=…` slog lines |
| 14 | ✅ **AST guard infrastructure** (Slices #34-#41) | AC-DB-47/50/51/52/53/54/55 — maintenance, driver-import, sql-type-leak, core-uses-store-only, RFC3339 datetime, boolean-positive, log-scan |
| 15 | ✅ **AC coverage audit infra** (Slices #117-#127) | `internal/specaudit/coverage_audit_test.go` with monotonic allowlist + stale-ref guard |
| 16 | ✅ **AC-SB family 100%** (Slices #126-#128) | All 24 settings-backend rows covered |
| 17 | ✅ **AC-SX family 100% headless** (Slice #130) | 5 AST/log scanners + 2 production redactor fixes for `IncognitoArg` leaks |
| 18 | ✅ **AC-PROJ family 70%** (Slice #131) | Spec linters: fyne-import scope, query-ref resolution, feature-folder shape; 2 deferred-skip scanners wired |
