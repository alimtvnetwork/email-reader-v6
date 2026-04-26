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
	"github.com/lovable/email-read/internal/watcher"
)

// DashboardOptions wires the dashboard to app state + actions.
type DashboardOptions struct {
	Alias        string
	OnStartWatch func()
	OnRefresh    func()
	LoadStats    LoadStatsFunc
	Bus          *watcher.Bus // optional; live counter row when non-nil
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
	live := newDashboardLiveRow(opts, refresh)
	return container.NewVBox(
		heading, subtitle, widget.NewSeparator(),
		cards.Row, widget.NewSeparator(),
		live, widget.NewSeparator(),
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

// liveTiles groups the four live counter cards. Reuses statCard so the
// styling matches the static stats row above.
type liveTiles struct {
	Polls   statCard
	NewMail statCard
	Matches statCard
	Opens   statCard
	Errors  statCard
}

func newLiveTiles() liveTiles {
	return liveTiles{
		Polls:   newStatCard("Polls", "0"),
		NewMail: newStatCard("New mail", "0"),
		Matches: newStatCard("Rule matches", "0"),
		Opens:   newStatCard("Opens", "0"),
		Errors:  newStatCard("Errors", "0"),
	}
}

// applyCounters writes the latest WatchCounters values into the tile labels.
func (t liveTiles) applyCounters(c WatchCounters) {
	t.Polls.Value.SetText(fmt.Sprintf("%d", c.Polls))
	t.NewMail.Value.SetText(fmt.Sprintf("%d", c.NewMail))
	t.Matches.Value.SetText(fmt.Sprintf("%d", c.Matches))
	t.Opens.Value.SetText(fmt.Sprintf("%d / %d", c.Opens, c.Opens+c.OpenFail))
	t.Errors.Value.SetText(fmt.Sprintf("%d", c.Errors))
}

// newDashboardLiveRow builds the live counter caption + tile row. When
// opts.Bus is nil (headless boot, missing runtime) it returns a single
// muted placeholder so the dashboard still renders. The `refresh`
// callback is invoked (debounced) on every EventNewMail so the static
// "Emails stored" tile auto-bumps without the user clicking Refresh.
func newDashboardLiveRow(opts DashboardOptions, refresh func()) fyne.CanvasObject {
	caption := widget.NewLabel(FormatDashboardCounterScope(DashboardCounterScope{Alias: opts.Alias}))
	if opts.Bus == nil {
		placeholder := widget.NewLabel("(live counters appear once the watcher is running)")
		return container.NewVBox(caption, placeholder)
	}
	tiles := newLiveTiles()
	row := container.NewGridWithColumns(5,
		tiles.Polls.Container, tiles.NewMail.Container,
		tiles.Matches.Container, tiles.Opens.Container, tiles.Errors.Container,
	)
	go subscribeDashboardBus(opts, tiles, refresh)
	return container.NewVBox(caption, row)
}

// dashboardRefreshDebounce is the minimum gap between consecutive
// auto-refresh triggers from EventNewMail. Picked to absorb backfill
// bursts (50 messages in <1 s) into a single COUNT(*) reload while
// still feeling instant for human-paced new mail.
const dashboardRefreshDebounce = 750 * time.Millisecond

// subscribeDashboardBus drains the watcher Bus and pushes the latest
// counters into the tile labels. Filters by alias when one is selected;
// otherwise aggregates across all aliases (dashboard-only behaviour).
// Closing the bus channel terminates the goroutine cleanly.
//
// On every accepted EventNewMail we also call `refresh` (the closure
// returned by makeDashboardRefresh) — debounced via
// ShouldRefreshDashboardOnEvent so backfill bursts don't hammer SQL.
func subscribeDashboardBus(opts DashboardOptions, tiles liveTiles, refresh func()) {
	events, cancel := opts.Bus.Subscribe()
	defer cancel()
	var counters WatchCounters
	var lastRefresh time.Time
	for ev := range events {
		if !DashboardAcceptsEvent(ev, opts.Alias) {
			continue
		}
		counters = AccumulateCounters(counters, ev)
		tiles.applyCounters(counters)
		if refresh == nil {
			continue
		}
		ok, next := ShouldRefreshDashboardOnEvent(ev, lastRefresh, time.Now(), dashboardRefreshDebounce)
		if ok {
			lastRefresh = next
			refresh()
		}
	}
}
