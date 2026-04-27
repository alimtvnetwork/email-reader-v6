// Package migrate is the typed schema-migration harness for the SQLite
// store. It owns:
//
//   - the `Migration` value type (Version + Name + Up SQL),
//   - an ordered registry (`All()`), populated by sibling files
//     `m0001_*.go` … `m000N_*.go` via `Register(Migration)` in their
//     `init()` blocks (added in slice P1.11),
//   - an idempotent `Apply(ctx, db)` entry-point that creates a
//     bookkeeping table `_SchemaVersion` (PascalCase house style; see
//     mem://workflow/phase1-plan.md "Open decisions before P1.10"),
//     records each successful migration's `Version`, and skips any
//     versions already present.
//
// Design decisions (locked at P1.10 land):
//
//   - **Ledger table name:** `_SchemaVersion`. Leading underscore
//     marks it as bookkeeping (won't show up in business queries),
//     PascalCase matches every other table in the schema.
//   - **No `Down` field.** YAGNI — we have zero reversible migrations
//     today. A future slice can add it without breaking call-sites
//     (struct literals here use named fields).
//   - **No transaction-per-migration wrapper yet.** SQLite's DDL is
//     transactional but `ALTER TABLE ADD COLUMN` interacts poorly with
//     long-held writer locks (see vacuum.go discussion). We re-evaluate
//     in P1.13 when the first non-DDL migration lands.
//   - **Error wrapping:** every failure is wrapped with
//     `errtrace.Wrap` / `Wrapf`, matching the existing store-package
//     convention (`store.go:62-70`). The dedicated `ErrDbMigrate` code
//     in `internal/errtrace/codes.go:26` is reserved for the typed
//     `WrapCode` migration introduced in slice P1.4.
//
// Spec: mem://workflow/phase1-plan.md (Block C, slice P1.10).
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"

	"github.com/lovable/email-read/internal/errtrace"
)

// Migration describes one ordered, forward-only schema change.
//
// Version is a strictly monotonic positive integer. The registry
// rejects duplicates and rejects zero/negative values.
//
// Name is a short snake_case identifier (e.g. "initial_schema",
// "add_watch_event"). Used in error messages and the ledger.
//
// Exactly one of Up or UpFunc must be non-zero:
//
//   - Up: SQL executed verbatim by `db.ExecContext`. Use for plain
//     idempotent DDL (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX
//     IF NOT EXISTS`). Most migrations should use this form.
//   - UpFunc: imperative migration that needs runtime introspection
//     (e.g. SQLite's `ALTER TABLE ADD COLUMN`, which has no `IF NOT
//     EXISTS` form and errors on duplicate columns). The function
//     receives the same `*sql.DB` Apply was given and the same ctx.
//     UpFunc MUST be self-idempotent — see m0005 for the canonical
//     PRAGMA-table_info pattern.
//
// Setting both is a programmer error and panics in Register.
type Migration struct {
	Version int
	Name    string
	Up      string
	UpFunc  func(ctx context.Context, db *sql.DB) error
}

// registry holds the ordered set of migrations. Sibling files under
// this package register into it from their `init()` blocks (added in
// P1.11). Guarded by registryMu so test packages that build a fresh
// in-process registry can do so safely.
var (
	registryMu sync.RWMutex
	registry   = make(map[int]Migration)
)

