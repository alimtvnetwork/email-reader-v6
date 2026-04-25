# Consolidated: App Issues — Complete Reference

**Version:** 3.3.0  
**Updated:** 2026-04-16

---

## Purpose

Root-level folder for **application-specific issue analysis** — bug documentation, root cause analysis, diagnosis, and resolution guidance. Every bug gets a markdown file with structured analysis before a fix is implemented.

An AI reading only this file must be able to create, triage, diagnose, and document application issues correctly.

---

## Issue File Structure

Each issue file follows this template:

```markdown
# Issue #NN — [Title]

**Severity:** Critical | High | Medium | Low  
**Status:** Open | Investigating | Resolved  
**Created:** YYYY-MM-DD  
**Resolved:** YYYY-MM-DD (if applicable)

## Symptom

What the user or developer observed. Include:
- Exact error message or unexpected behavior
- Steps to reproduce
- Environment (OS, browser, version)
- Frequency (always, intermittent, one-time)

## Diagnosis

Investigation steps taken:
1. What was checked first
2. What was ruled out
3. How the root cause was identified

## Root Cause

Technical explanation of why it happened. Include:
- Which component/module is responsible
- What condition triggers the bug
- Why it wasn't caught earlier

## Fix

What was changed and why:
- Files modified (with line-level detail)
- Why this approach was chosen over alternatives
- Any trade-offs accepted

## Verification

How the fix was verified:
- Manual test steps
- Automated test added (if applicable)
- Edge cases tested

## Prevention

How to prevent recurrence:
- Lint rule added
- Test coverage added
- Pattern documented in spec
- Code review checklist item

## Related

Links to affected spec modules, code files, or other issues.
```

---

## Severity Levels

| Level | Definition | Response Time | Examples |
|-------|-----------|---------------|---------|
| **Critical** | App unusable, data loss, security vulnerability | Immediate | Crash on startup, DB corruption, auth bypass |
| **High** | Major feature broken, significant UX degradation | Same day | REST endpoint returns 500, upload fails silently |
| **Medium** | Feature partially broken, workaround exists | Next sprint | Pagination off-by-one, filter ignores case |
| **Low** | Cosmetic, minor inconvenience | When convenient | Tooltip misaligned, log message typo |

---

## Triage Workflow

```
1. Report received
   ↓
2. Reproduce — Can we reproduce it?
   ├── Yes → 3. Severity assignment
   └── No → Request more details, mark "Needs Info"
   ↓
3. Classify severity (Critical/High/Medium/Low)
   ↓
4. Create issue file: `NN-kebab-case-title.md`
   ↓
5. Diagnose — investigate root cause
   ↓
6. Fix — implement and verify
   ↓
7. Document — complete all sections in issue file
   ↓
8. Close — update status to Resolved
```

---

## Resolution Patterns

### Pattern 1: Guard Clause Missing

**Symptom:** Null reference error deep in call stack  
**Root Cause:** Input not validated at entry point  
**Fix:** Add guard clause with early return  
**Prevention:** Enforce "fail fast" validation rule from coding guidelines

### Pattern 2: Race Condition

**Symptom:** Intermittent failures, data inconsistency  
**Root Cause:** Concurrent access to shared state without synchronization  
**Fix:** Add mutex/lock, use atomic operations, or serialize access  
**Prevention:** Review all shared state access in code review

### Pattern 3: Silent Error Swallowing

**Symptom:** Feature silently fails, no error logged  
**Root Cause:** Empty catch block or catch without re-throw/log  
**Fix:** Add proper error logging and propagation  
**Prevention:** Lint rule: no empty catch blocks (🔴 CODE RED)

### Pattern 4: Schema Drift

**Symptom:** Database queries fail or return unexpected results  
**Root Cause:** Code assumes a schema that doesn't match the actual DB  
**Fix:** Add migration to align schema, or fix code to match current schema  
**Prevention:** Migration runner with version tracking

### Pattern 5: Enum Value Mismatch

**Symptom:** Switch/match expression falls to default case unexpectedly  
**Root Cause:** New enum value added but not handled in all match expressions  
**Fix:** Add the missing case to all consumers  
**Prevention:** Use exhaustive match (no default) so compiler catches missing cases

### Pattern 6: Configuration Override Failure

**Symptom:** Settings revert to defaults after update  
**Root Cause:** Seed-and-merge logic doesn't preserve user overrides  
**Fix:** Fix merge strategy to prioritize user values over seed defaults  
**Prevention:** Test seed-merge with existing config before each release

### Pattern 7: File Path Resolution

**Symptom:** File not found errors in production but works in dev  
**Root Cause:** Hardcoded paths or relative paths that differ between environments  
**Fix:** Use `PathHelper` with proper base directory resolution  
**Prevention:** All file paths must go through `PathHelper` — no raw `__DIR__` concatenation

### Pattern 8: Upload Processing Failure

**Symptom:** File upload endpoint returns success but file is corrupt or missing  
**Root Cause:** `post_max_size` smaller than file, causing empty `$_FILES`  
**Fix:** Check for empty file params before validation, add server config health check  
**Prevention:** Health check endpoint validates PHP upload configuration

---

## Placement Decision Guide

**Place content here when:**
- It documents a **specific bug** and its root cause
- It provides **fix documentation** for an app-level failure
- It contains **post-mortem analysis** of an application issue

**Do NOT place here:**
- Cross-cutting coding violations → core fundamentals (`01–20`)
- Feature definitions → `21-app`
- Research → `11-research`

---

## Numbering

Folder `22` follows `21-app` in the app-specific range. Issue files use sequential numbering: `01-login-timeout-fix.md`, `02-cache-invalidation-bug.md`, etc.

---

## Issue-to-Memory Pipeline

When an issue reveals a **systemic pattern** worth remembering:

1. Resolve the issue fully in `22-app-issues/`
2. Extract the lesson into `.lovable/memory/issues/{issue-name}.md`
3. Add entry to `.lovable/memory/index.md`
4. Cross-reference the memory file in the issue's `## Related` section

This ensures institutional knowledge persists beyond the individual bug fix.

---

## Cross-References

| Reference | Location |
|-----------|----------|
| App Specs | `spec/21-app/00-overview.md` |
| Error Management | `spec/12-consolidated-guidelines/03-error-management.md` |
| Coding Guidelines | `spec/12-consolidated-guidelines/02-coding-guidelines.md` |
| .lovable Issues Folder | `.lovable/pending-issues/` → `.lovable/solved-issues/` |

---

*Consolidated app issues — v3.3.0 — 2026-04-16*
