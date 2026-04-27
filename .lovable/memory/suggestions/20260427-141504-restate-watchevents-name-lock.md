---
suggestionId: 20260427-141504-restate-watchevents-name-lock
createdAt: 2026-04-27T14:15:04Z
closedAt: 2026-04-27T15:00:00Z
source: Lovable
affectedProject: email-read
status: closed
priority: medium
resolvedBy: Slice #183
---

## Description
The schema-naming verdict (table = `WatchEvents`, event type = `WatchEvent`) is locked in `mem://design/schema-naming-convention` but not restated where new contributors first read: `spec/23-app-database/01-schema.md` headers.

## Rationale
Prevents T4 schema-evolution misfires (AI-driven rename attempts that crash migrations).

## Acceptance criteria
- [x] `01-schema.md` carries an explicit "Naming lock" callout under the WatchEvents section.
- [x] Same callout for `OpenedUrls.Decision` rollout plan (Q-OPEN-PRUNE split).

## Resolution (Slice #183)

Added two `🔒 Naming lock` blockquote callouts to `spec/23-app-database/01-schema.md`:

1. **Under §4 (`OpenedUrls`)** — locks the plural table name + singular Go struct, and pins the Q-OPEN-PRUNE `Decision` enum rollout plan (4 numbered constraints (a)–(d) any future migration must honour).
2. **Under §5 (`WatchEvents`)** — locks the `WatchEvents`/`WatchEvent` plural/singular pair, names the dependent migration (`m0008`), Go event type (`core.WatchEvent`), and bus (`eventbus.Bus[WatchEvent]`).

Bumped spec header `Version 1.0.0 → 1.0.1`, `Updated → 2026-04-27`.

**Verified:** `Test_NoBrokenSpecLinks_GreenInCi` + `Test_Project_NoOpenOiReferencesAtMergeTime` → `ok 0.067s`.
