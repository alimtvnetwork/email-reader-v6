# Spec Hand-off Risk Report — `email-read`

**Generated:** 2026-04-27
**Scope:** Reliability assessment of handing the current spec set (`spec/`) + memory (`.lovable/memory/`) to a fresh AI for autonomous implementation.
**Verdict:** ⚠️ **Conditionally ready.** Strong for headless backend / AC-coverage work. Blocked for Fyne canvas, perf-gated, and schema-evolution slices until infra and ~72 spec-debt items are cleaned.

---

## 1. Success probability by complexity tier

Estimates assume the next AI has: (a) read `.lovable/memory/index.md`, (b) read this report, (c) access to the same sandbox (no Go toolchain by default; `nix run nixpkgs#go -- ...` works for vet/test with `-tags nofyne`).

| Tier | Examples | Success | Confidence | Why |
|---|---|---|---|---|
| **T1 — Trivial spec/doc edits** | Grow `06-error-registry.md`, fix broken cross-tree links, flip OI-1..6 closed | **90–95%** | High | Pure markdown; deferred-skip tests auto-ratchet on success. |
| **T2 — Headless AST/log scanners** (Slice #132 pattern) | AC-DS AST gaps, AC-DS long-tail headless | **80–88%** | High | Strong template (`internal/specaudit/ast_*_test.go`); audit ratchet enforces honesty. |
| **T3 — Headless behaviour tests** | New `internal/core` / `internal/store` unit tests citing AC IDs | **70–80%** | Medium | Project-specific patterns (`errtrace.Result`, slog `event=…` tail format) need careful imitation. |
| **T4 — Schema-evolution behaviour** | Table rename `WatchEvents`→`WatchEvent`, enum CHECKs, FK SET NULL, gap/checksum/downgrade | **45–60%** | Medium | Touches `internal/store/migrate`; high blast radius; hard to verify without seed data. |
| **T5 — Fyne UI canvas slices** | AC-SF (21 rows), AC-DS canvas (~22), AC-SX-06 frontend | **15–25%** | Low | Sandbox lacks cgo/X11/GL — cannot compile or render; AI will guess and ship dead code. |
| **T6 — Perf gates** | AC-DBP (6), AC-SP (5) p95 budgets | **10–20%** | Low | No bench infra in CI; numbers are unverifiable. |
| **T7 — Multi-process E2E** | AC-PROJ-01..11/14/15 | **20–30%** | Low | No E2E harness; requires real IMAP fixture + Chrome process control. |

**Weighted overall** (by remaining-row count): **~55%** if the next AI tries the whole backlog. **~85%** if the next AI restricts to T1+T2+T3 (which is what the *Phase 3 — AC coverage rollout* milestone explicitly says to do).

---

## 2. Failure map

### Where failures cluster

| Failure mode | Likely site | Symptom |
|---|---|---|
| **Spec-debt dragnet** | `06-error-registry.md` (~39 missing ER codes), 33 broken cross-tree links | `Test_AllErrorRefsResolveInRegistry` and `Test_NoBrokenSpecLinks_GreenInCi` keep skipping; AC-PROJ-31/33 stay yellow forever. |
| **Honest-scope violation** | New tests citing an AC ID inside a `t.Skip` block | Audit treats AC as covered when it isn't → false green; user loses trust in the % done signal. |
| **Naming convention drift** | `WatchEvents` vs `WatchEvent`; `EmailEvent` vs `Email` | Migration runner crash; FK constraint violations at first boot. |
| **Color/theme leak** | New widget with hard-coded `theme.NewColor*` instead of design-system token | Slice #132 scanner immediately flags it — but only if the AI runs the audit before claiming done. |
| **Logging contract drift** | New error path that bypasses `errtrace.Wrap` or omits the `event=…` slog tail | AC-PROJ-13/14/15 regress silently (no test names them yet → no audit catch). |
| **Fyne dead code** | T5 slices written without compile verification | Code merges; user runs `run.ps1`; gets cgo errors → wasted slice. |
| **Sandbox-state amnesia** | AI assumes a previous session's edit persists | Files outside `.lovable/` revert; AI re-edits the same line and reports "fixed" twice. See `mem://workspace-revert-on-resume`. |
| **Version-bump skip** | Code change without bumping `cmd/email-read/main.go` `Version` (currently `0.28.0`) | Violates explicit user preference (logged in `.lovable/strictly-avoid.md`). |
| **Boilerplate epilogue** | "Do you have any questions?" suffix | Explicit user pref violation. |

### Why failures occur

1. **Cross-file contracts are large but implicit.** 186 AC rows across 5 spec families; only 96 have citing tests. A new AI has no fast way to know which contract a given file participates in beyond grepping.
2. **Two parallel realities.** Sandbox cannot build Fyne; sandbox can build everything else. Specs do not flag which slices are sandbox-feasible.
3. **Skipped tests look like passing tests.** Without the *honest-scope* rule (already in `mem://decisions/06-ac-coverage-rollout-pattern`), a fresh AI will happily cite AC IDs from inside `t.Skip` blocks.
4. **Ambiguity in 5 spec deltas:**
   - `WatchEvents` (table) vs `WatchEvent` (event type) — shape lock exists in `mem://design/schema-naming-convention` but isn't restated in `spec/23-app-database/01-schema.md` headlines.
   - `Q-OPEN-PRUNE-LAUNCHED` (365d) vs `Q-OPEN-PRUNE-BLOCKED` (90d) — split waiting on the `Decision` column, not flagged in `spec/23-app-database/02-queries.md`.
   - Density preference persistence — feature 07-settings says "deferred"; UI code already has a stub.
   - Six "scheduled" OI rows in `06-tools/99-consistency-report.md` — never resolved or removed.
   - Phase 1+2 status: feature `00-overview.md` table still says "Planned" for all UI columns even though everything shipped (Slices #1-#41).

---

## 3. Corrective actions (priority-ordered, with reliability gain)

| # | Action | Where | Effort | Reliability gain |
|---|---|---|---|---|
| 1 | Grow `06-error-registry.md` to define the 39 referenced ER codes (Settings 217xx, Migrations 218xx, UI 219xx, plus stragglers). | Project-wide error registry | M | +5% (unblocks AC-PROJ-31 deferred-skip → ratchet) |
| 2 | Fix the 33 broken cross-tree spec links surfaced by `Test_NoBrokenSpecLinks_GreenInCi`. | `02-coding-guidelines/`, `08-generic-update/`, `13-cicd-pipeline-workflows/` | M | +5% (unblocks AC-PROJ-33) |
| 3 | Update `spec/21-app/02-features/00-overview.md` UI status column from "Planned" → "✅ Done" for the 7 features. | One file | XS | +3% (eliminates false ambiguity for fresh AI) |
| 4 | Restate `WatchEvents` table-name lock and the `OpenedUrls.Decision` column rollout plan in `spec/23-app-database/01-schema.md` headers. | Two anchor sections | S | +4% (prevents T4 misfires) |
| 5 | In each per-feature `97-acceptance-criteria.md`, add a one-line "Sandbox feasibility" note: 🟢 headless · 🟡 cgo-required · 🔴 needs bench infra. | 7 spec files | S | +6% (lets fresh AI skip T5/T6 cleanly) |
| 6 | Resolve or delete the OI-1..OI-6 rows in `spec/21-app/02-features/06-tools/99-consistency-report.md`. | One file | XS | +2% (closes AC-PROJ-35) |
| 7 | Add a one-paragraph "honest-scope" callout to `mem://decisions/06-ac-coverage-rollout-pattern` showing the **wrong** pattern (citing AC inside `t.Skip`) explicitly. | One memory file | XS | +3% (prevents the most likely T2 regression) |
| 8 | Promote two tripwires from `t.Skip` → `t.Fatal` once items 1+2 land, so the audit catches reintroduction. | `internal/specaudit/ast_project_linters_test.go` | XS | +2% |

Items 1–3 together push T1+T2 success from ~85% → ~92% and unblock 5 AC rows.

---

## 4. Readiness decision

**Ready for:** T1 (spec/doc), T2 (headless scanners), T3 (headless behaviour) — i.e. the entire active *Phase 3 — AC coverage rollout* milestone.

**Not ready for** without infra/spec investment first:
- T4 schema-evolution → needs spec disambiguation (action #4) AND seed data fixture.
- T5 Fyne canvas → needs cgo workstation runner (CI/CD issue #03).
- T6 perf gates → needs bench infra (CI/CD issue #05).
- T7 multi-process E2E → needs E2E harness (tracked in plan).

**Minimum gate to start handing T2 slices to a fresh AI:** the next AI must read, in order:
1. `.lovable/memory/index.md`
2. `.lovable/memory/workflow/01-status.md`
3. `.lovable/memory/decisions/06-ac-coverage-rollout-pattern.md`
4. `.lovable/strictly-avoid.md`
5. This report.

---

## 5. Quick stats

- **Spec rows:** 186 AC total (21-app: 225 lines / 23-app-database: 102 / 24-app-design-system-and-ui: 90).
- **Coverage:** 96/186 (51.6%); allowlist 90; 0 stale code refs.
- **Spec debt items:** 39 missing ER codes + 33 broken links + 6 unresolved OI rows = **78** cleanup units.
- **Sandbox-blocked rows:** ~75 (Fyne 50 + perf 11 + schema 12 + E2E 13, with overlap).
- **Sandbox-feasible remaining:** ~22 rows (AC-DS AST 4 + AC-DS long-tail 18).
- **CLI Version constant:** `0.28.0` (in `cmd/email-read/main.go`).
