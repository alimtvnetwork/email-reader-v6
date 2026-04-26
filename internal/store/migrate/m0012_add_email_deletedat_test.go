// m0012_add_email_deletedat_test.go locks the P4.3 schema contract.
//
// Coverage:
//   - Registry entry at Version 12 with documented Name + UpFunc form.
//   - After Apply, `Emails.DeletedAt` exists, type INTEGER, nullable
//     (notnull=0), with no DEFAULT (NULL is the sentinel).
//   - Re-invoking UpFunc directly (bypassing the ledger) is a no-op —
//     the PRAGMA gate skips the ADD when the column already exists.
//   - Pre-existing rows that pre-date the migration read DeletedAt
//     IS NULL after Apply (backfill-free upgrade).
package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0012_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[12]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 12 not registered (m0012 init missing?)")
	}
	if m.Name != "add_email_deletedat" {
		t.Errorf("Name = %q, want %q", m.Name, "add_email_deletedat")
	}
	if m.UpFunc == nil {
		t.Error("UpFunc must be set (PRAGMA-gated ALTER pattern)")
	}
	if m.Up != "" {
		t.Errorf("Up must be empty when UpFunc is set, got %q", m.Up)
	}
}

func TestM0012_AddsDeletedAtColumn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var (
		cid     int
		name    string
		typ     string
		notnull int
		dflt    sql.NullString
		pk      int
	)
	err = db.QueryRow(
		`SELECT cid, name, type, "notnull", dflt_value, pk
		   FROM pragma_table_info('Emails') WHERE name = ?`, "DeletedAt",
	).Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
	if err != nil {
		t.Fatalf("read DeletedAt metadata: %v", err)
	}
	if typ != "INTEGER" {
		t.Errorf("DeletedAt.type = %q, want INTEGER", typ)
	}
	if notnull != 0 {
		t.Errorf("DeletedAt.notnull = %d, want 0 (NULL is the sentinel)", notnull)
	}
	if dflt.Valid {
		t.Errorf("DeletedAt should have no DEFAULT; got %q", dflt.String)
	}
}

func TestM0012_UpFunc_IsSelfIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := applyAddEmailDeletedAt(context.Background(), db); err != nil {
			t.Fatalf("UpFunc re-invocation #%d failed: %v", i+2, err)
		}
	}
}

func TestM0012_PreExistingRowsGetNullDeletedAt(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Apply only m0001..m0011 (skip m0012+) by snapshotting the
	// registry, trimming, applying, then restoring + applying again.
	full := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, full) })

	registryMu.Lock()
	registry = make(map[int]Migration)
	for v, m := range full {
		if v < 12 {
			registry[v] = m
		}
	}
	registryMu.Unlock()

	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply m0001..m0011: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO Emails (Alias, MessageId, Uid) VALUES (?, ?, ?)`,
		"user@x", "<pre-deletedat@y>", 99,
	); err != nil {
		t.Fatalf("seed pre-migration row: %v", err)
	}

	restoreRegistry(t, full)
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply m0012+: %v", err)
	}

	var deletedAt sql.NullInt64
	if err := db.QueryRow(
		`SELECT DeletedAt FROM Emails WHERE MessageId = ?`,
		"<pre-deletedat@y>",
	).Scan(&deletedAt); err != nil {
		t.Fatalf("read pre-existing row: %v", err)
	}
	if deletedAt.Valid {
		t.Errorf("pre-existing row DeletedAt = %d (valid), want NULL", deletedAt.Int64)
	}
}
