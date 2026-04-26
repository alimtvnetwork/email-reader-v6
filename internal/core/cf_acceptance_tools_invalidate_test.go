// cf_acceptance_tools_invalidate_test.go locks the END-TO-END
// cross-feature contract for AccountEvent → Tools-cache invalidation.
// The existing tools_invalidate_test.go covers the unit-level
// `handleAccountEvent` policy + the bus subscribe/unsubscribe smoke.
// This file goes one level higher: it drives the PUBLIC AddAccount /
// RemoveAccount API and proves that a `Tools` instance subscribed via
// `WatchAccountEvents` evicts its diagnose cache in response to
// real-world lifecycle calls — no internal helpers, no
// publishAccountEvent shortcut.
//
//	CF-AT1 — `RemoveAccount(alias)` evicts the diagnose entry for that
//	          alias on a subscribed Tools within ≤1s.
//	CF-AT2 — `AddAccount(existingAlias)` (re-save → AccountUpdated)
//	          evicts the diagnose entry for that alias.
//	CF-AT3 — A pure `AddAccount(newAlias)` (AccountAdded) does NOT
//	          evict an unrelated cached entry — over-eviction would
//	          waste a 60 s cache cycle on every account add.
//
// Spec: spec/21-app/02-features/06-tools/99-consistency-report.md
//       (informational CF "Tools cache invalidation post-AccountEvent").
package core

import (
	"context"
	"testing"
	"time"
)

// pollEvicted polls the diagnose cache for `alias` until the entry
// disappears or `deadline` elapses. Returns true on eviction.
func pollEvicted(tools *Tools, alias string, deadline time.Duration) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if _, hit := tools.diagCache().get(alias, time.Now()); !hit {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// addAccountInput builds a minimal-valid AccountInput; helper kept tiny.
func addAccountInput(alias string) AccountInput {
	return AccountInput{
		Alias: alias, Email: alias + "@x.test",
		ImapHost: "imap.x.test", ImapPort: 993, UseTLS: true, UseTLSExplicit: true,
		Mailbox: "INBOX", PlainPassword: "secret",
	}
}

// TestCF_AT1_RemoveAccount_EvictsToolsCache exercises the full chain:
// AddAccount → prime cache → RemoveAccount → bus → handleAccountEvent.
func TestCF_AT1_RemoveAccount_EvictsToolsCache(t *testing.T) {
	withIsolatedConfig(t, func() {
		t.Cleanup(resetAccountBusForTest)
		tools := newToolsForInvalidateTest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := tools.WatchAccountEvents(ctx)
		defer stop()

		if r := AddAccount(addAccountInput("at1")); r.HasError() {
			t.Fatalf("AddAccount: %v", r.Error())
		}
		primeDiagCache(t, tools, "at1")

		if r := RemoveAccount("at1"); r.HasError() {
			t.Fatalf("RemoveAccount: %v", r.Error())
		}
		if !pollEvicted(tools, "at1", time.Second) {
			t.Fatal("RemoveAccount did not evict diagnose cache within 1 s (CF-AT1)")
		}
	})
}

// TestCF_AT2_ResaveAccount_EvictsToolsCache locks AccountUpdated.
// Re-calling AddAccount with the same alias triggers an Updated event
// (per accounts.go:75); the cache must drop its stale entry.
func TestCF_AT2_ResaveAccount_EvictsToolsCache(t *testing.T) {
	withIsolatedConfig(t, func() {
		t.Cleanup(resetAccountBusForTest)
		tools := newToolsForInvalidateTest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := tools.WatchAccountEvents(ctx)
		defer stop()

		if r := AddAccount(addAccountInput("at2")); r.HasError() {
			t.Fatalf("AddAccount initial: %v", r.Error())
		}
		primeDiagCache(t, tools, "at2")

		// Re-save with a different host → triggers AccountUpdated.
		updated := addAccountInput("at2")
		updated.ImapHost = "imap2.x.test"
		if r := AddAccount(updated); r.HasError() {
			t.Fatalf("AddAccount re-save: %v", r.Error())
		}
		if !pollEvicted(tools, "at2", time.Second) {
			t.Fatal("AccountUpdated did not evict diagnose cache within 1 s (CF-AT2)")
		}
	})
}

// TestCF_AT3_AddAccount_NewAlias_DoesNotEvictOthers proves AccountAdded
// for a NEW alias never evicts an unrelated cached entry.
func TestCF_AT3_AddAccount_NewAlias_DoesNotEvictOthers(t *testing.T) {
	withIsolatedConfig(t, func() {
		t.Cleanup(resetAccountBusForTest)
		tools := newToolsForInvalidateTest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := tools.WatchAccountEvents(ctx)
		defer stop()

		// Pre-existing cached entry for "stable" — never touched by
		// the lifecycle event for the unrelated new alias.
		primeDiagCache(t, tools, "stable")

		if r := AddAccount(addAccountInput("brand-new")); r.HasError() {
			t.Fatalf("AddAccount: %v", r.Error())
		}
		// Give the bus a generous window in case of misclassification.
		time.Sleep(50 * time.Millisecond)
		if _, hit := tools.diagCache().get("stable", time.Now()); !hit {
			t.Fatal("unrelated cached entry was evicted by AccountAdded (CF-AT3)")
		}
	})
}
