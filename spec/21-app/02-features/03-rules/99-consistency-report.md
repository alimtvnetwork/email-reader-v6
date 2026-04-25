# 99 — Rules — Consistency Report

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

| Identifier         | 00-overview | 01-backend | 02-frontend | 97-acceptance | Status |
|--------------------|-------------|------------|-------------|---------------|--------|
| `Rule`             | ✅          | ✅         | ✅ (via VM) | ✅            | ✅     |
| `RuleSpec`         | ✅          | ✅         | ✅ (formSpec)| ✅           | ✅     |
| `RuleAction`       | ✅          | ✅         | ✅          | ✅            | ✅     |
| `RuleId`           | ✅ (reserved) | ✅ (reserved) | —      | —             | ✅     |
| `EmailSample`      | ✅          | ✅         | ✅ (DryRun) | ✅            | ✅     |
| `RuleMatch`        | ✅          | ✅         | ✅ (DryRun) | ✅            | ✅     |
| `RuleStat`         | ✅          | ✅ (table) | ✅ (counter)| ✅            | ✅     |
| `RuleWithStat`     | —           | ✅         | ✅ (rules binding) | (covered) | ✅     |
| `EmailTag`         | ✅ (Tag action) | ✅ (table) | ✅ (Tag action) | ✅       | ✅     |
| `core.Rules`       | ✅          | ✅         | ✅ (VM dep) | ✅            | ✅     |

All identifiers PascalCase. No drift.

## 2. Cross-Reference Integrity

| Link source → target                                         | Resolves? |
|--------------------------------------------------------------|-----------|
| `00-overview.md` → `../../07-architecture.md` §4.3           | ✅        |
| `00-overview.md` → `../../06-error-registry.md`              | ✅        |
| `01-backend.md` → `./00-overview.md`                         | ✅        |
| `01-backend.md` → `../../04-coding-standards.md`             | ✅        |
| `01-backend.md` → `06-seedable-config.md` (consolidated)     | ✅        |
| `02-frontend.md` → `./01-backend.md`                         | ✅        |
| `02-frontend.md` → `16-app-design-system-and-ui.md`          | ✅        |
| `97-acceptance-criteria.md` → all three siblings             | ✅        |

## 3. Method Signature Consistency

`List`, `Get`, `Create`, `Update`, `Rename`, `Delete`, `SetEnabled`, `Reorder`, `DryRun`, `BumpStat` signatures are consistent across `01-backend.md` (canonical) and `02-frontend.md` (consumed via `RulesVM.svc`). The overview lists capabilities at user-story granularity; canonical types live in §4 of overview and §2 of backend with no drift. ✅

## 4. Error Code Consistency

| Code  | Defined in 01-backend | Referenced in 97-acceptance | Reserved range 21300–21399 |
|-------|-----------------------|------------------------------|------------------------------|
| 21301 | ✅                    | E-01                         | ✅                           |
| 21302 | ✅                    | E-01                         | ✅                           |
| 21303 | ✅                    | (covered by E-07)            | ✅                           |
| 21310 | ✅                    | E-02                         | ✅                           |
| 21311 | ✅                    | E-02 / F-09 path             | ✅                           |
| 21312 | ✅                    | E-02 / F-18                  | ✅                           |
| 21313 | ✅                    | F-08                         | ✅                           |
| 21314 | ✅                    | F-09                         | ✅                           |
| 21315 | ✅                    | E-02                         | ✅                           |
| 21316 | ✅                    | E-02 / F-17                  | ✅                           |
| 21317 | ✅                    | E-02                         | ✅                           |
| 21318 | ✅                    | E-02                         | ✅                           |
| 21319 | ✅                    | E-05 / F-10 protect          | ✅                           |
| 21320 | ✅                    | F-14                         | ✅                           |
| 21330 | ✅                    | (covered by E-07)            | ✅                           |
| 21331 | ✅                    | (covered by E-07)            | ✅                           |
| 21332 | ✅                    | (covered by E-07)            | ✅                           |
| 21333 | ✅                    | E-03                         | ✅                           |
| 21334 | ✅                    | E-04                         | ✅                           |
| 21340 | ✅                    | (covered by E-02 + F-15)     | ✅                           |
| 21341 | ✅                    | (covered by E-07)            | ✅                           |
| 21350 | ✅                    | E-06 / G-04                  | ✅                           |

No code overlaps with Dashboard (21100–21199) or Emails (21200–21299) per `06-error-registry.md`.

## 5. Performance Budget Consistency

