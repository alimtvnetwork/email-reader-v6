// dashboard.go renders the Dashboard tab: high-level counts + a Start Watch
// CTA. Behind the !nofyne tag because it imports the Fyne widget set.
//go:build !nofyne

// Package views holds the Fyne widgets for each sidebar destination
// (Dashboard, Emails, Rules, Accounts, Watch, Tools). Each file exports a
// single Build* function that returns a fyne.CanvasObject so internal/ui's
// shell can stitch them in without any cross-view coupling.
package views

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// DashboardOptions wires the dashboard to app state + actions handled by
// the shell. OnStartWatch is invoked when the user clicks the CTA — the
// shell typically navigates to the Watch view (Phase 5 wires the actual
// watcher start).
type DashboardOptions struct {
	Alias        string         // currently selected account; "" ⇒ no alias-scoped count
	OnStartWatch func()         // called when the CTA is clicked; may be nil
	OnRefresh    func()         // optional — invoked after a manual refresh
	LoadStats    LoadStatsFunc  // override for tests; defaults to core.LoadDashboardStats
}

// LoadStatsFunc is the seam used by tests to inject deterministic counts.
type LoadStatsFunc func(ctx context.Context, alias string) (core.DashboardStats, error)

// BuildDashboard returns the Dashboard view. Counts load synchronously on
// first paint and refresh when the user clicks Refresh. Failures render
// inline as a warning label so the rest of the UI stays usable.
func BuildDashboard(opts DashboardOptions) fyne.CanvasObject {
	if opts.LoadStats == nil {
		opts.LoadStats = core.LoadDashboardStats
	}

	heading := widget.NewLabelWithStyle("Dashboard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Live counts from data/config.json + data/emails.db.")

	// Stat cards.
	accountsCard := newStatCard("Accounts", "—")
	rulesCard := newStatCard("Rules enabled", "—")
	emailsCard := newStatCard("Emails stored", "—")
	aliasCard := newStatCard("Selected account", "(none)")
	statsRow := container.NewGridWithColumns(4,
		accountsCard.Container, rulesCard.Container, emailsCard.Container, aliasCard.Container,
	)

	status := widget.NewLabel("Loaded just now.")
	status.Wrapping = fyne.TextWrapWord

	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s, err := opts.LoadStats(ctx, opts.Alias)
		if err != nil {
			status.SetText("⚠ Failed to load stats: " + err.Error())
			return
		}
		accountsCard.Value.SetText(fmt.Sprintf("%d", s.Accounts))
		rulesCard.Value.SetText(fmt.Sprintf("%d / %d", s.RulesEnabled, s.RulesTotal))
		emailsCard.Value.SetText(FormatEmailsValue(s))
		if s.Alias == "" {
			aliasCard.Value.SetText("(none)")
		} else {
			aliasCard.Value.SetText(s.Alias)
		}
		if s.Alias == "" {
			aliasCard.Value.SetText("(none)")
		} else {
			aliasCard.Value.SetText(s.Alias)
		}
		status.SetText("Loaded at " + time.Now().Format("15:04:05") + ".")
		if opts.OnRefresh != nil {
			opts.OnRefresh()
		}
	}
	refresh()

	refreshBtn := widget.NewButton("Refresh", refresh)
	startWatch := widget.NewButton("Start watching", func() {
		if opts.OnStartWatch != nil {
			opts.OnStartWatch()
		}
	})
	startWatch.Importance = widget.HighImportance

	cta := container.NewHBox(startWatch, refreshBtn)

	return container.NewVBox(
		heading,
		subtitle,
		widget.NewSeparator(),
		statsRow,
		widget.NewSeparator(),
		cta,
		status,
	)
}

// statCard groups the value Label so refresh can update text in place.
type statCard struct {
	Container *fyne.Container
	Value     *widget.Label
}

func newStatCard(title, initial string) statCard {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	v := widget.NewLabelWithStyle(initial, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	return statCard{
		Container: container.NewPadded(container.NewVBox(t, v)),
		Value:     v,
	}
}

// formatEmailsValue prefers the alias-scoped count when an alias is selected
// so the dashboard reflects the user's current focus.
func formatEmailsValue(s core.DashboardStats) string {
	if s.Alias != "" {
		return fmt.Sprintf("%d (%d total)", s.EmailsForAlias, s.EmailsTotal)
	}
	return fmt.Sprintf("%d", s.EmailsTotal)
}
