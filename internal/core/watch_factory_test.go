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
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/eventbus"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/watcher"
)

// hasCode is a small test helper: the project does not expose a
// HasCode shortcut so we unwrap to *errtrace.Coded the same way
// watch_test.go does.
func hasCode(err error, want errtrace.Code) bool {
	var c *errtrace.Coded
	if !errors.As(err, &c) {
		return false
	}
	return c.Code == want
}

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
			if !res.HasError() {
				t.Fatalf("expected error, got nil")
			}
			if !hasCode(res.Error(), errtrace.ErrCoreInvalidArgument) {
				t.Fatalf("want ER-COR-21701, got %v", res.Error())
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
	if fres.HasError() {
		t.Fatalf("factory build: %v", fres.Error())
	}
	loop := fres.Value().New(WatchOptions{Alias: "ghost", PollSeconds: 3})
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
	if !hasCode(err, errtrace.ErrWatchAccountNotFound) {
		t.Fatalf("want ER-WCH-21412, got %v", err)
	}
}

// TestWatch_Start_MirrorsLifecycleToWatcherBus locks the desktop Raw log
// contract: clicking Start must produce an immediate watcher.EventStarted
// line even if the real loop fails before its first IMAP poll.
func TestWatch_Start_MirrorsLifecycleToWatcherBus(t *testing.T) {
	t.Parallel()
	lowBus := watcher.NewBus(8)
	sub, cancel := lowBus.Subscribe()
	defer cancel()
	fres := NewRealLoopFactory(RealLoopFactoryDeps{
		Resolver: func(string) *config.Account { return nil },
		Store:    &store.Store{},
		Bus:      lowBus,
	})
	if fres.HasError() {
		t.Fatalf("factory build: %v", fres.Error())
	}
	wres := NewWatch(fres.Value(), eventbus.New[WatchEvent](8), time.Now)
	if wres.HasError() {
		t.Fatalf("watch build: %v", wres.Error())
	}
	if r := wres.Value().Start(context.Background(), WatchOptions{Alias: "ghost"}); r.HasError() {
		t.Fatalf("Start: %v", r.Error())
	}
	assertWatcherEventKind(t, sub, watcher.EventStarted)
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
	if f.HasError() {
		t.Fatalf("factory: %v", f.Error())
	}
	for i := 0; i < 3; i++ {
		_ = f.Value().New(WatchOptions{Alias: "a", PollSeconds: 3})
	}
	if calls != 3 {
		t.Fatalf("resolver call count: got %d, want 3 (one per New)", calls)
	}
}

// TestRealLoopFactory_AccountSnapshotPointer: the runner captures the
// resolver's account pointer; a Stop+Start cycle is the documented way
// to refresh credentials. This test pins that contract so a future
// copy-by-value refactor is a deliberate, reviewed change.
func TestRealLoopFactory_AccountSnapshotPointer(t *testing.T) {
	t.Parallel()

	acct := &config.Account{Alias: "primary", Email: "a@x"}
	deps := RealLoopFactoryDeps{
		Resolver: func(string) *config.Account { return acct },
		Store:    &store.Store{},
	}
	f := NewRealLoopFactory(deps)
	if f.HasError() {
		t.Fatalf("factory: %v", f.Error())
	}
	loop := f.Value().New(WatchOptions{Alias: "primary", PollSeconds: 1})
	rl, ok := loop.(*realLoop)
	if !ok {
		t.Fatalf("expected *realLoop, got %T", loop)
	}
	if rl.acct.Alias != "primary" {
		t.Fatalf("alias drift: got %q", rl.acct.Alias)
	}
}

// TestErrWatchAccountNotFound_DistinctCode: guards against future
// renumbering colliding with ER-COR-21701.
func TestErrWatchAccountNotFound_DistinctCode(t *testing.T) {
	t.Parallel()
	err := errtrace.NewCoded(errtrace.ErrWatchAccountNotFound, "x")
	if hasCode(err, errtrace.ErrCoreInvalidArgument) {
		t.Fatalf("ErrWatchAccountNotFound collides with ErrCoreInvalidArgument")
	}
	if !hasCode(err, errtrace.ErrWatchAccountNotFound) {
		t.Fatalf("self-check failed: hasCode could not match its own coded error")
	}
}
