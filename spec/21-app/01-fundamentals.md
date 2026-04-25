# 01 — Fundamentals

**Version:** 1.0.0
**Updated:** 2026-04-25

---

## 1. Product summary

`email-read` is a single Go codebase that ships **two entrypoints** sharing one library:

| Binary | Path | Audience | Build tag |
|---|---|---|---|
| `email-read` | `cmd/email-read/main.go` | Headless / scripting / CI | none |
| `email-read-ui` | `cmd/email-read-ui/main.go` | Desktop user (Fyne) | `!nofyne` |

Both call into `internal/core/*` (framework-agnostic operations). No business logic lives in `cmd/` or in Fyne views.

---

## 2. Layered architecture

```
┌─────────────────────────────────────────────────────────┐
│ cmd/email-read (Cobra CLI)   │  cmd/email-read-ui (Fyne)│
├─────────────────────────────────────────────────────────┤
│            internal/core/* (pure operations)             │
│   accounts · rules · emails · watch · read · export      │
│   diagnose · dashboard                                   │
├─────────────────────────────────────────────────────────┤
│ internal/config │ internal/store │ internal/mailclient   │
│ internal/imapdef│ internal/rules │ internal/browser      │
│ internal/watcher│ internal/exporter│ internal/errtrace   │
└─────────────────────────────────────────────────────────┘
```

### Rules

1. **`internal/core` is the only API surface** that `cmd/*` and Fyne views may call. Lower packages are private to core.
2. **No Fyne imports outside `internal/ui/` or `cmd/email-read-ui`.** Pure formatters live in `internal/ui/views/*_format.go` so they can be unit-tested without cgo.
3. **No CGO.** SQLite stays on `modernc.org/sqlite`. Hard-rejected by `.lovable/strictly-avoid.md`.
4. **All errors** go through `internal/errtrace` for `file:line` stack traces.
5. **Single-window UI.** Multi-window deferred (see `03-issues/`).

---

## 3. Data flow (single watch cycle)

1. `core.Watch(ctx, alias)` opens an IMAP connection (`internal/mailclient`).
2. Polls every `config.watch.pollSeconds` (default `3`).
3. New UIDs → fetch RFC822 → parse → save raw `.eml` to `email/<alias>/<YYYY-MM-DD>/` → upsert into `Emails` table.
4. Each new email is published as a `core.Event` on a fan-out channel. CLI subscribers print readable log lines; UI subscribers render structured cards + a raw log tab simultaneously.
5. Rules engine (`internal/rules`) evaluates the email; matching URLs flow through `internal/browser` → Chrome incognito → recorded in `OpenedUrls` (dedup ledger).
6. `WatchState.LastUid` updated. Repeat.

> Heartbeat invariant: every poll emits at least one log line including `messages=N uidNext=M` even when nothing changed. See `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`.

---

## 4. Configuration

Single file: `data/config.json` (next to the binary). Schema and field semantics are unchanged from `legacy/spec.md` §5 — they remain authoritative for the JSON shape until a separate `04-config-schema.md` is written.

Key invariants:
- Passwords stored Base64 (acknowledged: encoding, not encryption).
- `chromePath`/`incognitoArg` may be empty → auto-detect per OS (see `legacy/spec.md` §6).
- The Fyne **Settings** view is the only UI surface allowed to write `config.json` outside the existing CLI commands.

---

## 5. SQLite schema

Tables (`Emails`, `WatchState`, `OpenedUrls`) and PascalCase column conventions are unchanged from `legacy/spec.md` §7. They are reproduced in [`../23-app-database/00-overview.md`](../23-app-database/00-overview.md) (or will be — currently a placeholder).

---

## 6. UI fundamentals (Fyne)

| Concern | Decision |
|---|---|
| Framework | Fyne v2 (pure Go, single binary, cross-platform) |
| Layout | Sidebar (left) + detail pane (right), single window |
| Forms | **Inline in the detail pane** — never modal popups |
| Theme | Fyne dark by default; accent matches CLI banner; monospace font for log panels |
| State persistence | `data/ui-state.json` (last alias, sidebar position, last view) |
| Keyboard shortcuts | ⌘1–7 sidebar nav, ⌘N new fetch, ⌘. stop watch |
| System tray | Out of scope v1 |

Full visual standards live in [`../24-app-design-system-and-ui/00-overview.md`](../24-app-design-system-and-ui/00-overview.md) (placeholder until written).

---

## 7. Build & ship

| Target | Command |
|---|---|
| CLI dev build | `go build -o ./bin/email-read ./cmd/email-read` |
| UI dev build | `go build -o ./bin/email-read-ui ./cmd/email-read-ui` |
| UI package (per OS) | `fyne package -os {darwin\|windows\|linux} -src ./cmd/email-read-ui` |
| Headless test | `go test -tags nofyne ./...` |
| Windows bootstrap | `run.ps1` (existing, unchanged) |

`Version` constant in `cmd/email-read/main.go` and `cmd/email-read-ui/main.go` MUST stay in lockstep. Bump rule: every code change is at least a minor bump (per `.lovable/strictly-avoid.md`).

---

## 8. Out of scope (v1 of the UI)

- Sending email.
- OAuth flows (Gmail/Outlook still require app passwords).
- Web/remote UI.
- Multi-window UI.
- Removing or hiding the CLI — both binaries ship together.

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Feature Index | [02-features/00-overview.md](./02-features/00-overview.md) |
| Issue Index | [03-issues/00-overview.md](./03-issues/00-overview.md) |
| Legacy CLI spec | [legacy/spec.md](./legacy/spec.md) |
| Legacy Fyne plan | [legacy/plan-fyne-ui.md](./legacy/plan-fyne-ui.md) |
