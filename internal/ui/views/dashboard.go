//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/watcher"
)

// DashboardOptions wires the dashboard to app state + actions.
//
// **Phase 3.5 rename.** The transitional `LoadStats` seam (and the
// `*core.DashboardService.LoadStats` method it shadowed) have both
// been deleted. Production wiring uses `Service.Summary` (the
// spec-aligned name); tests inject deterministic counts via the
// `Summary` field on this Options struct. When `Summary` is nil and
// `Service` is non-nil we bind `Service.Summary`. When both are nil
// we render a degraded card row (status: "dashboard service not
// wired") rather than panicking — keeps headless / partial-bootstrap
// previews safe.
type DashboardOptions struct {
	Alias          string
	OnStartWatch   func()
	OnRefresh      func()
	Service        *core.DashboardService     // production seam — constructed in app bootstrap
	Summary        SummaryFunc                // test-only override; takes precedence over Service when non-nil
	Bus            *watcher.Bus               // optional; live counter row when non-nil
	HealthSource   core.AccountHealthSource   // Slice #103 production seam — per-account health rollup; nil → row hidden
	ActivitySource core.ActivitySource        // Slice #105 production seam — recent watch-event feed; nil → row hidden
}

// SummaryFunc is the seam used by tests to inject deterministic counts.
// Returns a Result envelope so failures carry an error code (Delta #2).
type SummaryFunc func(ctx context.Context, alias string) errtrace.Result[core.DashboardSummary]

// recentActivityRenderLimit is the cap fed into
// `(*DashboardService).RecentActivity` from the dashboard view. Picked
// to fit one screen of typical readout (10 lines × ~80 char ≈ what a
// glanceable activity feed should show); user-driven deeper feeds will
// land as a dedicated `Activity` nav slice with pagination.
const recentActivityRenderLimit = 10