// Register adds m to the package-level registry. Panics on duplicate
// Version or invalid Version (≤ 0) — both are programmer errors caught
// at process start, never at runtime.
func Register(m Migration) {
	if m.Version <= 0 {
		panic(fmt.Sprintf("migrate.Register: Version must be > 0, got %d (name=%q)", m.Version, m.Name))
	}
	if m.Name == "" {
		panic(fmt.Sprintf("migrate.Register: Name required for Version %d", m.Version))
	}
	hasUp := m.Up != ""
	hasFunc := m.UpFunc != nil
	if hasUp == hasFunc {
		panic(fmt.Sprintf("migrate.Register: Version %d (%s) must set exactly one of Up or UpFunc",
			m.Version, m.Name))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if existing, dup := registry[m.Version]; dup {
		panic(fmt.Sprintf("migrate.Register: duplicate Version %d (existing=%q new=%q)",
			m.Version, existing.Name, m.Name))
	}
	registry[m.Version] = m
}

// All returns a freshly-sorted slice of every registered migration,
// ascending by Version. Returns an empty slice (never nil) when the
// registry is empty — Apply treats that as a no-op.
func All() []Migration {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Migration, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}

// Reset clears the registry. **Test-only**: production code never
// calls this. Exposed (vs. unexported) so external test packages
// (e.g. `migrate_test` black-box tests) can isolate fixtures.
func Reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[int]Migration)
}

// schemaVersionDDL is the bookkeeping table. `IF NOT EXISTS` makes
// Apply safe on every startup (fresh DB and re-opened DB alike). The
// `AppliedAt` column uses SQLite's RFC3339 strftime so the value
// matches the rest of the schema's timestamp convention.
const schemaVersionDDL = `CREATE TABLE IF NOT EXISTS _SchemaVersion (
       Version    INTEGER PRIMARY KEY,
       Name       TEXT    NOT NULL,
       AppliedAt  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
     )`

// schemaVersionSelectAll lists every previously-applied Version.
// We load the full set into memory (it's tiny — N migrations, N < 100)
// instead of `WHERE Version = ?` per row to keep Apply's loop O(N+M)
// not O(N*M).
const schemaVersionSelectAll = `SELECT Version FROM _SchemaVersion`

// schemaVersionInsert records a successful migration. `INSERT OR
// IGNORE` is belt-and-braces against a concurrent Apply (which we
// don't expect — Store.Open is single-threaded — but cheap insurance).
const schemaVersionInsert = `INSERT OR IGNORE INTO _SchemaVersion (Version, Name) VALUES (?, ?)`

// Apply runs every registered migration whose Version is not yet in
// `_SchemaVersion`, in ascending Version order. It is idempotent:
// calling Apply twice in a row is a no-op on the second call.
//
// Apply does NOT wrap each migration in a transaction (see package
// doc). On mid-migration failure the caller observes a wrapped
// errtrace.ErrDbMigrate; the next Apply will re-run that migration's
// Up SQL — so individual migrations MUST be idempotent (`CREATE TABLE
// IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, etc.).
func Apply(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errtrace.Wrap(errtrace.New("nil db"), "migrate.Apply")
	}
	if _, err := db.ExecContext(ctx, schemaVersionDDL); err != nil {
		return errtrace.Wrap(err, "create _SchemaVersion")
	}

	applied, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err // already wrapped
	}

	for _, m := range All() {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		if m.UpFunc != nil {
			if err := m.UpFunc(ctx, db); err != nil {
				return errtrace.Wrapf(err, "apply migration %04d (%s)", m.Version, m.Name)
			}
		} else {
			if _, err := db.ExecContext(ctx, m.Up); err != nil {
				return errtrace.Wrapf(err, "apply migration %04d (%s)", m.Version, m.Name)
			}
		}
		if _, err := db.ExecContext(ctx, schemaVersionInsert, m.Version, m.Name); err != nil {
			return errtrace.Wrapf(err, "record migration %04d (%s)", m.Version, m.Name)
		}
	}
	return nil
}

// loadAppliedVersions reads the set of already-applied migration
// versions. Returned as a `map[int]struct{}` for O(1) membership tests
// in Apply's loop.
func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]struct{}, error) {
	rows, err := db.QueryContext(ctx, schemaVersionSelectAll)
	if err != nil {
		return nil, errtrace.Wrap(err, "load _SchemaVersion")
	}
	defer rows.Close()

	out := make(map[int]struct{})
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, errtrace.Wrap(err, "scan _SchemaVersion row")
		}
		out[v] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, errtrace.Wrap(err, "iterate _SchemaVersion")
	}
	return out, nil
}

// fmt is imported for Register's panic messages above.
var _ = fmt.Sprintf
