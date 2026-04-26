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
)

// WatchOptions wires the Watch view to app state. Alias is the alias
// shown in the header; "" means "no account selected" (the view still
// renders, but Start is disabled). Watch + PollSeconds are optional —
// when nil the header falls back to a disabled placeholder button so
// headless tests and the pre-wiring fallback both keep working.
type WatchOptions struct {
	Alias       string
	Watch       *core.Watch
	PollSeconds func() int
}

// BuildWatch returns the Watch view scaffold per spec §2.1.
//
// Layout: container.NewBorder(top=header, bottom=footer, center=tabs).
//   - header (§2.2): status dot + label + alias + ▶ Start/■ Stop button.
//   - tabs (§2.3 / §2.4): Cards + Raw log placeholders.
//   - footer (§2.5): poll cadence (CF-W3) + counter placeholders.
func BuildWatch(opts WatchOptions) fyne.CanvasObject {
	header := buildWatchHeader(opts)
	tabs := container.NewAppTabs(
		container.NewTabItem("Cards", buildWatchCardsPlaceholder()),
		container.NewTabItem("Raw log", buildWatchRawLogPlaceholder()),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	footer := buildWatchFooter()
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
		_ = opts.Watch.Stop(opts.Alias, 5*time.Second)
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

// buildWatchCardsPlaceholder renders the Cards-tab empty state per §2.3
// (cardsEmpty label).
func buildWatchCardsPlaceholder() fyne.CanvasObject {
	l := widget.NewLabel("No new mail yet — heartbeats stream to the Raw log tab.\n\nLive event stream awaiting Cards consumer (next slice on top of core.Watch).")
	l.Wrapping = fyne.TextWrapWord
	l.Alignment = fyne.TextAlignCenter
	return container.NewPadded(l)
}

// buildWatchRawLogPlaceholder renders the Raw log empty state per §2.4.
func buildWatchRawLogPlaceholder() fyne.CanvasObject {
	l := widget.NewLabel("(raw log — Watch event bus consumer wires up next)")
	l.Wrapping = fyne.TextWrapWord
	return container.NewPadded(l)
}

// buildWatchFooter renders the footer (§2.5) and wires the live cadence
// label (CF-W3): a Settings.Subscribe consumer updates the displayed
// poll-seconds whenever a Save / Reset event arrives.
func buildWatchFooter() fyne.CanvasObject {
	cadence := newCadenceIndicator()
	counters := widget.NewLabel("polls=— · newMail=— · matches=—")
	return container.NewBorder(widget.NewSeparator(), nil, counters, nil, cadence)
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
