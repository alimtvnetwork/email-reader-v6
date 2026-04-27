// tools_read.go renders the ReadOnce sub-tool tab inside Tools. It
// invokes core.Tools.ReadOnce with a streaming progress channel and
// renders header rows in a scrollable list.
//
// **Slice #116c (Phase 6.3) refactor.** The per-call `*core.Tools`
// constructor (`buildReadTools`) used to live here and call
// `config.Load()` inline. It is now sourced from the injected
// `ToolsFactory` (shared with OpenUrl / Export / Recent opens via
// the shell's `*Services.Tools` field) — see `tools_openurl.go`
// for the shared `buildToolsFromFactory` helper.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (Read sub-tool).
//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// BuildReadTab returns the Read sub-tool body: alias + limit inputs,
// Run button, and a streaming output panel for progress + headers.
func BuildReadTab() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Read — one-shot fetch",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := newReadSubtitle()
	aliasEntry, limitEntry := newReadInputs()
	output, status := newReadOutputs()
	runBtn := widget.NewButton("Run", func() {
		runReadIntoUI(aliasEntry.Text, limitEntry.Text, output, status)
	})
	runBtn.Importance = widget.HighImportance
	form := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("Alias:"), nil, aliasEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Limit:"), runBtn, limitEntry))
	header := container.NewVBox(heading, subtitle, form, status, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(output))
}

func newReadSubtitle() *widget.Label {
	l := widget.NewLabel("Connects once, fetches the most recent N headers, and returns. Nothing is saved or marked seen.")
	l.Wrapping = fyne.TextWrapWord
	return l
}

func newReadInputs() (*widget.Entry, *widget.Entry) {
	a := widget.NewEntry()
	a.SetPlaceHolder("alias (empty = first configured account)")
	l := widget.NewEntry()
	l.SetPlaceHolder("10")
	return a, l
}

func newReadOutputs() (*widget.Entry, *widget.Label) {
	o := widget.NewMultiLineEntry()
	o.SetMinRowsVisible(18)
	o.Disable()
	return o, widget.NewLabel("Click Run to fetch.")
}

// runReadIntoUI invokes core.Tools.ReadOnce and reflects progress + result.
func runReadIntoUI(alias, limitStr string, output *widget.Entry, status *widget.Label) {
	output.SetText("")
	status.SetText("Connecting…")
	tools, err := buildReadTools()
	if err != nil {
		status.SetText("⚠ setup: " + err.Error())
		return
	}
	limit := parseLimit(limitStr)
	progress := make(chan string, 16)
	done := make(chan struct{})
	go drainProgress(progress, output, done)
	r := tools.ReadOnce(context.Background(), core.ReadSpec{Alias: alias, Limit: limit}, progress)
	<-done
	finalizeRead(r, output, status)
}

func finalizeRead(r interface {
	HasError() bool
	Error() error
	Value() core.ReadResult
}, output *widget.Entry, status *widget.Label,
) {
	if r.HasError() {
		status.SetText("⚠ " + r.Error().Error())
		appendOutput(output, "ERROR: "+r.Error().Error())
		return
	}
	res := r.Value()
	status.SetText(fmt.Sprintf("✓ %d header(s) in %s", len(res.Headers), res.Duration.Round(1e6)))
	appendOutput(output, "")
	for _, h := range res.Headers {
		appendOutput(output, fmt.Sprintf("  uid=%d  %s  %s",
			h.Uid, h.ReceivedAt.Format("2006-01-02 15:04"), h.Subject))
	}
}

func drainProgress(ch <-chan string, output *widget.Entry, done chan<- struct{}) {
	for line := range ch {
		appendOutput(output, line)
	}
	close(done)
}

func parseLimit(s string) int {
	if s == "" {
		return 0 // → core defaults to 10
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 0
}

func buildReadTools() (*core.Tools, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	r := core.NewTools(browser.New(cfg.Browser), noopOpenedUrlStore{}, core.DefaultToolsConfig())
	if r.HasError() {
		return nil, r.Error()
	}
	return r.Value(), nil
}
