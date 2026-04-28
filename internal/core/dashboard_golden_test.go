// dashboard_golden_test.go — P3.6 golden-projection test for
// `(*DashboardService).Summary` against a **real** in-memory-style
// `*store.Store` (file-backed in `t.TempDir()` because the production
// `OpenAt` opens with WAL pragmas that don't apply to `:memory:`; per
// `internal/store/migrations_idempotent_test.go` notes, a temp-dir DB
// is the project's standard "real but disposable" pattern).
//
// What this pins that the closure-fake unit tests in `dashboard_test.go`
// cannot:
//
//   - The real `Store.CountEmails` SQL (`SELECT COUNT(*) ... WHERE
//     alias = ?`) is contract-correct: alias=="" returns total,
//     alias=="x" filters to that alias.
//   - `CountEnabledRules` rolls up matches the in-memory config
//     fixture exactly (no off-by-one between RulesTotal/RulesEnabled).
//   - The `DashboardSummary` (= DashboardStats) projection is byte-
//     exact for a known fixture — any future field addition will
//     fail this test until the fixture is updated, which is the
//     intended ratchet.
//
// Spec ref: spec/21-app/02-features/01-dashboard/01-backend.md §3.1
//
//	("Q1 Email totals" + Q3/Q4 in-memory config).
package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

func TestDashboard_Summary_GoldenProjection(t *testing.T) {
	t.Parallel()

	// --- arrange: real store, 5 emails across 2 aliases ---
	st := openGoldenStore(t)
	ctx := context.Background()
	seedGoldenEmails(ctx, t, st, "atto", 3)
	seedGoldenEmails(ctx, t, st, "ben", 2)

	// In-memory config: 3 accounts (one — "ghost" — has no stored
	// emails; that's intentional, locks the "configured but empty"
	// branch); 4 rules, 3 enabled.
	cfg := &config.Config{
		Accounts: []config.Account{{Alias: "atto"}, {Alias: "ben"}, {Alias: "ghost"}},
		Rules: []config.Rule{
			{Name: "r1", Enabled: true},
			{Name: "r2", Enabled: true},
			{Name: "r3", Enabled: false},
			{Name: "r4", Enabled: true},
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
	svc := mustService(t, loadCfg, count)

	// --- act + assert: global projection ---
	t.Run("global", func(t *testing.T) {
		res := svc.Summary(ctx, "")
		if res.HasError() {
			t.Fatalf("Summary: %v", res.Error())
		}
		want := DashboardSummary{
			Accounts:       3,
			RulesTotal:     4,
			RulesEnabled:   3,
			EmailsTotal:    5,
			EmailsForAlias: 0,
			Alias:          "",
		}
		if got := res.Value(); got != want {
			t.Fatalf("global summary mismatch:\n got: %+v\nwant: %+v", got, want)
		}
	})

	// --- act + assert: alias-scoped projection ---
	t.Run("alias_atto", func(t *testing.T) {
		res := svc.Summary(ctx, "atto")
		if res.HasError() {
			t.Fatalf("Summary: %v", res.Error())
		}
		want := DashboardSummary{
			Accounts:       3,
			RulesTotal:     4,
			RulesEnabled:   3,
			EmailsTotal:    5,
			EmailsForAlias: 3,
			Alias:          "atto",
		}
		if got := res.Value(); got != want {
			t.Fatalf("alias summary mismatch:\n got: %+v\nwant: %+v", got, want)
		}
	})

	// --- "configured but empty" — a real production case ---
	t.Run("alias_ghost_returns_zero", func(t *testing.T) {
		res := svc.Summary(ctx, "ghost")
		if res.HasError() {
			t.Fatalf("Summary: %v", res.Error())
		}
		if got := res.Value().EmailsForAlias; got != 0 {
			t.Fatalf("EmailsForAlias for ghost = %d, want 0", got)
		}
	})
}

// --- fixture helpers ---

// openGoldenStore opens a fresh file-backed store in a temp dir.
// Mirrors the project's `newTestStore` pattern in
// `internal/store/store_test.go` — file DB (not `:memory:`) so WAL
// pragmas apply and the migration ledger persists.
func openGoldenStore(t testing.TB) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenAt(filepath.Join(dir, "golden.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// seedGoldenEmails inserts `n` distinct emails for the given alias,
// each with a unique MessageId+Uid so UpsertEmail doesn't dedup them
// into a single row (which would silently break the count assertions).
func seedGoldenEmails(ctx context.Context, t testing.TB, st *store.Store, alias string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, _, err := st.UpsertEmail(ctx, &store.Email{
			Alias:      alias,
			MessageId:  alias + "-msg-" + itoa(i),
			Uid:        uint32(1000 + i),
			FromAddr:   "src@example.com",
			Subject:    "golden " + itoa(i),
			ReceivedAt: time.Now().UTC(),
			FilePath:   "/dev/null",
		})
		if err != nil {
			t.Fatalf("seed %s[%d]: %v", alias, i, err)
		}
	}
}
