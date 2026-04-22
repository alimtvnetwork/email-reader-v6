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
func LoadDashboardStats(ctx context.Context, alias string) (DashboardStats, error) {
	cfg, err := config.Load()
	if err != nil {
		return DashboardStats{}, errtrace.Wrap(err, "load config")
	}
	stats := DashboardStats{
		Accounts:     len(cfg.Accounts),
		RulesTotal:   len(cfg.Rules),
		RulesEnabled: CountEnabledRules(cfg.Rules),
		Alias:        alias,
	}
	total, err := CountEmails(ctx, "")
	if err != nil {
		return stats, errtrace.Wrap(err, "count emails total")
	}
	stats.EmailsTotal = total
	if alias != "" {
		n, err := CountEmails(ctx, alias)
		if err != nil {
			return stats, errtrace.Wrapf(err, "count emails for %s", alias)
		}
		stats.EmailsForAlias = n
	}
	return stats, nil
}
