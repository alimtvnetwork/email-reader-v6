// watch.go renders the Watch view per spec/21-app/02-features/05-watch/02-frontend.md.
//
// MVP wiring (2026-04-26): the **header Start/Stop button is now real**
// when a `*core.Watch` runtime is supplied. Without it the button stays
// disabled (preserves headless / pre-wiring behaviour for tests). The
// status label subscribes to `Watch.Subscribe()` and reflects WatchStart
// / WatchStop / WatchError events live. The Cards + Raw log tabs remain
// scaffolds — they consume the same event stream in the next slice.
//
// CF-W3 (cadence label) is unchanged: a Settings.Subscribe consumer
// keeps "every N s" current.
//
// Behind the !nofyne build tag because it imports the Fyne widget set.
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
	"github.com/lovable/email-read/internal/watcher"
)

// WatchOptions wires the Watch view to app state. Alias is the alias
// shown in the header; "" means "no account selected" (the view still
// renders, but Start is disabled). Watch + PollSeconds + Bus are
// optional — when nil the view falls back to disabled placeholders so
// headless tests and the pre-wiring fallback both keep working.
//
// Clipboard, when non-nil, enables the "Copy" buttons on the Raw log
// and Cards tabs (production wiring uses fyne.CurrentApp().Clipboard()).
// Left as a seam so headless tests stay clipboard-free.
type WatchOptions struct {
	Alias       string
	Watch       *core.Watch
	PollSeconds func() int
	Bus         *watcher.Bus
	Clipboard   fyne.Clipboard
}

// BuildWatch returns the Watch view per spec §2.1.
//
// Layout: container.NewBorder(top=header, bottom=footer, center=tabs).
//   - header (§2.2): status dot + label + alias + ▶ Start/■ Stop button.
//   - tabs (§2.3 / §2.4): live Cards + Raw log when Bus is wired,
//     placeholders otherwise.
//   - footer (§2.5): poll cadence (CF-W3) + live counters when Bus is
//     wired.
func BuildWatch(opts WatchOptions) fyne.CanvasObject {
	header := buildWatchHeader(opts)
	cards, cardsRefresh := buildWatchCardsTab(opts)
	rawlog, rawRefresh := buildWatchRawLogTab(opts)
	tabs := container.NewAppTabs(
		container.NewTabItem("Cards", cards),
		container.NewTabItem("Raw log", rawlog),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	footer, counterRefresh := buildWatchFooter(opts)
	if opts.Bus != nil && opts.Alias != "" {
		go subscribeWatchBus(opts, cardsRefresh, rawRefresh, counterRefresh)
	}
	return container.NewBorder(header, footer, nil, nil, tabs)
}

// buildWatchHeader renders the status header strip (§2.2) with a real
// Start/Stop toggle wired to opts.Watch when present.
func buildWatchHeader(opts WatchOptions) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Watch", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	statusLabel := widget.NewLabel("")
	subtitle := widget.NewLabel("Real-time IMAP monitor — start watching to stream events here.")
	subtitle.Wrapping = fyne.TextWrapWord
	btn := widget.NewButton("", nil)
	wireWatchHeaderControls(opts, statusLabel, btn)
	row := container.NewBorder(nil, nil, nil, btn, statusLabel)
	return container.NewVBox(heading, subtitle, row, widget.NewSeparator())
}

// wireWatchHeaderControls binds the Start/Stop button + status label
// to the provided runtime. When opts.Watch is nil we render the
// pre-wiring placeholder ("CLI" hint, button disabled). When present,
// the button toggles Start/Stop and the label reflects the current
// runner state, updated live via Watch.Subscribe.
func wireWatchHeaderControls(opts WatchOptions, statusLabel *widget.Label, btn *widget.Button) {
	if opts.Watch == nil {
		statusLabel.SetText(fmt.Sprintf("○ Idle · %s", aliasLabel(opts.Alias)))
		btn.SetText("▶ Start watching (CLI)")
		btn.Disable()
		return
	}
	if opts.Alias == "" {
		statusLabel.SetText("○ Idle · (no account)")
		btn.SetText("▶ Start watching")
		btn.Disable()
		return
	}
	renderHeaderState(opts, statusLabel, btn)
	btn.OnTapped = func() { handleWatchToggle(opts, statusLabel, btn) }
	go subscribeWatchHeader(opts, statusLabel, btn)
}

