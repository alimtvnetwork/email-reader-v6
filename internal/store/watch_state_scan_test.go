package store

import (
	"context"
	"testing"
	"time"
)

// TestGetWatchState_HandlesStringTimestamps reproduces the live poll-error
// chain reported in the Watch view ("sql: Scan error on column index 3,
// name \"LastReceivedAt\": unsupported Scan, storing driver.Value type
// string into type *time.Time"): the schema stores LastReceivedAt and
// UpdatedAt as RFC3339 strings (see formatRFC3339UTC and
// sqliteRFC3339NowExpr), so GetWatchState must scan into NullString /
// string and parse via parseSqliteRFC3339 — not into NullTime / Time.
//
// This test inserts a row with a raw RFC3339Nano string (the exact shape
// formatRFC3339UTC emits) so a regression that re-introduces NullTime
// scanning would crash here just like the live watcher did.
func TestGetWatchState_HandlesStringTimestamps(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Bypass UpsertWatchState so we control the exact on-disk format
	// independently of any future helper changes. This is the wire
	// shape modernc/sqlite returns to driver.Value as `string`.
	const recv = "2026-04-26T10:05:00.123Z"
	const upd = "2026-04-26T10:05:00.456Z"
	if _, err := s.DB.ExecContext(ctx,
		`INSERT INTO WatchState (Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt)
		 VALUES (?, ?, ?, ?, ?)`,
		"atto", uint32(7), "subj", recv, upd,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := s.GetWatchState(ctx, "atto")
	if err != nil {
		t.Fatalf("GetWatchState: %v", err)
	}
	if got.LastUid != 7 || got.LastSubject != "subj" {
		t.Fatalf("scalar fields drifted: %+v", got)
	}
	wantRecv, _ := time.Parse(time.RFC3339Nano, recv)
	wantUpd, _ := time.Parse(time.RFC3339Nano, upd)
	if !got.LastReceivedAt.Equal(wantRecv) {
		t.Fatalf("LastReceivedAt: got %v want %v", got.LastReceivedAt, wantRecv)
	}
	if !got.UpdatedAt.Equal(wantUpd) {
		t.Fatalf("UpdatedAt: got %v want %v", got.UpdatedAt, wantUpd)
	}
}

// TestGetWatchState_HandlesNullLastReceivedAt locks the
// "first-row, no-mail-seen-yet" path: LastReceivedAt may legitimately
// be NULL (no message has ever been received), and GetWatchState must
// return the zero time without erroring.
func TestGetWatchState_HandlesNullLastReceivedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.DB.ExecContext(ctx,
		`INSERT INTO WatchState (Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt)
		 VALUES (?, ?, ?, NULL, ?)`,
		"atto", uint32(0), "", "2026-04-26T10:05:00.000Z",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := s.GetWatchState(ctx, "atto")
	if err != nil {
		t.Fatalf("GetWatchState: %v", err)
	}
	if !got.LastReceivedAt.IsZero() {
		t.Fatalf("expected zero LastReceivedAt for NULL column, got %v", got.LastReceivedAt)
	}
}
