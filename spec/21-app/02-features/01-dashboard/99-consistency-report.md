# 99 — Dashboard — Consistency Report

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Audits this feature's four spec files against the project-wide guidelines and against each other. Any ❌ finding blocks merge until resolved.

Files audited:
1. [`./00-overview.md`](./00-overview.md)
2. [`./01-backend.md`](./01-backend.md)
3. [`./02-frontend.md`](./02-frontend.md)
4. [`./97-acceptance-criteria.md`](./97-acceptance-criteria.md)

---

## 1. Naming Consistency

| Identifier              | 00-overview | 01-backend | 02-frontend | 97-acceptance | Status |
|-------------------------|-------------|------------|-------------|---------------|--------|
| `DashboardSummary`      | ✅          | ✅         | ✅ (via VM) | ✅            | ✅     |
| `ActivityRow`           | ✅          | ✅         | ✅          | ✅            | ✅     |
| `AccountHealthRow`      | ✅          | ✅         | ✅          | ✅            | ✅     |
| `HealthLevel`           | ✅          | ✅         | ✅          | ✅            | ✅     |
| `ActivityKind`          | ✅          | ✅         | ✅          | ✅            | ✅     |
| `core.Dashboard`        | ✅          | ✅         | ✅ (VM dep) | ✅            | ✅     |
| `WatchEvent`            | ✅          | ✅         | ✅          | ✅            | ✅     |

All identifiers are PascalCase. No drift.

## 2. Cross-Reference Integrity

| Link source → target                                    | Resolves? |
|---------------------------------------------------------|-----------|
| `00-overview.md` → `../../07-architecture.md` §4.1      | ✅        |
| `00-overview.md` → `../../06-error-registry.md`         | ✅        |
| `01-backend.md` → `./00-overview.md`                    | ✅        |
| `01-backend.md` → `../../04-coding-standards.md`        | ✅        |
| `02-frontend.md` → `./01-backend.md`                    | ✅        |
| `97-acceptance-criteria.md` → all three siblings        | ✅        |

## 3. Method Signature Consistency

`Summary`, `RecentActivity`, `AccountHealth` signatures are identical across `00-overview.md`, `01-backend.md`, and `02-frontend.md` (consumed via `DashboardVM.svc`). ✅

## 4. Error Code Consistency

| Code  | Defined in 01-backend | Referenced in 97-acceptance | Reserved range 21100–21199 |
|-------|-----------------------|------------------------------|------------------------------|
| 21100 | ✅                    | (not user-facing)            | ✅                           |
| 21101 | ✅                    | E-01, E-02                   | ✅                           |
| 21102 | ✅                    | E-03                         | ✅                           |
| 21103 | ✅                    | (covered by E-04)            | ✅                           |
| 21104 | ✅                    | (covered by E-04)            | ✅                           |
| 21105 | ✅                    | (covered by E-04)            | ✅                           |

No code overlaps with other features (Emails 21200–21299, Rules 21300–21399, etc., per `06-error-registry.md`).

## 5. Performance Budget Consistency

| Budget                  | 00-overview | 01-backend | 02-frontend | 97-acceptance |
|-------------------------|-------------|------------|-------------|---------------|
| Cold mount < 100 ms     | ✅          | —          | ✅          | P-02 ✅       |
| Live insert < 16 ms     | ✅          | —          | ✅          | P-04 ✅       |
| `Summary` p95 ≤ 40 ms   | —           | ✅         | —           | P-01 ✅       |
| Refresh ≤ 50 ms         | —           | —          | ✅          | P-03 ✅       |
| Slow log > 100 ms       | —           | ✅         | —           | P-06 / G-02 ✅|

All budgets reconcile.

## 6. Architectural Compliance

| Rule                                                         | Status |
|--------------------------------------------------------------|--------|
| `internal/core/dashboard.go` does not import Fyne            | ✅     |
| `internal/ui/views/dashboard.go` is sole Fyne importer       | ✅     |
| VM does not import `internal/store` or `internal/mailclient` | ✅     |
| No direct CLI ↔ UI imports                                   | ✅     |
| All deps injected via constructor                            | ✅     |

DAG (per `07-architecture.md` §3) preserved.

## 7. Coding Standards Compliance

| Rule                                            | Status |
|-------------------------------------------------|--------|
| Method bodies ≤ 15 lines                        | Enforced via Q-01 |
| No `interface{}` / `any`                        | Enforced via Q-02 |
| No hex colors in views                          | Enforced via Q-03 |
| No `os.Exit` / `fmt.Print*`                     | Enforced via Q-04 |
| PascalCase everywhere                           | Enforced via Q-05 |
| `errtrace.Result[T]` returns                    | Specified in 01-backend §1 |
| `errtrace.Wrap(err, "Dashboard.<Method>")`      | Specified E-04 |

## 8. Logging Compliance

| Requirement                                  | Status |
|----------------------------------------------|--------|
| All log keys PascalCase                      | ✅ §5 of 01-backend |
| `TraceId` propagated via `context.Context`   | ✅ §5 of 01-backend |
| Slow-call WARN at 100 ms                     | ✅ G-02 |
| No PII in logs                               | ✅ G-04 |

Aligned with `05-logging-strategy.md`.

## 9. Database Compliance

| Requirement                                                | Status |
|------------------------------------------------------------|--------|
| Singular PascalCase table names                            | ✅ D-03 |
| Positive booleans (`IsRead`, not `Unread`)                 | ✅ §3.1 of 01-backend |
| `ON DELETE CASCADE` declared where FK relationships exist  | ✅ inherits from existing schema |
| No `SELECT *`                                              | ✅ D-04 |
| Migrations idempotent                                      | ✅ D-01 |

Aligned with `18-database-conventions.md`.

## 10. Open Issues

None. All findings ✅. **Ambiguity: None. Confidence: Production-Ready.**

---

## 11. Sign-off

| Reviewer       | Date | Result |
|----------------|------|--------|
| Spec author    | 2026-04-25 | ✅ Self-audit clean |
| Architecture   |            |        |
| QA             |            |        |

---

**End of `01-dashboard/99-consistency-report.md`**
