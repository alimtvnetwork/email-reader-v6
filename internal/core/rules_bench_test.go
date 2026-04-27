// rules_bench_test.go — Slice #116 perf gate for `(*RulesService).List`.
//
// Mirrors the proven harness in `dashboard_summary_bench_test.go` and
// `emails_list_bench_test.go`: one `Benchmark` for trend tracking and
// one `Test*_PerfGate` that asserts a wall-clock p95 budget so the
// build fails if regressions land. Both share `buildRulesListPerfFixture`
// so seeding cost is paid exactly once per probe.
//
// **Workload**: 500 rules in a single config — comfortably above the
// realistic upper bound (the spec's Rules screen targets dozens, not
// hundreds) so a regression in the load+filter+marshal path is
// visible. `List` does no DB work — it walks `cfg.Rules`, so the
// budget is dominated by `config.Load` (JSON decode of the config
// file). 25 ms is the gate; on a developer laptop List@500 runs in
// ~2 ms with a hot OS page cache.
//
// Skipped under `-short` so unit-test runs stay snappy.
package core

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

const (
	rulesListPerfBudget = 25 * time.Millisecond
	rulesListPerfCount  = 500
)

// buildRulesListPerfFixture seeds an isolated config with N rules and
// returns a closure that runs one `(*RulesService).List` call.
//
// Why isolated: `withIsolatedConfig` swaps `data/config.json` to a
// per-test path so we don't clobber dev data; we re-use it here so the
// bench's seeded rules don't leak into other suites.
func buildRulesListPerfFixture(tb testing.TB, run func(svc *RulesService)) {
	tb.Helper()
	withIsolatedConfigTB(tb, func() {
		svc := newDefaultRulesSvcTB(tb)
		// Seed N rules. Each Add does one Load+Save, so this is
		// O(N²) bytes of disk write; that's fine — it runs once,
		// outside the timed loop.
		for i := 0; i < rulesListPerfCount; i++ {
			r := svc.Add(RuleInput{
				Name:     "rule-" + strconv.Itoa(i),
				UrlRegex: `https?://example\.com/` + strconv.Itoa(i),
				Enabled:  i%2 == 0,
			})
			if r.HasError() {
				tb.Fatalf("seed rule %d: %v", i, r.Error())
			}
		}
		run(svc)
	})
}

// newDefaultRulesSvcTB is the testing.TB-widened twin of newRulesSvc
// (which is *testing.T-only). Bench helpers need testing.TB; we keep
// the production one untouched so existing tests stay byte-identical.
func newDefaultRulesSvcTB(tb testing.TB) *RulesService {
	tb.Helper()
	res := NewDefaultRulesService()
	if res.HasError() {
		tb.Fatalf("NewDefaultRulesService: %v", res.Error())
	}
	return res.Value()
}

func TestRules_List_500Rules_PerfGate(t *testing.T) {
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	t.Parallel()

	var samples []time.Duration
	buildRulesListPerfFixture(t, func(svc *RulesService) {
		// Warmup: prime the OS page cache for config.json so the
		// first iteration doesn't eat a cold-cache read and skew p95.
		if r := svc.List(); r.HasError() {
			t.Fatalf("warmup: %v", r.Error())
		}

		const iterations = 25
		samples = make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			r := svc.List()
			elapsed := time.Since(start)
			if r.HasError() {
				t.Fatalf("iter %d: %v", i, r.Error())
			}
			samples = append(samples, elapsed)
		}
	})
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[(len(samples)*95)/100]
	if p95 > rulesListPerfBudget {
		t.Fatalf("Rules.List p95 = %s, budget = %s (min=%s med=%s max=%s)",
			p95, rulesListPerfBudget, samples[0], samples[len(samples)/2], samples[len(samples)-1])
	}
	t.Logf("Rules.List p95 = %s (budget %s, %d rules, %d iters)",
		p95, rulesListPerfBudget, rulesListPerfCount, len(samples))
}

func BenchmarkRules_List_500Rules(b *testing.B) {
	buildRulesListPerfFixture(b, func(svc *RulesService) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r := svc.List()
			if r.HasError() {
				b.Fatalf("List: %v", r.Error())
			}
		}
	})
}
