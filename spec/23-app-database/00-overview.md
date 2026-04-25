# App Database

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Keywords

`app-database` · `schema` · `migrations` · `queries` · `data-model` · `sqlite` · `pascalcase`

---

## Scoring

| Criterion | Status |
|-----------|--------|
| `00-overview.md` present | ✅ |
| AI Confidence assigned | ✅ |
| Ambiguity assigned | ✅ |
| Keywords present | ✅ |
| Scoring table present | ✅ |

---

## Purpose

Authoritative, app-specific database specification for the Mailpulse desktop+CLI app. Defines:

- The single SQLite database file and its file-system contract.
- Every table with full PascalCase column list, types, constraints, and indexes.
- Every named query (`Q-*`) referenced by a feature backend spec.
- Migration ordering and idempotency rules.
- Retention / vacuum policy.
- The seam between `internal/store` (the only package allowed to touch SQL) and feature backends.

This complements the cross-cutting guidelines in `spec/12-consolidated-guidelines/18-database-conventions.md` and the engine choice notes in `spec/05-split-db-architecture/`.

---

## Document Inventory

| # | File | Purpose |
|---|------|---------|
| 1 | [`01-schema.md`](./01-schema.md) | Every table, column, type, constraint, index. PascalCase. |
| 2 | [`02-queries.md`](./02-queries.md) | Every named query (`Q-*`) with parameters, projection, and call site. |
| 3 | [`03-migrations.md`](./03-migrations.md) | Migration runner contract, ordering rules, idempotency, rollback policy. |
| 4 | [`04-retention-and-vacuum.md`](./04-retention-and-vacuum.md) | Lifetime, archive, prune, and `VACUUM` schedule. |
| 5 | [`97-acceptance-criteria.md`](./97-acceptance-criteria.md) | Binary, machine-checkable acceptance criteria. |
| 6 | [`99-consistency-report.md`](./99-consistency-report.md) | Cross-checks against feature backend specs. |

---

## File-System Contract

| Item | Value |
|---|---|
| Database file | `data/emails.db` (relative to current working directory) |
| WAL files | `data/emails.db-wal`, `data/emails.db-shm` (auto-created by SQLite) |
| Engine | SQLite 3.40+ via `modernc.org/sqlite` (pure Go, no CGO) |
| Journal mode | `WAL` (`PRAGMA journal_mode = WAL` on first open) |
| Synchronous | `NORMAL` (`PRAGMA synchronous = NORMAL`) |
| Foreign keys | `ON` (`PRAGMA foreign_keys = ON` on every connection) |
| Busy timeout | `5000` ms (`PRAGMA busy_timeout = 5000`) |
| Encoding | `UTF-8` (default) |
| Created by | `internal/store.Open` — only call site allowed to issue `CREATE TABLE` / migration SQL |

---

## Ownership

- **`internal/store`** is the **only** package permitted to import a SQL driver (`modernc.org/sqlite`) or compose SQL strings. Verified by AST scan in `97-acceptance-criteria.md`.
- Feature backends (`internal/core/*`) call typed methods on `store.Store` (e.g. `store.InsertOpenedUrl(ctx, OpenedUrl) errtrace.Result[int64]`). They never see `*sql.DB`, `*sql.Tx`, or raw rows.
- The migration runner (`internal/store/migrate`) is the only code permitted to issue DDL.

---

## Cross-References

- Conventions: [`spec/12-consolidated-guidelines/18-database-conventions.md`](../12-consolidated-guidelines/18-database-conventions.md) (singular PascalCase tables, FK rules, positive booleans)
- Engine partitioning: [`spec/05-split-db-architecture/00-overview.md`](../05-split-db-architecture/00-overview.md)
- Feature backends that consume the DB:
  - Emails: [`spec/21-app/02-features/02-emails/01-backend.md`](../21-app/02-features/02-emails/01-backend.md)
  - Watch: [`spec/21-app/02-features/05-watch/01-backend.md`](../21-app/02-features/05-watch/01-backend.md)
  - Tools: [`spec/21-app/02-features/06-tools/01-backend.md`](../21-app/02-features/06-tools/01-backend.md)
  - Dashboard: [`spec/21-app/02-features/01-dashboard/01-backend.md`](../21-app/02-features/01-dashboard/01-backend.md)
- Legacy reference (superseded by `01-schema.md`): [`spec/21-app/legacy/spec.md`](../21-app/legacy/spec.md) §7

---

*App database overview — rewritten 2026-04-25*
