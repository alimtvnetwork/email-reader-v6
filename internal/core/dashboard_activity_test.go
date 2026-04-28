// dashboard_activity_test.go — P3.4 RecentActivity contract.
//
// Locks the spec test matrix from
// spec/21-app/02-features/01-dashboard/01-backend.md §6:
//   - #3 RecentActivity_LimitClampedTo200 (limit=999 → src receives 200)
//   - #4 RecentActivity_NegativeLimit_ReturnsErr (caller bug)
//
// plus: nil-source guard, valid-limit forwarded verbatim, source error
// is wrapped (not silently swallowed).
package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func TestDashboardService_RecentActivity_LimitClampedTo200(t *testing.T) {
	t.Parallel()
	var seen int
	src := func(_ context.Context, limit int) errtrace.Result[[]ActivityRow] {
		seen = limit
		return errtrace.Ok([]ActivityRow{})
	}
	svc := mustService(t, okCfg, okCount)

	res := svc.RecentActivity(context.Background(), 999, src)
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	if seen != RecentActivityLimitMax {
		t.Fatalf("source received limit=%d, want clamped to %d", seen, RecentActivityLimitMax)
	}
}

func TestDashboardService_RecentActivity_NegativeLimit_ReturnsErr(t *testing.T) {
	t.Parallel()
	src := func(_ context.Context, _ int) errtrace.Result[[]ActivityRow] {
		t.Fatal("source should not be called when limit is invalid")
		return errtrace.Ok[[]ActivityRow](nil)
	}
	svc := mustService(t, okCfg, okCount)

	for _, lim := range []int{0, -1, -999} {
		res := svc.RecentActivity(context.Background(), lim, src)
		if !res.HasError() {
			t.Fatalf("limit=%d: want error, got rows=%v", lim, res.Value())
		}
	}
}

func TestDashboardService_RecentActivity_ValidLimitForwardedVerbatim(t *testing.T) {
	t.Parallel()
	for _, lim := range []int{1, 50, 200} {
		lim := lim
		t.Run("limit_"+itoa(lim), func(t *testing.T) {
			t.Parallel()
			var seen int
			src := func(_ context.Context, l int) errtrace.Result[[]ActivityRow] {
				seen = l
				return errtrace.Ok([]ActivityRow{})
			}
			svc := mustService(t, okCfg, okCount)
			res := svc.RecentActivity(context.Background(), lim, src)
			if res.HasError() {
				t.Fatalf("unexpected error: %v", res.Error())
			}
			if seen != lim {
				t.Fatalf("source received limit=%d, want %d (no clamping)", seen, lim)
			}
		})
	}
}

func TestDashboardService_RecentActivity_ReturnsSourceRows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	rows := []ActivityRow{
		{OccurredAt: now, Alias: "a", Kind: ActivityPollSucceeded},
		{OccurredAt: now.Add(-1 * time.Minute), Alias: "b", Kind: ActivityPollFailed, ErrorCode: 21104},
	}
	src := func(_ context.Context, _ int) errtrace.Result[[]ActivityRow] { return errtrace.Ok(rows) }
	svc := mustService(t, okCfg, okCount)

	res := svc.RecentActivity(context.Background(), 10, src)
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	got := res.Value()
	if len(got) != 2 || got[0].Alias != "a" || got[1].Kind != ActivityPollFailed {
		t.Fatalf("rows mismatch: %+v", got)
	}
}

func TestDashboardService_RecentActivity_RejectsNilSource(t *testing.T) {
	t.Parallel()
	svc := mustService(t, okCfg, okCount)
	res := svc.RecentActivity(context.Background(), 10, nil)
	if !res.HasError() {
		t.Fatalf("want error for nil src")
	}
}

func TestDashboardService_RecentActivity_PropagatesSourceErr(t *testing.T) {
	t.Parallel()
	src := func(_ context.Context, _ int) errtrace.Result[[]ActivityRow] {
		return errtrace.Err[[]ActivityRow](errors.New("watch_event boom"))
	}
	svc := mustService(t, okCfg, okCount)
	res := svc.RecentActivity(context.Background(), 10, src)
	if !res.HasError() {
		t.Fatalf("want source error to propagate")
	}
}

// --- shared helpers (scoped to dashboard_*_test.go in this package) ---

func okCfg() (*config.Config, error) { return &config.Config{}, nil }
func okCount(_ context.Context, _ string) errtrace.Result[int] {
	return errtrace.Ok(0)
}

// itoa keeps the file dependency-free (no strconv import for one call).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
