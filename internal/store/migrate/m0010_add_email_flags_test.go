// m0010_add_email_flags_test.go locks the P1.15 contract:
//
//   - Registry entry at Version 10 with documented Name + UpFunc form.
//   - After Apply, `Emails` has both `IsRead` and `IsFlagged` columns
//     of type INTEGER, NOT NULL, defaulting to 0.
//   - Re-running the migration's UpFunc against a DB where the
//     columns already exist is a no-op (mirrors the m0005 idempotency
//     guarantee that the ledger short-circuit isn't the only line of
//     defence).
//   - Existing rows that pre-date the migration get the default 0
//     for both flags (validates AC: backfill-free upgrade).
package migrate

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0010_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[10]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 10 not registered (m0010 init missing?)")
	}
	if m.Name != "add_email_flags" {
		t.Errorf("Name = %q, want %q", m.Name, "add_email_flags")
	}
	if m.UpFunc == nil {
		t.Error("UpFunc must be set (PRAGMA-gated ALTER pattern)")
	}
	if m.Up != "" {
		t.Errorf("Up must be empty when UpFunc is set, got %q", m.Up)
	}
}

func TestM0010_AddsBothFlagColumns(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, want := range []struct {
		col, typ, dflt string
	}{
		{"IsRead", "INTEGER", "0"},
		{"IsFlagged", "INTEGER", "0"},
	} {
		var (
			cid     int
			name    string
			typ     string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		err := db.QueryRow(
			`SELECT cid, name, type, "notnull", dflt_value, pk
			   FROM pragma_table_info('Emails') WHERE name = ?`, want.col,
		).Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
		if err != nil {
			t.Fatalf("read %s metadata: %v", want.col, err)
		}
		if typ != want.typ {
			t.Errorf("%s.type = %q, want %q", want.col, typ, want.typ)
		}
		if notnull != 1 {
			t.Errorf("%s.notnull = %d, want 1", want.col, notnull)
		}
		if !dflt.Valid || dflt.String != want.dflt {
			t.Errorf("%s.default = %q (valid=%v), want %q", want.col, dflt.String, dflt.Valid, want.dflt)
		}
	}
}

func TestM0010_UpFunc_IsSelfIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Bypass the ledger short-circuit and re-invoke UpFunc directly.
	// The PRAGMA gate must skip both ADDs (otherwise SQLite raises
	// "duplicate column name").
	if err := applyAddEmailFlags(context.Background(), db); err != nil {
		t.Fatalf("UpFunc re-invocation should be no-op, got: %v", err)
	}
	// Run twice more for paranoia — confirms no accumulated state.
	for i := 0; i < 2; i++ {
		if err := applyAddEmailFlags(context.Background(), db); err != nil {
			t.Fatalf("UpFunc re-invocation #%d failed: %v", i+2, err)
		}
	}
}

func TestM0010_PreExistingRowsGetZeroDefaults(t *testing.T) {
	// Simulate a user DB that was upgraded mid-life: insert a row
	// against a partially-migrated schema (no flags), THEN apply
	// m0010, THEN read the flags. The pre-existing row must read
	// IsRead=0 / IsFlagged=0 — no backfill required.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Apply only m0001..m0009 (skip m0010+) by snapshotting the
	// registry, trimming, applying, then restoring + applying again.
	full := snapshotRegistry(t)
	t.Cleanup(func() { restoreRegistry(t, full) })

	registryMu.Lock()
	registry = make(map[int]Migration)
	for v, m := range full {
		if v < 10 {
			registry[v] = m
		}
	}
	registryMu.Unlock()

	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply m0001..m0009: %v", err)
	}

	// Insert a row before the flag columns exist.
	if _, err := db.Exec(
		`INSERT INTO Emails (Alias, MessageId, Uid) VALUES (?, ?, ?)`,
		"user@x", "<pre-flags@y>", 42,
	); err != nil {
		t.Fatalf("seed pre-migration row: %v", err)
	}

	// Now restore the full registry (which includes m0010+) and
	// Apply again — the ledger has m0001..m0009 marked done, so
	// only m0010+ runs.
	restoreRegistry(t, full)
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply m0010+: %v", err)
	}

	var isRead, isFlagged int
	if err := db.QueryRow(
		`SELECT IsRead, IsFlagged FROM Emails WHERE MessageId = ?`,
		"<pre-flags@y>",
	).Scan(&isRead, &isFlagged); err != nil {
		t.Fatalf("read pre-existing row: %v", err)
	}
	if isRead != 0 || isFlagged != 0 {
		t.Errorf("pre-existing row flags = (IsRead=%d, IsFlagged=%d), want (0, 0)", isRead, isFlagged)
	}
}
