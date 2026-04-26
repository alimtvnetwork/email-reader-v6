// shims_consecutive_failures_test.go — Slice #106 coverage for the
// m0014 counter wired into the poll outcome path:
//
//   - `BumpConsecutiveFailures` on a missing alias inserts a row
//     with `ConsecutiveFailures = 1` (first-boot-error scenario).
//   - Repeated `BumpConsecutiveFailures` increments by 1 each call.
//   - `ResetConsecutiveFailures` zeroes the counter without
//     touching the cursor columns (LastUid, LastSubject) — proves
//     the watcher's resume position survives an error→success edge.
//   - `ResetConsecutiveFailures` against a missing alias is a
//     harmless no-op (zero rows affected, no error).
//   - The `AccountHealthSelectAll` projection surfaces the counter
//     in `StoreAccountHealthRow.ConsecutiveFailures`.
package store

import (
	"context"
	"testing"
	"time"
)

func TestBumpConsecutiveFailures_MissingAlias_InsertsWithOne(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const alias = "first-boot@example.com"

	if err := s.BumpConsecutiveFailures(ctx, alias); err != nil {
		t.Fatalf("bump: %v", err)
	}
	ws, err := s.GetWatchState(ctx, alias)
	if err != nil {
		t.Fatalf("GetWatchState: %v", err)
	}
	// GetWatchState doesn't return ConsecutiveFailures (intentional —
	// the watcher doesn't read it; only the dashboard does). Read
	// the column directly.
	var cf int
	if err := s.DB.QueryRow(
		`SELECT ConsecutiveFailures FROM WatchState WHERE Alias = ?`, alias,
	).Scan(&cf); err != nil {
		t.Fatalf("read ConsecutiveFailures: %v", err)
	}
	if cf != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1 (first failure seeds the row)", cf)
	}
	// LastUid stayed at the seeded zero — the bump path must not
	// clobber the cursor.
	if ws.LastUid != 0 {
		t.Errorf("LastUid = %d, want 0", ws.LastUid)
	}
}

func TestBumpConsecutiveFailures_Increments(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const alias = "loop@example.com"

	for i := 0; i < 4; i++ {
		if err := s.BumpConsecutiveFailures(ctx, alias); err != nil {
			t.Fatalf("bump #%d: %v", i, err)
		}
	}
	var cf int
	if err := s.DB.QueryRow(
		`SELECT ConsecutiveFailures FROM WatchState WHERE Alias = ?`, alias,
	).Scan(&cf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if cf != 4 {
		t.Errorf("ConsecutiveFailures = %d, want 4", cf)
	}
}

func TestResetConsecutiveFailures_PreservesCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const alias = "resume@example.com"

	// Seed via the regular UpsertWatchState path so the cursor
	// columns are populated as the watcher would leave them.
	if err := s.UpsertWatchState(ctx, WatchState{
		Alias:          alias,
		LastUid:        12345,
		LastSubject:    "subject",
		LastReceivedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.BumpConsecutiveFailures(ctx, alias); err != nil {
		t.Fatalf("bump: %v", err)
	}
	if err := s.BumpConsecutiveFailures(ctx, alias); err != nil {
		t.Fatalf("bump: %v", err)
	}
	if err := s.ResetConsecutiveFailures(ctx, alias); err != nil {
		t.Fatalf("reset: %v", err)
	}

	ws, err := s.GetWatchState(ctx, alias)
	if err != nil {
		t.Fatalf("GetWatchState: %v", err)
	}
	if ws.LastUid != 12345 {
		t.Errorf("LastUid = %d, want 12345 (reset must not clobber cursor)", ws.LastUid)
	}
	if ws.LastSubject != "subject" {
		t.Errorf("LastSubject = %q, want %q", ws.LastSubject, "subject")
	}
	var cf int
	if err := s.DB.QueryRow(
		`SELECT ConsecutiveFailures FROM WatchState WHERE Alias = ?`, alias,
	).Scan(&cf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if cf != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0 after reset", cf)
	}
}

func TestResetConsecutiveFailures_MissingAlias_NoError(t *testing.T) {
	s := newTestStore(t)
	if err := s.ResetConsecutiveFailures(context.Background(), "ghost@example.com"); err != nil {
		t.Errorf("reset on missing alias: %v (want nil — UPDATE is a no-op)", err)
	}
}

func TestQueryAccountHealth_SurfacesConsecutiveFailures(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const alias = "stuck@example.com"

	for i := 0; i < 3; i++ {
		if err := s.BumpConsecutiveFailures(ctx, alias); err != nil {
			t.Fatalf("bump #%d: %v", i, err)
		}
	}
	rows, err := s.QueryAccountHealth(ctx)
	if err != nil {
		t.Fatalf("QueryAccountHealth: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Alias != alias {
		t.Errorf("Alias = %q, want %q", rows[0].Alias, alias)
	}
	if rows[0].ConsecutiveFailures != 3 {
		t.Errorf("ConsecutiveFailures = %d, want 3", rows[0].ConsecutiveFailures)
	}
}
