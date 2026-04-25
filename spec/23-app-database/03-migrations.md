# 03 — Migrations

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the migration runner contract for `data/emails.db`: file format, ordering, idempotency, checksum verification, rollback policy, and the canonical migration list at v1.0.0.

Cross-references:
- Schema: [`./01-schema.md`](./01-schema.md) (especially §5 `SchemaMigration` table)
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`

---

## 1. Runner Contract

The runner lives in `internal/store/migrate`. Its public API:

```go
package migrate

type Migration struct {
    Version  uint16    // monotonic, starts at 1, no gaps
    Name     string    // e.g. "create-email-and-watchstate"
    SqlUp    string    // single-statement OR ;-separated batch
    Checksum string    // SHA-256(SqlUp), computed at compile time
}

func Apply(ctx context.Context, db *sql.DB, all []Migration) errtrace.Result[Report]

type Report struct {
    Applied     []uint16  // versions applied in this run
    AlreadyHad  []uint16  // versions already present
    NewestKnown uint16    // max(Version) in `all`
    NewestApplied uint16  // max(Version) in DB after run
}
```

`Apply` is the **only** path to DDL. Called once from `internal/store.Open` after PRAGMAs are set, before any feature code runs.

---

## 2. File Format

Each migration is a Go file under `internal/store/migrate/migrations/`:

```
migrations/
  001_create_email_and_watchstate.go
  002_create_openedurl.go
  003_…
```

```go
package migrations

