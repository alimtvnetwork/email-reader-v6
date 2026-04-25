# Consolidated: `.lovable/` Folder Structure

**Version:** 3.3.0  
**Updated:** 2026-04-16  
**Source:** [`spec/01-spec-authoring-guide/07-memory-folder-guide.md`](../01-spec-authoring-guide/07-memory-folder-guide.md)

---

## Purpose

This is the **standalone consolidated reference** for the `.lovable/` folder structure — the AI context layer. An AI reading only this file must be able to create, maintain, and navigate the `.lovable/` directory correctly.

---

## Canonical Structure

```
.lovable/
├── overview.md                     # AI onboarding — read FIRST
├── user-preferences                # User communication preferences
├── plan.md                         # Current roadmap / active plan
├── suggestions.md                  # Pending suggestions (bullet points)
├── strictly-avoid.md               # ⛔ Quick-read prohibition summary
│
├── memory/                         # Institutional knowledge
│   ├── index.md                    # Canonical index of all memory files
│   ├── architecture/               # System architecture decisions
│   ├── constraints/                # Hard constraints and rules
│   ├── done/                       # Completed tasks archive
│   ├── features/                   # Feature-specific knowledge
│   ├── issues/                     # Issue-specific knowledge
│   ├── patterns/                   # Reusable patterns/templates
│   ├── processes/                  # Workflow processes
│   ├── project/                    # Project-level status/decisions
│   ├── standards/                  # Technical standards
│   ├── style/                      # Code style rules
│   ├── suggestions/                # Suggestion tracker
│   └── workflow/                   # Workflow trackers
│
├── suggestions/                    # Suggestion details (one .md per suggestion)
│   └── completed/                  # Completed suggestions archive
│
├── pending-tasks/                  # Active work-in-progress tasks
├── completed-tasks/                # Finished tasks archive
│
├── pending-issues/                 # Open issues
├── solved-issues/                  # Resolved issues with root cause
│
└── strictly-avoid/                 # ⛔ Hard prohibitions (one .md per rule)
```

---

## Critical Rules

> **There is exactly ONE memory folder: `.lovable/memory/` (singular).** The variant `.lovable/memories/` (plural) is **prohibited**. If found, migrate contents and delete it.

> **`memory/index.md` is the single source of truth** for all memory files. Every memory file must be listed there. Orphaned files (in `memory/` but not in `index.md`) must be indexed or removed.

---

## AI Reading Order

1. `overview.md` → understand the project
2. `strictly-avoid.md` → know what NOT to do
3. `user-preferences` → adapt communication style
4. `memory/index.md` → survey all institutional knowledge
5. `plan.md` → understand current work context
6. `suggestions.md` → see pending ideas

---

## Naming Conventions

- **Folders:** kebab-case, no numeric prefixes (`memory/`, `pending-tasks/`)
- **Files:** kebab-case, optional numeric prefix (`index.md`, `01-plan-tracker.md`)
- **No spaces**, no underscores, no PascalCase in filenames

---

## Memory File Conventions

### File Structure

Every memory file follows this template:

```markdown
# Memory: [descriptive-title]

**Category:** architecture | constraints | features | issues | patterns | processes | project | standards | style | workflow  
**Created:** YYYY-MM-DD  
**Updated:** YYYY-MM-DD

---

## Summary

One-paragraph description of what this memory captures.

## Details

Full content — rules, decisions, patterns, constraints.

## Related

- Links to spec files, other memories, or code files
```

### Categories and When to Use

| Category | Use When | Examples |
|----------|----------|---------|
| `architecture/` | System-level structural decisions | Database schema, caching policy, split-DB pattern |
| `constraints/` | Hard rules that cannot be violated | Version pinning, forbidden patterns |
| `features/` | Feature-specific knowledge and requirements | Self-update architecture, visual rendering |
| `issues/` | Specific bugs and their root cause analysis | Nested code fence corruption |
| `patterns/` | Reusable code/design patterns | Template patterns, composition patterns |
| `processes/` | Workflow and operational procedures | Development workflow, automated enforcement |
| `project/` | Project-level status and decisions | Documentation standards, author attribution |
| `standards/` | Technical standards and coding rules | Code Red guidelines, TypeScript patterns, enum standards |
| `style/` | Code style and naming conventions | Naming conventions, PowerShell naming |
| `workflow/` | Workflow tracking state | Sprint trackers, migration status |

