// m0008_add_watch_events_test.go locks the P1.13 contract:
//
//   - The migration registers at Version 8 with Name "add_watch_events".
//   - After Apply, `WatchEvents` table exists with the documented
//     PascalCase columns (Id, Alias, Kind, Payload, OccurredAt).
//   - The composite (Alias, OccurredAt) index exists.
//   - Apply is idempotent for this slice (re-run = no-op).
//
// Uses the same in-memory SQLite fixture as `migrate_test.go`.
//
// IMPORTANT: this test is `package migrate` (white-box) so it can
// inspect the registry without going through `Reset()` — we want to
// verify the production `init()` registration, not a fixture.
package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0008_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[8]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 8 not registered (m0008_add_watch_events.go init missing?)")
	}
	if m.Name != "add_watch_events" {
		t.Errorf("Name = %q, want %q", m.Name, "add_watch_events")
	}
	if m.Up == "" {
		t.Error("Up SQL is empty")
	}
	if m.UpFunc != nil {
		t.Error("UpFunc must be nil for plain DDL migration")
	}
}

func TestM0008_AppliesSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Apply only m0008 against a clean DB. We don't go through the
	// full `Apply` path (which would also run m0001..m0007) because
	// m0008 has no schema dependencies — the table is self-contained.
	registryMu.RLock()
	m := registry[8]
	registryMu.RUnlock()
	if _, err := db.ExecContext(context.Background(), m.Up); err != nil {
		t.Fatalf("apply m0008: %v", err)
	}

	// Table shape: every documented column is present with the right
	// type. We use PRAGMA table_info because it returns a stable
	// (cid, name, type, notnull, dflt_value, pk) row shape.
	wantCols := map[string]string{
		"Id":         "INTEGER",
		"Alias":      "TEXT",
		"Kind":       "INTEGER",
		"Payload":    "TEXT",
		"OccurredAt": "DATETIME",
	}
	rows, err := db.Query(`PRAGMA table_info(WatchEvents)`)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	gotCols := map[string]string{}
	for rows.Next() {
		var (
			cid             int
			name, typ       string
			notnull         int
			dflt            sql.NullString
			pk              int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		gotCols[name] = typ
	}
	for name, want := range wantCols {
		if got := gotCols[name]; got != want {
			t.Errorf("column %s: type=%q, want %q", name, got, want)
		}
	}

	// Index presence — name must match the one referenced in the
	// migration doc-comment so doc/code drift is caught here.
	var idxName string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='IxWatchEventsAliasOccurredAt'`,
	).Scan(&idxName); err != nil {
		t.Fatalf("index missing: %v", err)
	}

	// Default for Payload must be the empty JSON object `{}` so
	// readers can `json.Unmarshal` without a nullability check.
	var dflt sql.NullString
	if err := db.QueryRow(
		`SELECT dflt_value FROM pragma_table_info('WatchEvents') WHERE name='Payload'`,
	).Scan(&dflt); err != nil {
		t.Fatalf("read Payload default: %v", err)
	}
	if !dflt.Valid || dflt.String != `'{}'` {
		t.Errorf("Payload default = %q, want %q", dflt.String, `'{}'`)
	}
}

func TestM0008_IsIdempotent_ViaApply(t *testing.T) {
	// Use the full `Apply` path so the ledger short-circuit covers
	// m0008 just like every other migration. Reset the registry to
	// only m0008 so we're not coupled to m0001..m0006 schema details.
	saved := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, saved) })

	registryMu.Lock()
	registry = map[int]Migration{8: saved[8]}
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
	if err := db.QueryRow(`SELECT COUNT(*) FROM _SchemaVersion WHERE Version=8`).Scan(&n); err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if n != 1 {
		t.Fatalf("ledger row count for v8 = %d, want 1", n)
	}
}

// snapshotRegistry / restoreRegistry let one test temporarily isolate
// a single migration without losing the production `init()`-registered
// set for sibling tests in the same binary.
func snapshotRegistry(t *testing.T) map[int]Migration {
	t.Helper()
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make(map[int]Migration, len(registry))
	for k, v := range registry {
		out[k] = v
	}
	return out
}

func restoreRegistry(t *testing.T, snap map[int]Migration) {
	t.Helper()
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[int]Migration, len(snap))
	for k, v := range snap {
		registry[k] = v
	}
}
