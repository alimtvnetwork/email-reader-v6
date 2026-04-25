---
name: spec-21-app-folder-rename
description: spec/21-golang-email-reader and spec/22-fyne-ui were merged into spec/21-app on 2026-04-25. Use the new path; legacy content is under spec/21-app/legacy/.
type: reference
---

# spec/21-app folder rename (2026-04-25)

## What changed

| Old path | New path |
|---|---|
| `spec/21-golang-email-reader/spec.md` | `spec/21-app/legacy/spec.md` |
| `spec/21-golang-email-reader/plan.md` | `spec/21-app/legacy/plan-cli.md` |
| `spec/22-fyne-ui/plan.md` | `spec/21-app/legacy/plan-fyne-ui.md` |

The two old folders were removed.

## Why

Per `spec/01-spec-authoring-guide/01-folder-structure.md` §"Required Root Folders", folder #21 must be `21-app/`. The previous `21-golang-email-reader/` violated that rule; `22-fyne-ui/` was scope-overlapping app content that belongs inside the same app folder.

## Authoritative spec now

`spec/21-app/` follows the App Project Template:
- `00-overview.md`
- `01-fundamentals.md` — shared `internal/core` + dual entrypoints (`email-read`, `email-read-ui`)
- `02-features/` — 7 features: dashboard, emails, rules, accounts, watch, tools, settings
- `03-issues/`
- `97-acceptance-criteria.md` (35 criteria)
- `99-consistency-report.md`

## How to apply

- Never recreate `21-golang-email-reader/` or `22-fyne-ui/`.
- Treat `legacy/*.md` as read-only history.
- New app spec edits go inside the App template structure.
