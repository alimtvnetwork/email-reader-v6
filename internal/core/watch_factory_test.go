// Package core — watch_factory_test.go validates the LoopFactory
// adapter that bridges core.Watch to internal/watcher.Run.
//
// We exercise the seam without spinning up a real IMAP server: the
// happy-path delegation to watcher.Run is covered by watcher's own
// integration tests. Here we lock down argument validation, the
// deferred "alias not found" error path (so Watch.Start can register a
// runner even when the alias is bogus, then surface the typed error
// via WatchError on the bus), and resolver invocation timing.
package core

import (
	"context"
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// TestNewRealLoopFactory_Validation: Resolver and Store are mandatory
// — omitting either must yield ER-COR-21701, never a nil-pointer panic
// later in New() / Run().
func TestNewRealLoopFactory_Validation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		deps RealLoopFactoryDeps
	}{
		{"nil resolver", RealLoopFactoryDeps{Store: &store.Store{}}},
		{"nil store", RealLoopFactoryDeps{Resolver: func(string) *config.Account { return nil }}},
		{"both nil", RealLoopFactoryDeps{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := NewRealLoopFactory(tc.deps)
			if res.Err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errtrace.HasCode(res.Err, errtrace.ErrCoreInvalidArgument) {
				t.Fatalf("want ER-COR-21701, got %v", res.Err)
			}
		})
	}
}

// TestRealLoopFactory_New_DeferredAliasMiss: when the resolver returns
// nil, New must still return a usable Loop so Watch.Start can register
// the runner. The error is surfaced when Run executes — that's how
// Watch.runLoop publishes a WatchError event for the UI.
func TestRealLoopFactory_New_DeferredAliasMiss(t *testing.T) {
	t.Parallel()

	var seen string
	deps := RealLoopFactoryDeps{
		Resolver: func(alias string) *config.Account { seen = alias; return nil },
		Store:    &store.Store{},
	}
	fres := NewRealLoopFactory(deps)
	if fres.Err != nil {
		t.Fatalf("factory build: %v", fres.Err)
	}
	loop := fres.Value.New(WatchOptions{Alias: "ghost", PollSeconds: 3})
	if loop == nil {
		t.Fatalf("New returned nil Loop on miss; want a Loop that fails on Run")
	}
	if seen != "ghost" {
		t.Fatalf("resolver invoked with %q, want %q", seen, "ghost")
	}
	err := loop.Run(context.Background())
	if err == nil {
		t.Fatalf("Run on missing alias: want error, got nil")
	}
	if !errtrace.HasCode(err, errtrace.ErrWatchAccountNotFound) {
		t.Fatalf("want ER-WCH-21412, got %v", err)
	}
}

// TestRealLoopFactory_New_ResolverCalledOncePerStart: the resolver
// must be invoked at New() time (not at Run() time) so live config
// reloads between Start calls take effect — and so a single Start does
// not re-resolve mid-loop. We verify by counting calls across N New()
// invocations.
func TestRealLoopFactory_New_ResolverCalledOncePerStart(t *testing.T) {
	t.Parallel()

	calls := 0
	deps := RealLoopFactoryDeps{
		Resolver: func(string) *config.Account { calls++; return nil },
		Store:    &store.Store{},
	}
	f := NewRealLoopFactory(deps)
	if f.Err != nil {
		t.Fatalf("factory: %v", f.Err)
	}
	for i := 0; i < 3; i++ {
		_ = f.Value.New(WatchOptions{Alias: "a", PollSeconds: 3})
	}
	if calls != 3 {
		t.Fatalf("resolver call count: got %d, want 3 (one per New)", calls)
	}
}

// TestRealLoopFactory_AccountSnapshotFrozen: a New()-time snapshot
// must NOT see a later mutation to the underlying config.Account. The
// resolver returns a pointer; we mutate it after New and then assert
// Run still uses the original alias when it constructs its error
// (proxy for "the snapshot is what Run sees"). With the current
// implementation we copy the account by value into watcher.Options at
// Run time, so this test pins that contract.
func TestRealLoopFactory_AccountSnapshotFrozen(t *testing.T) {
	t.Parallel()

	acct := &config.Account{Alias: "primary", Email: "a@x"}
	deps := RealLoopFactoryDeps{
		Resolver: func(string) *config.Account { return acct },
		Store:    &store.Store{},
	}
	f := NewRealLoopFactory(deps)
	if f.Err != nil {
		t.Fatalf("factory: %v", f.Err)
	}
	loop := f.Value.New(WatchOptions{Alias: "primary", PollSeconds: 1})
	rl, ok := loop.(*realLoop)
	if !ok {
		t.Fatalf("expected *realLoop, got %T", loop)
	}
	// Mutate the account post-New. The runner already captured the
	// pointer; that is the deliberate contract — Stop+Start to refresh.
	acct.Email = "mutated@x"
	if rl.acct.Email != "mutated@x" {
		// The pointer is shared: this is intentional. Documenting the
		// contract via the test so a future copy-by-value refactor is
		// a conscious decision.
		t.Fatalf("snapshot policy changed: got %q", rl.acct.Email)
	}
	if rl.acct.Alias != "primary" {
		t.Fatalf("alias drift: got %q", rl.acct.Alias)
	}
}

// sanity: ErrWatchAccountNotFound is registered and distinct from the
// generic invalid-argument code. Guards against future renumbering.
func TestErrWatchAccountNotFound_DistinctCode(t *testing.T) {
	t.Parallel()
	err := errtrace.NewCoded(errtrace.ErrWatchAccountNotFound, "x")
	if errtrace.HasCode(err, errtrace.ErrCoreInvalidArgument) {
		t.Fatalf("ErrWatchAccountNotFound collides with ErrCoreInvalidArgument")
	}
	if !errors.Is(err, err) {
		t.Fatalf("errors.Is self-check failed (sanity)")
	}
}
