// m0009_opened_urls_opened_at_index_test.go locks the P1.14 contract:
//
//   - The migration registers at Version 9 with the documented Name.
//   - After a full Apply, `IxOpenedUrlsOpenedAt` exists on OpenedUrls.
//   - The index is on the OpenedAt column (one column, not a composite).
//   - A query planner check confirms the bounded prune sub-select
//     uses an index (not a SCAN), preventing future schema changes
//     from silently regressing PruneOpenedUrlsBatched performance.
//
// Uses the production registry — m0009 sits on top of m0004 (table)
// and inherits the m0001..m0008 chain via the standard Apply path.
package migrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestM0009_RegistryEntry(t *testing.T) {
	registryMu.RLock()
	m, ok := registry[9]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("Version 9 not registered (m0009 init missing?)")
	}
	if m.Name != "opened_urls_opened_at_index" {
		t.Errorf("Name = %q, want %q", m.Name, "opened_urls_opened_at_index")
	}
	if m.UpFunc != nil {
		t.Error("UpFunc must be nil for plain DDL migration")
	}
	if !strings.Contains(m.Up, "IxOpenedUrlsOpenedAt") {
		t.Errorf("Up missing index name: %q", m.Up)
	}
	if !strings.Contains(m.Up, "OpenedUrls(OpenedAt)") {
		t.Errorf("Up missing OpenedUrls(OpenedAt) target: %q", m.Up)
	}
}

func TestM0009_IndexExistsAfterApply(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Index presence — name is part of the public schema contract
	// (referenced in the migration doc-comment); drift caught here.
	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master
		   WHERE type='index' AND tbl_name='OpenedUrls' AND name='IxOpenedUrlsOpenedAt'`,
	).Scan(&name); err != nil {
		t.Fatalf("IxOpenedUrlsOpenedAt missing after Apply: %v", err)
	}

	// Index shape — must be one column on `OpenedAt`, not composite.
	// `PRAGMA index_info` rows are (seqno, cid, name).
	rows, err := db.Query(`PRAGMA index_info(IxOpenedUrlsOpenedAt)`)
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
	if len(cols) != 1 || cols[0] != "OpenedAt" {
		t.Errorf("index columns = %v, want [OpenedAt]", cols)
	}
}

// TestM0009_PrunePathUsesIndex asserts the SQLite query planner picks
// IxOpenedUrlsOpenedAt for `PruneOpenedUrlsBatched`'s bounded
// sub-select. Without this index the inner SELECT becomes a full
// scan of OpenedUrls, which defeats the whole point of the batched
// vacuum (held writer lock would scale with table size, not LIMIT).
//
// We assert on `EXPLAIN QUERY PLAN` output containing the index name
// — looser than asserting "no SCAN" because SQLite's planner output
// format varies subtly across versions (`USING INDEX X` vs
// `SEARCH ... USING INDEX X`).
func TestM0009_PrunePathUsesIndex(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Mirror the inner sub-select of `queries.PruneOpenedUrlsBatched`.
	// Asserting on the inner SELECT (not the outer DELETE) keeps the
	// test honest: a future planner change to the DELETE wrapper
	// won't mask a regression in the bounded scan.
	const innerSelect = `EXPLAIN QUERY PLAN
		SELECT rowid FROM OpenedUrls WHERE OpenedAt < ? LIMIT ?`

	rows, err := db.Query(innerSelect, "2099-01-01T00:00:00Z", 100)
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
	if !strings.Contains(got, "IxOpenedUrlsOpenedAt") {
		t.Errorf("EXPLAIN QUERY PLAN does not reference IxOpenedUrlsOpenedAt — bounded prune would full-scan.\nplan:\n%s", got)
	}
}
