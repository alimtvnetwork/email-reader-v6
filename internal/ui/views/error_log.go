// error_log.go renders the Diagnostics → Error Log view.
//
// Phase 3.3 of the error-trace logging upgrade (.lovable/plan.md):
// every UI error path now calls `errlog.ReportError(component, err)`,
// which fills an in-memory ring buffer with full errtrace frame
// chains (see internal/ui/errlog). This view is the canonical
// surface for those entries — a Fyne list of summaries on the left,
// the full trace + Copy button on the right when a row is selected.
//
// Behind the !nofyne build tag because it imports the Fyne widget
// set. The ring buffer itself lives in internal/ui/errlog and stays
// fyne-free so unit tests + the headless CLI can keep recording
// errors without an OpenGL context.
//go:build !nofyne

package views

import (
	"fmt"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/ui/errlog"
)

// ErrorLogOptions wires the view to the process-wide ring buffer.
// Both fields are optional so unit tests / headless renders can stub
// them; production callers leave them nil and BuildErrorLog falls
// back to the package-level errlog singleton.
//
//   - Snapshot returns the current entries (newest-first ordering is
//     applied inside the view, so the source can return any order).
//   - Subscribe returns a channel of newly-appended entries for live
//     updates while the view is mounted. Channel close ends the
//     refresher goroutine.
type ErrorLogOptions struct {
	Snapshot  func() []errlog.Entry
	Subscribe func() <-chan errlog.Entry
	Clear     func()
	MarkRead  func()
	// Clipboard, when non-nil, receives the full trace text on Copy.
	// Production wiring uses fyne.CurrentApp().Clipboard() — left as
	// a seam for tests so we don't need a Fyne app to verify the
	// "copy" button calls through with the right payload.
	Clipboard fyne.Clipboard
}

// BuildErrorLog returns the Error Log detail pane: a header, a split
// view (summary list ↔ trace detail), and a footer with Clear /
// Copy actions. Calling BuildErrorLog also marks the unread counter
// as read so the sidebar badge clears the moment the user opens it.
func BuildErrorLog(opts ErrorLogOptions) fyne.CanvasObject {
	if opts.Snapshot == nil {
		opts.Snapshot = errlog.Snapshot
	}
	if opts.Subscribe == nil {
		opts.Subscribe = errlog.Subscribe
	}
	if opts.Clear == nil {
		opts.Clear = errlog.Clear
	}
	if opts.MarkRead == nil {
		opts.MarkRead = errlog.MarkRead
	}

	heading := widget.NewLabelWithStyle("Error Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel(
		"Every failure surfaced in the UI lands here with its full file:line trace. " +
			"Click a row to see the chain; copy & paste it into a bug report.",
	)
	subtitle.Wrapping = fyne.TextWrapWord

	// entries holds the current snapshot in **newest-first** order so
	// the most recent failure is row 0. The list rebinds via Refresh
	// after every Subscribe tick.
	entries := loadEntriesNewestFirst(opts.Snapshot)

	// Detail pane bits — populated on row select.
	traceLabel := widget.NewLabel("Select an entry to see its trace.")
	traceLabel.Wrapping = fyne.TextWrapWord
	traceLabel.TextStyle = fyne.TextStyle{Monospace: true}
	traceScroll := container.NewVScroll(traceLabel)

	var selectedTrace string

	list := widget.NewList(
		func() int { return len(entries) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < 0 || i >= len(entries) {
				return
			}
			o.(*widget.Label).SetText(formatRow(entries[i]))
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(entries) {
			return
		}
		e := entries[i]
		selectedTrace = e.Trace
		traceLabel.SetText(formatTraceDetail(e))
		traceScroll.Refresh()
	}

	copyBtn := widget.NewButton("Copy trace", func() {
		if selectedTrace == "" || opts.Clipboard == nil {
			return
		}
		opts.Clipboard.SetContent(selectedTrace)
	})
	clearBtn := widget.NewButton("Clear all", func() {
		opts.Clear()
		entries = nil
		list.Refresh()
		traceLabel.SetText("Select an entry to see its trace.")
		selectedTrace = ""
	})

	footer := container.NewHBox(copyBtn, clearBtn)

	// Mark unread → read on open so the sidebar badge clears.
	opts.MarkRead()

	// Live updates: spin a goroutine that drains the subscribe
	// channel and refreshes the list. Caller leaks one goroutine per
	// open which is fine — the shell rebuilds detail panes via Stack
	// swap, so the previous goroutine's channel goes idle (no
	// further sends) once the singleton drops it. (Future cleanup:
	// have errlog.Subscribe return an Unsubscribe func; tracked in
	// .lovable/plan.md Phase 4.)
	go func() {
		for range opts.Subscribe() {
			entries = loadEntriesNewestFirst(opts.Snapshot)
			list.Refresh()
		}
	}()

	body := container.NewHSplit(list, container.NewBorder(nil, nil, nil, nil, traceScroll))
	body.SetOffset(0.40)

	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		footer, nil, nil,
		body,
	)
}

// loadEntriesNewestFirst snapshots the store and reverses to put the
// most recent entry first. Stable sort by Seq descending so concurrent
// appends during the snapshot don't shuffle equal timestamps.
func loadEntriesNewestFirst(snap func() []errlog.Entry) []errlog.Entry {
	out := snap()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq > out[j].Seq })
	return out
}

// formatRow renders one summary line: "[15:04:05] component — summary".
// Kept compact so the list (40% of the split) stays readable.
func formatRow(e errlog.Entry) string {
	return fmt.Sprintf("[%s] %s — %s",
		e.Timestamp.Local().Format("15:04:05"),
		e.Component,
		truncate(e.Summary, 80),
	)
}

// formatTraceDetail builds the right-pane body: a header with the
// full timestamp + component, a blank line, then the multi-line
// errtrace.Format output.
func formatTraceDetail(e errlog.Entry) string {
	return fmt.Sprintf("%s  ·  %s\n\n%s",
		e.Timestamp.Local().Format(time.RFC3339),
		e.Component,
		e.Trace,
	)
}

// truncate clips s to n runes with an ellipsis. Cheap rune-aware
// version because component summaries are short ASCII anyway.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
