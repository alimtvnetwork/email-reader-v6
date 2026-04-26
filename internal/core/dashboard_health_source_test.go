// dashboard_health_source_test.go — Slice #102 coverage for
// `NewStoreAccountHealthSource`. The adapter is a thin field-copy
// wrapper, so the test surface is small but important: prove the
// nil-store guard, prove the field copy preserves every store-side
// field, prove zero-rows is a non-error empty slice, and prove
// store errors get wrapped with `ErrDbOpen`.
//
// We use a real `*store.Store` (in-memory via `t.TempDir()` +
// `store.OpenAt`) for the happy paths because the alternative —
// faking `*store.Store` — would require exporting an interface from
// the store package just for this test, and the wrapper itself is
// trivial enough that "real DB" is the more honest fixture.
package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestNewStoreAccountHealthSource_NilStore_ReturnsNil(t *testing.T) {
	if got := NewStoreAccountHealthSource(nil); got != nil {
		t.Errorf("nil store should yield nil source, got %v", got)
	}
}

func TestNewStoreAccountHealthSource_EmptyDB_NoError(t *testing.T) {
	src := NewStoreAccountHealthSource(openTestStore(t))
	if src == nil {
		t.Fatal("source unexpectedly nil")
	}
	res := src(context.Background())
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	if got := res.Value(); len(got) != 0 {
		t.Errorf("want empty slice, got %d rows", len(got))
	}
}

func TestNewStoreAccountHealthSource_FieldCopy_Preserves(t *testing.T) {
	s := openTestStore(t)
	// Seed via raw SQL — same convention as the store-side shim
	// tests; keeps this file independent of any future seed-helper
	// extraction.
	if _, err := s.DB.Exec(
		`INSERT INTO WatchEvents (Alias, Kind, OccurredAt) VALUES ('a', 1, '2026-04-26T10:00:00.000Z')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.DB.Exec(
		`INSERT INTO Emails (Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
		   Subject, BodyText, BodyHtml, ReceivedAt, FilePath, IsRead)
		 VALUES ('a', '<m1>', 1, '', '', '', '', '', '', '2026-04-01T00:00:00.000Z', '', 0)`,
	); err != nil {
		t.Fatalf("seed email: %v", err)
	}

	src := NewStoreAccountHealthSource(s)
	res := src(context.Background())
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	rows := res.Value()
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Alias != "a" {
		t.Errorf("Alias = %q, want a", r.Alias)
	}
	wantPoll, _ := time.Parse(time.RFC3339Nano, "2026-04-26T10:00:00.000Z")
	if !r.LastPollAt.Equal(wantPoll) {
		t.Errorf("LastPollAt = %v, want %v", r.LastPollAt, wantPoll)
	}
	if !r.LastErrorAt.IsZero() {
		t.Errorf("LastErrorAt should be zero, got %v", r.LastErrorAt)
	}
	if r.EmailsStored != 1 || r.UnreadCount != 1 {
		t.Errorf("counts = stored:%d unread:%d, want 1/1", r.EmailsStored, r.UnreadCount)
	}
	// ConsecutiveFailures and Health are not populated by the source —
	// the service overwrites Health, and ConsecutiveFailures awaits
	// a follow-on slice (per queries.AccountHealthSelectAll docs).
	if r.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures should be zero (deferred), got %d", r.ConsecutiveFailures)
	}
	if r.Health != "" {
		t.Errorf("Health should be empty (service computes it), got %q", r.Health)
	}
}

func TestNewStoreAccountHealthSource_StoreError_WrappedWithErrDbOpen(t *testing.T) {
	// Open then immediately close so QueryContext fails with
	// "sql: database is closed". This is the cleanest way to exercise
	// the err-wrap path without mocking the *sql.DB.
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.Close()

	src := NewStoreAccountHealthSource(s)
	res := src(context.Background())
	if !res.HasError() {
		t.Fatal("expected error after Close")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbOpen {
		t.Errorf("expected ErrDbOpen wrap, got %v", res.Error())
	}
}
