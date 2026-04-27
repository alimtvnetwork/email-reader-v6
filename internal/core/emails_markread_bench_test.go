// emails_markread_bench_test.go — Slice #116e per-feature performance
// gate for `(*EmailsService).MarkRead` against a *real* SQLite-backed
// store, complementing the existing fake-store contract test in
// `emails_markread_test.go`.
//
// Spec `spec/21-app/02-features/02-emails/01-backend.md` §2.3:
//
//   "Budget: ≤ 150 ms for 500 UIDs."
//
// The original `TestEmailsService_MarkRead_500Uids_Under150ms` proves
// the *service-layer overhead* is under budget by routing through a
// `fakeEmailsStore` whose `SetEmailRead` is a free no-op. That keeps
// the contract honest but does not exercise the real
// `Store.SetEmailRead` SQL path — so a future regression that
// degrades the underlying batched UPDATE (e.g. losing the `IN(...)`
// chunking, dropping an index, or accidentally reopening the DB
// per-UID) would slip through.
//
// This file plugs that gap by mirroring the proven harness from
// `emails_list_bench_test.go` (P4.7):
//
//   1. `BenchmarkEmails_MarkRead_500Uids` — bench-only ns/op probe
//      so regressions show up in `go test -bench`. Not a CI gate.
//   2. `TestEmails_MarkRead_500Uids_RealStore_PerfGate` — wall-clock
//      p95 budget against a seeded `*store.Store` (10 aliases × 1000
//      rows each = 10 000 emails; we mark 500 UIDs of one alias).
//      Skipped under `-short` to keep unit-test runs snappy.
//
// Reuses `openGoldenStore` from `dashboard_golden_test.go` so we get
// a real SQLite file with full migrations applied (M0010 indexes
// included). We seed lightly compared to the 100 k List fixture
// because MarkRead's hot path is N UPDATEs, not a 100k-row scan —
// 10 k rows is plenty to exercise index lookups + page cache without
// dragging the suite past ~1 s of seed time.
package core

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// markReadRealStorePerfBudget mirrors the spec §2.3 ceiling.
// We use the same 150 ms budget as the fake-store gate; a real
// SQLite UPDATE of 500 rows in a single chunk should comfortably
// land an order of magnitude under this on any modern laptop.
const markReadRealStorePerfBudget = 150 * time.Millisecond

const (
	markReadPerfAccounts       = 10
	markReadPerfEmailsPerAlias = 1_000 // 10 × 1 000 = 10 000 rows total
	markReadPerfBatchSize      = 500   // exactly the spec's batch size
	markReadPerfTargetAlias    = "acc0"
)

// buildMarkReadPerfFixture seeds a fresh real-store fixture and
// returns a closure that flips the read flag on 500 UIDs of
// `acc0`. The closure does not re-seed — each iteration just
// toggles the flag, so cumulative state drift is bounded
// (true→false→true→…) and we never grow the row count inside the
// timed loop.
//
// We return both the runner and a `*bool` ping-pong cursor so the
// caller can alternate `read=true` / `read=false` across iterations
// without measurable overhead.
func buildMarkReadPerfFixture(t testing.TB) (run func(read bool) errtrace.Result[int64], uids []uint32) {
	t.Helper()
	st := openGoldenStore(t)
	ctx := context.Background()

	seedMarkReadPerfEmails(ctx, t, st)

	// Reuse the same opener pattern as `buildListPerfFixture`: the
	// production `defaultStoreOpener` would open a *separate*
	// file-backed store and miss our seeded rows, so we hand out
	// the already-seeded handle with a no-op close (cleanup closes
	// the real store via t.Cleanup inside openGoldenStore).
	opener := func() (emailsStore, func() error, error) {
		return st, func() error { return nil }, nil
	}
	res := NewEmailsService(opener)
	if res.HasError() {
		t.Fatalf("NewEmailsService: %v", res.Error())
	}
	svc := res.Value()

	// Deterministic UID set: the first 500 UIDs we seeded for acc0.
	// `seedMarkReadPerfEmails` uses `1_000_000 + i` for acc0, so we
	// reproduce that here verbatim.
	uids = make([]uint32, markReadPerfBatchSize)
	for i := 0; i < markReadPerfBatchSize; i++ {
		uids[i] = uint32(1_000_000 + i)
	}

	run = func(read bool) errtrace.Result[int64] {
		r := svc.MarkRead(ctx, markReadPerfTargetAlias, uids, read)
		if r.HasError() {
			return errtrace.Err[int64](r.Error())
		}
		// Service returns Result[struct{}]; we don't have a direct
		// rowcount channel here, but downstream callers don't need
		// it — the perf gate only cares about latency + nil-error.
		return errtrace.Ok[int64](int64(len(uids)))
	}
	return run, uids
}

