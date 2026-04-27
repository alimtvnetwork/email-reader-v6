// watch_bench_test.go — Slice #116 perf probe for the `Watch` lifecycle
// pipeline (Start → Stop). Unlike Dashboard/Emails/Rules/Accounts —
// where the dominant cost is a query over a fixture — Watch's hot path
// is goroutine spawn + cancel + done-channel sync, so the bench/perf
// gate measures one full Start/Stop pair against an in-memory
// fakeLoop (defined in watch_test.go).
//
// **Budget**: 5 ms per Start+Stop pair p95. Goroutine spawn on Linux
// is < 10 µs; the budget absorbs scheduler jitter on shared CI hosts
// without making the gate slack.
//
// Reuses the existing `fakeFactory` / `fakeLoop` helpers from
// watch_test.go (same package) so we don't drift two stub fleets.
package core

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/eventbus"
)

const watchStartStopPerfBudget = 5 * time.Millisecond

// newWatchForBench is the testing.TB-widened twin of newWatchForTest
// (which is *testing.T-only). Same construction, same fakeFactory.
func newWatchForBench(tb testing.TB) (*Watch, *fakeFactory) {
	tb.Helper()
	f := newFakeFactory()
	bus := eventbus.New[WatchEvent](64)
	r := NewWatch(f, bus, time.Now)
	if r.HasError() {
		tb.Fatalf("NewWatch: %v", r.Error())
	}
	return r.Value(), f
}

// runOneStartStop spawns a runner for `alias`, lets it reach select
// (via the fake-factory's bookkeeping), then Stops it and waits for
// graceful shutdown. Returns the elapsed wall-clock time.
func runOneStartStop(tb testing.TB, w *Watch, alias string) time.Duration {
	start := time.Now()
	if r := w.Start(context.Background(), WatchOptions{Alias: alias}); r.HasError() {
		tb.Fatalf("Start: %v", r.Error())
	}
	// 100 ms shutdown timeout — fakeLoop returns nil on ctx cancel
	// instantly, so this should never trigger.
	if r := w.Stop(alias, 100*time.Millisecond); r.HasError() {
		tb.Fatalf("Stop: %v", r.Error())
	}
	return time.Since(start)
}

func TestWatch_StartStop_PerfGate(t *testing.T) {
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	t.Parallel()
	w, _ := newWatchForBench(t)

	// Warmup: first goroutine spawn pays runtime cost (G allocation,
	// stack init); subsequent spawns reuse pooled Gs.
	_ = runOneStartStop(t, w, "warmup")

	const iterations = 25
	durs := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		durs = append(durs, runOneStartStop(t, w, "alias-"+strconv.Itoa(i)))
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[(len(durs)*95)/100]
	if p95 > watchStartStopPerfBudget {
		t.Fatalf("Watch Start+Stop p95 = %s, budget = %s (min=%s med=%s max=%s)",
			p95, watchStartStopPerfBudget, durs[0], durs[len(durs)/2], durs[len(durs)-1])
	}
	t.Logf("Watch Start+Stop p95 = %s (budget %s, %d iters)",
		p95, watchStartStopPerfBudget, iterations)
}

func BenchmarkWatch_StartStop(b *testing.B) {
	w, _ := newWatchForBench(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = runOneStartStop(b, w, "alias-"+strconv.Itoa(i))
	}
}
