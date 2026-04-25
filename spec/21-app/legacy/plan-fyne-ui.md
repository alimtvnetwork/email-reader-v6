# email-read — Fyne Desktop UI plan

Date: 2026-04-22 (Asia/Kuala_Lumpur)
Target version after completion: 0.30.0
Framework: **Fyne v2** (pure Go, single binary, cross-platform)

---

## Goals

Build a Fyne desktop UI on top of the existing Go CLI without breaking the CLI.
- **Sidebar + detail** layout: left = navigation + account picker, right = active view.
- All CLI commands surfaced as **buttons that open inline forms** (not modals).
- Email viewer: subject, from, date, decoded body, extracted links (clickable → incognito).
- Live watcher: **two tabs** — structured event cards + raw log stream.

---

## Architecture (one-time refactor)

The CLI logic currently lives partly in `internal/cli/*`. To share it with the UI we extract pure functions into a new `internal/core` package. CLI and UI both call `core`.

```
internal/
  core/                  # NEW — framework-agnostic operations
    accounts.go          # add/list/remove account
    rules.go             # add/list/remove/toggle rule
    emails.go            # list/get/search persisted emails
    watch.go             # start/stop watcher, exposes event channel
    read.go              # one-shot fetch
    export.go            # CSV export
    diagnose.go          # connection diagnose
  cli/                   # thin wrappers around core (unchanged behavior)
  ui/                    # NEW — Fyne app
    app.go               # fyne.App + main window
    sidebar.go           # nav + account picker
    views/
      dashboard.go
      emails.go          # list + detail split
      rules.go           # list + add form
      accounts.go        # list + add form
      watch.go           # tabs: cards | raw log
      tools.go           # export-csv, diagnose, read forms
    components/
      eventcard.go
      logstream.go
      formfield.go
cmd/
  email-read/main.go     # CLI entry (unchanged)
  email-read-ui/main.go  # NEW — Fyne entry
```

The watcher already produces structured events; we'll expose them via a `chan core.Event` instead of only logging, so the UI can subscribe.

---

## Atomic steps

Each step is independently verifiable. Order matters — earlier steps unblock later ones.

### Phase 1 — Refactor (no UI yet)

1. **Extract `internal/core/accounts.go`** from `internal/cli/cli.go`. CLI commands become thin wrappers. Add unit test.
2. **Extract `internal/core/rules.go`** from `internal/cli/rules_export.go`. Same pattern. Test.
3. **Extract `internal/core/emails.go`** — query helpers over `internal/store` (list by account, get by uid, search by subject/from). Test.
4. **Extract `internal/core/read.go`** from `internal/cli/read.go`. Test.
5. **Extract `internal/core/export.go`** + `diagnose.go`. Test.
6. **Refactor watcher to emit events.** `internal/watcher` gains a `Subscribe() <-chan Event` channel; existing log output stays. CLI prints from channel; UI will consume the same channel. Test with a fake mailclient.
7. **Bump version 0.19.0**, run all tests, confirm CLI behavior unchanged.

### Phase 2 — Fyne shell

8. **Add Fyne dependency** (`fyne.io/fyne/v2`) and create `cmd/email-read-ui/main.go` with an empty window. Verify it builds on macOS/Windows/Linux.
9. **Build sidebar + detail layout** (`internal/ui/app.go`, `sidebar.go`). Sidebar items: Dashboard, Emails, Rules, Accounts, Watch, Tools. Right pane swaps `fyne.CanvasObject` on selection.
10. **Account picker in sidebar** populated from `core.ListAccounts()`. Selection stored in app state, passed to views.

### Phase 3 — Read-only views

11. **Dashboard view** — counts (accounts, rules, emails today), last 5 events, "Start watch" CTA.
12. **Emails view** — left list (uid, from, subject, date) from `core.ListEmails(alias)`, right detail pane (subject, from, date, decoded body, extracted links as buttons → `core.OpenIncognito(url)`).
13. **Rules view** — table of rules with enabled toggle. Toggle calls `core.SetRuleEnabled(name, bool)`.
14. **Accounts view** — table of accounts (alias, host, user, last seen UID).

### Phase 4 — Forms (mutating commands)

Each form lives **inline in the right pane** (not a modal popup) per your spec.

15. **Add account form** — fields: alias, email, password, host (autodiscover button), port, TLS. Submit → `core.AddAccount()`. Show success/error inline.
16. **Add rule form** — fields: name, urlRegex, fromRegex (optional), subjectRegex (optional), enabled. Submit → `core.AddRule()`.
17. **Remove account / remove rule** — confirmation inline (not modal): "Type the name to confirm" then Delete button.
18. **Read (one-shot fetch) form** — pick account, optional limit, "Run". Streams results into a log panel below the form.
19. **Export CSV form** — pick account, date range, output path picker (`dialog.ShowFileSave`). Submit → `core.ExportCSV()`.
20. **Diagnose form** — pick account, "Run". Shows IMAP connect / login / folder stats inline.

### Phase 5 — Watch view

21. **Watch tab 1 — Structured cards.** Subscribe to `core.Events()`. Each new email renders a card: from, subject, decoded snippet, extracted links (click → incognito), rule match badge. Newest on top, capped at 200.
22. **Watch tab 2 — Raw log.** Same event stream formatted as the current readable log lines (re-use the formatter from `internal/watcher`). Scrollable, auto-tails, "Pause" + "Clear" buttons.
23. **Watch controls** — Start / Stop buttons in the view header, status indicator (idle / watching / error). Start spawns the watcher goroutine; Stop cancels its context.

### Phase 6 — Polish & ship

24. **Theme** — Fyne dark theme by default, accent color matching the CLI banner. Custom font for monospace log.
25. **Keyboard shortcuts** — ⌘1–6 for sidebar nav, ⌘N for new email fetch, ⌘. to stop watch.
26. **Settings view** — config.json path, data dir, browser path override, poll interval.
27. **Build scripts** — `make ui-mac`, `make ui-win`, `make ui-linux` producing single binaries via `fyne package`.
28. **Bump version 0.30.0**, update README with UI section + screenshots, write `.lovable/solved-issues/09-fyne-ui.md`.

---

## Open questions (none blocking — defaults shown)

- **Single window vs multi-window?** Default: single window. (Fyne supports multi but adds complexity.)
- **Persist UI state (last selected account, sidebar position)?** Default: yes, in `data/ui-state.json`.
- **System tray icon for background watch?** Default: phase 6 stretch goal — skip unless asked.

---

## Out of scope

- Removing the CLI. The CLI stays as the headless entry point.
- Replacing the React/Vite scaffold in `src/` (it's unused per `.lovable/overview.md`).
- Web/remote UI — Fyne is desktop-only.

---

## Verification per phase

- **Phase 1:** `go test ./...` green, `email-read` CLI behaves identically.
- **Phase 2:** UI binary launches an empty window on the target OS.
- **Phase 3:** Selecting an account shows real emails/rules/accounts from disk.
- **Phase 4:** Submitting each form mutates `data/config.json` and reflects in the CLI.
- **Phase 5:** Sending a test email produces a card AND a raw log line within ~5s.
- **Phase 6:** `fyne package` produces a runnable `.app` / `.exe` / binary.
