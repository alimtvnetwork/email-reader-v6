// dashboard.go aggregates the high-level counts shown on the UI Dashboard.
// Pure data layer — no fyne, no printing — so the same function feeds both
// the Fyne dashboard view and any future CLI `status` command.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// DashboardStats is the snapshot the Dashboard view renders.
type DashboardStats struct {
	Accounts       int    // total configured accounts
	RulesTotal     int    // configured rules (enabled + disabled)
	RulesEnabled   int    // enabled rules only
	EmailsTotal    int    // emails persisted across all accounts
	EmailsForAlias int    // emails persisted for the currently-selected alias (0 when alias == "")
	Alias          string // echoes the alias passed in, for callers that want it on the view
}

// LoadDashboardStats gathers the counts from config + store. alias narrows
// EmailsForAlias; pass "" to leave it zero. Errors from any source are
// wrapped so the caller knows which subsystem failed.
func LoadDashboardStats(ctx context.Context, alias string) (DashboardStats, error) {
	cfg, err := config.Load()
	if err != nil {
		return DashboardStats{}, errtrace.Wrap(err, "load config")
	}
	out := DashboardStats{
		Accounts:     len(cfg.Accounts),
		RulesTotal:   len(cfg.Rules),
		RulesEnabled: CountEnabledRules(cfg.Rules),
		Alias:        alias,
	}

	total, err := CountEmails(ctx, "")
	if err != nil {
		return out, errtrace.Wrap(err, "count all emails")
	}
	out.EmailsTotal = total

	if alias != "" {
		n, err := CountEmails(ctx, alias)
		if err != nil {
			return out, errtrace.Wrapf(err, "count emails for %s", alias)
		}
		out.EmailsForAlias = n
	}
	return out, nil
}
