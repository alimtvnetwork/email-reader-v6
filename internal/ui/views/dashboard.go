//go:build !nofyne

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

// DashboardOptions wires the dashboard to app state + actions.
type DashboardOptions struct {
	Alias        string
	OnStartWatch func()
	OnRefresh    func()
	LoadStats    LoadStatsFunc
}

// LoadStatsFunc is the seam used by tests to inject deterministic counts.
type LoadStatsFunc func(ctx context.Context, alias string) (core.DashboardStats, error)

func BuildDashboard(opts DashboardOptions) fyne.CanvasObject {
	if opts.LoadStats == nil {
		opts.LoadStats = core.LoadDashboardStats
	}
	heading := widget.NewLabelWithStyle("Dashboard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Live counts from data/config.json + data/emails.db.")

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

	return container.NewVBox(
		heading, subtitle, widget.NewSeparator(),
		statsRow, widget.NewSeparator(),
		container.NewHBox(startWatch, refreshBtn),
		status,
	)
}

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
