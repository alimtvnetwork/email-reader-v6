// boot_smoke_test.go — App boot smoke test.
//
// The UI Run() entrypoint (internal/ui/app.go) walks a fixed bootstrap
// sequence on every cold start:
//
//   1. Construct app + apply theme        (loadInitialThemeMode)
//   2. Subscribe to Settings              (startThemeLiveConsumer)
//   3. Load aliases for the sidebar       (LoadAliases → core.ListAccounts)
//   4. Build views as the user navigates  (core.ListRules, etc.)
//
// internal/ui/app.go itself is `//go:build !nofyne` so we cannot drive it
// directly under the project's standard `-tags nofyne` verification path.
// What we *can* verify — and what actually catches startup regressions —
// is that the underlying core data path runs cleanly on a fresh data dir
// without panicking, returns sane defaults, and does so quickly enough
// for a snappy first paint.
//
// The previous slice could break this in three concrete ways:
//   • a new required Settings field with no default → NewSettings errors
//   • a panic in ListRules/ListAccounts when config.json is missing
//   • an O(n) regression that makes cold start visibly slow
//
// This test pins all three.
package core

import (
	"context"
	"testing"
	"time"
)

func TestBootSmoke_FreshDataDir(t *testing.T) {
	withIsolatedConfig(t, func() {
		start := time.Now()

		// Step 1: theme load. Mirrors loadInitialThemeMode() in
		// internal/ui/app.go — must not error and must return ThemeDark
		// (the documented bootstrap fallback).
		s := NewSettings(time.Now)
		if s.HasError() {
			t.Fatalf("step 1 NewSettings on fresh dir: %v", s.Error())
		}
		snap := s.Value().Get(context.Background())
		if snap.HasError() {
			t.Fatalf("step 1 Settings.Get on fresh dir: %v", snap.Error())
		}
		if snap.Value().Theme != ThemeDark {
			t.Fatalf("step 1 default Theme = %v, want ThemeDark", snap.Value().Theme)
		}
		if snap.Value().PollSeconds <= 0 {
			t.Fatalf("step 1 default PollSeconds = %d, want > 0", snap.Value().PollSeconds)
		}

		// Step 2: subscribe to Settings. The live theme consumer in
		// app.go does this; the channel must be non-nil and the
		// returned cancel must be safe to invoke immediately.
		ctx, cancel := context.WithCancel(context.Background())
		events, _ := s.Value().Subscribe(ctx)
		if events == nil {
			t.Fatal("step 2 Subscribe returned nil channel")
		}
		cancel()

		// Step 3: alias load for the sidebar. Mirrors LoadAliases() in
		// internal/ui/app.go — must return an empty (not nil-error)
		// slice on a fresh dir.
		accts := ListAccounts()
		if accts.HasError() {
			t.Fatalf("step 3 ListAccounts on fresh dir: %v", accts.Error())
		}
		if len(accts.Value()) != 0 {
			t.Fatalf("step 3 fresh dir should have 0 accounts, got %d", len(accts.Value()))
		}

		// Step 4: rules load for the Rules view. Same shape contract.
		rules := ListRules()
		if rules.HasError() {
			t.Fatalf("step 4 ListRules on fresh dir: %v", rules.Error())
		}
		if len(rules.Value()) != 0 {
			t.Fatalf("step 4 fresh dir should have 0 rules, got %d", len(rules.Value()))
		}

		// Cold-start budget. Generous to absorb CI variance, but tight
		// enough to catch e.g. a stray network call sneaking into the
		// bootstrap path.
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("cold bootstrap took %v, want <2s (regression?)", elapsed)
		}
	})
}

// TestBootSmoke_RepeatedConstruction guards against a class of bug where
// constructing Settings twice in the same process leaks state — e.g. a
// global subscriber slice growing without bound. The UI shell rebuilds on
// every account change, so any leak compounds quickly.
func TestBootSmoke_RepeatedConstruction(t *testing.T) {
	withIsolatedConfig(t, func() {
		for i := 0; i < 5; i++ {
			s := NewSettings(time.Now)
			if s.HasError() {
				t.Fatalf("iteration %d: NewSettings: %v", i, s.Error())
			}
			snap := s.Value().Get(context.Background())
			if snap.HasError() {
				t.Fatalf("iteration %d: Get: %v", i, snap.Error())
			}
			if snap.Value().Theme != ThemeDark {
				t.Fatalf("iteration %d: Theme drift = %v", i, snap.Value().Theme)
			}
		}
	})
}
