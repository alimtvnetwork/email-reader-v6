// dashboard_activity_persistor_test.go — Slice #107 coverage for the
// WatchEvent → SQLite audit-row pipeline.
package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/eventbus"
	"github.com/lovable/email-read/internal/store"
)

func openPersistorTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestEncodeWatchEventPayload_EmptyMessage_ReturnsSchemaDefault(t *testing.T) {
	got := encodeWatchEventPayload(WatchEvent{Kind: WatchHeartbeat, Alias: "a"})
	if got != "{}" {
		t.Errorf("got %q, want %q (schema default for byte parity)", got, "{}")
	}
}

func TestEncodeWatchEventPayload_WithMessage_EncodesJSON(t *testing.T) {
	got := encodeWatchEventPayload(WatchEvent{Message: "new mail"})
	want := `{"Message":"new mail"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStartWatchEventPersistor_NilSink_NoPanic(t *testing.T) {
	bus := eventbus.New[WatchEvent](4)
	stop := StartWatchEventPersistor(context.Background(), bus, nil)
	stop()
	stop() // idempotent
}

func TestStartWatchEventPersistor_NilBus_NoPanic(t *testing.T) {
	s := openPersistorTestStore(t)
	stop := StartWatchEventPersistor(context.Background(), nil, s)
	stop()
}

func TestStartWatchEventPersistor_PersistsAllKinds(t *testing.T) {
	s := openPersistorTestStore(t)
	bus := eventbus.New[WatchEvent](16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartWatchEventPersistor(ctx, bus, s)
	defer stop()

	at := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	for i, k := range []WatchEventKind{WatchStart, WatchHeartbeat, WatchError, WatchEmailStored, WatchRuleMatched} {
		bus.Publish(WatchEvent{Kind: k, Alias: "a", At: at.Add(time.Duration(i) * time.Second), Message: "m"})
	}

	// Poll until 5 rows land or 2s elapses.
	deadline := time.Now().Add(2 * time.Second)
	var n int
	for time.Now().Before(deadline) {
		if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM WatchEvents`).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		if n >= 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n != 5 {
		t.Fatalf("got %d persisted rows, want 5", n)
	}

	rows, err := s.QueryRecentActivity(ctx, 10)
	if err != nil {
		t.Fatalf("QueryRecentActivity: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("recent: got %d rows, want 5", len(rows))
	}
	// Verify Kind=5 and Kind=6 made it through unchanged.
	seen := map[int]bool{}
	for _, r := range rows {
		seen[r.Kind] = true
	}
	if !seen[int(WatchEmailStored)] || !seen[int(WatchRuleMatched)] {
		t.Errorf("missing new kinds in persisted rows; saw %+v", seen)
	}
}

// ----- Slice #108: ErrorCode payload extraction --------------------

func TestExtractErrorCode_Table(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain error has no code", errors.New("boom"), 0},
		{"coded with numeric tail",
			errtrace.NewCoded(errtrace.Code("ER-EXP-21601"), "do thing"), 21601},
		{"wrapped coded is found via errors.As",
			fmt.Errorf("outer: %w",
				errtrace.NewCoded(errtrace.Code("ER-CFG-21042"), "open")), 21042},
		{"empty code", errtrace.NewCoded(errtrace.Code(""), "x"), 0},
		{"malformed (no dash)", errtrace.NewCoded(errtrace.Code("ER21999"), "x"), 0},
		{"malformed (trailing dash)", errtrace.NewCoded(errtrace.Code("ER-FOO-"), "x"), 0},
		{"malformed (non-numeric tail)", errtrace.NewCoded(errtrace.Code("ER-FOO-BAR"), "x"), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractErrorCode(tc.err); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEncodeWatchEventPayload_WithErrorCode(t *testing.T) {
	ev := WatchEvent{
		Kind:  WatchError,
		Alias: "a",
		Err:   errtrace.NewCoded(errtrace.Code("ER-EXP-21601"), "expand"),
	}
	got := encodeWatchEventPayload(ev)
	want := `{"ErrorCode":21601}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEncodeWatchEventPayload_MessageAndErrorCode(t *testing.T) {
	ev := WatchEvent{
		Message: "expand failed",
		Err:     errtrace.NewCoded(errtrace.Code("ER-EXP-21601"), "expand"),
	}
	got := encodeWatchEventPayload(ev)
	want := `{"Message":"expand failed","ErrorCode":21601}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEncodeWatchEventPayload_NilErrAndEmptyMessage_StillSchemaDefault(t *testing.T) {
	if got := encodeWatchEventPayload(WatchEvent{Kind: WatchHeartbeat}); got != "{}" {
		t.Errorf("got %q, want %q", got, "{}")
	}
}

func TestPersistor_RoundTripsErrorCodeThroughSQLite(t *testing.T) {
	s := openPersistorTestStore(t)
	bus := eventbus.New[WatchEvent](4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartWatchEventPersistor(ctx, bus, s)
	defer stop()

	bus.Publish(WatchEvent{
		Kind:  WatchError,
		Alias: "a",
		At:    time.Now(),
		Err:   errtrace.NewCoded(errtrace.Code("ER-EXP-21601"), "expand"),
	})

	// Poll for the row to appear.
	deadline := time.Now().Add(2 * time.Second)
	var rows []store.StoreActivityRow
	for time.Now().Before(deadline) {
		var err error
		rows, err = s.QueryRecentActivity(ctx, 1)
		if err != nil {
			t.Fatalf("QueryRecentActivity: %v", err)
		}
		if len(rows) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].ErrorCode != 21601 {
		t.Errorf("ErrorCode round-trip: got %d, want 21601", rows[0].ErrorCode)
	}
}
