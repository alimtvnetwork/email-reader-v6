// dashboard_health_test.go — P3.3 AccountHealth contract.
//
// Locks the spec test matrix from
// spec/21-app/02-features/01-dashboard/01-backend.md §6:
//   - #5 AccountHealth_NoWatchStateRow_ReturnsWarning
//   - #6 AccountHealth_ThreeConsecutiveFailures_ReturnsError
//   - #8 ComputeHealth_MatrixTable (all 4 decision branches)
//
// plus a nil-source guard and a config-load error propagation case.
package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func TestComputeHealth_MatrixTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		row  AccountHealthRow
		want HealthLevel
	}{
		{
			name: "ThreeFailures_Error",
			row:  AccountHealthRow{ConsecutiveFailures: 3, LastPollAt: now},
			want: HealthError,
		},
		{
			name: "FourFailures_Error_evenIfFresh",
			row:  AccountHealthRow{ConsecutiveFailures: 4, LastPollAt: now},
			want: HealthError,
		},
		{
			name: "StalePoll_Warning",
			row:  AccountHealthRow{LastPollAt: now.Add(-16 * time.Minute)},
			want: HealthWarning,
		},
		{
			name: "ZeroPoll_Warning",
			row:  AccountHealthRow{}, // zero LastPollAt → infinitely stale
			want: HealthWarning,
		},
		{
			name: "ErrorAfterPoll_Warning",
			row: AccountHealthRow{
				LastPollAt:  now.Add(-1 * time.Minute),
				LastErrorAt: now.Add(-30 * time.Second),
			},
			want: HealthWarning,
		},
		{
			name: "FreshPoll_NoErr_Healthy",
			row:  AccountHealthRow{LastPollAt: now.Add(-1 * time.Minute)},
			want: HealthHealthy,
		},
		{
			name: "FreshPoll_OldErr_Healthy",
			row: AccountHealthRow{
				LastPollAt:  now.Add(-1 * time.Minute),
				LastErrorAt: now.Add(-1 * time.Hour),
			},
			want: HealthHealthy,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeHealth(tc.row, now)
			if got != tc.want {
				t.Fatalf("ComputeHealth(%+v) = %q, want %q", tc.row, got, tc.want)
			}
		})
	}
}

func TestDashboardService_AccountHealth_NoWatchStateRow_ReturnsWarning(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Accounts: []config.Account{{Alias: "ghost"}}}
	loadCfg := func() (*config.Config, error) { return cfg, nil }
	count := func(_ context.Context, _ string) errtrace.Result[int] { return errtrace.Ok(0) }
	src := func(_ context.Context) errtrace.Result[[]AccountHealthRow] {
		return errtrace.Ok[[]AccountHealthRow](nil) // empty source
	}
	svc := mustService(t, loadCfg, count)

	res := svc.AccountHealth(context.Background(), src)
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	rows := res.Value()
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Alias != "ghost" {
		t.Fatalf("alias: got %q", rows[0].Alias)
	}
	if rows[0].Health != HealthWarning {
		t.Fatalf("Health: got %q, want Warning (synthesised row)", rows[0].Health)
	}
}

func TestDashboardService_AccountHealth_ThreeConsecutiveFailures_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Accounts: []config.Account{{Alias: "a"}}}
	loadCfg := func() (*config.Config, error) { return cfg, nil }
	count := func(_ context.Context, _ string) errtrace.Result[int] { return errtrace.Ok(0) }
	src := func(_ context.Context) errtrace.Result[[]AccountHealthRow] {
		return errtrace.Ok([]AccountHealthRow{{
			Alias:               "a",
			LastPollAt:          time.Now(), // fresh, but failures dominate
			ConsecutiveFailures: 3,
		}})
	}
	svc := mustService(t, loadCfg, count)

	res := svc.AccountHealth(context.Background(), src)
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	if res.Value()[0].Health != HealthError {
		t.Fatalf("Health: got %q, want Error", res.Value()[0].Health)
	}
}

func TestDashboardService_AccountHealth_RejectsNilSource(t *testing.T) {
	t.Parallel()
	loadCfg := func() (*config.Config, error) { return &config.Config{}, nil }
	count := func(_ context.Context, _ string) errtrace.Result[int] { return errtrace.Ok(0) }
	svc := mustService(t, loadCfg, count)

	res := svc.AccountHealth(context.Background(), nil)
	if !res.HasError() {
		t.Fatalf("want error for nil src, got none")
	}
}

func TestDashboardService_AccountHealth_PropagatesCfgError(t *testing.T) {
	t.Parallel()
	loadCfg := func() (*config.Config, error) { return nil, errors.New("boom") }
	count := func(_ context.Context, _ string) errtrace.Result[int] { return errtrace.Ok(0) }
	src := func(_ context.Context) errtrace.Result[[]AccountHealthRow] {
		return errtrace.Ok[[]AccountHealthRow](nil)
	}
	svc := mustService(t, loadCfg, count)

	res := svc.AccountHealth(context.Background(), src)
	if !res.HasError() {
		t.Fatalf("want cfg error to propagate")
	}
}
