---
suggestionId: 20260427-141505-add-sandbox-feasibility-tag
createdAt: 2026-04-27T14:15:05Z
closedAt: 2026-04-27T15:30:00Z
source: Lovable
affectedProject: email-read
status: closed
priority: medium
resolvedBy: Slice #184
---

## Description
Each per-feature `97-acceptance-criteria.md` should declare its sandbox feasibility per row: 🟢 headless · 🟡 cgo-required · 🔴 needs bench/E2E infra.

## Rationale
A fresh AI currently has no fast way to know which AC rows are sandbox-implementable. This is the single biggest source of wasted-slice risk (T5/T6 misfires).

## Acceptance criteria
- [x] All 7 per-feature 97 files plus `23-app-database/97` and `24-app-design-system-and-ui/97` carry the legend + per-row tag.
- [x] Risk-report Tier table updated to cite the tag.

## Resolution (Slice #184)

Added uniform sandbox-feasibility infrastructure to all 9 AC files via deterministic Python script (`/tmp/feasibility_tag.py`):

1. **Legend block** (`## Sandbox feasibility legend`) inserted after `## Purpose` in all 9 files. Documents the 4-tag system (🟢 headless / 🟡 cgo-required / 🔴 bench+E2E / ⚪ N/A) with cross-references to starter-slice memories (#178 bench, #179 E2E, #180 canvas).

2. **Per-section `**Sandbox:**` tag** under every numbered (`## 1.`) and lettered (`## A.`) heading. Heuristic:
   * Sign-off → ⚪ N/A
   * Performance → 🔴 bench
   * Frontend / Accessibility / Layout / Components / Tokens / Theme implementation → 🟡 cgo
   * Audit-Trail Invariants & OpenUrl Functional → 🟢 + 🔴 split (per-row interpretation)
   * Heartbeat / Reconnect / Mode-Negotiation / Event-Fan-out → 🟢 + 🔴 split
   * Cross-Feature Settings AC-S1..S5 → 🟢 headless
   * Default (Functional / Code Quality / Logging / Database / Atomicity / Security / Live-Update / Error-Handling / Migrations / Maintenance / PRAGMAs / Named Queries / Schema Integrity / Ownership) → 🟢 headless

   Per-row precision (split sections) is left to the reader — the section tag tells you whether to expect a sandbox win, and the row text disambiguates.

3. **Spec headers bumped** v1.0.0 → v1.0.1 in all 9 files; `Updated → 2026-04-27`.

4. **Risk report Tier table** (`.lovable/reports/01-spec-handoff-risk.md` §1) extended with a `Sandbox tag` column mapping each Tier (T1–T7) to the corresponding 🟢/🟡/🔴 tag, plus a callout pointing fresh AIs at the inline tags. Corrective-actions table (#3, #4, #5, #6) struck through with closure-slice references (#181/#183/#184/#182).

**Verified:** `Test_NoBrokenSpecLinks_GreenInCi` (fail-loud since Slice #164) → `ok 0.092s`. `Test_Project_NoOpenOiReferencesAtMergeTime` → green. No coverage-monotonic regression.

**Files edited (12):**
- 9 × `97-acceptance-criteria.md` (per-feature + DB + design system)
- `.lovable/reports/01-spec-handoff-risk.md` (Tier table + corrective actions)
- `.lovable/memory/suggestions/20260427-141505-add-sandbox-feasibility-tag.md` (this file)
- `.lovable/memory/suggestions/index.md`
