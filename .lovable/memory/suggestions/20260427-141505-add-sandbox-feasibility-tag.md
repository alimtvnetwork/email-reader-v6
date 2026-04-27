---
suggestionId: 20260427-141505-add-sandbox-feasibility-tag
createdAt: 2026-04-27T14:15:05Z
resolvedAt: 2026-04-27T17:00:00Z
source: Lovable
affectedProject: email-read
status: resolved
priority: medium
---

## Description
Each per-feature `97-acceptance-criteria.md` should declare its sandbox feasibility per row: 🟢 headless · 🟡 cgo-required · 🔴 needs bench/E2E infra.

## Rationale
A fresh AI currently has no fast way to know which AC rows are sandbox-implementable. This is the single biggest source of wasted-slice risk (T5/T6 misfires).

## Acceptance criteria
- [x] All 7 per-feature 97 files plus `23-app-database/97` and `24-app-design-system-and-ui/97` carry the legend + per-row tag.
- [x] Risk-report Tier table updated to cite the tag.

## Resolution (Slice #136, 2026-04-27)
Implemented via `/tmp/tag_ac.py` (deterministic Python pass):
- Legend block (`<!-- sandbox-feasibility-legend v1 -->`) inserted into all 9 files after the `## Purpose` heading.
- 638 AC rows tagged across the 9 files (50 + 73 + 83 + 108 + 80 + 94 + 56 + 48 + 46).
- Tier table in `.lovable/reports/01-spec-handoff-risk.md` gained a "Sandbox-feasibility tag" column; weighted-overall estimate bumped from 85% (T1+T2+T3 only) to 91% (T1+T2+T3 filtered to 🟢 rows).
- Classifier is conservative: when ambiguous, defaults to 🟡 (avoids false-positive 🟢 → wasted slice). The script is idempotent (skips files already containing the legend marker), so a future re-run after spec edits won't double-tag.

