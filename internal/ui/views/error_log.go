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
	// LogPath is the on-disk path of the persisted error log
	// (`<dataDir>/error-log.jsonl`). When empty, the "Open log file"
	// button is hidden — keeps the headless test path tidy and the
	// view degraded-but-functional when persistence failed at boot.
	LogPath string
	// OpenPath, when non-nil, is invoked with LogPath when the user
	// clicks "Open log file". Production wiring builds a closure
	// around `fyne.CurrentApp().OpenURL(file://…)` so the OS default
	// handler picks up the .jsonl file. Tests substitute a recorder
	// so we can assert the button calls through with the right path.
	OpenPath func(path string) error
}

// BuildErrorLog returns the Error Log detail pane: a header, a split
// view (summary list ↔ trace detail), and a footer with Clear /
// Copy actions. Calling BuildErrorLog also marks the unread counter
// as read so the sidebar badge clears the moment the user opens it.
func BuildErrorLog(opts ErrorLogOptions) fyne.CanvasObject {
	opts = applyErrorLogDefaults(opts)
	header := buildErrorLogHeader()
	entries := loadEntriesNewestFirst(opts.Snapshot)
	traceLabel, traceScroll := newErrorLogTracePane()
	selected := &errorLogSelection{}
	list := newErrorLogList(&entries, traceLabel, traceScroll, selected)
	footer := newErrorLogFooter(opts, &entries, list, traceLabel, selected)
	opts.MarkRead() // sidebar badge clears on open
	startErrorLogRefresher(opts, &entries, list)
	body := container.NewHSplit(list, container.NewBorder(nil, nil, nil, nil, traceScroll))
	body.SetOffset(0.40)
	return container.NewBorder(header, footer, nil, nil, body)
}

// errorLogSelection is the mutable holder for the currently-selected
// trace string. Lives behind a pointer so the list-select closure and
// the Copy/Clear button closures share one source of truth without
// closing over a bare local that BuildErrorLog can't easily pass to
// extracted helpers.
type errorLogSelection struct {
	trace string
}

// applyErrorLogDefaults binds the package-level errlog singleton into
// any nil seam — production callers leave them nil; tests pre-populate
// to inject deterministic data. Extracted from BuildErrorLog to keep it
// under the 15-statement linter budget (AC-PROJ-20).
func applyErrorLogDefaults(opts ErrorLogOptions) ErrorLogOptions {
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
	return opts
}

// buildErrorLogHeader composes the heading + word-wrapped subtitle +
// separator that sits above the split body.
func buildErrorLogHeader() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Error Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel(
		"Every failure surfaced in the UI lands here with its full file:line trace. " +
			"Click a row to see the chain; copy & paste it into a bug report.",
	)
	subtitle.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, subtitle, widget.NewSeparator())
}

// newErrorLogTracePane returns the monospaced trace label wrapped in a
// vertical scroller — the right-hand pane of the split body.
func newErrorLogTracePane() (*widget.Label, *container.Scroll) {
	traceLabel := widget.NewLabel("Select an entry to see its trace.")
	traceLabel.Wrapping = fyne.TextWrapWord
	traceLabel.TextStyle = fyne.TextStyle{Monospace: true}
	traceScroll := container.NewVScroll(traceLabel)
	return traceLabel, traceScroll
}

// newErrorLogList builds the virtualised entries list and wires the
// row-select handler so picking a row repaints the trace pane.
func newErrorLogList(
	entries *[]errlog.Entry,
	traceLabel *widget.Label,
	traceScroll *container.Scroll,
	selected *errorLogSelection,
) *widget.List {
	list := widget.NewList(
		func() int { return len(*entries) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < 0 || i >= len(*entries) {
				return
			}
			o.(*widget.Label).SetText(formatRow((*entries)[i]))
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(*entries) {
			return
		}
		e := (*entries)[i]
		selected.trace = e.Trace
		traceLabel.SetText(formatTraceDetail(e))
		traceScroll.Refresh()
	}
	return list
}

// newErrorLogFooter composes the Copy / Clear / Open-log-file buttons
// plus the inline status label. The "Open log file" button is disabled
// when persistence is unavailable (boot fallback).
func newErrorLogFooter(
	opts ErrorLogOptions,
	entries *[]errlog.Entry,
	list *widget.List,
	traceLabel *widget.Label,
	selected *errorLogSelection,
) fyne.CanvasObject {
	copyBtn := widget.NewButton("Copy trace", func() {
		if selected.trace == "" || opts.Clipboard == nil {
			return
		}
		opts.Clipboard.SetContent(selected.trace)
	})
	clearBtn := widget.NewButton("Clear all", func() {
		opts.Clear()
		*entries = nil
		list.Refresh()
		traceLabel.SetText("Select an entry to see its trace.")
		selected.trace = ""
	})
	openBtn, openStatus := newOpenLogButton(opts)
	return container.NewHBox(copyBtn, clearBtn, openBtn, openStatus)
}

// newOpenLogButton builds the "Open log file" button + its sibling
// status label. Disabled with a "Disk log unavailable." caption when
// LogPath is empty (errlog persistence failed at boot).
func newOpenLogButton(opts ErrorLogOptions) (*widget.Button, *widget.Label) {
	openStatus := widget.NewLabel("")
	openStatus.Importance = widget.LowImportance
	openBtn := widget.NewButton("Open log file", func() {
		msg := openLogFile(opts.LogPath, opts.OpenPath)
		openStatus.SetText(msg)
	})
	if opts.LogPath == "" {
		openBtn.Disable()
		openStatus.SetText("Disk log unavailable.")
	}
	return openBtn, openStatus
}

// startErrorLogRefresher spins the goroutine that drains the subscribe
// channel and reloads the snapshot on every tick. Caller leaks one
// goroutine per open which is fine — the shell rebuilds detail panes
// via Stack swap, so the previous goroutine's channel goes idle once
// the singleton drops it. (Future cleanup: have errlog.Subscribe
// return an Unsubscribe func; tracked in `.lovable/plan.md` Phase 4.)
func startErrorLogRefresher(opts ErrorLogOptions, entries *[]errlog.Entry, list *widget.List) {
	go func() {
		for range opts.Subscribe() {
			*entries = loadEntriesNewestFirst(opts.Snapshot)
			list.Refresh()
		}
	}()
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

// openLogFile invokes opener with path and returns a short status
// string suitable for a single-line label. Pulled out as a pure
// helper so the headless test path can assert behavior without a
// Fyne app. nil opener is treated as "not wired" — matches how the
// Copy button degrades when Clipboard is nil.
func openLogFile(path string, opener func(string) error) string {
	if path == "" {
		return "Disk log unavailable."
	}
	if opener == nil {
		return "Open handler not wired."
	}
	if err := opener(path); err != nil {
		return "Open failed: " + err.Error()
	}
	return "Opened " + path
}
