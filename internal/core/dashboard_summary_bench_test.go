// dashboard_summary_bench_test.go — P3.8 perf gate for
// `(*DashboardService).Summary` per spec/21-app/02-features/01-dashboard/01-backend.md §2.1
// ("Total p95 budget: 40 ms at 100 000 emails / 10 accounts — we
// ratchet a looser-but-meaningful 100 ms gate at 10 k emails so the
// suite stays fast on developer laptops; the dedicated 100k bench
// follows in a later slice once we tune fixtures).
//
// Two sibling probes for the same code path:
//
//   1. `BenchmarkDashboard_Summary_10kEmails` — reports ns/op so we
//      can track regression trends in `go test -bench`. Bench-only,
//      not a CI gate.
//   2. `TestDashboard_Summary_10kEmails_PerfGate` — runs the same
//      workload N times under a wall-clock budget and fails the
//      build if the p95 exceeds 100 ms. Skipped under `testing.Short`
//      so unit-test runs stay snappy; runs by default in `go test`.
//
// Both reuse `openGoldenStore` + `seedGoldenEmails` from
// `dashboard_golden_test.go` (P3.6); those helpers were widened to
// `testing.TB` in the same slice so a single fixture builder serves
// both `*testing.T` and `*testing.B`.
package core

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// summaryPerfBudget is the wall-clock p95 ceiling for one
// Summary call against the 10 k-email fixture. Sourced from spec
// §2.1 (40 ms @ 100 k); we use 100 ms here as a conservative
// gate at 10 k that absorbs CI noise without going slack.
const summaryPerfBudget = 100 * time.Millisecond

// summaryPerfFixtureSize defines the workload: 10 000 emails across
// 10 aliases (1 000 each). Matches the spec's account dimension
// exactly; emails dimension is 1/10 of the spec target to keep
// dev-laptop fixture seeding under ~1 s.
const (
	summaryPerfAccounts       = 10
	summaryPerfEmailsPerAlias = 1_000
)

// buildSummaryPerfFixture seeds a fresh store with 10 aliases ×
// 1 000 emails each, builds the matching `*DashboardService`, and
// returns a closure that runs one full alias-scoped Summary call.
// The closure is what the bench / perf gate iterate over — no
// fixture-build cost inside the timed loop.
func buildSummaryPerfFixture(t testing.TB) func() errtrace.Result[DashboardSummary] {
	t.Helper()
	st := openGoldenStore(t)
	ctx := context.Background()

	accounts := make([]config.Account, summaryPerfAccounts)
	for i := 0; i < summaryPerfAccounts; i++ {
		alias := "acc" + strconv.Itoa(i)
		accounts[i] = config.Account{Alias: alias}
		seedGoldenEmails(ctx, t, st, alias, summaryPerfEmailsPerAlias)
	}
	cfg := &config.Config{
		Accounts: accounts,
		Rules: []config.Rule{
			{Name: "r1", Enabled: true},
			{Name: "r2", Enabled: false},
		},
	}
	loadCfg := func() (*config.Config, error) { return cfg, nil }
	count := func(ctx context.Context, alias string) errtrace.Result[int] {
		n, err := st.CountEmails(ctx, alias)
		if err != nil {
			return errtrace.Err[int](err)
		}
		return errtrace.Ok(n)
	}
	res := NewDashboardService(loadCfg, count)
	if res.HasError() {
		t.Fatalf("NewDashboardService: %v", res.Error())
	}
	svc := res.Value()
	return func() errtrace.Result[DashboardSummary] {
		return svc.Summary(ctx, "acc0")
	}
}

func TestDashboard_Summary_10kEmails_PerfGate(t *testing.T) {
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	t.Parallel()
	run := buildSummaryPerfFixture(t)

	// Sanity: one warmup call to populate page cache + SQLite plan
	// cache. Without this the first iteration eats the cold-cache
	// hit and skews the p95 upward by ~30 %.
	if r := run(); r.HasError() {
		t.Fatalf("warmup: %v", r.Error())
	}

	const iterations = 25
	durs := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		r := run()
		elapsed := time.Since(start)
		if r.HasError() {
			t.Fatalf("iter %d: %v", i, r.Error())
		}
		durs = append(durs, elapsed)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	// p95 of 25 samples = index 23 (0-indexed) per nearest-rank.
	p95 := durs[(len(durs)*95)/100]
	if p95 > summaryPerfBudget {
		t.Fatalf("Summary p95 = %s, budget = %s (samples sorted: min=%s med=%s max=%s)",
			p95, summaryPerfBudget, durs[0], durs[len(durs)/2], durs[len(durs)-1])
	}
	t.Logf("Summary p95 = %s (budget %s, %d iters)", p95, summaryPerfBudget, iterations)
}

func BenchmarkDashboard_Summary_10kEmails(b *testing.B) {
	// Bench fixture seeding is expensive (~10 k INSERTs).
	// Stop the timer until we're inside the iteration loop.
	b.StopTimer()
	run := buildSummaryPerfFixture(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if r := run(); r.HasError() {
			b.Fatalf("Summary: %v", r.Error())
		}
	}
}
