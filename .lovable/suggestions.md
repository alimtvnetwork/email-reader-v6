# Suggestions

> **Note (Slice #185, 2026-04-27).** This file is the legacy free-form suggestions list. The structured per-suggestion files live under `.lovable/memory/suggestions/`; the index is at `.lovable/memory/suggestions/index.md`. New suggestions should be filed there. This file is kept as historical context only.

## Active Suggestions

### 🔴 HIGH — Rotate seeded credentials (now broader scope)
- **Status:** Open — needs user verdict
- **Priority:** HIGH (security)
- **Owner:** User (password rotation) → AI (code cleanup)
- **Detail:** See `.lovable/memory/suggestions/20260427-155000-rotate-seeded-credentials-broader-scope.md` — Slice #185 audit found PII in 3 locations including a **plaintext IMAP password in `internal/config/seed.go` line 33** that ships in the binary. Legacy spec redacted in #185; source-code rotation needs user verdict (a/b/c policy choice).
- **Original entry:** Filed 2026-04-21 session.

### 🟡 MEDIUM — Add unit tests for rules edge cases
- **Status:** Open — sandbox-doable next slice
- **Priority:** Medium
- **Detail:** `internal/rules/rules_test.go` only has 3 tests (matches/dedupes, no-from-match, empty-pattern). Missing: invalid regex (no-crash), URLs with `&` query params (URL-extract regex correctness), multi-URL emails (open-all vs first-only semantics), case-sensitivity flag behaviour. Slice #186 will pick this up.
- **Added:** 2026-04-21 session.

## Closed (moved to structured `.lovable/memory/suggestions/`)

| Title | Closed by |
|---|---|
| Grow `06-error-registry.md` to unblock AC-PROJ-31 | Open in `mem://suggestions/20260427-141500-...` (blocked on STO/DB human verdict — see `mem://workflow/er-code-gap-audit.md`) |
| Fix ~33 broken cross-tree spec links to unblock AC-PROJ-33 | ✅ Slice #164/#172 — `Test_NoBrokenSpecLinks_GreenInCi` passing |
| Flip OI-1..OI-6 in `06-tools/99-consistency-report.md` | ✅ Slice #182 (audit-only) |
| Implement Fyne UI per `spec/21-app/02-features/` | ✅ Phases 1+2 (Slices #1-#41) |

## Other context

- `mem://suggestions/index` is the canonical index for new structured suggestions.
- `mem://workflow/progress-tracker` is the canonical % done signal.
- `.lovable/cicd-issues/` has 5 documented sandbox-infra blockers (no Go toolchain, workspace revert, Fyne+cgo, no race gate, no bench infra) — all have starter-slice plans in `mem://workflow/{bench,e2e,canvas}-harness-starter.md`.
