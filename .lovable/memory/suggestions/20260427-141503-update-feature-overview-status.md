---
suggestionId: 20260427-141503-update-feature-overview-status
createdAt: 2026-04-27T14:15:03Z
source: Lovable
affectedProject: email-read
status: open
priority: low
---

## Description
`spec/21-app/02-features/00-overview.md` still shows UI status as `Planned` for all 7 features even though Phases 1+2 shipped (Slices #1-#41).

## Rationale
Misleading for any fresh AI reading the spec — implies UI work is unstarted.

## Acceptance criteria
- [ ] All 7 rows show `✅ Done` in the UI status column.
- [ ] CLI status column reviewed for the same drift.
