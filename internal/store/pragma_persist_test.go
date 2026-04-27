// pragma_persist_test.go closes the AC-DB-10 / AC-DB-11 cluster
// (Slice #129). Both rows describe SQLite-runtime invariants that the
// rest of the test suite relies on but never asserts directly:
//
//   - AC-DB-10: every connection from the pool must report
//     journal_mode=wal, synchronous=NORMAL, foreign_keys=ON,
//     busy_timeout=5000.
//   - AC-DB-11: closing then reopening the same DB file must
//     preserve journal_mode=wal (i.e. WAL is on-disk, not just
//     per-connection).
//
// The other AC-DB rows in `coverageGapAllowlist` (06-09 enum CHECK,
// 32-36 migrate-runner negatives, 02/03 schema introspection) describe
// a future-target schema that doesn't match the implemented one
// (pluralised tables, no Decision/Origin enums, no version-gap
// detection in the runner). They stay in the allowlist with the
// existing cross-reference comment in coverage_audit_test.go.
package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

// Satisfies AC-DB-10 — every connection from the pool reports
// journal_mode=wal, synchronous=1 (NORMAL), foreign_keys=1, and
// busy_timeout=5000ms.
//
// Strategy: bump MaxOpenConns above the implicit cap so the pool
// hands us several distinct connections, then probe each via a tx
// (`BeginTx` pins one connection for the duration). We use four
// concurrent transactions to virtually guarantee the pool serves
// four physical connections.
func Test_Store_PragmaOnEveryConn(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Force the pool to materialise four physical connections.
	s.DB.SetMaxOpenConns(4)
	s.DB.SetMaxIdleConns(4)

	const probes = 4
	type sample struct {
		journalMode  string
		synchronous  int
		foreignKeys  int
		busyTimeout  int
	}
	samples := make([]sample, probes)

	// Hold all four txns open simultaneously so each comes from a
	// distinct physical connection. SQLite's WAL mode permits
	// multiple concurrent readers.
	txs := make([]*sql.Tx, probes)
	defer func() {
		for _, tx := range txs {
			if tx != nil {
				_ = tx.Rollback()
			}
		}
	}()

	for i := 0; i < probes; i++ {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx %d: %v", i, err)
		}
		txs[i] = tx

		row := tx.QueryRowContext(ctx, `PRAGMA journal_mode`)
		if err := row.Scan(&samples[i].journalMode); err != nil {
			t.Fatalf("probe %d journal_mode: %v", i, err)
		}
		row = tx.QueryRowContext(ctx, `PRAGMA synchronous`)
		if err := row.Scan(&samples[i].synchronous); err != nil {
			t.Fatalf("probe %d synchronous: %v", i, err)
		}
		row = tx.QueryRowContext(ctx, `PRAGMA foreign_keys`)
		if err := row.Scan(&samples[i].foreignKeys); err != nil {
			t.Fatalf("probe %d foreign_keys: %v", i, err)
		}
		row = tx.QueryRowContext(ctx, `PRAGMA busy_timeout`)
		if err := row.Scan(&samples[i].busyTimeout); err != nil {
			t.Fatalf("probe %d busy_timeout: %v", i, err)
		}
	}

	for i, got := range samples {
		if !strings.EqualFold(got.journalMode, "wal") {
			t.Errorf("conn[%d] journal_mode = %q, want wal", i, got.journalMode)
		}
		// SQLite reports NORMAL as 1, FULL as 2. The driver string
		// we pass omits an explicit synchronous PRAGMA so SQLite
		// uses the WAL default of NORMAL (=1). If a future change
		// hard-codes FULL we want to know.
		if got.synchronous != 1 {
			t.Errorf("conn[%d] synchronous = %d, want 1 (NORMAL)", i, got.synchronous)
		}
		if got.foreignKeys != 1 {
			t.Errorf("conn[%d] foreign_keys = %d, want 1", i, got.foreignKeys)
		}
		if got.busyTimeout != 5000 {
			t.Errorf("conn[%d] busy_timeout = %d, want 5000", i, got.busyTimeout)
		}
	}
}

// Satisfies AC-DB-11 — closing then reopening the same DB file
// preserves journal_mode=wal (it does NOT silently fall back to
// rollback-journal). The on-disk -wal / -shm sidecars persist and the
// reopened connection negotiates WAL again.
func Test_Store_WalPersists(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Sanity: first open is in WAL mode.
	var mode string
	if err := s.DB.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("initial journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("baseline journal_mode = %q, want wal", mode)
	}

	// Force at least one write so the WAL header is materialised on
	// disk before close. A SchemaVersion ledger row from migrate
	// already qualifies, but we add an explicit no-op write to keep
	// the test self-contained against future migration trims.
	if _, err := s.DB.ExecContext(ctx,
		`INSERT INTO WatchState(Alias, LastUid) VALUES ('wal-probe', 0)
		 ON CONFLICT(Alias) DO NOTHING`); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	path := s.Path
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := OpenAt(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if err := reopened.DB.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("reopened journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("reopened journal_mode = %q, want wal (silent fallback to delete is forbidden)", mode)
	}
}
