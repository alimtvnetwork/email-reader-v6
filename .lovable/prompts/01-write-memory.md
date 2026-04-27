# Write Memory

> **Purpose:** After completing work or at the end of a session, persist everything learned, did, and left undone — so the next AI session can pick up seamlessly with zero context loss.
> **When to run:** End of every session, after a task batch, or when the user says `write memory` / `end memory`.
> **Trigger phrases:** `write memory`, `end memory`.

---

## Core Principle
The memory system is the project's brain. If you did something and didn't write it down, it didn't happen. Write as if the next AI has amnesia — because it does.

---

## Phase 1 — Audit Current State
- What was done this session? (files created/modified/deleted, decisions made)
- What is still pending? (started-but-unfinished, discussed-but-unstarted, blockers)
- What was learned? (patterns, gotchas, user preferences)
- What went wrong? (bugs, failed approaches, never-repeat items)

## Phase 2 — Update Memory Files (`.lovable/memory/`)
1. Read `index.md` first; never duplicate.
2. Update existing files for affected sections; preserve all other content.
3. Create new files (`XX-name.md`) ONLY if knowledge doesn't fit anywhere; immediately add to `index.md`.
4. Update `workflow/` with status markers: ✅ Done · 🔄 In Progress · ⏳ Pending · 🚫 Blocked.

## Phase 3 — Plans & Suggestions
- `.lovable/plan.md` — single file. Move fully complete items to `## Completed`; never delete.
- `.lovable/suggestions.md` — single file with `## Active Suggestions` and `## Implemented Suggestions`.

## Phase 4 — Issues
- `.lovable/pending-issues/XX-name.md` for unresolved bugs.
- Move to `.lovable/solved-issues/XX-name.md` on resolution; add `## Solution`, `## Iteration Count`, `## Learning`, `## What NOT to Repeat`.
- `.lovable/strictly-avoid.md` — append never-do-this patterns.

## Phase 4b — CI/CD Issues
- `.lovable/cicd-issues/XX-name.md` — one file per CI/CD issue (sandbox/toolchain/runner/perf-infra).
- `.lovable/cicd-index.md` — table summary; update in same operation.
- Collect all known CI/CD issues; never duplicate.

## Phase 5 — Consistency Validation
- Every memory file is in `index.md`.
- Every `✅ Done` plan item has evidence (memory/code/solved-issue).
- No file in both `pending-issues/` and `solved-issues/`.
- Solved issues all have `## Solution`.

## Phase 6 — Specs Already Written to FS
If a substantial spec/doc was authored this session into the project file system, record a pointer in memory (path + 1-line summary) so the next AI can find it.

## Phase 7 — Prompts Folder
- This prompt itself lives at `.lovable/prompts/01-write-memory.md`.
- `.lovable/prompt.md` indexes all prompt files.

## File Naming Rules
- Lowercase, hyphen-separated, numeric prefix: `01-name.md` ✅ / `01_Name.md` ❌.
- Plans + suggestions = single file each. Pending/solved/cicd issues = one file per issue.
- NEVER use `.lovable/memories/` (with `s`). Always `.lovable/memory/`.

## Anti-Corruption Rules
1. Never delete history — mark/move, don't remove.
2. Never overwrite blindly — read first, preserve unrelated content.
3. Never leave orphans — every file indexed; every reference resolves.
4. Never split unified files (plan/suggestions).
5. Never mix states (an issue is pending OR solved, never both).
6. Never skip the index update.
7. Never assume the next AI knows anything.

## Final Confirmation Format
```
✅ Memory update complete.
Session Summary:
- Tasks completed: [X]
- Tasks pending: [Y]
- New memory files: [Z]
- Issues resolved: [N] / opened: [M]
- Suggestions added: [S] / implemented: [T]
Files modified: [list]
Inconsistencies fixed: [list or "None"]
Next AI picks up from: [state + next step]
```
