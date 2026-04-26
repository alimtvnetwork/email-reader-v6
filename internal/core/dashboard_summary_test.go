// dashboard_summary_test.go — P3.2 contract.
//
// Locks two invariants for the spec-aligned façade:
//  1. `Summary` returns byte-equivalent results to `LoadStats` for
//     identical inputs (so the rename is a pure alias, not a behaviour
//     change).
//  2. `DashboardSummary` is assignable from / to `DashboardStats`
//     without conversion (alias type, not a distinct type).
package core

import (
	"context"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func TestDashboardService_Summary_MatchesLoadStats(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Accounts: []config.Account{{Alias: "a"}, {Alias: "b"}},
		Rules:    []config.Rule{{Enabled: true}, {Enabled: false}},
	}
	loadCfg := func() (*config.Config, error) { return cfg, nil }
	count := func(_ context.Context, alias string) errtrace.Result[int] {
		if alias == "" {
			return errtrace.Ok(42)
		}
		return errtrace.Ok(7)
	}

	svc := mustService(t, loadCfg, count)

	loadRes := svc.LoadStats(context.Background(), "a")
	sumRes := svc.Summary(context.Background(), "a")

	if loadRes.HasError() || sumRes.HasError() {
		t.Fatalf("unexpected error: load=%v sum=%v", loadRes.Error(), sumRes.Error())
	}
	if loadRes.Value() != sumRes.Value() {
		t.Fatalf("Summary diverged from LoadStats:\n load=%+v\n sum =%+v",
			loadRes.Value(), sumRes.Value())
	}
}

func TestDashboardSummary_IsAliasOfDashboardStats(t *testing.T) {
	t.Parallel()

	// If DashboardSummary were a distinct named type, this assignment
	// would not compile. The test exists as a compile-time guard
	// against an accidental future divergence (e.g. someone redeclares
	// it as `type DashboardSummary struct{ ... }`).
	var stats DashboardStats
	var sum DashboardSummary = stats
	stats = sum
	_ = stats
}
