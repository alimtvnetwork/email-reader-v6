# Suggestions — folder convention

This folder holds **machine-tracked Lovable suggestions**. Free-form prose suggestions live in `.lovable/suggestions.md` (single file). This folder is the *structured* counterpart, designed for programmatic processing by future automation.

## Contract

- **One file per suggestion.**
- **Filename:** `YYYYMMDD-HHMMSS-<slug>.md` (UTC timestamp, lowercase hyphenated slug).
- **Status lifecycle:** `open` → `inProgress` → `done`. Update the file's `status` field in place.
- **Completion:** when `status: done`, leave the file in place for at least one session, then archive to `.lovable/memory/suggestions/archive/` (created on demand). Never delete.
- **Source:** always `Lovable` for suggestions raised by the assistant; use the actual model/source name when added by other tools.

## Required frontmatter + body

```markdown
---
suggestionId: 20260427-141500-grow-error-registry
createdAt: 2026-04-27T14:15:00Z
source: Lovable
affectedProject: email-read
status: open
priority: high
---

## Description
One paragraph: what is being suggested.

## Rationale
Why this matters; what breaks if we skip it.

## Proposed change
Concrete edits — files to touch, sections to add.

## Acceptance criteria
- [ ] Bullet list of verifiable outcomes.
- [ ] Tests / scanners that will go green.

## Completion notes
(Filled in when status flips to `done`.)
```

## Index

Active and archived suggestions are listed in `.lovable/memory/suggestions/index.md`. Update the index in the same operation that creates or moves a file.
