---
suggestionId: 20260427-141505-add-sandbox-feasibility-tag
createdAt: 2026-04-27T14:15:05Z
source: Lovable
affectedProject: email-read
status: open
priority: medium
---

## Description
Each per-feature `97-acceptance-criteria.md` should declare its sandbox feasibility per row: 🟢 headless · 🟡 cgo-required · 🔴 needs bench/E2E infra.

## Rationale
A fresh AI currently has no fast way to know which AC rows are sandbox-implementable. This is the single biggest source of wasted-slice risk (T5/T6 misfires).

## Acceptance criteria
- [ ] All 7 per-feature 97 files plus `23-app-database/97` and `24-app-design-system-and-ui/97` carry the legend + per-row tag.
- [ ] Risk-report Tier table updated to cite the tag.
