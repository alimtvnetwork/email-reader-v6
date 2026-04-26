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
	"github.com/lovable/email-read/internal/errtrace"
)

// DashboardOptions wires the dashboard to app state + actions.
type DashboardOptions struct {
	Alias        string
	OnStartWatch func()
	OnRefresh    func()
	LoadStats    LoadStatsFunc
}

// LoadStatsFunc is the seam used by tests to inject deterministic counts.
// Returns a Result envelope so failures carry an error code (Delta #2).
type LoadStatsFunc func(ctx context.Context, alias string) errtrace.Result[core.DashboardStats]

func BuildDashboard(opts DashboardOptions) fyne.CanvasObject {
	if opts.LoadStats == nil {
		opts.LoadStats = core.LoadDashboardStats
	}
	heading := widget.NewLabelWithStyle("Dashboard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Live counts from data/config.json + data/emails.db.")

	cards := newDashboardCards()
	status := widget.NewLabel("Loaded just now.")
	status.Wrapping = fyne.TextWrapWord

	autoStart := newAutoStartIndicator()
	refresh := makeDashboardRefresh(opts, cards, status)
	refresh()

	actions := newDashboardActions(opts, refresh)
	return container.NewVBox(
		heading, subtitle, widget.NewSeparator(),
		cards.Row, widget.NewSeparator(),
		actions, autoStart, status,
	)
}

// newAutoStartIndicator returns a label that shows the current
// `Settings.AutoStartWatch` value and updates live on every SettingsEvent
// (CF-D1). Constructs its own Settings client + background subscriber so
// the dashboard owns no extra options. On any setup failure it shows a
// neutral "Auto-start: unknown" — never blocks the UI.
//
// Spec: spec/21-app/02-features/07-settings/99-consistency-report.md CF-D1.
func newAutoStartIndicator() *widget.Label {
	lbl := widget.NewLabel("Auto-start watcher: unknown")
	s := core.NewSettings(time.Now)
	if s.HasError() {
		return lbl
	}
	svc := s.Value()
	if snap := svc.Get(context.Background()); !snap.HasError() {
		lbl.SetText(formatAutoStart(snap.Value().AutoStartWatch))
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, _ := svc.Subscribe(ctx)
	go forwardAutoStartEvents(events, lbl, cancel)
	return lbl
}

// forwardAutoStartEvents drains Settings events and updates the label.
// The cancel func keeps `ctx` alive for the goroutine's lifetime — when
// the channel closes we release it. (Dashboard widget tear-down is not
// observable from here in current Fyne; the leak is bounded to one
// goroutine per BuildDashboard, which only runs on shell rebuilds.)
func forwardAutoStartEvents(events <-chan core.SettingsEvent, lbl *widget.Label, cancel context.CancelFunc) {
	defer cancel()
	for ev := range events {
		lbl.SetText(formatAutoStart(ev.Snapshot.AutoStartWatch))
	}
}

// formatAutoStart renders the auto-start value as a human-readable label.
func formatAutoStart(on bool) string {
	if on {
		return "Auto-start watcher: ● ON"
	}
	return "Auto-start watcher: ○ off"
}

// dashboardCards groups the four stat tiles plus their parent grid container.
type dashboardCards struct {
	Accounts statCard
	Rules    statCard
	Emails   statCard
	Alias    statCard
	Row      *fyne.Container
}

// newDashboardCards builds the four stat tiles in a single row.
func newDashboardCards() dashboardCards {
	c := dashboardCards{
		Accounts: newStatCard("Accounts", "—"),
		Rules:    newStatCard("Rules enabled", "—"),
		Emails:   newStatCard("Emails stored", "—"),
		Alias:    newStatCard("Selected account", "(none)"),
	}
	c.Row = container.NewGridWithColumns(4,
		c.Accounts.Container, c.Rules.Container, c.Emails.Container, c.Alias.Container,
	)
	return c
}

// makeDashboardRefresh returns a closure that reloads stats and updates the cards.
func makeDashboardRefresh(opts DashboardOptions, cards dashboardCards, status *widget.Label) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res := opts.LoadStats(ctx, opts.Alias)
		if res.HasError() {
			status.SetText("⚠ Failed to load stats: " + res.Error().Error())
			return
		}
		s := res.Value()
		cards.Accounts.Value.SetText(fmt.Sprintf("%d", s.Accounts))
		cards.Rules.Value.SetText(fmt.Sprintf("%d / %d", s.RulesEnabled, s.RulesTotal))
		cards.Emails.Value.SetText(FormatEmailsValue(s))
		if s.Alias == "" {
			cards.Alias.Value.SetText("(none)")
		} else {
			cards.Alias.Value.SetText(s.Alias)
		}
		status.SetText("Loaded at " + time.Now().Format("15:04:05") + ".")
		if opts.OnRefresh != nil {
			opts.OnRefresh()
		}
	}
}

// newDashboardActions builds the start-watch + refresh button row.
func newDashboardActions(opts DashboardOptions, refresh func()) *fyne.Container {
	refreshBtn := widget.NewButton("Refresh", refresh)
	startWatch := widget.NewButton("Start watching", func() {
		if opts.OnStartWatch != nil {
			opts.OnStartWatch()
		}
	})
	startWatch.Importance = widget.HighImportance
	return container.NewHBox(startWatch, refreshBtn)
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
