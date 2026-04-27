---
suggestionId: 20260427-141504-restate-watchevents-name-lock
createdAt: 2026-04-27T14:15:04Z
source: Lovable
affectedProject: email-read
status: open
priority: medium
---

## Description
The schema-naming verdict (table = `WatchEvents`, event type = `WatchEvent`) is locked in `mem://design/schema-naming-convention` but not restated where new contributors first read: `spec/23-app-database/01-schema.md` headers.

## Rationale
Prevents T4 schema-evolution misfires (AI-driven rename attempts that crash migrations).

## Acceptance criteria
- [ ] `01-schema.md` carries an explicit "Naming lock" callout under the WatchEvents section.
- [ ] Same callout for `OpenedUrls.Decision` rollout plan (Q-OPEN-PRUNE split).