func BuildDashboard(opts DashboardOptions) fyne.CanvasObject {
	if opts.Summary == nil && opts.Service != nil {
		// Bind the service's typed method to the SummaryFunc shape so
		// downstream code (refresh closure, tests) sees one uniform seam.
		opts.Summary = opts.Service.Summary
	}
	heading := widget.NewLabelWithStyle("Dashboard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Live counts from data/config.json + data/emails.db.")

	cards := newDashboardCards()
	status := widget.NewLabel("Loaded just now.")
	status.Wrapping = fyne.TextWrapWord

	// Slice #103: per-account health rollup row. Hidden when neither
	// the Service nor a HealthSource is wired (degraded boot).
	health := widget.NewLabel("")
	health.Wrapping = fyne.TextWrapWord
	health.Hide()

	// Slice #113: recent-activity feed as a virtualised, clickable
	// `widget.List`. Replaces the previous multi-line `widget.Label`
	// — gives us per-row severity coloring, scroll on overflow, and
	// click-to-toggle row expansion (clicking a row prints its full
	// payload to `status` so error-code messages aren't truncated).
	//
	// State: `activityRows` is the live slice driving the list; the
	// closure below mutates it on Refresh. Hidden until we have at
	// least one row OR an explicit error message — keeps boot quiet.
	var activityRows []core.ActivityRow
	activityHeader := widget.NewLabelWithStyle(
		"Recent activity:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	activityHeader.Hide()
	activityList := newActivityList(&activityRows, status)
	activityList.Hide()
	activityErr := widget.NewLabel("")
	activityErr.Wrapping = fyne.TextWrapWord
	activityErr.Hide()
	// Wrap the virtualised list in a fixed-height slot so the parent
	// VBox reserves `activityListMaxHeight` pixels for it regardless
	// of how many rows it currently holds. Without this, `widget.List`
	// reports a 1-row MinSize and the rows visually overlap whatever
	// widget VBox places below the list (the live-counters tile row).
	activityListSlot := container.New(fixedHeightLayout{Height: activityListMaxHeight}, activityList)
	activityBox := container.NewVBox(activityHeader, activityErr, activityListSlot)

	autoStart := newAutoStartIndicator()
	refresh := makeDashboardRefresh(opts, cards, status)
	refreshHealth := makeDashboardHealthRefresh(opts, health)
	refreshActivity := makeDashboardActivityRefresh(opts, &activityRows, activityHeader, activityList, activityErr)
	combined := func() {
		refresh()
		refreshHealth()
		refreshActivity()
	}
	combined()

	actions := newDashboardActions(opts, combined)
	live := newDashboardLiveRow(opts, combined)
	return container.NewVBox(
		heading, subtitle, widget.NewSeparator(),
		cards.Row, widget.NewSeparator(),
		health,
		activityBox,
		live, widget.NewSeparator(),
		actions, autoStart, status,
	)
}

// activityListMaxHeight caps the recent-activity list at ~10 rows so
// the dashboard's vertical rhythm stays predictable. Picked to match
// `recentActivityRenderLimit` × ~24px row height.
const activityListMaxHeight = 240

// newActivityList constructs the virtualised activity feed. The list
// reads from `*rows` on every refresh tick (caller mutates the slice
// in-place then calls `list.Refresh()`), so we own zero state here —
// matches Fyne's `widget.List` data-binding contract.
//
// Click handler echoes the clicked row's full payload into `status`
// so a long error message that the truncated row hid is recoverable
// without leaving the dashboard.
func newActivityList(rows *[]core.ActivityRow, status *widget.Label) *widget.List {
	list := widget.NewList(
		func() int { return len(*rows) },
		func() fyne.CanvasObject {
			// Template row: a single label that will be SetText'd per
			// row. Wrapping is OFF on purpose — long error messages
			// truncate with "…" rather than reflowing the row height,
			// which would break the virtualised-scroll height math.
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			r := (*rows)[i]
			lbl.SetText(formatActivityRow(r))
			// Severity-based importance hint — Fyne's theme renders
			// HighImportance in the accent colour and DangerImportance
			// in red, so PollFailed rows pop without hard-coding HSL.
			lbl.Importance = activityImportance(r.Kind)
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(*rows) {
			return
		}
		r := (*rows)[i]
		status.SetText(formatActivityRowDetail(r))
		// Deselect immediately so a second click on the same row
		// re-fires the handler — `widget.List` swallows repeats on a
		// sticky selection.
		list.UnselectAll()
	}
	return list
}

// activityImportance maps an ActivityKind to a Fyne importance level
// so the row colour mirrors severity without hard-coding theme HSL.
func activityImportance(k core.ActivityKind) widget.Importance {
	switch k {
	case core.ActivityPollFailed:
		return widget.DangerImportance
	case core.ActivityPollSucceeded, core.ActivityEmailStored:
		return widget.SuccessImportance
	case core.ActivityRuleMatched:
		return widget.HighImportance
	default:
		return widget.MediumImportance
	}
}

// makeDashboardHealthRefresh returns a closure that loads per-account
// health rows and renders them as a one-line summary on `lbl`. When
// neither `opts.Service` nor `opts.HealthSource` is wired the label
// stays hidden (degraded boot — Slice #103 wiring may not yet be
// active if WatchRuntime failed to open the store).
//
// Errors surface inline with a "⚠" prefix so a transient store
// failure doesn't blank the whole dashboard.
func makeDashboardHealthRefresh(opts DashboardOptions, lbl *widget.Label) func() {
	return func() {
		if opts.Service == nil || opts.HealthSource == nil {
			lbl.Hide()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res := opts.Service.AccountHealth(ctx, opts.HealthSource)
		if res.HasError() {
			lbl.SetText("⚠ Health: " + res.Error().Error())
			lbl.Show()
			return
		}
		lbl.SetText(formatHealthRollup(res.Value()))
		lbl.Show()
	}
}

// formatHealthRollup renders a one-line "Health: 3 ● healthy · 1 ◐ warn · 0 ✗ error"
// summary across all configured accounts. Empty input → "Health: (no accounts)".
func formatHealthRollup(rows []core.AccountHealthRow) string {
	if len(rows) == 0 {
		return "Health: (no accounts configured)"
	}
	var healthy, warn, errCount int
	for _, r := range rows {
		switch r.Health {
		case core.HealthHealthy:
			healthy++
		case core.HealthWarning:
			warn++
		case core.HealthError:
			errCount++
		}
	}
	return fmt.Sprintf("Health: %d ● healthy · %d ◐ warning · %d ✗ error",
		healthy, warn, errCount)
}

// makeDashboardActivityRefresh returns a closure that loads the most
// recent N activity rows (`recentActivityRenderLimit`) into the
// virtualised `widget.List` (Slice #113). When neither `opts.Service`
// nor `opts.ActivitySource` is wired everything stays hidden
// (degraded boot — Slice #105 wiring may not yet be active if
// WatchRuntime failed to open the store).
//
// Refresh contract:
//   - Mutates `*rows` in place then calls `list.Refresh()` so Fyne
//     re-asks the length/update callbacks. We never re-create the
//     list — that would lose scroll position on every poll tick.
//   - Empty result → list hidden, header shows "Recent activity: (no
//     recent activity)" so the absence is explicit, not a gap.
//   - Error → `errLbl` shown with "⚠ " prefix, list + header hidden.
func makeDashboardActivityRefresh(opts DashboardOptions, rows *[]core.ActivityRow,
	header *widget.Label, list *widget.List, errLbl *widget.Label) func() {
	return func() {
		if opts.Service == nil || opts.ActivitySource == nil {
			header.Hide()
			list.Hide()
			errLbl.Hide()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res := opts.Service.RecentActivity(ctx, recentActivityRenderLimit, opts.ActivitySource)
		if res.HasError() {
			errLbl.SetText("⚠ Recent activity: " + res.Error().Error())
			errLbl.Show()
			header.Hide()
			list.Hide()
			return
		}
		errLbl.Hide()
		*rows = res.Value()
		if len(*rows) == 0 {
			header.SetText("Recent activity: (no recent activity)")
			header.Show()
			list.Hide()
			return
		}
		header.SetText("Recent activity:")
		header.Show()
		// Height is reserved by the parent fixedHeightLayout slot
		// (see BuildDashboard) — no need to Resize the list here;
		// doing so fought the parent layout and produced the visual
		// overlap with the live-counters row below.
		list.Show()
		list.Refresh()
	}
}

// formatActivityRow renders one ActivityRow as the single-line string
// shown in the `widget.List` (Slice #113). Layout matches the legacy
// multi-line block: "HH:MM:SS  alias  <glyph> Kind · Message (err NN)".
// Empty Alias / Message / ErrorCode are omitted so a heartbeat row
// reads "10:05:30  ▶ PollStarted" without dangling separators.
func formatActivityRow(r core.ActivityRow) string {
	var b strings.Builder
	b.WriteString(r.OccurredAt.Format("15:04:05"))
	b.WriteString("  ")
	if r.Alias != "" {
		b.WriteString(r.Alias)
		b.WriteString("  ")
	}
	b.WriteString(activityGlyph(r.Kind))
	b.WriteString(" ")
	b.WriteString(string(r.Kind))
	if r.Message != "" {
		b.WriteString(" · ")
		b.WriteString(r.Message)
	}
	if r.ErrorCode != 0 {
		b.WriteString(fmt.Sprintf(" (err %d)", r.ErrorCode))
	}
	return b.String()
}

// formatActivityRowDetail renders the full payload of a single row
// for the click-to-expand handler — pushes the result into the
// dashboard `status` label so a long error message that the row
// truncated is recoverable. Includes a UTC date prefix that the
// per-row label omits to save horizontal space.
func formatActivityRowDetail(r core.ActivityRow) string {
	parts := []string{
		r.OccurredAt.UTC().Format("2006-01-02 15:04:05 UTC"),
	}
	if r.Alias != "" {
		parts = append(parts, "alias="+r.Alias)
	}
	parts = append(parts, "kind="+string(r.Kind))
	if r.Message != "" {
		parts = append(parts, "msg="+r.Message)
	}
	if r.ErrorCode != 0 {
		parts = append(parts, fmt.Sprintf("err=%d", r.ErrorCode))
	}
	return strings.Join(parts, " · ")
}

// formatRecentActivity is the legacy multi-line formatter retained
// for the existing test surface (`dashboard_activity_test.go`). The
// production render path now drives `widget.List` row-by-row via
// `formatActivityRow`; this helper composes the same per-row output
// under a "Recent activity:" header so behavioural tests stay valid
// without dragging Fyne into them.
func formatRecentActivity(rows []core.ActivityRow) string {
	if len(rows) == 0 {
		return "Recent activity:\n  (no recent activity)"
	}
	var b strings.Builder
	b.WriteString("Recent activity:")
	for _, r := range rows {
		b.WriteString("\n  ")
		b.WriteString(formatActivityRow(r))
	}
	return b.String()
}

// activityGlyph returns the single-glyph prefix for an ActivityKind.
// Picked so the column is visually distinct at a glance even before
// the user reads the kind word.
func activityGlyph(k core.ActivityKind) string {
	switch k {
	case core.ActivityPollStarted:
		return "▶"
	case core.ActivityPollSucceeded:
		return "✓"
	case core.ActivityPollFailed:
		return "✗"
	case core.ActivityEmailStored:
		return "✉"
	case core.ActivityRuleMatched:
		return "◆"
	default:
		return "•"
	}
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
		if opts.Summary == nil {
			// Degraded path: bootstrap didn't wire a *DashboardService
			// and no test override was supplied. Surface the wiring
			// gap in the status line instead of panicking.
			status.SetText("⚠ Dashboard service not wired (no Service or Summary injected)")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res := opts.Summary(ctx, opts.Alias)
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
// callback is invoked (debounced) on every EventNewMail or
// EventUrlOpened so the static stat cards auto-bump without the user
// clicking Refresh.
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
// auto-refresh triggers (NewMail or UrlOpened). Picked to absorb
// backfill bursts (50 messages in <1 s) and rule-fan-out (one URL per
// match) into a single COUNT(*) reload while still feeling instant for
// human-paced activity.
const dashboardRefreshDebounce = 750 * time.Millisecond

// subscribeDashboardBus drains the watcher Bus and pushes the latest
// counters into the tile labels. Filters by alias when one is selected;
// otherwise aggregates across all aliases (dashboard-only behaviour).
// Closing the bus channel terminates the goroutine cleanly.
//
// On every accepted EventNewMail / EventUrlOpened we also call
// `refresh` (the closure returned by makeDashboardRefresh) — debounced
// via ShouldRefreshDashboardOnEvent so backfill/fan-out bursts don't
// hammer SQL.
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
