---
suggestionId: 20260427-141501-fix-broken-spec-links
createdAt: 2026-04-27T14:15:01Z
source: Lovable
affectedProject: email-read
status: open
priority: medium
---

## Description
Fix the 33 broken local spec links surfaced by `Test_NoBrokenSpecLinks_GreenInCi`.

## Rationale
Unblocks AC-PROJ-33. Most are imported from adjacent doc trees and reference moved/renamed files.

## Proposed change
Concentrated in `02-coding-guidelines/`, `08-generic-update/`, `13-cicd-pipeline-workflows/`. Run the test to dump the pair list, then fix in batches.

## Acceptance criteria
- [ ] Test logs report 0 broken links.
- [ ] Test promoted from `t.Skip` to `t.Fatal`.
- [ ] AC-PROJ-33 removed from allowlist.
