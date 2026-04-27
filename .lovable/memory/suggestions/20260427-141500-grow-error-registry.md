---
suggestionId: 20260427-141500-grow-error-registry
createdAt: 2026-04-27T14:15:00Z
source: Lovable
affectedProject: email-read
status: open
priority: high
---

## Description
Define the 39 ER codes that specs already reference but `06-error-registry.md` does not yet declare.

## Rationale
`Test_AllErrorRefsResolveInRegistry` (Slice #131) is wired but `t.Skip`s until the registry catches up. AC-PROJ-31 stays yellow until then.

## Proposed change
Add entries to `spec/.../06-error-registry.md` for: Settings 217xx (ER-SET-21770..21782), Migrations 218xx (ER-MIG-21800..21806), UI 219xx (ER-UI-21900..21913), plus stragglers ER-ACC-21430/21431, ER-COR-21710..21712/21770, ER-MAIL-2120, ER-RUL-21260, ER-TOOL-2176/21761/21762, ER-WATCH-21503, ER-WCH-21412.

## Acceptance criteria
- [ ] All 39 codes defined with description + recovery hint.
- [ ] `Test_AllErrorRefsResolveInRegistry` flips from `t.Skip` to `t.Fatal` on missing.
- [ ] AC-PROJ-31 removed from `coverageGapAllowlist`.
