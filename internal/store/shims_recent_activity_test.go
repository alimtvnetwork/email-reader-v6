// shims_recent_activity_test.go — Slice #104 coverage for
// `(*Store).QueryRecentActivity` against a real in-memory SQLite
// (via `newTestStore`). Pins the contract documented on
// `queries.RecentActivitySelectN` + `Store.QueryRecentActivity`:
//
//   - Empty DB → empty slice, no error.
//   - DESC ordering by OccurredAt (with Id DESC tie-break).
//   - LIMIT honoured (fewer rows requested than exist).
//   - Limit ≤0 short-circuits to empty without touching SQL.
//   - Payload JSON decoded into Message + ErrorCode.
//   - Empty `'{}'` payload yields zero-valued Message/ErrorCode.
//   - Malformed payload tolerated — row still returned with empty
//     Message/zero ErrorCode (defensive contract).
package store

import (
	"context"
	"testing"
)

func seedActivity(t *testing.T, s *Store, alias string, kind int, occurredAt, payload string) {
	t.Helper()
	if _, err := s.DB.Exec(
		`INSERT INTO WatchEvents (Alias, Kind, OccurredAt, Payload) VALUES (?, ?, ?, ?)`,
		alias, kind, occurredAt, payload,
	); err != nil {
		t.Fatalf("seed WatchEvents (%s, %d, %s, %s): %v", alias, kind, occurredAt, payload, err)
	}
}

func TestQueryRecentActivity_EmptyDB_EmptySlice(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.QueryRecentActivity(context.Background(), 10)
	if err != nil {
		t.Fatalf("QueryRecentActivity: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty DB: got %d rows, want 0", len(rows))
	}
}

func TestQueryRecentActivity_NonPositiveLimit_NoSQL(t *testing.T) {
	s := newTestStore(t)
	// Seed something so we'd notice if the SQL ran and returned it.
	seedActivity(t, s, "a", 1, "2026-04-26T10:00:00.000Z", "{}")
	for _, lim := range []int{0, -1, -100} {
		rows, err := s.QueryRecentActivity(context.Background(), lim)
		if err != nil {
			t.Fatalf("limit=%d: unexpected error: %v", lim, err)
		}
		if len(rows) != 0 {
			t.Fatalf("limit=%d: want empty, got %d rows", lim, len(rows))
		}
	}
}

func TestQueryRecentActivity_DescOrderingAndLimit(t *testing.T) {
	s := newTestStore(t)
	// Seed 3 events at distinct times, out-of-order on insert so the
	// ORDER BY (not insert order) is what proves the contract.
	seedActivity(t, s, "a", 1, "2026-04-26T10:00:00.000Z", "{}")
	seedActivity(t, s, "b", 4, "2026-04-26T10:02:00.000Z", "{}")
	seedActivity(t, s, "c", 3, "2026-04-26T10:01:00.000Z", "{}")

	rows, err := s.QueryRecentActivity(context.Background(), 2)
	if err != nil {
		t.Fatalf("QueryRecentActivity: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("limit=2: got %d rows, want 2", len(rows))
	}
	// Newest first: b (10:02) then c (10:01).
	if rows[0].Alias != "b" || rows[1].Alias != "c" {
		t.Fatalf("DESC order broken: got [%s, %s], want [b, c]",
			rows[0].Alias, rows[1].Alias)
	}
	if rows[0].Kind != 4 || rows[1].Kind != 3 {
		t.Fatalf("Kind round-trip broken: %+v", rows)
	}
}

func TestQueryRecentActivity_PayloadDecoded(t *testing.T) {
	s := newTestStore(t)
	seedActivity(t, s, "a", 3, "2026-04-26T10:00:00.000Z",
		`{"Message":"poll #847 boom","ErrorCode":21104}`)
	rows, err := s.QueryRecentActivity(context.Background(), 10)
	if err != nil {
		t.Fatalf("QueryRecentActivity: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Message != "poll #847 boom" {
		t.Fatalf("Message: got %q, want %q", rows[0].Message, "poll #847 boom")
	}
	if rows[0].ErrorCode != 21104 {
		t.Fatalf("ErrorCode: got %d, want 21104", rows[0].ErrorCode)
	}
}

func TestQueryRecentActivity_EmptyPayloadYieldsZeroValues(t *testing.T) {
	s := newTestStore(t)
	seedActivity(t, s, "a", 1, "2026-04-26T10:00:00.000Z", "{}")
	rows, err := s.QueryRecentActivity(context.Background(), 10)
	if err != nil {
		t.Fatalf("QueryRecentActivity: %v", err)
	}
	if len(rows) != 1 || rows[0].Message != "" || rows[0].ErrorCode != 0 {
		t.Fatalf("empty payload: got %+v, want zero Message/ErrorCode", rows[0])
	}
}

func TestQueryRecentActivity_MalformedPayloadTolerated(t *testing.T) {
	s := newTestStore(t)
	// Garbage JSON — the row should still be returned with the
	// timestamp/Kind/Alias intact and zero-valued Message/ErrorCode.
	seedActivity(t, s, "a", 3, "2026-04-26T10:00:00.000Z", `{"Mes`)
	rows, err := s.QueryRecentActivity(context.Background(), 10)
	if err != nil {
		t.Fatalf("QueryRecentActivity: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Alias != "a" || rows[0].Kind != 3 {
		t.Fatalf("core fields lost: %+v", rows[0])
	}
	if rows[0].Message != "" || rows[0].ErrorCode != 0 {
		t.Fatalf("malformed payload should yield zero values, got %+v", rows[0])
	}
}
