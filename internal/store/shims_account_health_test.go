// shims_account_health_test.go — Slice #102 coverage for
// `(*Store).QueryAccountHealth` against a real in-memory SQLite (via
// `newTestStore`). Pins the contract documented on
// `queries.AccountHealthSelectAll` + `Store.QueryAccountHealth`:
//
//   - Empty DB → empty slice, no error.
//   - Alias seen only via WatchEvents (no Emails) → row with zero
//     EmailsStored / UnreadCount.
//   - Alias seen only via Emails (no WatchEvents) → row with zero
//     LastPollAt / LastErrorAt.
//   - Mixed alias → all four fields populated; LastPollAt picks
//     the latest of {Start, Heartbeat}; Stop events do NOT advance
//     LastPollAt; LastErrorAt picks the latest Error.
//   - UnreadCount counts only `IsRead = 0` rows.
//   - Deterministic ordering (alias ASC) — stops downstream golden
//     tests from flaking on the union's iteration order.
package store

import (
	"context"
	"testing"
	"time"
)

// seedWatchEvent inserts one WatchEvents row with an explicit
// timestamp. We pass the timestamp instead of relying on the
// default `strftime('now')` so the test can exercise the
// "latest Heartbeat wins over older Start" case deterministically.
func seedWatchEvent(t *testing.T, s *Store, alias string, kind int, occurredAt string) {
	t.Helper()
	if _, err := s.DB.Exec(
		`INSERT INTO WatchEvents (Alias, Kind, OccurredAt) VALUES (?, ?, ?)`,
		alias, kind, occurredAt,
	); err != nil {
		t.Fatalf("seed WatchEvents (%s, %d, %s): %v", alias, kind, occurredAt, err)
	}
}

// seedEmail inserts a minimal Emails row. `isRead` controls the
// IsRead column added by m0010.
func seedEmail(t *testing.T, s *Store, alias string, uid uint32, isRead bool) {
	t.Helper()
	read := 0
	if isRead {
		read = 1
	}
	if _, err := s.DB.Exec(
		`INSERT INTO Emails (Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
		   Subject, BodyText, BodyHtml, ReceivedAt, FilePath, IsRead)
		 VALUES (?, ?, ?, '', '', '', '', '', '', '2026-04-01T00:00:00.000Z', '', ?)`,
		alias, "<msg-"+alias+"-"+itoa(int(uid))+">", uid, read,
	); err != nil {
		t.Fatalf("seed Emails (%s, %d): %v", alias, uid, err)
	}
}

// itoa lives in ast_no_sql_type_leak_test.go (same package); reused here.

func TestQueryAccountHealth_EmptyDB_ReturnsEmptySlice(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.QueryAccountHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("want empty, got %d rows: %+v", len(rows), rows)
	}
}

func TestQueryAccountHealth_OnlyWatchEvents_NoEmails(t *testing.T) {
	s := newTestStore(t)
	seedWatchEvent(t, s, "alpha", 1, "2026-04-26T10:00:00.000Z") // Start
	seedWatchEvent(t, s, "alpha", 4, "2026-04-26T10:05:00.000Z") // Heartbeat (latest poll)
	seedWatchEvent(t, s, "alpha", 3, "2026-04-26T10:03:00.000Z") // Error

	rows, err := s.QueryAccountHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.Alias != "alpha" {
		t.Errorf("Alias = %q, want alpha", r.Alias)
	}
	wantPoll, _ := time.Parse(time.RFC3339Nano, "2026-04-26T10:05:00.000Z")
	if !r.LastPollAt.Equal(wantPoll) {
		t.Errorf("LastPollAt = %v, want %v (latest Heartbeat)", r.LastPollAt, wantPoll)
	}
	wantErr, _ := time.Parse(time.RFC3339Nano, "2026-04-26T10:03:00.000Z")
	if !r.LastErrorAt.Equal(wantErr) {
		t.Errorf("LastErrorAt = %v, want %v", r.LastErrorAt, wantErr)
	}
	if r.EmailsStored != 0 || r.UnreadCount != 0 {
		t.Errorf("expected zero email counts, got Stored=%d Unread=%d", r.EmailsStored, r.UnreadCount)
	}
}

func TestQueryAccountHealth_OnlyEmails_NoWatchEvents(t *testing.T) {
	s := newTestStore(t)
	seedEmail(t, s, "beta", 1, false) // unread
	seedEmail(t, s, "beta", 2, true)  // read
	seedEmail(t, s, "beta", 3, false) // unread

	rows, err := s.QueryAccountHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Alias != "beta" {
		t.Errorf("Alias = %q, want beta", r.Alias)
	}
	if !r.LastPollAt.IsZero() {
		t.Errorf("LastPollAt should be zero (no WatchEvents), got %v", r.LastPollAt)
	}
	if !r.LastErrorAt.IsZero() {
		t.Errorf("LastErrorAt should be zero (no WatchEvents), got %v", r.LastErrorAt)
	}
	if r.EmailsStored != 3 {
		t.Errorf("EmailsStored = %d, want 3", r.EmailsStored)
	}
	if r.UnreadCount != 2 {
		t.Errorf("UnreadCount = %d, want 2", r.UnreadCount)
	}
}

func TestQueryAccountHealth_StopKindDoesNotAdvanceLastPoll(t *testing.T) {
	s := newTestStore(t)
	// A Stop (kind=2) at 10:30 must NOT win over a Start (kind=1) at 10:00,
	// because LastPollAt is "watcher is alive" — Stop is the opposite signal.
	seedWatchEvent(t, s, "gamma", 1, "2026-04-26T10:00:00.000Z") // Start
	seedWatchEvent(t, s, "gamma", 2, "2026-04-26T10:30:00.000Z") // Stop (must be ignored)

	rows, err := s.QueryAccountHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	wantPoll, _ := time.Parse(time.RFC3339Nano, "2026-04-26T10:00:00.000Z")
	if !rows[0].LastPollAt.Equal(wantPoll) {
		t.Errorf("LastPollAt = %v, want %v (Stop should not advance LastPollAt)",
			rows[0].LastPollAt, wantPoll)
	}
}

func TestQueryAccountHealth_MixedAliases_OrderedAlphabetically(t *testing.T) {
	s := newTestStore(t)
	// Insert in non-alpha order to prove the ORDER BY in the query.
	seedEmail(t, s, "zeta", 1, false)
	seedWatchEvent(t, s, "zeta", 4, "2026-04-26T11:00:00.000Z")
	seedWatchEvent(t, s, "alpha", 1, "2026-04-26T09:00:00.000Z")
	seedEmail(t, s, "mike", 1, true)

	rows, err := s.QueryAccountHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(rows), rows)
	}
	want := []string{"alpha", "mike", "zeta"}
	for i, r := range rows {
		if r.Alias != want[i] {
			t.Errorf("rows[%d].Alias = %q, want %q (alphabetical ordering broken)",
				i, r.Alias, want[i])
		}
	}
	// Spot-check the union: 'alpha' has watch events but no emails;
	// 'mike' has emails but no watch events; 'zeta' has both.
	if rows[0].EmailsStored != 0 {
		t.Errorf("alpha.EmailsStored = %d, want 0", rows[0].EmailsStored)
	}
	if !rows[1].LastPollAt.IsZero() {
		t.Errorf("mike.LastPollAt = %v, want zero", rows[1].LastPollAt)
	}
	if rows[2].EmailsStored != 1 || rows[2].UnreadCount != 1 {
		t.Errorf("zeta counts = stored:%d unread:%d, want 1/1",
			rows[2].EmailsStored, rows[2].UnreadCount)
	}
}
