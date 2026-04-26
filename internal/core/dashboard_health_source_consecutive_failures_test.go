// dashboard_health_source_consecutive_failures_test.go — Slice #106
// adapter coverage. Confirms `NewStoreAccountHealthSource` now
// propagates `ConsecutiveFailures` from the store row through to
// `AccountHealthRow` (vs. Slice #102 where the field was hard-zeroed).
package core

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/store"
)

func TestNewStoreAccountHealthSource_PropagatesConsecutiveFailures(t *testing.T) {
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "cf.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	const alias = "stuck@example.com"
	for i := 0; i < 5; i++ {
		if err := s.BumpConsecutiveFailures(ctx, alias); err != nil {
			t.Fatalf("bump #%d: %v", i, err)
		}
	}

	src := NewStoreAccountHealthSource(s)
	if src == nil {
		t.Fatal("NewStoreAccountHealthSource returned nil for a real store")
	}
	res := src(ctx)
	if res.IsErr() {
		t.Fatalf("source: %v", res.Err())
	}
	rows := res.Value()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].ConsecutiveFailures != 5 {
		t.Errorf("ConsecutiveFailures = %d, want 5 (Slice #106 wiring)", rows[0].ConsecutiveFailures)
	}
}