### index.md Structure

The `memory/index.md` has two sections:

```markdown
# Project Memory

## Core
- 🔴 CODE RED: [critical rule — one line, <150 chars]
- [universal rule that applies to every action]

## Memories
- [Descriptive title](mem://category/filename) — One-line description
```

**Core section rules:**
- Only rules that apply to **every** action across the entire project
- Max ~150 characters per entry
- Prefixed with 🔴 for CODE RED severity items

**Memories section rules:**
- Every memory file in `memory/` must have a corresponding entry
- Format: `[Title](mem://category/filename) — Description`
- Description must be specific enough to judge relevance without opening the file
- Sort by category, then alphabetically within category

### Creating a New Memory

1. Write the file to `memory/{category}/{kebab-case-name}.md`
2. Add entry to `memory/index.md` under `## Memories`
3. Verify no duplicate or overlapping memory already exists

### Updating a Memory

1. Edit the file directly
2. Update the `Updated:` date in the frontmatter
3. If the description changed materially, update the index entry too

### Deleting a Memory

1. Remove the file from `memory/{category}/`
2. Remove its entry from `memory/index.md`

---

## Workflows

### Tasks: `plan.md` → `pending-tasks/` → `completed-tasks/`

1. High-level items tracked in `plan.md` as a roadmap
2. When work begins, create a detailed file in `pending-tasks/`
3. On completion, move to `completed-tasks/` with results noted

### Task File Template

```markdown
# Pending Task: [Title]

**Priority:** High | Medium | Low  
**Status:** Not Started | In Progress | Complete ✅  
**Created:** YYYY-MM-DD

---

## Description

What needs to be done.

## Items

- [ ] Subtask 1
- [ ] Subtask 2
- [x] Completed subtask

---

*Pending task — [context] — vX.Y.Z — YYYY-MM-DD*
```

### Suggestions: `suggestions.md` → `suggestions/` → `suggestions/completed/`

1. Quick bullet points in `suggestions.md` for pending ideas
2. Detailed analysis in `suggestions/{suggestion-name}.md`
3. On implementation, move to `suggestions/completed/`

### Issues: `pending-issues/` → `solved-issues/`

1. Document the issue with symptom, diagnosis, and attempted fixes
2. On resolution, move to `solved-issues/` with root cause and fix documented

### Issue File Template

```markdown
# Issue: [Title]

**Severity:** Critical | High | Medium | Low  
**Status:** Open | Investigating | Resolved  
**Created:** YYYY-MM-DD

---

## Symptom

What was observed.

## Root Cause

Why it happened (filled after diagnosis).

## Fix

What was changed and why.

## Prevention

How to prevent recurrence.
```

---

## Strictly-Avoid Rules

### Summary File (`strictly-avoid.md`)

Quick-reference list of all prohibitions. One line per rule, linking to the detailed file.

### Detail Files (`strictly-avoid/{rule-name}.md`)

```markdown
# Strictly Avoid: [Rule Title]

**Rule:** One-sentence prohibition.

---

## What Is Prohibited

Specific behavior or pattern that must not occur.

## Why

Rationale — what goes wrong if violated.

## What To Do Instead

The correct alternative approach.
```

---

## Root-Level Files

### `overview.md`

Project onboarding for AI. Must contain:
- Project name and purpose
- Tech stack summary
- Key architecture decisions
- Links to critical spec modules

### `user-preferences`

User communication preferences (plain text, no frontmatter). Applied to every AI response. Examples:
- Preferred language/tone
- Timezone
- Response format preferences
- Things to always/never do

### `plan.md`

Current roadmap with active items. Updated as priorities shift.

### `suggestions.md`

Bullet-point list of pending suggestions with one-line descriptions.

---

*Consolidated .lovable folder structure — v3.3.0 — 2026-04-16*
