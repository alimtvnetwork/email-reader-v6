---
name: Implementation progress tracker (spec/21-app)
description: Canonical denominator for the "% done" signal shown after every slice. Update on every slice.
type: feature
---
# Implementation Progress Tracker — spec/21-app

**Last updated:** 2026-04-29 (after **Slice #212 — Settings redesign Phase 1**: 9 flat form rows regrouped into 3 `widget.Card`s — Appearance / Watcher / Database maintenance. Read-only paths panel (`newSettingsPaths` → `newSettingsPathsCard`) becomes a card with monospace value + Copy + Open buttons; Config row gets a third Reveal button. Cross-platform `browser.RevealInFileManager` (`open -R` / `explorer /select,` / `xdg-open` parent) added to keep all shell-outs centralized in `internal/browser/` (preserves OI-5 single-allowlist). Settings AC-SF-03 audit re-pinned. Headless `settings_paths_test.go` covers the three pure helpers. Full repo green: vet + test ./... pass under `-tags nofyne`.)
**Overall: 100% done · 0% remaining (original roadmap complete) · UX bugfix/RCA slices applied through #212. UX-redesign roadmap P1 of 6 complete; P2 sidebar hit-area, P3 error-log per-row Copy/Delete, P4 Dashboard cards, P5 Accounts cards, P6 theme polish remain as user-facing UX phases (out-of-scope for the spec %).**