// handleWatchToggle drives Watch.Start / Watch.Stop based on the
// current runner state. UI is refreshed eagerly so the user sees an
// immediate response — the Subscribe goroutine then keeps it in sync
// if the runner exits on its own (error, ctx cancel, etc.).
func handleWatchToggle(opts WatchOptions, statusLabel *widget.Label, btn *widget.Button) {
	if opts.Watch.IsRunning(opts.Alias) {
		// 10s covers the 5s live watcher dial timeout plus login/logout slack
		// so Stop still observes WatchStop even during a hung IMAP dial.
		_ = opts.Watch.Stop(opts.Alias, 10*time.Second)
	} else {
		_ = opts.Watch.Start(context.Background(), core.WatchOptions{
			Alias:       opts.Alias,
			PollSeconds: pickPollSeconds(opts),
		})
	}
	renderHeaderState(opts, statusLabel, btn)
}

// renderHeaderState updates the label + button text from the current
// IsRunning(alias) snapshot. Pure function over Watch state so it is
// safe to call from both the UI thread and the Subscribe goroutine
// (Fyne widget setters are goroutine-safe per its docs).
func renderHeaderState(opts WatchOptions, statusLabel *widget.Label, btn *widget.Button) {
	if opts.Watch.IsRunning(opts.Alias) {
		statusLabel.SetText(fmt.Sprintf("● Watching · %s", opts.Alias))
		btn.SetText("■ Stop")
		btn.Enable()
		return
	}
	statusLabel.SetText(fmt.Sprintf("○ Idle · %s", opts.Alias))
	btn.SetText("▶ Start")
	btn.Enable()
}

// subscribeWatchHeader drains Watch events and re-renders the header
// on every change for `opts.Alias`. Other-alias events are ignored —
// a multi-alias dashboard would consume the same stream with its own
// filter. Goroutine exits when the bus channel closes (Watch never
// exposes a Close today, so this leaks at most one goroutine per
// BuildWatch / shell rebuild — acceptable per project guidance).
func subscribeWatchHeader(opts WatchOptions, statusLabel *widget.Label, btn *widget.Button) {
	events, _ := opts.Watch.Subscribe()
	for ev := range events {
		if ev.Alias != opts.Alias {
			continue
		}
		renderHeaderState(opts, statusLabel, btn)
	}
}

// pickPollSeconds prefers the live Settings-backed accessor when
// supplied; falls back to the conservative default to keep the call
// site nil-safe.
func pickPollSeconds(opts WatchOptions) int {
	if opts.PollSeconds != nil {
		if v := opts.PollSeconds(); v > 0 {
			return v
		}
	}
	return 3
}

// buildWatchCardsTab returns the Cards-tab widget plus a refresh
// callback the bus subscriber invokes when a new card lands. The
// caller threads `cards` slice ownership into the closure so the
// rendering stays single-goroutine (Fyne widget setters are safe but
// the slice itself is not).
//
// When opts.Bus or opts.Alias is empty we render the empty-state
// label and return a no-op refresh — keeps headless tests + pre-
// wiring callers honest.
func buildWatchCardsTab(opts WatchOptions) (fyne.CanvasObject, func(WatchCard)) {
	if opts.Bus == nil || opts.Alias == "" {
		l := widget.NewLabel("No new mail yet — heartbeats stream to the Raw log tab.\n\nLive event stream awaiting account selection.")
		l.Wrapping = fyne.TextWrapWord
		l.Alignment = fyne.TextAlignCenter
		return container.NewPadded(l), func(WatchCard) {}
	}
	body := widget.NewLabel("(awaiting first event…)")
	body.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(body)
	var cards []WatchCard
	refresh := func(c WatchCard) {
		cards = AppendBounded(cards, c, watchCardsCap)
		body.SetText(renderCards(cards))
	}
	return scroll, refresh
}

