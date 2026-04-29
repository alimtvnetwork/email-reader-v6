---
name: Settings redesign Phase 1 (Slice #212)
description: 9 flat rows → 3 cards (Appearance/Watcher/Maintenance); paths-card with Copy/Open/Reveal; reveal helper centralized in internal/browser.
type: feature
---
# Slice #212 — Settings redesign Phase 1 (LOCKED 2026-04-29)

## What changed
- `internal/ui/views/settings.go`:
  - `buildSettingsForm` now wraps cards into a `container.NewVScroll`.
  - Old single `widget.Form` (9 rows) replaced by `newSettingsCards` →
    3 `widget.Card`s grouped per user lock-in:
    - **Appearance**: Theme, Density.
    - **Watcher**: Poll interval, Chrome / Chromium path.
    - **Database maintenance**: Retention, weekday, hour, WAL hours, prune batch.
  - `newSettingsPaths` → `newSettingsPathsCard`. Each row: monospace
    path + Copy + Open buttons; Config row gets a third Reveal button.
    Status label inline (no popup).
  - New pure helpers: `handleCopyPath`, `handleOpenPath`,
    `handleRevealPath` — fully unit-tested in `settings_paths_test.go`
    via a `recordingClipboard` fyne.Clipboard stub + plain `func(string) error` seams.
  - `SettingsOptions` extended with `Clipboard`, `OpenPath`, `RevealPath` seams.
- `internal/ui/app.go`:
  - `NavSettings` case wires `Clipboard: fyneClipboard()`,
    `OpenPath: openLogFileWithFyne`, `RevealPath: browser.RevealInFileManager`.
  - New helper `fyneClipboard()` — extracted from inline Watch/Error Log copy patterns.
- `internal/browser/reveal.go` — new file. `RevealInFileManager(path)`:
  - macOS: `open -R <abs>`
  - Windows: `explorer /select,<abs>`
  - Linux/other: `xdg-open <parentDir>` (xdg-open has no select flag).
  - Centralized here so `linters/no-other-browser-launch.sh` (OI-5)
    keeps its single-allowlist guarantee — all shell-outs to OS
    handlers live in `internal/browser/`.
- `internal/specaudit/ast_settings_paths_readonly_test.go`:
  - Re-pinned `fnName` constant `newSettingsPaths` → `newSettingsPathsCard`.
  - AC-SF-03 contract unchanged: no `widget.NewEntry` allowed inside
    the paths-card constructor (paths stay read-only — Copy/Open/Reveal
    buttons are the only interaction).

## Verification
- `nix run nixpkgs#go -- vet -tags nofyne ./...` — clean.
- `nix run nixpkgs#go -- test -tags nofyne ./...` — all 22 packages pass.
- Specaudit & linters green: `Test_AST_SettingsPaths_NoEntryWidget` matches
  new symbol; `Test_Linter_NoOtherBrowserLaunch` passes (the new
  `open -R` / `xdg-open` calls live under the allowlisted
  `internal/browser/` prefix).

## Lessons re-confirmed
- Renaming a function the AST audits pin requires a one-line update
  to the audit's `fnName` constant in the SAME slice (audit's docstring
  even tells you so — same template as Slice #147 exportStreamFiles).
- Any `exec.Command("open"|"xdg-open"|...)` outside `internal/browser/`
  trips the OI-5 linter. Solution: put the helper in `internal/browser/`,
  not allowlist-extend.

## Roadmap context
This is **Phase 1 of 6** in the user-locked UX redesign roadmap
(2026-04-29). Remaining phases are user-facing UX, out of the spec/21-app %:

  - P2 — Sidebar hit-area & visual polish (fixes "Diagnose opens when
    I click Error Log" misclick + button hover affordance).
  - P3 — Error Log per-row Copy + per-row Delete (in-memory only;
    JSONL stays append-only audit trail).
  - P4 — Dashboard redesign → card-grid layout.
  - P5 — Accounts redesign → card-rows.
  - P6 — Theme polish (focus ring, hover bg, button consistency).

Each is a single slice. Execute when user says "next".
