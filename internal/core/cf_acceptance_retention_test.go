// cf_acceptance_retention_test.go locks CF-S-RET: the OpenedUrls audit
// retention contract end-to-end. The Settings service surfaces an
// `OpenUrlsRetentionDays` knob; the store exposes
// `PruneOpenedUrlsBefore(cutoff)`; the helpers `RetentionCutoff` +
// `ShouldRunRetentionTick` decide *when* and *what cutoff*. This test
// drives the full chain on a real on-disk SQLite DB to prove that:
//
//	(1) A retention setting > 0 produces a non-zero cutoff, and feeding
//	    that cutoff into the store DELETEs exactly the rows older than it.
//	(2) Setting `OpenUrlsRetentionDays = 0` produces a zero cutoff, which
//	    is a no-op — never deletes anything.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §1, §3.
package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/store"
)

func TestCF_S_RET_PruneRespectsSettingsKnob(t *testing.T) {
	dir := t.TempDir()
	s, err := store.OpenAt(filepath.Join(dir, "cf-ret.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	id, _, err := s.UpsertEmail(ctx, &store.Email{
		Alias: "a", MessageId: "m1", Uid: 1,
	})
	if err != nil {
		t.Fatalf("UpsertEmail: %v", err)
	}
	if _, err := s.RecordOpenedUrl(ctx, id, "r", "https://stale/"); err != nil {
		t.Fatalf("record stale: %v", err)
	}
	if _, err := s.RecordOpenedUrl(ctx, id, "r", "https://fresh/"); err != nil {
		t.Fatalf("record fresh: %v", err)
	}
	stale := time.Now().UTC().Add(-365 * 24 * time.Hour)
	if _, err := s.DB.Exec(`UPDATE OpenedUrls SET OpenedAt=? WHERE Url=?`,
		stale, "https://stale/"); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	now := time.Now().UTC()

	// (1) Retention enabled at 90d → stale row gone, fresh row stays.
	cutoff := RetentionCutoff(now, DefaultSettingsInput().OpenUrlsRetentionDays)
	if cutoff.IsZero() {
		t.Fatalf("RetentionCutoff with default days = zero (expected non-zero)")
	}
	deleted, err := s.PruneOpenedUrlsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (stale row only)", deleted)
	}
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM OpenedUrls`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("remaining = %d, want 1 (fresh row)", n)
	}

	// (2) Retention disabled (days=0) → cutoff zero → prune is a no-op
	//     even when called directly. Insert another stale row to prove it.
	if _, err := s.RecordOpenedUrl(ctx, id, "r", "https://stale2/"); err != nil {
		t.Fatalf("record stale2: %v", err)
	}
	if _, err := s.DB.Exec(`UPDATE OpenedUrls SET OpenedAt=? WHERE Url=?`,
		stale, "https://stale2/"); err != nil {
		t.Fatalf("backdate2: %v", err)
	}
	zeroCutoff := RetentionCutoff(now, 0)
	if !zeroCutoff.IsZero() {
		t.Fatalf("RetentionCutoff(0) = %v, want zero", zeroCutoff)
	}
	deleted, err = s.PruneOpenedUrlsBefore(ctx, zeroCutoff)
	if err != nil {
		t.Fatalf("Prune disabled: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("disabled prune deleted %d rows, want 0", deleted)
	}
}

// TestCF_S_RET_SettingsRoundTripPersistsRetention proves the
// OpenUrlsRetentionDays field round-trips through Save + Get and
// validates the 0..3650 range.
func TestCF_S_RET_SettingsRoundTripPersistsRetention(t *testing.T) {
	withIsolatedConfig(t, func() {
		r := NewSettings(time.Now)
		if r.HasError() {
			t.Fatalf("NewSettings: %v", r.Error())
		}
		s := r.Value()

		in := DefaultSettingsInput()
		in.OpenUrlsRetentionDays = 30
		if rs := s.Save(context.Background(), in); rs.HasError() {
			t.Fatalf("Save: %v", rs.Error())
		}
		got := s.Get(context.Background())
		if got.HasError() {
			t.Fatalf("Get: %v", got.Error())
		}
		if got.Value().OpenUrlsRetentionDays != 30 {
			t.Fatalf("RetentionDays round-trip = %d, want 30",
				got.Value().OpenUrlsRetentionDays)
		}

		// Out-of-range rejected.
		bad := DefaultSettingsInput()
		bad.OpenUrlsRetentionDays = 9999
		if rs := s.Save(context.Background(), bad); !rs.HasError() {
			t.Fatalf("Save with 9999 days should fail validation")
		}
	})
}
