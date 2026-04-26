// migrations_idempotent_test.go is the P1.17 cross-package guard.
//
// Goal: prove the *full* m0001..m000N migration stack — exactly as
// shipped to users via `store.Open` — re-applies cleanly on a
// previously-migrated DB. Two failure modes this test catches that
// the per-migration `TestM000X_IsIdempotent_ViaApply` siblings can't:
//
//  1. **Inter-migration ordering bugs.** Each per-migration test
//     isolates its slice via `snapshotRegistry` + a 1-entry registry.
//     That misses regressions where, say, m0011 silently depends on
//     a column m0010 added but nobody declared the order.
//  2. **Schema drift across re-open.** Closing the file DB, re-opening
//     it via the same `store.OpenAt`, and re-running migrations is
//     the exact lifecycle a user sees on every app launch. A migration
//     that *looks* idempotent in :memory: can still leave the file in
//     a state that fails the next launch (e.g. an `ALTER TABLE` that
//     errors with "duplicate column" because PRAGMA introspection was
//     skipped).
//
// The test asserts two stable invariants across two open→close→open
// cycles on the same on-disk file:
//
//   - **Ledger row count is stable.** After the first open, the
//     `_SchemaVersion` table contains exactly len(migrate.All()) rows.
//     After the second open it MUST contain the same count — proving
//     no migration re-recorded itself and no duplicate ledger writes
//     occurred.
//   - **Schema fingerprint is stable.** The set of all CREATE
//     statements in `sqlite_master` (tables + indexes + views) is
//     identical between the two opens. Catches accidental schema
//     mutation by an "idempotent" migration that actually re-runs DDL.
//
// Why a file DB (not `:memory:`)? `:memory:` databases are destroyed
// on `db.Close()` — re-opening would give a fresh DB and trivially
// pass. We need persistence across the close/reopen boundary, so we
// use `t.TempDir()`.
package store

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/store/migrate"
)

func TestMigrations_FullStack_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "idempotent.db")

	// Pass 1 — fresh file DB, first-ever open. `OpenAt` invokes
	// `migrate.Apply` internally (see store.go:92).
	s1, err := OpenAt(dbPath)
	if err != nil {
		t.Fatalf("first OpenAt: %v", err)
	}
	wantCount := len(migrate.All())
	if wantCount == 0 {
		t.Fatal("migrate.All() is empty — registry not initialised; init() ordering bug?")
	}
	gotCount1 := ledgerCount(t, s1)
	if gotCount1 != wantCount {
		t.Fatalf("pass 1 ledger count = %d, want %d (one row per registered migration)",
			gotCount1, wantCount)
	}
	fingerprint1 := schemaFingerprint(t, s1)
	if err := s1.Close(); err != nil {
		t.Fatalf("close pass 1: %v", err)
	}

	// Pass 2 — re-open the SAME file. Every migration in m0001..m000N
	// must short-circuit via the ledger; UpFunc migrations
	// (m0005, m0010) must additionally short-circuit via PRAGMA
	// introspection if the ledger were ever bypassed.
	s2, err := OpenAt(dbPath)
	if err != nil {
		t.Fatalf("second OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	gotCount2 := ledgerCount(t, s2)
	if gotCount2 != wantCount {
		t.Fatalf("pass 2 ledger count = %d, want %d (idempotence violated — a migration re-recorded itself)",
			gotCount2, wantCount)
	}
	fingerprint2 := schemaFingerprint(t, s2)
	if fingerprint1 != fingerprint2 {
		// Diff the two fingerprints line-by-line so the failure
		// message points at the offending object name.
		t.Fatalf("schema fingerprint changed across re-open:\n--- pass 1 ---\n%s\n--- pass 2 ---\n%s",
			fingerprint1, fingerprint2)
	}

	// Belt-and-braces: every registered Version must have exactly
	// one ledger row. Catches the (theoretical) case where total
	// count matches but version IDs got remapped.
	for _, m := range migrate.All() {
		var n int
		if err := s2.DB.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM _SchemaVersion WHERE Version = ?`, m.Version,
		).Scan(&n); err != nil {
			t.Fatalf("count v%d: %v", m.Version, err)
		}
		if n != 1 {
			t.Errorf("ledger row count for v%d (%s) = %d, want exactly 1",
				m.Version, m.Name, n)
		}
	}
}

// ledgerCount returns total `_SchemaVersion` rows. Lives here (not in
// migrate/) so the test stays cross-package and exercises the public
// `*Store.DB` handle the rest of the codebase uses.
func ledgerCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM _SchemaVersion`,
	).Scan(&n); err != nil {
		t.Fatalf("ledger count: %v", err)
	}
	return n
}

// schemaFingerprint returns a deterministic, sorted concatenation of
// every CREATE statement in `sqlite_master`. Excludes:
//
//   - `sqlite_*` internal objects (autoindexes for PRIMARY KEY, the
//     `sqlite_sequence` AUTOINCREMENT counter, etc.) — these can vary
//     based on insertion order on the second open even when the schema
//     is byte-identical.
//   - rows where `sql` is NULL (auto-generated indexes).
//
// The result is intentionally a single multi-line string so the test
// failure diff is human-readable rather than a slice dump.
func schemaFingerprint(t *testing.T, s *Store) string {
	t.Helper()
	rows, err := s.DB.QueryContext(context.Background(),
		`SELECT type, name, sql FROM sqlite_master
         WHERE name NOT LIKE 'sqlite_%' AND sql IS NOT NULL
         ORDER BY type, name`)
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var typ, name, sql string
		if err := rows.Scan(&typ, &name, &sql); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		// Normalise whitespace so cosmetic-only diffs (e.g. SQLite
		// re-formatting CREATE on rewrite) don't false-positive.
		normalised := strings.Join(strings.Fields(sql), " ")
		lines = append(lines, typ+"\t"+name+"\t"+normalised)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
