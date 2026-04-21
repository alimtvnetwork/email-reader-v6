# Project Memory — index

## Core
- Product is a **Go CLI for Windows**, not a web app. The React `src/` scaffold is unused.
- Version lives in `cmd/email-read/main.go` `Version` constant. Bump **at least minor** per code change.
- SQLite is pure-Go (`modernc.org/sqlite`) — never reintroduce CGO.
- Passwords in `data/config.json` are **Base64-encoded** via `internal/config` helpers, never plaintext.
- Build is **never run from sandbox**. User runs `.\run.ps1` on Windows (or PowerShell Core on macOS).
- All `.md` filenames are lowercase-hyphenated with a numeric prefix (e.g. `01-foo.md`).
- Plans and suggestions each live in **one file** — never fragment.
- Never touch `.release/` or read-only lockfiles.
- User timezone: Malaysia (UTC+8). Always list remaining tasks at end of each session.
- Watch loops MUST emit a per-poll heartbeat log line — silence on success is forbidden.
- IMAP `AUTHENTICATIONFAILED` is always a wrong-password input issue, never the Base64 layer.

## Memories
- [Workflow status](mem://memory/workflow/01-status.md) — Phase progress and current milestone marker
- [Architecture decisions](mem://memory/decisions/01-architecture.md) — Stack choices and rationale (CGO-free SQLite, Cobra, etc.)
- [Build & deploy](mem://memory/decisions/02-build-and-deploy.md) — `run.ps1` design, PATH handling, version bump policy
- [Testing guide](mem://memory/testing-guide.md) — Step-by-step end-to-end verification on macOS/Windows
- [Session log 2026-04-21](mem://memory/sessions/01-2026-04-21.md) — Initial build session: steps 9 & 10 complete
- [Session log 2026-04-21 debugging](mem://memory/sessions/02-2026-04-21-debugging.md) — Auth troubleshooting + verbose poll logging
