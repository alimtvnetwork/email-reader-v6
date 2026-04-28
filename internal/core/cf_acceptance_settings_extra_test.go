// cf_acceptance_settings_extra_test.go locks two informational
// cross-feature contracts that did not make the original 11-row CF
// matrix but were called out in `spec/21-app/02-features/07-settings/
// 99-consistency-report.md` as worth regression-locking:
//
//	CF-S-MONO — `Settings.UpdatedAt` is monotonically non-decreasing
//	            across consecutive Save calls. Two saves with a strictly
//	            advancing clock MUST produce a strictly later UpdatedAt;
//	            two saves at the same clock instant MUST produce equal
//	            UpdatedAt (no regression to an older time). This is the
//	            ordering guarantee Tools / Watch consumers rely on when
//	            comparing Snapshot freshness.
//
//	CF-S-EVENT-MONO — Same ordering applies to `SettingsEvent.Snapshot.
//	                  UpdatedAt`: the event the bus publishes carries
//	                  the same RFC3339Nano value persisted on disk
//	                  (i.e., subscribers see the same "as-of" time the
//	                  next `Get()` would return).
//
// Both tests inject a controllable clock so the assertions are
// deterministic — production code reads `s.clock` for `UpdatedAt`.
//
// Spec: spec/21-app/02-features/07-settings/01-backend.md §4
//
//	(UpdatedAt) and §6 (Subscribe contract).
package core

import (
	"context"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// newSettingsWithClock builds a Settings whose `now()` returns the
// values supplied by `tickFn`. Each call to tickFn advances the
// in-memory clock; tests pass a closure over a slice / counter.
func newSettingsWithClock(t *testing.T, tickFn func() time.Time) *Settings {
	t.Helper()
	r := NewSettings(tickFn)
	if r.HasError() {
		t.Fatalf("NewSettings: %v", r.Error())
	}
	return r.Value()
}

// TestCF_S_Mono_UpdatedAt_AdvancesWithClock proves that two Save calls
// separated by a strictly later clock tick produce a strictly later
// UpdatedAt on the next Get.
func TestCF_S_Mono_UpdatedAt_AdvancesWithClock(t *testing.T) {
	withIsolatedConfig(t, func() {
		ticks := []time.Time{
			time.Unix(1_700_000_000, 0).UTC(),
			time.Unix(1_700_000_001, 500_000_000).UTC(),
		}
		i := 0
		s := newSettingsWithClock(t, func() time.Time {
			ts := ticks[i]
			if i < len(ticks)-1 {
				i++
			}
			return ts
		})

		first := saveAndGetUpdatedAt(t, s, 5)
		second := saveAndGetUpdatedAt(t, s, 7)
		if !second.After(first) {
			t.Fatalf("UpdatedAt regressed or stalled: first=%s second=%s",
				first.Format(time.RFC3339Nano),
				second.Format(time.RFC3339Nano))
		}
	})
}

// TestCF_S_Mono_UpdatedAt_StableAtSameInstant proves that two Saves at
// the SAME clock instant produce IDENTICAL UpdatedAt (no fake advance,
// no regression). This locks the contract that UpdatedAt is sourced
// purely from the clock — never from a hidden monotonic side-channel.
func TestCF_S_Mono_UpdatedAt_StableAtSameInstant(t *testing.T) {
	withIsolatedConfig(t, func() {
		fixed := time.Unix(1_700_000_000, 123_456_789).UTC()
		s := newSettingsWithClock(t, func() time.Time { return fixed })
		first := saveAndGetUpdatedAt(t, s, 5)
		second := saveAndGetUpdatedAt(t, s, 9)
		if !first.Equal(second) {
			t.Fatalf("UpdatedAt not stable at fixed clock: first=%s second=%s",
				first.Format(time.RFC3339Nano),
				second.Format(time.RFC3339Nano))
		}
	})
}

// TestCF_S_EventMono_PublishedSnapshotMatchesGet proves the published
// SettingsEvent.Snapshot.UpdatedAt equals the value the next Get()
// returns — subscribers MUST see the same as-of time as direct readers.
func TestCF_S_EventMono_PublishedSnapshotMatchesGet(t *testing.T) {
	withIsolatedConfig(t, func() {
		fixed := time.Unix(1_700_000_500, 0).UTC()
		s := newSettingsWithClock(t, func() time.Time { return fixed })
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		events, sub := s.Subscribe(ctx)
		defer sub()

		in := DefaultSettingsInput()
		in.PollSeconds = 11
		if r := s.Save(context.Background(), in); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		var evUpdatedAt time.Time
		select {
		case ev := <-events:
			evUpdatedAt = ev.Snapshot.UpdatedAt
		case <-time.After(time.Second):
			t.Fatal("no SettingsEvent within 1s of Save")
		}

		got := s.Get(context.Background())
		if got.HasError() {
			t.Fatalf("Get: %v", got.Error())
		}
		if !evUpdatedAt.Equal(got.Value().UpdatedAt) {
			t.Fatalf("event UpdatedAt %s != Get UpdatedAt %s",
				evUpdatedAt.Format(time.RFC3339Nano),
				got.Value().UpdatedAt.Format(time.RFC3339Nano))
		}
		if evUpdatedAt.IsZero() {
			t.Fatal("event UpdatedAt is zero — Save did not stamp the field")
		}
	})
}

// saveAndGetUpdatedAt persists `pollSeconds` and returns the
// post-Save UpdatedAt as observed by Get. Wraps the boilerplate so the
// test bodies stay readable.
func saveAndGetUpdatedAt(t *testing.T, s *Settings, pollSeconds uint16) time.Time {
	t.Helper()
	in := DefaultSettingsInput()
	in.PollSeconds = pollSeconds
	if r := s.Save(context.Background(), in); r.HasError() {
		t.Fatalf("Save(poll=%d): %v", pollSeconds, r.Error())
	}
	r := s.Get(context.Background())
	if r.HasError() {
		t.Fatalf("Get: %v", r.Error())
	}
	return r.Value().UpdatedAt
}

// _ keeps the errtrace import live; future regression tests in this
// file will likely need it for direct error-code assertions.
var _ = errtrace.Ok[struct{}]