// buildWatchRawLogTab mirrors buildWatchCardsTab for the raw line
// log. The buffer is much larger (events stream faster than cards) so
// we use a multiline read-only Entry (selectable + native
// keyboard-shortcut copy) plus an explicit "Copy all" button so the
// user can grab the entire visible buffer for a bug report.
//
// Why Entry instead of Label: widget.Label renders as static
// SimpleText and intercepts no pointer events — the user can't
// highlight a sentence to copy, which is the regression the user
// reported in the screenshot.
func buildWatchRawLogTab(opts WatchOptions) (fyne.CanvasObject, func(string)) {
	if opts.Bus == nil || opts.Alias == "" {
		l := widget.NewLabel("(raw log — start watching to stream events)")
		l.Wrapping = fyne.TextWrapWord
		return container.NewPadded(l), func(string) {}
	}
	body := widget.NewMultiLineEntry()
	body.Wrapping = fyne.TextWrapWord
	body.TextStyle = fyne.TextStyle{Monospace: true}
	// Read-only-but-selectable: Disable() keeps text greyed and blocks
	// selection on some Fyne versions, so we re-enable selection by
	// installing an OnChanged guard that reverts user edits. This
	// preserves "select to copy" + Cmd/Ctrl-C while preventing typing.
	currentText := ""
	body.OnChanged = func(s string) {
		if s != currentText {
			body.SetText(currentText)
		}
	}
	scroll := container.NewVScroll(body)

	var lines []string
	refresh := func(line string) {
		lines = AppendBounded(lines, line, watchRawLogCap)
		currentText = joinLines(lines)
		body.SetText(currentText)
	}

	copyAll := widget.NewButton("Copy all", func() {
		if opts.Clipboard == nil || currentText == "" {
			return
		}
		opts.Clipboard.SetContent(currentText)
	})
	if opts.Clipboard == nil {
		copyAll.Disable()
	}

	toolbar := container.NewBorder(nil, nil, nil, copyAll, widget.NewLabel(""))
	return container.NewBorder(toolbar, nil, nil, nil, scroll), refresh
}

// buildWatchFooter renders the footer (§2.5): live cadence label
// (CF-W3) on one side, live counters on the other. Returns a refresh
// callback the bus consumer fires after every event so the counter
// totals stay current.
func buildWatchFooter(opts WatchOptions) (fyne.CanvasObject, func(WatchCounters)) {
	cadence := newCadenceIndicator()
	counters := widget.NewLabel(WatchCounters{}.FormatCounters())
	if opts.Bus == nil || opts.Alias == "" {
		counters.SetText("polls=— · newMail=— · matches=— · opens=— · errors=—")
	}
	refresh := func(c WatchCounters) { counters.SetText(c.FormatCounters()) }
	return container.NewBorder(widget.NewSeparator(), nil, counters, nil, cadence), refresh
}

// watchCardsCap / watchRawLogCap bound the rolling buffers so a
// long-running watcher does not balloon UI memory. Tuned by feel:
// cards represent user-actionable events (new mail, matches, opens)
// so 50 covers most "since I sat down" sessions; raw log is firehose-
// y so 500 lines ≈ 25 minutes of 3-s polls.
const (
	watchCardsCap  = 50
	watchRawLogCap = 500
)