// seedMarkReadPerfEmails inserts 10 000 rows (10 aliases × 1 000)
// with deterministic UIDs so the harness can address exactly the
// first 500 UIDs of `acc0` in the timed loop.
//
// UID layout: `1_000_000 + a*1000 + i` — i.e. acc0 occupies
// 1 000 000…1 000 999, acc1 occupies 1 001 000…1 001 999, etc.
// This guarantees no UID collision across aliases (the unique
// index is on (alias,uid) but predictability helps debugging).
func seedMarkReadPerfEmails(ctx context.Context, t testing.TB, st *store.Store) {
	t.Helper()
	for a := 0; a < markReadPerfAccounts; a++ {
		alias := "acc" + strconv.Itoa(a)
		for i := 0; i < markReadPerfEmailsPerAlias; i++ {
			_, _, err := st.UpsertEmail(ctx, &store.Email{
				Alias:      alias,
				MessageId:  alias + "-" + strconv.Itoa(i),
				Uid:        uint32(1_000_000 + a*markReadPerfEmailsPerAlias + i),
				FromAddr:   "src@example.com",
				Subject:    "subj-" + strconv.Itoa(i),
				ReceivedAt: time.Now().UTC(),
				FilePath:   "/dev/null",
			})
			if err != nil {
				t.Fatalf("seed acc%d[%d]: %v", a, i, err)
			}
		}
	}
}

// TestEmails_MarkRead_500Uids_RealStore_PerfGate enforces the spec
// §2.3 budget against a real SQLite store. Skipped under -short.
func TestEmails_MarkRead_500Uids_RealStore_PerfGate(t *testing.T) {
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	t.Parallel()
	run, _ := buildMarkReadPerfFixture(t)

	// Warmup: SQLite plan cache + page cache. Without this the
	// first iteration eats a cold-cache hit that skews p95 upward.
	if r := run(true); r.HasError() {
		t.Fatalf("warmup: %v", r.Error())
	}

	const iterations = 25
	durs := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		read := i%2 == 0 // ping-pong to avoid pure-noop UPDATEs
		start := time.Now()
		r := run(read)
		elapsed := time.Since(start)
		if r.HasError() {
			t.Fatalf("iter %d: %v", i, r.Error())
		}
		durs = append(durs, elapsed)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[(len(durs)*95)/100] // nearest-rank p95
	if p95 > markReadRealStorePerfBudget {
		t.Fatalf("MarkRead p95 = %s, budget = %s (samples sorted: min=%s med=%s max=%s)",
			p95, markReadRealStorePerfBudget, durs[0], durs[len(durs)/2], durs[len(durs)-1])
	}
	t.Logf("MarkRead p95 = %s (budget %s, %d iters, 500 uids, real store)",
		p95, markReadRealStorePerfBudget, iterations)
}

// BenchmarkEmails_MarkRead_500Uids reports ns/op for tracking
// regression trends under `go test -bench`. Bench-only, not a gate.
func BenchmarkEmails_MarkRead_500Uids(b *testing.B) {
	// Fixture seeding inserts 10 000 rows; keep it out of the timer.
	b.StopTimer()
	run, _ := buildMarkReadPerfFixture(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		read := i%2 == 0
		if r := run(read); r.HasError() {
			b.Fatalf("MarkRead: %v", r.Error())
		}
	}
}