var M001 = migrate.Migration{
    Version: 1,
    Name:    "create-email-and-watchstate",
    SqlUp:   `
        CREATE TABLE Email ( ... );
        CREATE UNIQUE INDEX UX_Email_Alias_MessageId ON Email (Alias, MessageId);
        ...
        CREATE TABLE WatchState ( ... );
    `,
    // Checksum auto-filled by go:generate; verified at runtime.
}
```

**No down-migrations.** Rationale: this is a single-user SQLite app; a failed deploy is fixed forward. See §5.

---

## 3. Ordering Rules

1. `Version` is `uint16`, monotonic, starts at `1`, no gaps. Gaps cause `ER-MIG-21801` at startup.
2. `Apply` sorts `all` by `Version` and applies in order.
3. Each migration runs in its own `BEGIN IMMEDIATE … COMMIT` tx. A failure rolls the tx back and aborts the whole run with `ER-MIG-21802`; subsequent migrations are not attempted.
4. After successful tx commit, the runner inserts a `SchemaMigration` row in a separate tx. If process dies between the two txs, the next start re-applies the migration — therefore migrations MUST be **idempotent** (use `CREATE TABLE IF NOT EXISTS`, etc.).

---

## 4. Checksum Verification

On every startup, after reading `SchemaMigration`, the runner compares each stored `Checksum` to the in-memory migration's checksum.

| Outcome | Action | Code |
|---|---|---|
| Match | Continue | — |
| Mismatch | Abort startup, surface as fatal app error | `ER-MIG-21803` |
| Stored row but no in-memory migration with that `Version` | Abort (downgrade detected) | `ER-MIG-21804` |

This catches accidental in-place edits to a shipped migration. The fix is always: write a NEW migration with a higher `Version`.

---

## 5. Rollback Policy

**No automated rollback.** The runner has no `SqlDown`. If a migration is bad:

1. Ship a fix-forward migration that repairs the schema.
2. For dataloss bugs: surface a one-time recovery flow in the app (out of scope here; tracked under issues).

The user's data file is recoverable via the WAL + the `email/<Alias>/...` `.eml` archive — the DB can be rebuilt from the `.eml` files plus `config.json`.

---

## 6. Canonical Migration List (v1.0.0)

| Version | Name | Effect |
|---|---|---|
| 1 | `create-email-and-watchstate` | Creates `Email`, `WatchState`, all their indexes. Per `01-schema.md` §2, §3. |
| 2 | `create-openedurl` | Creates `OpenedUrl` and its indexes. Per `01-schema.md` §4. |
| 3 | `create-schemamigration` | Self-bootstrap — see §7. |
| 4 | `rename-legacy-plurals` | If legacy tables `Emails` / `OpenedUrls` exist (from `legacy/spec.md` §7 era), `ALTER TABLE … RENAME TO …`. Idempotent: skip when source table absent. |

Total at v1.0.0: 4 migrations. Each has a Go file, a `*_test.go` that runs it against an empty DB and asserts the schema, and an entry in `internal/store/migrate/migrations.go` that exports `All = []Migration{M001, M002, M003, M004}`.

---

## 7. Self-Bootstrap

The `SchemaMigration` table is itself created by an unconditional `CREATE TABLE IF NOT EXISTS` issued by the runner **before** it reads existing rows. This is the only DDL outside a numbered migration. It is byte-identical to `M003.SqlUp` so subsequent re-application is a no-op.

---

## 8. Concurrency

- The migration runner takes a `BEGIN IMMEDIATE` lock per migration; if another process holds the database, it waits up to `busy_timeout` (5 s) then fails with `ER-MIG-21805`.
- Mailpulse is a single-user desktop app — concurrent runners are not an expected case but the lock is correct-by-construction.

---

## 9. Error Registry — Block 21800–21809

| Code | Meaning | Recovery |
|---|---|---|
| `ER-MIG-21800` | Reserved (sentinel) | — |
| `ER-MIG-21801` | Version gap detected | Fix migration list at compile time |
| `ER-MIG-21802` | Migration tx failed | Read SQL error in surrounding log entry; fix forward |
| `ER-MIG-21803` | Checksum mismatch | Never edit a shipped migration; write a new one |
| `ER-MIG-21804` | Downgrade detected | User opened a newer DB with an older binary — refuse to start, ask to upgrade |
| `ER-MIG-21805` | Busy timeout acquiring lock | Close other instance; retry |
| `ER-MIG-21806..21809` | Reserved | — |

Block 21800–21809 is reserved exclusively for the migration runner.

---

## 10. Logging

Per `05-logging-strategy.md`. Each migration emits at INFO:

```
component=migrate version=2 name=create-openedurl duration_ms=14 rows_changed=0
```

A successful run summary is emitted once at startup:

```
component=migrate event=apply_complete applied=[2,3] already_had=[1] newest=3
```

No SQL strings are ever logged at INFO+ (they may contain table names that match user-supplied aliases).

---

## 11. Testing Contract

Tests live in `internal/store/migrate/migrate_test.go`. 14 required cases:

1. Apply on empty DB applies all migrations in order.
2. Apply on up-to-date DB is a no-op (`Report.Applied == nil`).
3. Apply with a version gap returns `ER-MIG-21801`.
4. Apply with a tampered checksum returns `ER-MIG-21803`.
5. Apply with a stored unknown version returns `ER-MIG-21804`.
6. Apply mid-tx failure leaves the DB at the prior version (no partial row in `SchemaMigration`).
7. Apply after a partial run (DB has migration applied but no `SchemaMigration` row) re-applies idempotently.
8. M004 on a DB with legacy `Emails`/`OpenedUrls` renames them.
9. M004 on a DB without legacy tables is a no-op.
10. Concurrent `Apply` × 2 via two processes: one wins, the other gets `ER-MIG-21805`.
11. After full run, `EXPLAIN QUERY PLAN` for every `Q-*` matches the golden snapshot in `02-queries.md` §4.
12. Migration files lint-clean (no `interface{}`, no `any`, no `fmt.Errorf` per coding standards).
13. AST scan: `internal/store/migrate` is the only package importing a SQL driver (other than `internal/store` itself).
14. AST scan: no production package executes `CREATE`, `ALTER`, or `DROP` SQL outside `internal/store/migrate`.
