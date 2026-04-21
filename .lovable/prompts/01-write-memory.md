# Write Memory

> **Purpose:** After completing work or at the end of a session, the AI must persist everything it learned, did, and left undone — so the next AI session can pick up seamlessly with zero context loss.
> **When to run:** At the end of every session, after completing a task batch, or when explicitly asked to "update memory" or "write memory".
> **Trigger phrases:** `write memory`, `end memory`.

---

## Table of Contents
1. Core Principle
2. Phase 1 — Audit Current State
3. Phase 2 — Update Memory Files
4. Phase 3 — Update Plans & Suggestions
5. Phase 4 — Update Issues
6. Phase 5 — Consistency Validation
7. File Naming & Structure Rules
8. Anti-Corruption Rules

---

## Core Principle
The memory system is the project's brain. If you did something and didn't write it down, it didn't happen. Write memory as if the next AI has amnesia — because it does.

---

## Phase 1 — Audit Current State
Before writing anything, take inventory:
- What was done this session? (files created/modified/deleted, decisions made)
- What is still pending? (started-but-unfinished, discussed-but-unstarted, blockers)
- What was learned? (patterns, gotchas, user preferences)
- What went wrong? (bugs, failed approaches, things never to repeat)

---

## Phase 2 — Update Memory Files
Target: `.lovable/memory/`

1. Read `.lovable/memory/index.md` first — never duplicate.
2. Update existing files in place — never truncate unrelated entries.
3. Create new topic files as `XX-descriptive-name.md` and immediately add to the index.
4. Update workflow status with markers: ✅ Done · 🔄 In Progress · ⏳ Pending · 🚫 Blocked — [reason].

---

## Phase 3 — Update Plans & Suggestions

### 3A — Plans (`.lovable/plan.md`)
- Update task statuses. Move fully complete items to a `## Completed` section in the same file (never delete).

### 3B — Suggestions (`.lovable/suggestions.md`)
Single file with two sections: `## Active Suggestions` and `## Implemented Suggestions`. Each entry: Title, Status, Priority, Description, Added. When implemented, move and add notes.

---

## Phase 4 — Update Issues

- Pending → `.lovable/pending-issues/XX-short-description.md` with: Description, Root Cause, Steps to Reproduce, Attempted Solutions, Priority, Blocked By.
- Solved → move file to `.lovable/solved-issues/` and append: Solution, Iteration Count, Learning, What NOT to Repeat.
- Forbidden patterns → add a one-liner to `.lovable/strictly-avoid.md` referencing the solved-issue file.

---

## Phase 5 — Consistency Validation
- Every file in `.lovable/memory/` must appear in `index.md`.
- Every ✅ Done in `plan.md` needs evidence (memory entry, solved issue, or code change).
- No file may exist in both `pending-issues/` and `solved-issues/`.
- Final response template:

```
✅ Memory update complete.
Session Summary:
- Tasks completed: X
- Tasks pending: Y
- New memory files created: Z
- Issues resolved: N
- Issues opened: M
- Suggestions added: S
- Suggestions implemented: T

Files modified:
- [list]

The next AI session can pick up from: [describe state and next step]
```

---

## File Naming & Structure Rules
- All files: lowercase, hyphen-separated, numeric prefix (`01-foo.md`).
- Plans → single `.lovable/plan.md`.
- Suggestions → single `.lovable/suggestions.md`.
- Pending/solved issues → one file per issue.
- Memory grouped by topic under `.lovable/memory/<topic>/`.
- Completed plans/suggestions → `## Completed` section in same file (no `completed/` folders).
- ⚠️ NEVER create `.lovable/memories/` (with trailing s). Correct path: `.lovable/memory/`.

---

## Anti-Corruption Rules
1. Never delete history — mark done, move to completed sections.
2. Never overwrite blindly — read first, preserve existing content.
3. Never leave orphans — every file must be indexed.
4. Never split what should be unified — plans and suggestions are one file each.
5. Never mix states — an issue is pending OR solved, not both.
6. Never skip the index update when adding a memory file.
7. Never assume the next AI knows anything — write for a stranger.
8. Anything the user said to skip or avoid → log in `.lovable/strictly-avoid.md`.

---

*Version 1.0.*
