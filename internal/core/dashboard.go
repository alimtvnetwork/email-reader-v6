// dashboard.go — pure-Go dashboard stat aggregation. Counts accounts,
// rules, and stored emails for the Dashboard view. Lives in core (no Fyne)
// so the CLI can also render a "status" command in the future without
// pulling in the UI tree.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// DashboardStats is the projection rendered by the Dashboard view.
type DashboardStats struct {
	Accounts       int
	RulesTotal     int
	RulesEnabled   int
	EmailsTotal    int
	EmailsForAlias int
	Alias          string // echoed back so the view can render the active scope
}

// LoadDashboardStats reads config + emails store and returns aggregate counts.
// Alias may be empty; when set, EmailsForAlias is populated as well.
//
// Returns errtrace.Result[DashboardStats] per Delta #2 — callers use
// HasError/Value/PropagateError instead of the legacy (T, error) pair.
func LoadDashboardStats(ctx context.Context, alias string) errtrace.Result[DashboardStats] {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Err[DashboardStats](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen, "core.LoadDashboardStats"),
		)
	}
	stats := DashboardStats{
		Accounts:     len(cfg.Accounts),
		RulesTotal:   len(cfg.Rules),
		RulesEnabled: CountEnabledRules(cfg.Rules),
		Alias:        alias,
	}
	totalRes := CountEmails(ctx, "")
	if totalRes.HasError() {
		return errtrace.Err[DashboardStats](
			errtrace.WrapCode(totalRes.Error(), errtrace.ErrDbQueryEmail, "core.LoadDashboardStats").
				WithContext("scope", "total"),
		)
	}
	stats.EmailsTotal = totalRes.Value()
	if alias != "" {
		nRes := CountEmails(ctx, alias)
		if nRes.HasError() {
			return errtrace.Err[DashboardStats](
				errtrace.WrapCode(nRes.Error(), errtrace.ErrDbQueryEmail, "core.LoadDashboardStats").
					WithContext("scope", "alias").
					WithContext("alias", alias),
			)
		}
		stats.EmailsForAlias = nRes.Value()
	}
	return errtrace.Ok(stats)
}
