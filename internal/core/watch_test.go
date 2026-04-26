// watch_test.go locks the runners-map contracts: Start/Stop lifecycle,
// idempotence guards (already-started, not-running), event emission
// (Start / Stop / Error), List snapshot, and Subscribe fan-out.
package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/eventbus"
)

// fakeLoop is a controllable Loop. Each instance blocks in Run until
// either ctx is cancelled or `terminate` is closed (simulating an
// upstream IMAP fatal). Calls are recorded on `factory`.
type fakeLoop struct {
	alias     string
	factory   *fakeFactory
	terminate chan struct{}
	runErr    error
}

func (l *fakeLoop) Run(ctx context.Context) error {
	l.factory.mu.Lock()
	l.factory.running[l.alias] = true
	l.factory.mu.Unlock()
	defer func() {
		l.factory.mu.Lock()
		l.factory.running[l.alias] = false
		l.factory.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil
	case <-l.terminate:
		return l.runErr
	}
}

type fakeFactory struct {
	mu       sync.Mutex
	running  map[string]bool
	loops    map[string]*fakeLoop
	newCalls int
}

func newFakeFactory() *fakeFactory {
	return &fakeFactory{running: map[string]bool{}, loops: map[string]*fakeLoop{}}
}

func (f *fakeFactory) New(opts WatchOptions) Loop {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newCalls++
	l := &fakeLoop{alias: opts.Alias, factory: f, terminate: make(chan struct{})}
	f.loops[opts.Alias] = l
	return l
}

func (f *fakeFactory) terminateLoop(alias string, err error) {
	f.mu.Lock()
	l := f.loops[alias]
	if l != nil {
		l.runErr = err
		close(l.terminate)
	}
	f.mu.Unlock()
}

func (f *fakeFactory) isRunning(alias string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running[alias]
}

func newWatchForTest(t *testing.T) (*Watch, *fakeFactory) {
	t.Helper()
	ff := newFakeFactory()
	bus := eventbus.New[WatchEvent](16)
	r := NewWatch(ff, bus, time.Now)
	if r.HasError() {
		t.Fatalf("NewWatch: %v", r.Error())
	}
	return r.Value(), ff
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting: %s", msg)
}

func TestNewWatch_RejectsNilDeps(t *testing.T) {
	bus := eventbus.New[WatchEvent](4)
	for _, c := range []struct {
		name string
		lf   LoopFactory
		bus  *eventbus.Bus[WatchEvent]
		now  func() time.Time
	}{
		{"nil factory", nil, bus, time.Now},
		{"nil bus", newFakeFactory(), nil, time.Now},
		{"nil clock", newFakeFactory(), bus, nil},
	} {
		r := NewWatch(c.lf, c.bus, c.now)
		if !r.HasError() {
			t.Errorf("%s: expected error", c.name)
			continue
		}
		var coded *errtrace.Coded
		if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrCoreInvalidArgument {
			t.Errorf("%s: want ErrCoreInvalidArgument, got %v", c.name, r.Error())
		}
	}
}

func TestWatch_StartStopLifecycle(t *testing.T) {
	w, ff := newWatchForTest(t)
	sub, cancelSub := w.Subscribe()
	defer cancelSub()

	r := w.Start(context.Background(), WatchOptions{Alias: "work", PollSeconds: 3})
	if r.HasError() {
		t.Fatalf("Start: %v", r.Error())
	}
	waitFor(t, func() bool { return ff.isRunning("work") }, time.Second, "loop start")
	if !w.IsRunning("work") {
		t.Fatal("IsRunning(work) should be true")
	}
	if got := w.List(); len(got) != 1 || got[0] != "work" {
		t.Fatalf("List() = %v, want [work]", got)
	}

	rs := w.Stop("work", time.Second)
	if rs.HasError() {
		t.Fatalf("Stop: %v", rs.Error())
	}
	waitFor(t, func() bool { return !ff.isRunning("work") }, time.Second, "loop stop")
	if w.IsRunning("work") {
		t.Fatal("IsRunning(work) should be false after Stop")
	}

	// Two events expected: Start + Stop (in order).
	got := drainEvents(sub, 2, time.Second)
	if len(got) != 2 || got[0].Kind != WatchStart || got[1].Kind != WatchStop {
		t.Fatalf("event sequence wrong: %+v", got)
	}
	if got[0].Alias != "work" || got[1].Alias != "work" {
		t.Fatalf("event alias wrong: %+v", got)
	}
}

func TestWatch_StartRejectsEmptyAlias(t *testing.T) {
	w, _ := newWatchForTest(t)
	r := w.Start(context.Background(), WatchOptions{Alias: ""})
	if !r.HasError() {
		t.Fatal("expected ErrWatchAliasRequired")
	}
	var coded *errtrace.Coded
	if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrWatchAliasRequired {
		t.Fatalf("want ErrWatchAliasRequired, got %v", r.Error())
	}
}

