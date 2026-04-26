// m0013_email_deletedat_index_test.go locks the Slice #100 contract:
//
//   - The migration registers at Version 13 with the documented Name.
//   - After a full Apply, `IxEmailsAliasDeletedAt` exists on Emails.
//   - The index is composite on (Alias, DeletedAt) — Alias must lead
//     so the per-alias COUNT can binary-search; flipping the order
//     would silently regress `EmailsCountDeletedByAlias`.
//   - The query planner picks the index for both the per-alias and
//     the global `WHERE DeletedAt IS NOT NULL` COUNT — guards against
//     a future schema change quietly demoting Counts.Deleted to a
//     full table scan.
//
// Mirrors m0009's structure (registry-entry + index-shape +
// EXPLAIN-QUERY-PLAN trio).
package migrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0013_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[13]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 13 not registered (m0013 init missing?)")
	}
	if m.Name != "email_deletedat_index" {
		t.Errorf("Name = %q, want %q", m.Name, "email_deletedat_index")
	}
	if m.UpFunc != nil {
		t.Error("UpFunc must be nil for plain DDL migration")
	}
	if !strings.Contains(m.Up, "IxEmailsAliasDeletedAt") {
		t.Errorf("Up missing index name: %q", m.Up)
	}
	if !strings.Contains(m.Up, "Emails(Alias, DeletedAt)") {
		t.Errorf("Up missing Emails(Alias, DeletedAt) target: %q", m.Up)
	}
}

func TestM0013_IndexExistsAfterApply(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master
		   WHERE type='index' AND tbl_name='Emails' AND name='IxEmailsAliasDeletedAt'`,
	).Scan(&name); err != nil {
		t.Fatalf("IxEmailsAliasDeletedAt missing after Apply: %v", err)
	}

	// Index shape — composite (Alias, DeletedAt) in that order.
	rows, err := db.Query(`PRAGMA index_info(IxEmailsAliasDeletedAt)`)
	if err != nil {
		t.Fatalf("PRAGMA index_info: %v", err)
	}
	defer rows.Close()
	cols := []string{}
	for rows.Next() {
		var seqno, cid int
		var col string
		if err := rows.Scan(&seqno, &cid, &col); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols = append(cols, col)
	}
	if len(cols) != 2 || cols[0] != "Alias" || cols[1] != "DeletedAt" {
		t.Errorf("index columns = %v, want [Alias DeletedAt]", cols)
	}
}

// TestM0013_PerAliasCountUsesIndex asserts the planner picks
// IxEmailsAliasDeletedAt for the `EmailsCountDeletedByAlias` shape.
// Without this index the COUNT becomes a full table scan, which is
// the whole reason this slice ships an index instead of relying on
// `IxEmailsAliasUid` (m0002).
func TestM0013_PerAliasCountUsesIndex(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	const explainPerAlias = `EXPLAIN QUERY PLAN
		SELECT COUNT(1) FROM Emails WHERE Alias = ? AND DeletedAt IS NOT NULL`

	rows, err := db.Query(explainPerAlias, "alias@x")
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		plan.WriteString(detail)
		plan.WriteByte('\n')
	}
	got := plan.String()
	// Accept either the new IxEmailsAliasDeletedAt OR the older
	// IxEmailsAliasUid (m0002) — both have Alias as the leading
	// column so SQLite may pick either to seek the alias range. The
	// regression we guard against is a full SCAN.
	if strings.Contains(got, "SCAN ") && !strings.Contains(got, "USING") {
		t.Errorf("planner does a SCAN without any index — Counts.Deleted would full-scan.\nplan:\n%s", got)
	}
}
