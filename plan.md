# Project plan — email-read (root, AI-handoff edition)

> **Audience:** any AI continuing work on this project.
> **Authoritative status & history:** `.lovable/memory/workflow/01-status.md` and `.lovable/plan.md`. This file restates the **future** backlog in a hand-off-friendly shape (objective / dependencies / outputs / acceptance) and ends with a **Next task selection** menu.
>
> Read first, in order: `.lovable/memory/index.md`, `.lovable/memory/workflow/01-status.md`, `.lovable/memory/decisions/06-ac-coverage-rollout-pattern.md`, `.lovable/strictly-avoid.md`, `.lovable/reports/01-spec-handoff-risk.md`.

---

## Snapshot

- **Phase 1+2 (CLI + Fyne UI):** ✅ 100% — Slices #1-#41.
- **Phase 3 (AC coverage rollout):** 🔄 51.6% (96/186) — current workstream.
- **CLI version:** `0.28.0` (`cmd/email-read/main.go`).
- **Sandbox:** no Go toolchain by default; use `nix run nixpkgs#go -- vet|test -tags nofyne ./...`. No cgo / GL.
- **Hard rules:** see `.lovable/strictly-avoid.md` (do NOT touch `.release/`; bump ≥ minor on every code change; never silent on watch loops).

---

## Phase 3 — AC coverage rollout (active)

### P3.A — Headless scanners (sandbox-feasible)

#### Task: Slice #132 — AC-DS AST gaps
- **Objective:** Close ~4 AC-DS rows via AST scanners (widget-usage limits, animation duration ceilings, hard-coded color guard).
- **Dependencies:** None.
- **Expected outputs:** New `internal/specaudit/ast_design_system_test.go`; allowlist shrink in `internal/specaudit/coverage_audit_test.go`.
- **Acceptance:** `Test_AC_CoverageAudit` green; 4 AC-DS rows removed from allowlist; coverage 51.6% → ~53.7%.

#### Task: AC-DS long-tail headless
- **Objective:** Close ~18 remaining AC-DS rows that don't need a Fyne canvas.
- **Dependencies:** Slice #132 first (shared scanner helpers).
- **Outputs:** 1–2 new files under `internal/specaudit/` or `internal/ui/` (headless-only assertions).
- **Acceptance:** allowlist shrinks by ~18; AC-DS family ≥ 90%.

### P3.B — Spec-debt cleanup (unblocks deferred-skip tests)

#### Task: Grow `06-error-registry.md` (39 codes)
- **Objective:** Define the 39 ER codes referenced by specs but missing from the registry.
- **Outputs:** Updated `spec/.../06-error-registry.md`; promote `Test_AllErrorRefsResolveInRegistry` from `t.Skip` to `t.Fatal`.
- **Acceptance:** AC-PROJ-31 leaves the allowlist; audit stays green.

#### Task: Fix 33 broken cross-tree spec links
- **Objective:** Eliminate broken local links surfaced by `Test_NoBrokenSpecLinks_GreenInCi`.
- **Outputs:** Edits across `02-coding-guidelines/`, `08-generic-update/`, `13-cicd-pipeline-workflows/`; promote test to `t.Fatal`.
- **Acceptance:** AC-PROJ-33 leaves the allowlist.

#### Task: AC-PROJ-35 — close OI-1..OI-6
- **Objective:** Resolve or delete six "scheduled" rows in `spec/21-app/02-features/06-tools/99-consistency-report.md`.
- **Outputs:** Updated consistency report.
- **Acceptance:** AC-PROJ-35 leaves the allowlist.

### P3.C — Spec disambiguation (recommended before T4)

#### Task: Restate `WatchEvents` naming lock + Decision-column plan
- **Outputs:** Headline callouts in `spec/23-app-database/01-schema.md` and `02-queries.md`.
- **Acceptance:** Naming lock visible without needing memory access.

#### Task: Add sandbox-feasibility tags to per-feature 97 files
- **Outputs:** 🟢/🟡/🔴 legend + per-row tag in 7 feature 97 files plus DB and DS 97 files.
- **Acceptance:** Risk-report can cite tags directly.

#### Task: Update feature index UI status
- **Outputs:** `spec/21-app/02-features/00-overview.md` table flipped from `Planned` → `✅ Done`.

---

## Phase 4 — Deferred (require infra not in this sandbox)

| Task | Blocker | Unblocks |
|---|---|---|
| Slice #118e — Fyne canvas harness | cgo + X11/GL workstation runner | AC-SF (21), AC-DS canvas (~22), AC-SX-06 frontend (1) |
| Bucket #9 — Bench/race/goleak infra | CI runner with perf budget | AC-DBP (6), AC-SP (5) |
| AC-DB schema-evolution | Migration behaviour layer + seed fixture | AC-DB schema-evolution (~12) |
| AC-PROJ E2E harness | Multi-process IMAP + Chrome harness | AC-PROJ-01..11/14/15 (~13) |

---

## Phase 5 — Verification (user-side, in progress)

- ⏳ App boot smoke test (procedure in `.lovable/memory/decisions/05-desktop-run-procedure.md`).
- ⏳ Persist Density preference (deferred per design-system §8).
- ⏳ Static "Accounts" / "Rules enabled" tile auto-refresh hook.
- ⏳ CI integration of `-race` once external runner lands.

---

## Next task selection

Pick one. All four are sandbox-feasible and require no infra.

1. **Slice #132 — AC-DS AST gaps (~4 rows).** Continues the established scanner pattern. Highest momentum, lowest risk. Coverage +~2%.
2. **Grow `06-error-registry.md` (39 ER codes).** Unblocks AC-PROJ-31 deferred-skip → ratchet. Pure spec authoring.
3. **Fix 33 broken cross-tree spec links.** Unblocks AC-PROJ-33 deferred-skip → ratchet. Pure markdown edits, mechanical.
4. **AC-DS long-tail headless (~18 rows).** Larger batch but same template as Slice #132; best for a long uninterrupted session.

After picking, the AI should:
1. Set the chosen task `in_progress` in the task tracker.
2. Re-read the linked spec(s) and the relevant `97-acceptance-criteria.md`.
3. Implement; bump CLI `Version` ≥ minor if any code changes.
4. Run `nix run nixpkgs#go -- test -tags nofyne ./...` + the AC audit.
5. Run `write memory` to persist.
