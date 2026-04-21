# Project Memory — index

## Core
- Product is a **Go CLI for Windows**, not a web app. The React `src/` scaffold is unused.
- Version lives in `cmd/email-read/main.go` `Version` constant. Bump **at least minor** per code change.
- SQLite is pure-Go (`modernc.org/sqlite`) — never reintroduce CGO.
- Passwords in `data/config.json` are **Base64-encoded** via `internal/config` helpers, never plaintext.
- Build is **never run from sandbox**. User runs `.\run.ps1` on Windows.
- All `.md` filenames are lowercase-hyphenated with a numeric prefix (e.g. `01-foo.md`).
- Plans and suggestions each live in **one file** — never fragment.
- Never touch `.release/` or read-only lockfiles.
- User timezone: Malaysia (UTC+8). Always list remaining tasks at end of each session.

## Memories
- [Workflow status](mem://memory/workflow/01-status.md) — Phase progress and current milestone marker
- [Architecture decisions](mem://memory/decisions/01-architecture.md) — Stack choices and rationale (CGO-free SQLite, Cobra, etc.)
- [Build & deploy](mem://memory/decisions/02-build-and-deploy.md) — `run.ps1` design, PATH handling, version bump policy
- [Session log 2026-04-21](mem://memory/sessions/01-2026-04-21.md) — What was done in this session
