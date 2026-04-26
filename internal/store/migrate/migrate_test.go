// migrate_test.go covers the P1.10 harness contract:
//
//   - Apply on an empty registry creates _SchemaVersion and is a no-op.
//   - Apply runs each registered Migration once, in ascending Version order.
//   - Apply is idempotent: a second call records no new rows and runs no Up SQL.
//   - Register panics on duplicate Version, zero/negative Version, or empty Name.
//   - All() returns migrations sorted by Version regardless of registration order.
//
// We use an in-memory SQLite DB (`:memory:`) per test so registry state
// and DB state stay isolated. Each test calls migrate.Reset() in a
// t.Cleanup so the package-level registry doesn't leak across tests.
package migrate_test

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite" // pure-Go driver, matches store package
	"github.com/lovable/email-read/internal/store/migrate"
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func resetRegistry(t *testing.T) {
	t.Helper()
	migrate.Reset()
	t.Cleanup(migrate.Reset)
}

func countSchemaVersionRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _SchemaVersion`).Scan(&n); err != nil {
		t.Fatalf("count _SchemaVersion: %v", err)
	}
	return n
}

func TestApply_EmptyRegistry_CreatesLedgerAndNoOps(t *testing.T) {
	resetRegistry(t)
	db := newDB(t)

	if err := migrate.Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply on empty registry: %v", err)
	}
	if got := countSchemaVersionRows(t, db); got != 0 {
		t.Fatalf("_SchemaVersion should be empty, got %d rows", got)
	}
}

func TestApply_RunsEachMigrationOnce_InVersionOrder(t *testing.T) {
	resetRegistry(t)
	db := newDB(t)

	// Register out of order to prove All() sorts.
	migrate.Register(migrate.Migration{Version: 3, Name: "third", Up: `CREATE TABLE T3 (Id INTEGER)`})
	migrate.Register(migrate.Migration{Version: 1, Name: "first", Up: `CREATE TABLE T1 (Id INTEGER)`})
	migrate.Register(migrate.Migration{Version: 2, Name: "second", Up: `CREATE TABLE T2 (Id INTEGER)`})

	if err := migrate.Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := countSchemaVersionRows(t, db); got != 3 {
		t.Fatalf("expected 3 ledger rows, got %d", got)
	}

	// Verify ledger order matches registry sort.
	rows, err := db.Query(`SELECT Version, Name FROM _SchemaVersion ORDER BY Version ASC`)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	defer rows.Close()
	want := []struct {
		v    int
		name string
	}{{1, "first"}, {2, "second"}, {3, "third"}}
	i := 0
	for rows.Next() {
		var v int
		var name string
		if err := rows.Scan(&v, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if v != want[i].v || name != want[i].name {
			t.Fatalf("ledger[%d] = (%d,%q), want (%d,%q)", i, v, name, want[i].v, want[i].name)
		}
		i++
	}

	// Verify the actual side-effect tables exist.
	for _, tbl := range []string{"T1", "T2", "T3"} {
		var n string
		if err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&n); err != nil {
			t.Fatalf("table %s missing after Apply: %v", tbl, err)
		}
	}
}

func TestApply_Idempotent_SecondCallSkipsAppliedMigrations(t *testing.T) {
	resetRegistry(t)
	db := newDB(t)

	// Use an Up SQL that would FAIL on a second exec (no IF NOT EXISTS) —
	// proves the harness short-circuits via the ledger, not via SQL idempotency.
	var execCount int32
	migrate.Register(migrate.Migration{
		Version: 1, Name: "create_once",
		Up: `CREATE TABLE OnlyOnce (Id INTEGER PRIMARY KEY)`,
	})

	for pass := 1; pass <= 2; pass++ {
		// Wrap db.ExecContext-equivalent counting via a sentinel: we
		// can't intercept Apply's internal ExecContext, so we instead
		// rely on the fact that re-running `CREATE TABLE OnlyOnce`
		// (without IF NOT EXISTS) raises "table already exists". If
		// the harness short-circuits correctly, pass 2 returns nil.
		if err := migrate.Apply(context.Background(), db); err != nil {
			t.Fatalf("Apply pass %d: %v", pass, err)
		}
		atomic.AddInt32(&execCount, 1)
	}

	if got := countSchemaVersionRows(t, db); got != 1 {
		t.Fatalf("ledger row count after 2x Apply = %d, want 1", got)
	}
}

func TestApply_NewMigrationAddedAfterFirstApply_RunsOnNextApply(t *testing.T) {
	resetRegistry(t)
	db := newDB(t)

	migrate.Register(migrate.Migration{Version: 1, Name: "v1", Up: `CREATE TABLE V1 (Id INTEGER)`})
	if err := migrate.Apply(context.Background(), db); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	migrate.Register(migrate.Migration{Version: 2, Name: "v2", Up: `CREATE TABLE V2 (Id INTEGER)`})
	if err := migrate.Apply(context.Background(), db); err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	if got := countSchemaVersionRows(t, db); got != 2 {
		t.Fatalf("ledger row count = %d, want 2", got)
	}
	// V2 must exist now.
	var n string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='V2'`,
	).Scan(&n); err != nil {
		t.Fatalf("V2 missing after incremental Apply: %v", err)
	}
}

func TestApply_NilDB_ReturnsError(t *testing.T) {
	resetRegistry(t)
	if err := migrate.Apply(context.Background(), nil); err == nil {
		t.Fatal("expected error on nil db, got nil")
	}
}

func TestRegister_RejectsDuplicateVersion(t *testing.T) {
	resetRegistry(t)
	migrate.Register(migrate.Migration{Version: 7, Name: "a", Up: `SELECT 1`})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate Version, got none")
		}
	}()
	migrate.Register(migrate.Migration{Version: 7, Name: "b", Up: `SELECT 1`})
}

func TestRegister_RejectsZeroVersion(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on Version=0, got none")
		}
	}()
	migrate.Register(migrate.Migration{Version: 0, Name: "x", Up: `SELECT 1`})
}

func TestRegister_RejectsEmptyName(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty Name, got none")
		}
	}()
	migrate.Register(migrate.Migration{Version: 1, Name: "", Up: `SELECT 1`})
}

func TestAll_ReturnsSortedCopy(t *testing.T) {
	resetRegistry(t)
	migrate.Register(migrate.Migration{Version: 5, Name: "e", Up: `SELECT 1`})
	migrate.Register(migrate.Migration{Version: 2, Name: "b", Up: `SELECT 1`})
	migrate.Register(migrate.Migration{Version: 9, Name: "i", Up: `SELECT 1`})

	got := migrate.All()
	want := []int{2, 5, 9}
	if len(got) != len(want) {
		t.Fatalf("All() len = %d, want %d", len(got), len(want))
	}
	for i, m := range got {
		if m.Version != want[i] {
			t.Fatalf("All()[%d].Version = %d, want %d", i, m.Version, want[i])
		}
	}
}
