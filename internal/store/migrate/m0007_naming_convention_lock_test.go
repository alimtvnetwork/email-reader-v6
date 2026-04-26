// m0007_naming_convention_lock_test.go locks the doc-only-migration
// contract for Phase 2.1's schema-naming verdict slot:
//
//   - Version 7 is registered with Name "naming_convention_lock".
//   - The migration uses `Up` (not `UpFunc`) and is a no-op SELECT.
//   - Apply records exactly one ledger row for v=7.
//   - Apply does NOT create, drop, or alter any user-visible object
//     (the schema fingerprint before and after Apply is identical
//     except for the `_SchemaVersion` ledger insert).
//
// This guards against accidentally turning the slot into a real
// migration via a future copy-paste edit — if someone adds DDL here,
// `TestM0007_DoesNotMutateUserSchema` will catch it.
package migrate

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0007_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[7]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 7 not registered (m0007_naming_convention_lock.go init missing?)")
	}
	if m.Name != "naming_convention_lock" {
		t.Errorf("Name = %q, want %q", m.Name, "naming_convention_lock")
	}
	if m.UpFunc != nil {
		t.Error("UpFunc must be nil — slot is doc-only, not imperative")
	}
	if m.Up == "" {
		t.Error("Up must be a no-op SQL statement (e.g. SELECT 1), not empty")
	}
}

func TestM0007_DoesNotMutateUserSchema(t *testing.T) {
	// Run only m0007 (not the full stack — that's covered by
	// TestMigrations_FullStack_Idempotent in internal/store/). The
	// fingerprint before and after Apply must be identical EXCEPT
	// for the `_SchemaVersion` bookkeeping table that Apply itself
	// always creates.
	saved := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, saved) })

	registryMu.Lock()
	registry = map[int]Migration{7: saved[7]}
	registryMu.Unlock()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Inventory all user-visible tables/views/indexes after Apply.
	// Excludes `_SchemaVersion` (Apply's own bookkeeping) and
	// `sqlite_*` internals.
	objects := userObjects(t, db)
	if len(objects) != 0 {
		t.Errorf("m0007 created user-visible schema objects (must be doc-only, no DDL): %s",
			strings.Join(objects, ", "))
	}

	// Ledger row must exist exactly once.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _SchemaVersion WHERE Version=7`).Scan(&n); err != nil {
		t.Fatalf("ledger count: %v", err)
	}
	if n != 1 {
		t.Errorf("ledger row count for v7 = %d, want 1", n)
	}
}

func TestM0007_IsIdempotent_ViaApply(t *testing.T) {
	saved := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, saved) })

	registryMu.Lock()
	registry = map[int]Migration{7: saved[7]}
	registryMu.Unlock()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for pass := 1; pass <= 2; pass++ {
		if err := Apply(context.Background(), db); err != nil {
			t.Fatalf("Apply pass %d: %v", pass, err)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _SchemaVersion WHERE Version=7`).Scan(&n); err != nil {
		t.Fatalf("ledger count: %v", err)
	}
	if n != 1 {
		t.Fatalf("ledger row count for v7 = %d, want 1 (idempotence violated)", n)
	}
}

// userObjects returns the sorted list of `type:name` for every
// object in `sqlite_master` that isn't `_SchemaVersion` or a
// `sqlite_*` internal. Used to assert m0007 is truly DDL-free.
func userObjects(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(
		`SELECT type, name FROM sqlite_master
         WHERE name NOT LIKE 'sqlite_%' AND name != '_SchemaVersion'
         ORDER BY type, name`)
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var typ, name string
		if err := rows.Scan(&typ, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, typ+":"+name)
	}
	sort.Strings(out)
	return out
}
