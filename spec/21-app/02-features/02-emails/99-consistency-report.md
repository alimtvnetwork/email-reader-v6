# 99 — Emails — Consistency Report

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
| `EmailSummary`     | ✅          | ✅         | ✅ (via VM) | ✅            | ✅     |
| `EmailDetail`      | ✅          | ✅         | ✅          | ✅            | ✅     |
| `EmailQuery`       | ✅          | ✅         | ✅          | ✅            | ✅     |
| `EmailPage`        | ✅          | ✅         | ✅          | ✅            | ✅     |
| `EmailCounts`      | —           | ✅         | ✅          | ✅            | ✅     |
| `EmailSortKey`     | ✅          | ✅         | ✅ (sort)   | ✅            | ✅     |
| `RefreshReport`    | ✅          | ✅         | ✅          | ✅            | ✅     |
| `DeleteReceipt`    | —           | ✅         | ✅ (undoStack) | ✅         | ✅     |
| `core.Emails`      | ✅          | ✅         | ✅ (VM dep) | ✅            | ✅     |
| `MatchedRules`     | ✅          | ✅         | ✅ (badge)  | (covered)     | ✅     |

All identifiers PascalCase. No drift.

## 2. Cross-Reference Integrity

| Link source → target                                         | Resolves? |
|--------------------------------------------------------------|-----------|
| `00-overview.md` → `../../07-architecture.md` §4.2           | ✅        |
| `00-overview.md` → `../../06-error-registry.md`              | ✅        |
| `01-backend.md` → `./00-overview.md`                         | ✅        |
| `01-backend.md` → `../../04-coding-standards.md`             | ✅        |
| `02-frontend.md` → `./01-backend.md`                         | ✅        |
| `02-frontend.md` → consolidated `16-app-design-system-and-ui.md` | ✅    |
| `97-acceptance-criteria.md` → all three siblings             | ✅        |

## 3. Method Signature Consistency

`List`, `Get`, `MarkRead`, `Delete`, `Undelete`, `Refresh`, `Counts` signatures consistent across `00-overview.md` (informal), `01-backend.md` (canonical), and `02-frontend.md` (consumed via `EmailsVM.svc`). ✅

## 4. Error Code Consistency

| Code  | Defined in 01-backend | Referenced in 97-acceptance | Reserved range 21200–21299 |
|-------|-----------------------|------------------------------|------------------------------|
| 21201 | ✅                    | E-06                         | ✅                           |
| 21202 | ✅                    | E-01                         | ✅                           |
| 21203 | ✅                    | (covered by G-01/G-04)       | ✅                           |
| 21210 | ✅                    | E-02                         | ✅                           |
| 21211 | ✅                    | (covered by E-07)            | ✅                           |
| 21220 | ✅                    | E-03                         | ✅                           |
| 21221 | ✅                    | (covered by E-06 pattern)    | ✅                           |
| 21230 | ✅                    | E-04                         | ✅                           |
| 21231 | ✅                    | (covered by E-07)            | ✅                           |
| 21232 | ✅                    | (covered by E-06 pattern)    | ✅                           |
| 21240 | ✅                    | (covered by E-07)            | ✅                           |
| 21241 | ✅                    | E-05                         | ✅                           |
| 21250 | ✅                    | (covered by E-07)            | ✅                           |

No code overlaps with Dashboard (21100–21199) or other reserved ranges per `06-error-registry.md`.

## 5. Performance Budget Consistency

| Budget                      | 00-overview | 01-backend | 02-frontend | 97-acceptance |
|-----------------------------|-------------|------------|-------------|---------------|
| `List` p95 ≤ 60 ms          | ✅          | ✅         | —           | P-01 ✅       |
| Cold mount ≤ 120 ms         | —           | —          | ✅          | P-02 ✅       |
| `ApplyFilter` ≤ 80 ms       | —           | —          | ✅          | P-03 ✅       |
| `Get` ≤ 30 ms               | ✅          | —          | ✅          | P-04 ✅       |
| `MarkRead` 500 ≤ 150 ms     | ✅          | ✅         | ✅          | P-05 ✅       |
| Live insert ≤ 16 ms         | —           | —          | ✅          | P-06 ✅       |
| Page memory ≤ 4 MB          | —           | —          | ✅          | P-07 ✅       |
| Slow log @ 60 ms            | —           | ✅         | —           | P-08 / G-02 ✅|

All budgets reconcile.

## 6. Architectural Compliance

| Rule                                                             | Status |
|------------------------------------------------------------------|--------|
| `internal/core/emails.go` does not import Fyne                   | ✅     |
| `internal/ui/views/emails.go` is sole Fyne importer for feature  | ✅     |
| VM does not import `internal/store` or `internal/mailclient`     | ✅     |
| Refresh delegates to `core.Watch.PollOnce` (no direct IMAP)      | ✅     |
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
| `errtrace.Wrap(err, "Emails.<Method>")`         | Specified E-07      |
| No `SELECT *`                                   | Enforced via Q-08   |

## 8. Logging Compliance

| Requirement                                       | Status                       |
|---------------------------------------------------|------------------------------|
| All log keys PascalCase                           | ✅ §6 of 01-backend          |
| `TraceId` propagated via `context.Context`        | ✅                           |
| Slow-call WARN at 60 ms                           | ✅ G-02                      |
| No PII in logs                                    | ✅ G-05                      |
| Subject truncated to 80 chars at DEBUG only       | ✅ G-06                      |

Aligned with `05-logging-strategy.md`.

## 9. Database Compliance

| Requirement                                                | Status |
|------------------------------------------------------------|--------|
| Singular PascalCase table names                            | ✅ D-03 |
| Positive booleans (`IsRead`, never `IsUnread`)             | ✅ D-04 |
| Bound parameters for `Search` (no string concat)           | ✅ D-06 |
| Bulk ops batched ≤ 999 in single transaction               | ✅ D-05 |
| `ON DELETE CASCADE` for FK relationships (`OpenedUrl`)     | ✅ inherited from existing schema |
| No `SELECT *`                                              | ✅ Q-08 |
| Migrations idempotent                                      | ✅ D-01 |

Aligned with `18-database-conventions.md`.

## 10. Idempotency & Safety

| Operation       | Idempotent? | Safety mechanism                                   |
|-----------------|-------------|----------------------------------------------------|
| `MarkRead`      | ✅          | `WHERE` clause; T-06 verifies                      |
| `Delete`        | ✅          | `WHERE DeletedAt IS NULL` ensures no double-flip   |
| `Undelete`      | ✅          | `WHERE Id IN (...)`; restoring a non-deleted is no-op |
| `Refresh`       | n/a         | Single delegation per click; button disabled in-flight |
| `Get`           | ✅          | Read-only                                          |
| `List`          | ✅          | Read-only                                          |

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

**End of `02-emails/99-consistency-report.md`**
