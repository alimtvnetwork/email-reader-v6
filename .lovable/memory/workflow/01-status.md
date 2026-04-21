# Workflow status

Last updated: 2026-04-21 (UTC+8) — debugging session

## Current milestone
🎯 **Verbose per-poll logging shipped (will be v0.9.0 once Version bumped).** User-side end-to-end test still pending.

## Phases

| Phase | Status |
|---|---|
| 1. Scaffold Go module + repo layout | ✅ Done |
| 2. Config layer (Base64 passwords) | ✅ Done |
| 3. IMAP defaults + add/list/remove | ✅ Done |
| 4. SQLite store + migrations | ✅ Done |
| 5. IMAP mail client + .eml archive | ✅ Done |
| 6. Rules engine + Chrome launcher | ✅ Done |
| 7. Watch loop + default alias | ✅ Done |
| 8. rules list/enable/disable + export-csv | ✅ Done |
| 9. run.ps1 bootstrap | ✅ Done |
| 10. README | ✅ Done |
| 11. Verbose per-poll logging in watcher | ✅ Done (this session) |
| 12. Bump Version → 0.9.0 in cmd/email-read/main.go | ⏳ Pending — flagged for next session |
| 13. Local end-to-end verification | 🔄 In progress (user-side, blocked on rebuild) |
| 14. Credential rotation post-verification | ⏳ Pending (see suggestions) |

## Next logical step for the next AI session
1. Bump `Version` constant in `cmd/email-read/main.go` from `0.8.0` → `0.9.0`.
2. Ask the user to rerun `.\run.ps1` then `email-read watch ab` and paste the new (verbose) logs.
3. From the new logs, determine whether mail is reaching the mailbox or not (look at `mailbox "INBOX" stats: messages=N`). If `messages` never increments after sending a test email, the issue is mail delivery, not the watcher.
4. If verification passes → move credential rotation to in-progress.
