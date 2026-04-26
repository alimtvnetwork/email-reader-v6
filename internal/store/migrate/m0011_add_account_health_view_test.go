// m0011_add_account_health_view_test.go locks the P1.16 contract:
//
//   - Version 11 is registered with Name "add_account_health_view".
//   - After Apply the `v_AccountHealth` view exists and exposes the
//     documented columns (Alias, LastEventId, LastKind, LastOccurredAt,
//     Status).
//   - The view collapses multi-event histories down to ONE row per
//     alias, picking the most-recent event by OccurredAt then Id.
//   - The `Status` mapping matches the doc-comment table
//     (1/4 → ok, 2 → warn, 3 → err).
//   - Apply is idempotent for this slice (re-run = no-op, view stays).
//
// Tests run against an in-memory SQLite DB seeded with the m0008
// `WatchEvents` schema — m0011 has a hard dependency on m0008, so we
// register both and run the full Apply path.
package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0011_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[11]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 11 not registered (m0011_add_account_health_view.go init missing?)")
	}
	if m.Name != "add_account_health_view" {
		t.Errorf("Name = %q, want %q", m.Name, "add_account_health_view")
	}
	if m.Up == "" {
		t.Error("Up SQL is empty")
	}
	if m.UpFunc != nil {
		t.Error("UpFunc must be nil for plain DDL view migration")
	}
}

func TestM0011_ViewExistsAfterApply(t *testing.T) {
	db := newM0011DB(t)
	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='view' AND name='v_AccountHealth'`,
	).Scan(&name); err != nil {
		t.Fatalf("v_AccountHealth view not present: %v", err)
	}
}

func TestM0011_PicksMostRecentPerAlias(t *testing.T) {
	db := newM0011DB(t)

	// Seed two aliases with multiple events each. We use literal
	// timestamps (instead of `strftime('now')`) so the test is
	// deterministic regardless of execution speed. Ordering matters:
	// for "alpha" the heartbeat (Kind=4) is the latest → Status 'ok'.
	// For "beta" the error (Kind=3) is the latest → Status 'err'.
	seed := []struct {
		alias      string
		kind       int
		occurredAt string
	}{
		{"alpha", 1, "2026-04-26T10:00:00.000Z"}, // start
		{"alpha", 4, "2026-04-26T10:05:00.000Z"}, // heartbeat (latest)
		{"beta", 1, "2026-04-26T09:00:00.000Z"},  // start
		{"beta", 3, "2026-04-26T09:30:00.000Z"},  // error (latest)
		{"beta", 2, "2026-04-26T09:15:00.000Z"},  // stop (older — must be ignored)
	}
	for _, s := range seed {
		if _, err := db.Exec(
			`INSERT INTO WatchEvents (Alias, Kind, OccurredAt) VALUES (?, ?, ?)`,
			s.alias, s.kind, s.occurredAt,
		); err != nil {
			t.Fatalf("seed (%s, %d, %s): %v", s.alias, s.kind, s.occurredAt, err)
		}
	}

	type row struct {
		alias, occurredAt, status string
		kind                      int
	}
	rows, err := db.Query(`SELECT Alias, LastKind, LastOccurredAt, Status FROM v_AccountHealth ORDER BY Alias`)
	if err != nil {
		t.Fatalf("select view: %v", err)
	}
	defer rows.Close()
	got := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.alias, &r.kind, &r.occurredAt, &r.status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	want := []row{
		{alias: "alpha", kind: 4, occurredAt: "2026-04-26T10:05:00.000Z", status: "ok"},
		{alias: "beta", kind: 3, occurredAt: "2026-04-26T09:30:00.000Z", status: "err"},
	}
	if len(got) != len(want) {
		t.Fatalf("row count = %d, want %d (got=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestM0011_StatusMapping(t *testing.T) {
	db := newM0011DB(t)

	// One alias per kind, each with a single event so "latest" is
	// trivially that one row. Locks the doc-comment status table.
	cases := []struct {
		alias  string
		kind   int
		status string
	}{
		{"a-start", 1, "ok"},
		{"a-stop", 2, "warn"},
		{"a-error", 3, "err"},
		{"a-heart", 4, "ok"},
	}
	for _, c := range cases {
		if _, err := db.Exec(
			`INSERT INTO WatchEvents (Alias, Kind, OccurredAt) VALUES (?, ?, '2026-04-26T11:00:00.000Z')`,
			c.alias, c.kind,
		); err != nil {
			t.Fatalf("seed %s: %v", c.alias, err)
		}
	}
	for _, c := range cases {
		var got string
		if err := db.QueryRow(
			`SELECT Status FROM v_AccountHealth WHERE Alias = ?`, c.alias,
		).Scan(&got); err != nil {
			t.Fatalf("query %s: %v", c.alias, err)
		}
		if got != c.status {
			t.Errorf("alias=%s kind=%d: Status=%q, want %q", c.alias, c.kind, got, c.status)
		}
	}
}

func TestM0011_IsIdempotent_ViaApply(t *testing.T) {
	saved := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, saved) })

	// Need m0008 (WatchEvents table) + m0011 (the view that reads it).
	registryMu.Lock()
	registry = map[int]Migration{8: saved[8], 11: saved[11]}
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
	if err := db.QueryRow(`SELECT COUNT(*) FROM _SchemaVersion WHERE Version=11`).Scan(&n); err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if n != 1 {
		t.Fatalf("ledger row count for v11 = %d, want 1", n)
	}
	// And the view still resolves on pass 2.
	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='view' AND name='v_AccountHealth'`,
	).Scan(&name); err != nil {
		t.Fatalf("v_AccountHealth missing after re-apply: %v", err)
	}
}

// newM0011DB opens an in-memory DB with only m0008 + m0011 applied —
// just enough schema to exercise the view without coupling the test
// to every prior migration.
func newM0011DB(t *testing.T) *sql.DB {
	t.Helper()
	saved := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, saved) })

	registryMu.Lock()
	registry = map[int]Migration{8: saved[8], 11: saved[11]}
	registryMu.Unlock()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return db
}