| Budget                          | 00-overview | 01-backend | 02-frontend | 97-acceptance |
|---------------------------------|-------------|------------|-------------|---------------|
| `List` p95 ≤ 20 ms (200 rules)  | ✅          | ✅         | —           | P-01 ✅       |
| Cold mount ≤ 100 ms             | —           | —          | ✅          | P-02 ✅       |
| `Refresh` 200 rules ≤ 40 ms     | —           | —          | ✅          | P-03 ✅       |
| `DryRun` server ≤ 15 ms         | ✅          | ✅         | —           | P-04 ✅       |
| `DryRun` end-to-end ≤ 30 ms     | —           | —          | ✅          | P-04 ✅       |
| Drag feedback ≤ 16 ms           | —           | —          | ✅          | P-05 ✅       |
| Live update ≤ 16 ms             | —           | —          | ✅          | P-06 ✅       |
| Field-error highlight ≤ 16 ms   | —           | —          | ✅          | P-07 ✅       |
| Memory ≤ 3 MB                   | —           | —          | ✅          | P-08 ✅       |
| Slow-log threshold 20 ms        | —           | ✅         | —           | P-09 / G-03 ✅|

All budgets reconcile.

## 6. Architectural Compliance

| Rule                                                             | Status |
|------------------------------------------------------------------|--------|
| `internal/core/rules.go` does not import Fyne                    | ✅     |
| `internal/ui/views/rules.go` is sole Fyne importer for feature   | ✅     |
| VM does not import `internal/rules`, `internal/store`, or `internal/config` | ✅ |
| Rule evaluation lives in `internal/rules` only                   | ✅     |
| Watcher is sole caller of `BumpStat`                             | ✅     |
| Deps injected via constructor                                    | ✅     |
| No CLI ↔ UI imports                                              | ✅     |

DAG (per `07-architecture.md` §3) preserved.

## 7. Coding Standards Compliance

| Rule                                            | Status              |
|-------------------------------------------------|---------------------|
| Method bodies ≤ 15 lines                        | Enforced via Q-01   |
| No `interface{}` / `any`                        | Enforced via Q-02   |
| No hex colors in views                          | Enforced via Q-03   |
| No `os.Exit` / `fmt.Print*`                     | Enforced via Q-04   |
| PascalCase everywhere                           | Enforced via Q-05   |
| `errtrace.Result[T]` returns                    | Specified §1 of 01-backend |
| `errtrace.Wrap(err, "Rules.<Method>")`          | Specified E-07      |
| `errtrace.WithField("Field", ...)` for field errors | Specified §7 of 01-backend |
| No `SELECT *`                                   | Enforced via Q-08 / D-07 |

## 8. Logging Compliance

| Requirement                                   | Status                     |
|-----------------------------------------------|----------------------------|
| All log keys PascalCase                       | ✅ §8 of 01-backend        |
| `TraceId` propagated via `context.Context`    | ✅                         |
| Slow-call WARN at 20 ms                       | ✅ G-03                    |
| `BumpStat` failure stays at WARN              | ✅ G-04                    |
| No PII in logs                                | ✅ G-06                    |
| `RuleDryRun` logs only structural counters    | ✅ G-06                    |

Aligned with `05-logging-strategy.md`.

## 9. Database Compliance

| Requirement                                                | Status |
|------------------------------------------------------------|--------|
| Singular PascalCase table names                            | ✅ D-03 |
| Positive booleans (`Enabled`, never `Disabled`)            | ✅ §4.1 of 00-overview |
| `ON DELETE CASCADE` on `EmailTag.EmailId`                  | ✅ D-04 |
| `BEGIN IMMEDIATE` for cross-table writes (Rename/Delete)   | ✅ D-05 |
| `BumpStat` uses UPSERT (no read-modify-write)              | ✅ D-06 |
| Atomic config writes (temp + rename)                       | ✅ X-03 |
| Migrations idempotent                                      | ✅ D-01 |
| No `SELECT *`                                              | ✅ D-07 |

Aligned with `18-database-conventions.md` and `06-seedable-config.md`.

## 10. Atomicity & Safety

| Operation        | Cross-storage? | Mechanism                                     | Verified by |
|------------------|----------------|-----------------------------------------------|-------------|
| Rename           | config + SQLite | snapshot → SQLite tx → atomic config write → revert on failure | T-06 / X-01 |
| Delete           | config + SQLite | SQLite tx + atomic config write               | X-02        |
| Reorder          | config-only    | temp-file write + `os.Rename`                 | X-03        |
| SetEnabled       | config-only    | idempotent flip                               | X-04        |
| DryRun           | none (read-only)| zero EXEC, zero Save                         | X-05 / F-16 |
| BumpStat         | SQLite-only    | single UPSERT                                 | D-06        |
| Create           | config-only + SQLite empty insert | single SQLite insert + atomic config write | (covered) |
| Update           | config-only    | atomic config write                           | (covered)   |

## 11. Open Issues

None. All findings ✅. **Ambiguity: None. Confidence: Production-Ready.**

---

## 12. Sign-off

| Reviewer       | Date | Result |
|----------------|------|--------|
| Spec author    | 2026-04-25 | ✅ Self-audit clean |
| Architecture   |            |        |
| QA             |            |        |

---

**End of `03-rules/99-consistency-report.md`**
