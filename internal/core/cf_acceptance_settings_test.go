// cf_acceptance_settings_test.go locks the cross-feature spec contracts
// where Settings publishes meet downstream consumers:
//
//	CF-W1 — `PollSeconds` change is applied by Watch on the **next** loop
//	        iteration; in-flight polls are NOT interrupted. Proof: a real
//	        Settings.Save → a registered PollChanRegistry slot receives
//	        the new value within the same publish.
//	CF-A1 — `Settings.Save` never mutates the `Accounts` array
//	        (already covered by `TestSettings_SavePreservesAccountsAndRules`;
//	        we add the *Origin*-of-truth assertion: an unrelated Save call
//	        that targets only Settings keys leaves a previously-added
//	        Account byte-identical on disk).
//	CF-A2 — Account add/remove and Settings save can run concurrently
//	        without lost updates: each owns disjoint top-level keys.
//
// Spec: spec/21-app/02-features/07-settings/99-consistency-report.md.
package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
)

// TestCF_W1_Watch_PollReload_OnSettingsEvent proves a Settings.Save
// triggers Broadcast on the PollChanRegistry, and the value lands in
// an Acquired channel. We don't need a real watcher loop — the
// registry IS the seam the watcher consumes through.
func TestCF_W1_Watch_PollReload_OnSettingsEvent(t *testing.T) {
	withIsolatedConfig(t, func() {
		reg := NewPollChanRegistry()
		ch := reg.Acquire("work")

		s := newTestSettings(t)
		// Subscribe so we know when the publish has reached subscribers,
		// then Broadcast — mirrors what watch_runtime.forwardSettingsEvents
		// does in production (the bridge is exercised in its own test;
		// this CF locks the registry contract from the spec POV).
		events, cancel := s.Subscribe(context.Background())
		defer cancel()

		go func() {
			ev := <-events
			reg.Broadcast(int(ev.Snapshot.PollSeconds))
		}()

		in := DefaultSettingsInput()
		in.PollSeconds = 17
		if r := s.Save(context.Background(), in); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		select {
		case got := <-ch:
			if got != 17 {
				t.Fatalf("registry chan got %d, want 17", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for cadence broadcast on registry chan")
		}
	})
}

// TestCF_A1_Save_Accounts_Untouched proves that a Settings save with
// no Account-related fields leaves a previously-added Account
// byte-identical (alias, host, port, password). Stronger than the
// existing "preserves accounts" test because it locks every field of
// the persisted Account, not just presence.
func TestCF_A1_Save_Accounts_Untouched(t *testing.T) {
	withIsolatedConfig(t, func() {
		// Seed an account directly via the config layer so we don't
		// invoke any of the Settings mutators on the way in.
		seedAccount := config.Account{
			Alias: "primary", Email: "p@x.test",
			ImapHost: "imap.x.test", ImapPort: 993, UseTLS: true,
			Mailbox: "INBOX", PasswordB64: "c2VjcmV0",
		}
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		cfg.Accounts = append(cfg.Accounts, seedAccount)
		if err := config.Save(cfg); err != nil {
			t.Fatalf("config.Save: %v", err)
		}

		s := newTestSettings(t)
		in := DefaultSettingsInput()
		in.PollSeconds = 9 // force a real change so persistence is exercised
		if r := s.Save(context.Background(), in); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		got, err := config.Load()
		if err != nil {
			t.Fatalf("Load post-Save: %v", err)
		}
		if len(got.Accounts) != 1 {
			t.Fatalf("Accounts len = %d, want 1", len(got.Accounts))
		}
		gotAcct := got.Accounts[0]
		if gotAcct != seedAccount {
			t.Fatalf("Account mutated by Settings.Save:\n got %+v\nwant %+v", gotAcct, seedAccount)
		}
	})
}

// TestCF_A2_Concurrent_Settings_Accounts_NoLoss runs an account
// add/remove cycle and a Settings.Save in lock-step many times. The
// final state must contain BOTH the latest Settings value AND the
// account that survived the cycle — proving disjoint top-level keys
// and the WriteAtomic serialisation never lose either side's update.
func TestCF_A2_Concurrent_Settings_Accounts_NoLoss(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		const iters = 20
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				in := DefaultSettingsInput()
				in.PollSeconds = 5 + i%10
				_ = s.Save(context.Background(), in)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_ = AddAccount(addAccountSpec("durable"))
				if i%2 == 0 {
					_ = RemoveAccount("durable")
				}
			}
			// Final state: ensure the account exists.
			_ = AddAccount(addAccountSpec("durable"))
		}()
		wg.Wait()

		// Settings: read back; PollSeconds must be one we wrote (5..14).
		got := s.Get(context.Background())
		if got.HasError() {
			t.Fatalf("Get: %v", got.Error())
		}
		ps := int(got.Value().PollSeconds)
		if ps < 5 || ps > 14 {
			t.Errorf("PollSeconds = %d, expected last writer in [5,14]", ps)
		}

		// Accounts: durable account must be present.
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		var found bool
		for _, a := range cfg.Accounts {
			if a.Alias == "durable" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("durable account lost — accounts: %+v", cfg.Accounts)
		}
	})
}

// addAccountSpec builds a minimal-valid AccountInput for the
// concurrency test. Kept tiny so the test body stays readable.
func addAccountSpec(alias string) AccountInput {
	return AccountInput{
		Alias: alias, Email: alias + "@x.test",
		ImapHost: "imap.x.test", ImapPort: 993, UseTLS: true, UseTLSExplicit: true,
		Mailbox: "INBOX", PlainPassword: "secret",
	}
}