func TestWatch_StartTwiceFails(t *testing.T) {
	w, _ := newWatchForTest(t)
	if r := w.Start(context.Background(), WatchOptions{Alias: "a"}); r.HasError() {
		t.Fatalf("first Start: %v", r.Error())
	}
	defer func() { _ = w.Stop("a", time.Second) }()
	r := w.Start(context.Background(), WatchOptions{Alias: "a"})
	if !r.HasError() {
		t.Fatal("second Start must fail")
	}
	var coded *errtrace.Coded
	if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrWatchAlreadyStarted {
		t.Fatalf("want ErrWatchAlreadyStarted, got %v", r.Error())
	}
}

func TestWatch_StopUnknownAliasFails(t *testing.T) {
	w, _ := newWatchForTest(t)
	r := w.Stop("ghost", time.Second)
	if !r.HasError() {
		t.Fatal("Stop on unknown alias must fail")
	}
	var coded *errtrace.Coded
	if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrWatchNotRunning {
		t.Fatalf("want ErrWatchNotRunning, got %v", r.Error())
	}
}

func TestWatch_LoopErrorPublishesWatchError(t *testing.T) {
	w, ff := newWatchForTest(t)
	sub, cancelSub := w.Subscribe()
	defer cancelSub()
	if r := w.Start(context.Background(), WatchOptions{Alias: "boom"}); r.HasError() {
		t.Fatalf("Start: %v", r.Error())
	}
	waitFor(t, func() bool { return ff.isRunning("boom") }, time.Second, "loop start")

	ff.terminateLoop("boom", errors.New("imap fatal"))

	got := drainEvents(sub, 2, time.Second) // Start + Error
	if len(got) < 2 {
		t.Fatalf("want >=2 events, got %d (%+v)", len(got), got)
	}
	last := got[len(got)-1]
	if last.Kind != WatchError || last.Err == nil || last.Alias != "boom" {
		t.Fatalf("last event wrong: %+v", last)
	}
}

func TestWatch_MultipleAliasesIsolated(t *testing.T) {
	w, ff := newWatchForTest(t)
	for _, a := range []string{"work", "personal", "ops"} {
		if r := w.Start(context.Background(), WatchOptions{Alias: a}); r.HasError() {
			t.Fatalf("Start %s: %v", a, r.Error())
		}
	}
	waitFor(t, func() bool {
		return ff.isRunning("work") && ff.isRunning("personal") && ff.isRunning("ops")
	}, time.Second, "all 3 loops running")
	if len(w.List()) != 3 {
		t.Fatalf("want 3 in List, got %d", len(w.List()))
	}
	// Stop one — others must keep running.
	if r := w.Stop("personal", time.Second); r.HasError() {
		t.Fatalf("Stop personal: %v", r.Error())
	}
	waitFor(t, func() bool { return !ff.isRunning("personal") }, time.Second, "personal stopped")
	if !ff.isRunning("work") || !ff.isRunning("ops") {
		t.Fatal("isolation broken: stopping personal stopped others")
	}
	for _, a := range []string{"work", "ops"} {
		if r := w.Stop(a, time.Second); r.HasError() {
			t.Fatalf("Stop %s: %v", a, r.Error())
		}
	}
}

func TestWatch_StopTimeoutSurfaces(t *testing.T) {
	// Build a Loop that ignores ctx so Stop's wait must time out.
	bus := eventbus.New[WatchEvent](4)
	wRes := NewWatch(stuckFactory{}, bus, time.Now)
	if wRes.HasError() {
		t.Fatalf("NewWatch: %v", wRes.Error())
	}
	w := wRes.Value()
	if r := w.Start(context.Background(), WatchOptions{Alias: "stuck"}); r.HasError() {
		t.Fatalf("Start: %v", r.Error())
	}
	r := w.Stop("stuck", 50*time.Millisecond)
	if !r.HasError() {
		t.Fatal("Stop must timeout when loop ignores ctx")
	}
	var coded *errtrace.Coded
	if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrWatcherShutdown {
		t.Fatalf("want ErrWatcherShutdown, got %v", r.Error())
	}
}

func drainEvents(ch <-chan WatchEvent, want int, timeout time.Duration) []WatchEvent {
	out := make([]WatchEvent, 0, want)
	deadline := time.After(timeout)
	for len(out) < want {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	return out
}

// stuckFactory returns Loops that ignore ctx — used to exercise
// Stop's timeout branch.
type stuckFactory struct{}

func (stuckFactory) New(_ WatchOptions) Loop { return stuckLoop{} }

type stuckLoop struct{}

func (stuckLoop) Run(_ context.Context) error {
	time.Sleep(time.Hour)
	return nil
}
