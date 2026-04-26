// watch.go renders the Watch view per spec/21-app/02-features/05-watch/02-frontend.md.
//
// Scope vs the full spec (1.0.0): this is an **MVP scaffold** that ships
// the widget tree (status header / Cards|RawLog tabs / footer) and the
// **CF-W3 live cadence label** wired to `core.Settings.Subscribe`. The
// real-time event-bus consumers (cards, raw log, status dot, counters)
// land when `core.Watch` + `internal/eventbus` ship per `01-backend.md`.
// Until then, the tabs render an honest "awaiting Watch service" empty
// state and the footer shows the cadence + alias.
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
// renders, but Start is disabled).
type WatchOptions struct {
	Alias string
}

// BuildWatch returns the Watch view scaffold per spec §2.1.
//
// Layout: container.NewBorder(top=header, bottom=footer, center=tabs).
//   - header (§2.2): status dot + label + alias + ▶ Start placeholder.
//   - tabs (§2.3 / §2.4): Cards + Raw log placeholders.
//   - footer (§2.5): poll cadence (CF-W3) + counter placeholders.
func BuildWatch(opts WatchOptions) fyne.CanvasObject {
	header := buildWatchHeader(opts.Alias)
	tabs := container.NewAppTabs(
		container.NewTabItem("Cards", buildWatchCardsPlaceholder()),
		container.NewTabItem("Raw log", buildWatchRawLogPlaceholder()),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	footer := buildWatchFooter()
	return container.NewBorder(header, footer, nil, nil, tabs)
}

// buildWatchHeader renders the status header strip (§2.2). Today the
// status dot/label are static placeholders pending core.Watch; the alias
// and ▶ Start CTA are real.
func buildWatchHeader(alias string) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Watch", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	statusLabel := widget.NewLabel(fmt.Sprintf("○ Idle · %s", aliasLabel(alias)))
	subtitle := widget.NewLabel("Real-time IMAP monitor — start watching to stream events here.")
	subtitle.Wrapping = fyne.TextWrapWord
	startBtn := widget.NewButton("▶ Start watching (CLI)", nil)
	startBtn.Disable() // wired to core.Watch.Start once it ships
	row := container.NewBorder(nil, nil, nil, startBtn, statusLabel)
	return container.NewVBox(heading, subtitle, row, widget.NewSeparator())
}

// buildWatchCardsPlaceholder renders the Cards-tab empty state per §2.3
// (cardsEmpty label).
func buildWatchCardsPlaceholder() fyne.CanvasObject {
	l := widget.NewLabel("No new mail yet — heartbeats stream to the Raw log tab.\n\nLive event stream awaiting core.Watch service (per spec 02-features/05-watch/01-backend.md).")
	l.Wrapping = fyne.TextWrapWord
	l.Alignment = fyne.TextAlignCenter
	return container.NewPadded(l)
}

// buildWatchRawLogPlaceholder renders the Raw log empty state per §2.4.
func buildWatchRawLogPlaceholder() fyne.CanvasObject {
	l := widget.NewLabel("(raw log — awaiting core.Watch event bus)")
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
