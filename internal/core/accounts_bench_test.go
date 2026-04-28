// accounts_bench_test.go — Slice #116 perf gate for
// `(*AccountsService).List`.
//
// Same harness as `rules_bench_test.go` / `emails_list_bench_test.go`:
//
//  1. `BenchmarkAccounts_List_50Accounts` — ns/op trend probe.
//  2. `TestAccounts_List_50Accounts_PerfGate` — wall-clock p95 gate.
//
// **Workload**: 50 accounts. The Accounts screen targets <10 in
// practice; 50 is a deliberate over-provision so a regression in
// `config.Load` JSON decode of the Accounts slice (the dominant cost
// for List) shows up clearly. Budget 25 ms — same as rules, since
// both surfaces share the same config-file Load path.
package core

import (
	"sort"
	"strconv"
	"testing"
	"time"
)

const (
	accountsListPerfBudget = 25 * time.Millisecond
	accountsListPerfCount  = 50
)

// buildAccountsListPerfFixture seeds N accounts in an isolated config
// and invokes `run` with a constructed `*AccountsService`.
func buildAccountsListPerfFixture(tb testing.TB, run func(svc *AccountsService)) {
	tb.Helper()
	withIsolatedConfigTB(tb, func() {
		res := NewDefaultAccountsService()
		if res.HasError() {
			tb.Fatalf("NewDefaultAccountsService: %v", res.Error())
		}
		svc := res.Value()
		for i := 0; i < accountsListPerfCount; i++ {
			alias := "acc" + strconv.Itoa(i)
			r := svc.Add(AccountInput{
				Alias:         alias,
				Email:         alias + "@gmail.com",
				PlainPassword: "secret-" + strconv.Itoa(i),
			})
			if r.HasError() {
				tb.Fatalf("seed account %d: %v", i, r.Error())
			}
		}
		run(svc)
	})
}

func TestAccounts_List_50Accounts_PerfGate(t *testing.T) {
	if perfGateSkipRace(t) {
		return
	}
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	t.Parallel()

	var samples []time.Duration
	buildAccountsListPerfFixture(t, func(svc *AccountsService) {
		// Warmup — see rules_bench_test.go for rationale.
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
	if p95 > accountsListPerfBudget {
		t.Fatalf("Accounts.List p95 = %s, budget = %s (min=%s med=%s max=%s)",
			p95, accountsListPerfBudget, samples[0], samples[len(samples)/2], samples[len(samples)-1])
	}
	t.Logf("Accounts.List p95 = %s (budget %s, %d accounts, %d iters)",
		p95, accountsListPerfBudget, accountsListPerfCount, len(samples))
}

func BenchmarkAccounts_List_50Accounts(b *testing.B) {
	buildAccountsListPerfFixture(b, func(svc *AccountsService) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r := svc.List()
			if r.HasError() {
				b.Fatalf("List: %v", r.Error())
			}
		}
	})
}