// subscribeWatchBus is the single goroutine that drains opts.Bus,
// filters by alias, and dispatches to the three refresh callbacks.
// Counters are accumulated locally (not shared with other views) so
// no mutex is needed. Closing the bus channel terminates the
// goroutine cleanly. Bounded leak: one goroutine per BuildWatch (i.e.
// per shell rebuild), matching the cadence-label consumer's
// lifecycle.
func subscribeWatchBus(opts WatchOptions, cardsRefresh func(WatchCard), rawRefresh func(string), counterRefresh func(WatchCounters)) {
	events, cancel := opts.Bus.Subscribe()
	defer cancel()
	var counters WatchCounters
	for ev := range events {
		if ev.Alias != opts.Alias {
			continue
		}
		// Mirror error-bearing events into the process-wide error log
		// so Diagnostics → Error Log + data/error-log.jsonl capture
		// what the user sees in the Raw log tab. Without this hook the
		// Error Log file stays empty during a noisy poll-failure run,
		// which is the regression the user reported.
		ReportWatchEventError(ev)
		rawRefresh(FormatRawLogLine(ev))
		counters = AccumulateCounters(counters, ev)
		counterRefresh(counters)
		if card, ok := EventToCard(ev); ok {
			cardsRefresh(card)
		}
	}
}

// renderCards renders the rolling card buffer as a multi-line text
// blob. A real Fyne List with custom item widgets is the long-term
// home, but a single Label keeps this slice's surface small and
// matches how the Raw log tab renders. Newest card on top.
func renderCards(cards []WatchCard) string {
	if len(cards) == 0 {
		return "(awaiting first event…)"
	}
	parts := make([]string, 0, len(cards))
	for _, c := range cards {
		parts = append(parts, formatCardLine(c))
	}
	return joinLines(parts)
}

// formatCardLine renders one card as "HH:MM:SS  TONE  Title — Body".
// Tone glyph parallels the Raw log glyphs so the two tabs feel
// related.
func formatCardLine(c WatchCard) string {
	return fmt.Sprintf("%s  %s  %s — %s",
		c.At.Format("15:04:05"), toneGlyph(c.Tone), c.Title, c.Body)
}

// toneGlyph maps CardTone to a single-rune visual cue. Kept tiny so
// it inlines and so future tones are a one-line addition.
func toneGlyph(t CardTone) string {
	switch t {
	case CardToneSuccess:
		return "✓"
	case CardToneWarn:
		return "⚠"
	case CardToneError:
		return "✗"
	}
	return "·"
}

// joinLines concatenates with newline separators. Local helper so we
// avoid importing strings just for one Join.
func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// newCadenceIndicator returns a label showing the current
// `Settings.PollSeconds` and updating live on every SettingsEvent
// (CF-W3). Constructs its own Settings client + background subscriber so
// the view owns no extra options. On any setup failure the label shows
// "cadence: unknown" — never blocks the UI.
//
// Spec: spec/21-app/02-features/07-settings/99-consistency-report.md CF-W3.
func newCadenceIndicator() *widget.Label {
	lbl := widget.NewLabel("cadence: unknown")
	s := core.NewSettings(time.Now)
	if s.HasError() {
		return lbl
	}
	svc := s.Value()
	if snap := svc.Get(context.Background()); !snap.HasError() {
		lbl.SetText(formatCadence(snap.Value().PollSeconds))
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, _ := svc.Subscribe(ctx)
	go forwardCadenceEvents(events, lbl, cancel)
	return lbl
}

// forwardCadenceEvents drains Settings events and updates the cadence
// label. Channel close (via cancel) terminates the goroutine cleanly.
// The cancel func keeps `ctx` alive for the goroutine's lifetime — when
// the channel closes we release it. Bounded leak: one goroutine per
// BuildWatch, only on shell rebuilds.
func forwardCadenceEvents(events <-chan core.SettingsEvent, lbl *widget.Label, cancel context.CancelFunc) {
	defer cancel()
	for ev := range events {
		lbl.SetText(formatCadence(ev.Snapshot.PollSeconds))
	}
}

// formatCadence renders the cadence value as a human-readable label
// (e.g. "cadence: every 3 s").
func formatCadence(secs uint16) string {
	return fmt.Sprintf("cadence: every %d s", secs)
}

// aliasLabel returns "(no account)" when alias is empty, else alias.
func aliasLabel(alias string) string {
	if alias == "" {
		return "(no account)"
	}
	return alias
}
