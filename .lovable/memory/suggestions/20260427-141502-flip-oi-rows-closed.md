---
suggestionId: 20260427-141502-flip-oi-rows-closed
createdAt: 2026-04-27T14:15:02Z
closedAt: 2026-04-27T14:45:00Z
source: Lovable
affectedProject: email-read
status: closed
priority: medium
resolvedBy: Slice #182 (audit-only)
---

## Description
Resolve or delete OI-1..OI-6 in `spec/21-app/02-features/06-tools/99-consistency-report.md`.

## Rationale
Six rows marked "scheduled" but never flipped. Closing unblocks AC-PROJ-35.

## Acceptance criteria
- [x] Each OI row is either ✅ Closed (with resolution note) or removed.
- [x] AC-PROJ-35 removed from allowlist.

## Resolution (Slice #182)

Audit on 2026-04-27 found this suggestion was already satisfied by prior slices:

1. **OI-1..OI-6 all ✅ Closed** in `spec/21-app/02-features/06-tools/99-consistency-report.md` §14
   (closed across Slices #133/#134/#135/#136 + cross-test F-23 by Slice #139). Zero
   "scheduled"/🟡/⚠/TODO/FIXME markers remain on any OI row (verified by grep).

2. **AC-PROJ-35 not in `coverageGapAllowlist`** — already removed; doc comment in
   `internal/specaudit/coverage_audit_test.go` lines 362–365 explicitly notes closure
   by Slice #144 (`Test_Project_NoOpenOiReferencesAtMergeTime` in
   `internal/specaudit/project_no_open_oi_test.go`).

3. **Test green**: `nix run nixpkgs#go -- test -tags nofyne -run Test_Project_NoOpenOiReferencesAtMergeTime ./internal/specaudit/` → `ok 0.012s`.

No file edits required for this slice — closure is bookkeeping only.
