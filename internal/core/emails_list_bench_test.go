// emails_list_bench_test.go — Phase 4 (P4.7) perf gate for
// `(*EmailsService).List` per spec
// `spec/21-app/02-features/02-emails/01-backend.md` §2.1:
//
//   "p95 ≤ 60 ms with 100 000 rows + 3-char Search."
//
// Two sibling probes for the same code path (mirrors the proven
// Phase 3.8 harness in `dashboard_summary_bench_test.go`):
//
//   1. `BenchmarkEmails_List_100kRows_3CharSearch` — reports ns/op
//      so we can track regression trends in `go test -bench`.
//      Bench-only, not a CI gate.
//   2. `TestEmails_List_100kRows_PerfGate` — runs the same workload
//      N times under a wall-clock budget and fails the build if the
//      p95 exceeds 60 ms. Skipped under `testing.Short` so unit-test
//      runs stay snappy.
//
// Reuses `openGoldenStore` from `dashboard_golden_test.go` — that
// helper already returns a real `*store.Store` over a `t.TempDir()`
// SQLite file with full migrations applied (so M0010's
// `IxEmailAliasReceived`-equivalent + the `Subject` LIKE plan exist).
//
// Why we don't reuse `seedGoldenEmails`: that helper emits a
// constant Subject prefix "golden N" which gives every row the
// same 3-char hit ("gol") — useless for stressing the LIKE
// predicate selectivity. We seed with varied Subjects below so a
// 3-char search returns a realistic ~few-percent slice.
package core

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// listPerfBudget is the spec §2.1 wall-clock p95 ceiling for one
// List call against the 100 000-row fixture with a 3-char Search.
const listPerfBudget = 60 * time.Millisecond

const (
	listPerfAccounts       = 10
	listPerfEmailsPerAlias = 10_000 // 10 × 10 000 = 100 000 rows total
	listPerfPageLimit      = 50     // typical UI page size
	listPerfSearch         = "abc"  // exactly 3 chars per spec
)

// buildListPerfFixture seeds a fresh store with 10 aliases × 10 000
// emails each (Subjects vary so the 3-char "abc" predicate has
// realistic selectivity), wires a real `*EmailsService` over the
// underlying store, and returns a closure that executes one
// 50-row alias-scoped page query with the 3-char Search.
//
// The closure is what the bench / perf gate iterate over — no
// fixture-build cost inside the timed loop.
func buildListPerfFixture(t testing.TB) func() errtrace.Result[[]EmailSummary] {
	t.Helper()
	st := openGoldenStore(t)
	ctx := context.Background()

	seedListPerfEmails(ctx, t, st)

	// Wire EmailsService over the same underlying store. The
	// production `defaultStoreOpener` opens a *separate* file-backed
	// store (via `store.Open()`), which would not see our seeded
	// rows. So we inject a closure opener that hands out the already-
	// seeded handle and a no-op close (the test cleanup closes it).
	opener := func() (emailsStore, func() error, error) {
		return st, func() error { return nil }, nil
	}
	res := NewEmailsService(opener)
	if res.HasError() {
		t.Fatalf("NewEmailsService: %v", res.Error())
	}
	svc := res.Value()

	return func() errtrace.Result[[]EmailSummary] {
		return svc.List(ctx, ListEmailsOptions{
			Alias:  "acc0",
			Search: listPerfSearch,
			Limit:  listPerfPageLimit,
			Offset: 0,
		})
	}
}

// seedListPerfEmails inserts 100 000 rows (10 aliases × 10 000)
// with varied Subjects. Roughly ~10 % of rows contain the literal
// "abc" substring (every 10th row gets a Subject that includes it),
// so the 3-char Search returns a realistic mid-size match set
// rather than 0 or all rows — both of which would be unfaithful to
// the spec's intent ("3-char Search" implies a meaningful predicate
// that exercises the index/scan path).
func seedListPerfEmails(ctx context.Context, t testing.TB, st *store.Store) {
	t.Helper()
	for a := 0; a < listPerfAccounts; a++ {
		alias := "acc" + strconv.Itoa(a)
		for i := 0; i < listPerfEmailsPerAlias; i++ {
			subject := "subj-" + strconv.Itoa(i)
			if i%10 == 0 {
				subject = "abc-match-" + strconv.Itoa(i)
			}
			_, _, err := st.UpsertEmail(ctx, &store.Email{
				Alias:      alias,
				MessageId:  alias + "-" + strconv.Itoa(i),
				Uid:        uint32(1_000_000 + a*listPerfEmailsPerAlias + i),
				FromAddr:   "src@example.com",
				Subject:    subject,
				ReceivedAt: time.Now().UTC(),
				FilePath:   "/dev/null",
			})
			if err != nil {
				t.Fatalf("seed acc%d[%d]: %v", a, i, err)
			}
		}
	}
}

func TestEmails_List_100kRows_PerfGate(t *testing.T) {
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	t.Parallel()
	run := buildListPerfFixture(t)

	// Warmup: SQLite plan cache + page cache. Without this the first
	// iteration eats a cold-cache hit that skews p95 upward.
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
	p95 := durs[(len(durs)*95)/100] // nearest-rank p95
	if p95 > listPerfBudget {
		t.Fatalf("List p95 = %s, budget = %s (samples sorted: min=%s med=%s max=%s)",
			p95, listPerfBudget, durs[0], durs[len(durs)/2], durs[len(durs)-1])
	}
	t.Logf("List p95 = %s (budget %s, %d iters, 100k rows, 3-char search)",
		p95, listPerfBudget, iterations)
}

func BenchmarkEmails_List_100kRows_3CharSearch(b *testing.B) {
	// Bench fixture seeding is expensive (~100 k INSERTs).
	// Stop the timer until we're inside the iteration loop.
	b.StopTimer()
	run := buildListPerfFixture(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if r := run(); r.HasError() {
			b.Fatalf("List: %v", r.Error())
		}
	}
}

// silence unused-import lint if filepath ends up unused in a future
// trim — we keep the import explicit because openGoldenStore
// uses it transitively.
var _ = filepath.Join
