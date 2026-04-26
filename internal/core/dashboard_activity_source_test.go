// dashboard_activity_source_test.go — Slice #104 coverage for the
// `NewStoreActivitySource` adapter. The adapter is a thin field-copy
// + Kind-mapping loop, so the tests focus on:
//
//   - nil-store guard.
//   - Empty DB → empty slice, no error.
//   - WatchStart/Heartbeat/Error map to PollStarted/PollSucceeded/PollFailed.
//   - WatchStop is dropped (no spec ActivityKind for clean shutdown).
//   - Unknown integer kind is dropped (forward-compat).
//   - Payload Message/ErrorCode flow through.
//   - Post-Close store error is wrapped with `ErrDbOpen`.
package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

func openActivityTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "act.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedAct(t *testing.T, s *store.Store, alias string, kind int, occurredAt, payload string) {
	t.Helper()
	if _, err := s.DB.Exec(
		`INSERT INTO WatchEvents (Alias, Kind, OccurredAt, Payload) VALUES (?, ?, ?, ?)`,
		alias, kind, occurredAt, payload,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestNewStoreActivitySource_NilStore_ReturnsNil(t *testing.T) {
	if got := NewStoreActivitySource(nil); got != nil {
		t.Fatalf("nil store: want nil source, got non-nil")
	}
}

func TestNewStoreActivitySource_EmptyDB_NoError(t *testing.T) {
	src := NewStoreActivitySource(openActivityTestStore(t))
	if src == nil {
		t.Fatal("source unexpectedly nil")
	}
	res := src(context.Background(), 10)
	if res.HasError() {
		t.Fatalf("empty DB: %v", res.Error())
	}
	if len(res.Value()) != 0 {
		t.Fatalf("empty DB: got %d rows, want 0", len(res.Value()))
	}
}

func TestNewStoreActivitySource_KindMappingAndStopDropped(t *testing.T) {
	s := openActivityTestStore(t)
	// Seed one of each: Start (1), Stop (2), Error (3), Heartbeat (4),
	// plus an unknown future kind (99). Order them DESC by time so
	// the returned slice order is predictable: 99, 4, 3, 2, 1.
	seedAct(t, s, "a", 1, "2026-04-26T10:00:00.000Z", "{}")
	seedAct(t, s, "a", 2, "2026-04-26T10:01:00.000Z", "{}")
	seedAct(t, s, "a", 3, "2026-04-26T10:02:00.000Z", `{"ErrorCode":42}`)
	seedAct(t, s, "a", 4, "2026-04-26T10:03:00.000Z", "{}")
	seedAct(t, s, "a", 99, "2026-04-26T10:04:00.000Z", "{}")

	src := NewStoreActivitySource(s)
	res := src(context.Background(), 10)
	if res.HasError() {
		t.Fatalf("source: %v", res.Error())
	}
	rows := res.Value()
	// 5 store rows in, but Stop (2) and unknown (99) drop → 3 out.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (Stop+unknown dropped):\n%+v", len(rows), rows)
	}
	// Order is DESC by OccurredAt: Heartbeat(4) → Error(3) → Start(1)
	// (after dropping 99 and 2).
	wantKinds := []ActivityKind{
		ActivityPollSucceeded, // Heartbeat
		ActivityPollFailed,    // Error
		ActivityPollStarted,   // Start
	}
	for i, want := range wantKinds {
		if rows[i].Kind != want {
			t.Fatalf("rows[%d].Kind: got %q, want %q", i, rows[i].Kind, want)
		}
	}
	// Spot-check the ErrorCode flow on the failed row.
	if rows[1].ErrorCode != 42 {
		t.Fatalf("ErrorCode flow: got %d, want 42", rows[1].ErrorCode)
	}
}

func TestNewStoreActivitySource_StoreError_WrappedWithErrDbOpen(t *testing.T) {
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "boom.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	_ = s.Close() // force a "DB closed" error on the next QueryContext.

	src := NewStoreActivitySource(s)
	res := src(context.Background(), 10)
	if !res.HasError() {
		t.Fatalf("post-Close: want error, got %d rows", len(res.Value()))
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) {
		t.Fatalf("error not coded: %T %v", res.Error(), res.Error())
	}
	if coded.Code != errtrace.ErrDbOpen {
		t.Fatalf("wrong code: got %v, want ErrDbOpen", coded.Code)
	}
}

func TestMapWatchKindToActivityKind_Table(t *testing.T) {
	cases := []struct {
		in   int
		want ActivityKind
		ok   bool
	}{
		{int(WatchStart), ActivityPollStarted, true},
		{int(WatchHeartbeat), ActivityPollSucceeded, true},
		{int(WatchError), ActivityPollFailed, true},
		{int(WatchEmailStored), ActivityEmailStored, true},
		{int(WatchRuleMatched), ActivityRuleMatched, true},
		{int(WatchStop), "", false},
		{0, "", false},
		{99, "", false},
	}
	for _, c := range cases {
		got, ok := mapWatchKindToActivityKind(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("kind=%d: got (%q, %v), want (%q, %v)",
				c.in, got, ok, c.want, c.ok)
		}
	}
}
