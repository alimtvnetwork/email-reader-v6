// dashboard_health_rollup_test.go — P3.7 AccountHealth rollup math.
//
// Where `dashboard_health_test.go` (P3.3) locks the **single-row**
// decision matrix in `ComputeHealth` and the synthesised-Warning
// branch for unconfigured aliases, this test pins the **multi-row
// rollup** behaviour of `(*DashboardService).AccountHealth`:
//
//   - Output order matches the configured-accounts order from
//     `cfg.Accounts`, not the source order. (UI consumers rely on
//     this so the row order in the table is stable across refreshes.)
//   - Per-row counters (EmailsStored, UnreadCount) survive the
//     join — the service must not zero them out when computing
//     Health.
//   - All three Health buckets coexist in one response when the
//     fixtures span them: Healthy (fresh poll, no errs), Warning
//     (stale poll), Error (≥3 consecutive failures).
//   - Aliases configured-but-absent surface as synthesised Warning
//     rows even when the source returns rows for *other* aliases.
//   - Source rows for aliases NOT in cfg are silently dropped (we
//     project the configured set, not the source set).
//
// No store I/O — uses the existing closure-fake source pattern from
// P3.3. The store-backed variant lands once `QueryAccountHealth`
// shim + M0009 WatchState health columns ship (P3.3b in the
// roadmap); until then, the source-fake covers the full rollup
// surface area exercised by the UI.
package core

import (
	"context"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func TestDashboard_AccountHealth_RollupMath(t *testing.T) {
	t.Parallel()

	now := time.Now()
	// Four configured aliases spanning every Health bucket plus the
	// "configured but no source row" case. Order is intentional —
	// asserted below.
	cfg := &config.Config{Accounts: []config.Account{
		{Alias: "fresh"}, // → Healthy
		{Alias: "stale"}, // → Warning (LastPollAt > 15 min old)
		{Alias: "stuck"}, // → Error   (3 consecutive failures)
		{Alias: "ghost"}, // → Warning (synthesised: not in source)
	}}
	srcRows := []AccountHealthRow{
		{
			Alias:        "fresh",
			LastPollAt:   now.Add(-30 * time.Second),
			EmailsStored: 12, UnreadCount: 3,
		},
		{
			Alias:        "stale",
			LastPollAt:   now.Add(-20 * time.Minute),
			EmailsStored: 7, UnreadCount: 1,
		},
		{
			Alias:               "stuck",
			LastPollAt:          now.Add(-10 * time.Second), // fresh poll, but…
			ConsecutiveFailures: 3,                          // …failures dominate
			EmailsStored:        5, UnreadCount: 5,
		},
		// Source includes a stranger that's NOT in cfg — must be
		// silently dropped; does not appear in the output.
		{Alias: "stranger", LastPollAt: now, EmailsStored: 999},
	}
	src := func(_ context.Context) errtrace.Result[[]AccountHealthRow] {
		return errtrace.Ok(srcRows)
	}
	svc := mustService(t, func() (*config.Config, error) { return cfg, nil }, okCount)

	res := svc.AccountHealth(context.Background(), src)
	if res.HasError() {
		t.Fatalf("AccountHealth: %v", res.Error())
	}
	got := res.Value()

	// --- (1) length + ordering: 4 rows in cfg order, no stranger ---
	wantAliases := []string{"fresh", "stale", "stuck", "ghost"}
	if len(got) != len(wantAliases) {
		t.Fatalf("row count = %d, want %d (rows=%+v)", len(got), len(wantAliases), got)
	}
	for i, w := range wantAliases {
		if got[i].Alias != w {
			t.Errorf("row[%d].Alias = %q, want %q (cfg order must be preserved)", i, got[i].Alias, w)
		}
	}

	// --- (2) Health bucket per row ---
	wantHealth := map[string]HealthLevel{
		"fresh": HealthHealthy,
		"stale": HealthWarning,
		"stuck": HealthError,
		"ghost": HealthWarning,
	}
	for _, r := range got {
		if r.Health != wantHealth[r.Alias] {
			t.Errorf("alias %q: Health = %q, want %q", r.Alias, r.Health, wantHealth[r.Alias])
		}
	}

	// --- (3) per-row counters survive the join ---
	wantCounters := map[string][2]int{ // [EmailsStored, UnreadCount]
		"fresh": {12, 3},
		"stale": {7, 1},
		"stuck": {5, 5},
		"ghost": {0, 0}, // synthesised row → zeros
	}
	for _, r := range got {
		w := wantCounters[r.Alias]
		if r.EmailsStored != w[0] || r.UnreadCount != w[1] {
			t.Errorf("alias %q: counters = (%d,%d), want (%d,%d)",
				r.Alias, r.EmailsStored, r.UnreadCount, w[0], w[1])
		}
	}

	// --- (4) all three buckets are present in one response ---
	seen := map[HealthLevel]int{}
	for _, r := range got {
		seen[r.Health]++
	}
	if seen[HealthHealthy] < 1 || seen[HealthWarning] < 2 || seen[HealthError] < 1 {
		t.Errorf("bucket coverage = %v, want ≥1 Healthy, ≥2 Warning, ≥1 Error", seen)
	}
}
