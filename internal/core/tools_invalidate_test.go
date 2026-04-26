// tools_invalidate_test.go locks the AccountEvent → diagnose-cache
// invalidation contract (spec §2.3 follow-up).
package core

import (
	"context"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// helper: build a Tools with a stub launcher + recorder so we can
// exercise diagCache without a real browser/store.
func newToolsForInvalidateTest(t *testing.T) *Tools {
	t.Helper()
	r := NewTools(stubLauncher{}, stubRecorder{}, DefaultToolsConfig())
	if r.HasError() {
		t.Fatalf("NewTools: %v", r.Error())
	}
	return r.Value()
}

// stubLauncher / stubRecorder are minimal nil-impls; OpenUrl is never
// invoked in this file, so the methods are intentionally unused.
type stubLauncher struct{}

func (stubLauncher) Open(string) error    { return nil }
func (stubLauncher) Path() (string, error) { return "/usr/bin/stub", nil }

type stubRecorder struct{}

func (stubRecorder) HasOpenedUrl(context.Context, int64, string) (bool, error) { return false, nil }
func (stubRecorder) RecordOpenedUrl(context.Context, int64, string, string) (bool, error) {
	return false, nil
}

// primeDiagCache pre-populates the cache with a fake entry for `alias`.
// We bypass CachedDiagnose so the test stays focused on the eviction
// path — equivalent to a prior successful Diagnose run.
func primeDiagCache(t *testing.T, tools *Tools, alias string) {
	t.Helper()
	tools.diagCache().put(alias, DiagnosticsReport{
		Alias:    alias,
		StoredAt: time.Now(),
		Events:   []DiagnoseEvent{{Stage: "primed"}},
	})
	if _, hit := tools.diagCache().get(alias, time.Now()); !hit {
		t.Fatalf("primeDiagCache: cache miss right after put for %q", alias)
	}
}

// Test_HandleAccountEvent_EvictsOnUpdated locks the §2.3 contract:
// updating an account drops its cached diagnose trail.
func Test_HandleAccountEvent_EvictsOnUpdated(t *testing.T) {
	tools := newToolsForInvalidateTest(t)
	primeDiagCache(t, tools, "work")
	tools.handleAccountEvent(AccountEvent{Kind: AccountUpdated, Alias: "work"})
	if _, hit := tools.diagCache().get("work", time.Now()); hit {
		t.Errorf("AccountUpdated did not evict cache entry for %q", "work")
	}
}

// Test_HandleAccountEvent_EvictsOnRemoved mirrors the Updated case.
func Test_HandleAccountEvent_EvictsOnRemoved(t *testing.T) {
	tools := newToolsForInvalidateTest(t)
	primeDiagCache(t, tools, "personal")
	tools.handleAccountEvent(AccountEvent{Kind: AccountRemoved, Alias: "personal"})
	if _, hit := tools.diagCache().get("personal", time.Now()); hit {
		t.Errorf("AccountRemoved did not evict cache entry for %q", "personal")
	}
}

// Test_HandleAccountEvent_AddedIsNoop confirms AccountAdded does not
// evict (there can't be a stale entry; over-eviction is harmless but
// indicates a misclassification).
func Test_HandleAccountEvent_AddedIsNoop(t *testing.T) {
	tools := newToolsForInvalidateTest(t)
	primeDiagCache(t, tools, "shared")
	tools.handleAccountEvent(AccountEvent{Kind: AccountAdded, Alias: "shared"})
	if _, hit := tools.diagCache().get("shared", time.Now()); !hit {
		t.Errorf("AccountAdded should NOT evict an unrelated cached entry")
	}
}

// Test_HandleAccountEvent_PerAliasIsolation verifies that an event for
// one alias never evicts a different alias's entry.
func Test_HandleAccountEvent_PerAliasIsolation(t *testing.T) {
	tools := newToolsForInvalidateTest(t)
	primeDiagCache(t, tools, "work")
	primeDiagCache(t, tools, "personal")
	tools.handleAccountEvent(AccountEvent{Kind: AccountUpdated, Alias: "work"})
	if _, hit := tools.diagCache().get("work", time.Now()); hit {
		t.Errorf("work entry not evicted")
	}
	if _, hit := tools.diagCache().get("personal", time.Now()); !hit {
		t.Errorf("personal entry was evicted by an unrelated work event")
	}
}

// Test_WatchAccountEvents_EndToEnd is the integration smoke: subscribe
// via the real bus, publish an Updated event, observe eviction.
func Test_WatchAccountEvents_EndToEnd(t *testing.T) {
	t.Cleanup(resetAccountBusForTest)
	tools := newToolsForInvalidateTest(t)
	primeDiagCache(t, tools, "ops")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := tools.WatchAccountEvents(ctx)
	defer stop()
	publishAccountEvent(AccountUpdated, "ops")
	// Bus delivery is async; poll for up to 1 s.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, hit := tools.diagCache().get("ops", time.Now()); !hit {
			return // success
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("WatchAccountEvents did not evict ops entry within 1 s")
}

// Test_WatchAccountEvents_StopUnsubscribes confirms calling stop
// detaches the goroutine — subsequent events do NOT evict.
func Test_WatchAccountEvents_StopUnsubscribes(t *testing.T) {
	t.Cleanup(resetAccountBusForTest)
	tools := newToolsForInvalidateTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := tools.WatchAccountEvents(ctx)
	stop()
	primeDiagCache(t, tools, "later")
	publishAccountEvent(AccountUpdated, "later")
	time.Sleep(20 * time.Millisecond) // give any (incorrectly attached) goroutine a chance
	if _, hit := tools.diagCache().get("later", time.Now()); !hit {
		t.Errorf("entry evicted after stop() — unsubscribe leak")
	}
}

// Test_AccountBus_DropsOnSlowSubscriber confirms the non-blocking
// publish contract: a saturated subscriber drops events instead of
// blocking the publisher (mirrors watcher.Bus). We saturate by
// subscribing without draining and publishing > bufSize events.
func Test_AccountBus_DropsOnSlowSubscriber(t *testing.T) {
	t.Cleanup(resetAccountBusForTest)
	_, cancel := SubscribeAccountEvents()
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			publishAccountEvent(AccountUpdated, "spam")
		}
		close(done)
	}()
	select {
	case <-done:
		// publisher completed → non-blocking drop contract holds.
	case <-time.After(time.Second):
		t.Errorf("publisher blocked on a saturated subscriber")
	}
}

// Test_PublishAccountEvent_FromAddRemove integrates the bus with the
// AddAccount / RemoveAccount publishers via a stubbed-out config layer.
// Skipped here because the wiring is exercised by the existing
// accounts_test.go round-trip test; this file focuses on Tools-side
// invalidation. Kept as a placeholder comment so future readers know
// where to add coverage if the publish path regresses.
//
// (Intentionally no test body — this is a doc-only marker.)
var _ = errtrace.Ok[struct{}] // keep errtrace import live for future tests
