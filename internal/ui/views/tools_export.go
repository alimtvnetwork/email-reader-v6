// tools_export.go renders the Export CSV sub-tool tab inside Tools. It
// invokes core.Tools.ExportCsv with a phased progress channel and shows
// the resulting file path on completion.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (Export sub-tool).
//go:build !nofyne

package views

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/ui/errlog"
)

// BuildExportTab returns the Export CSV sub-tool body: a Run button + a
// streaming progress panel + a final result line with the output path.
//
// `factory` is the injected per-call `*core.Tools` builder; see
// `BuildOpenUrlTab` for the contract.
func BuildExportTab(factory ToolsFactory) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Export CSV — full Emails table",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Streams the full Emails table to ./data/export-<ts>.csv. Per-alias / date filtering ships in the next slice.")
	subtitle.Wrapping = fyne.TextWrapWord

	output := widget.NewMultiLineEntry()
	output.SetMinRowsVisible(12)
	output.Disable()
	status := widget.NewLabel("Click Run to export.")

	runBtn := widget.NewButton("Run", func() { runExportIntoUI(factory, output, status) })
	runBtn.Importance = widget.HighImportance

	header := container.NewVBox(heading, subtitle,
		container.NewHBox(runBtn), status, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(output))
}

// runExportIntoUI invokes core.Tools.ExportCsv and reflects progress + result.
func runExportIntoUI(factory ToolsFactory, output *widget.Entry, status *widget.Label) {
	output.SetText("")
	status.SetText("Counting rows…")
	tools, err := buildToolsFromFactory(factory)
	if err != nil {
		errlog.ReportError("tools.export.setup", err)
		status.SetText("⚠ setup: " + err.Error() + " — see Diagnostics → Error Log")
		return
	}
	progress := make(chan core.ExportProgress, 16)
	done := make(chan struct{})
	go drainExportProgress(progress, output, done)
	r := tools.ExportCsv(context.Background(), core.ExportSpec{}, progress)
	<-done
	finalizeExport(r, output, status)
}

func finalizeExport(r interface {
	HasError() bool
	Error() error
	Value() core.ExportReport
}, output *widget.Entry, status *widget.Label,
) {
	if r.HasError() {
		errlog.ReportError("tools.export", r.Error())
		status.SetText("⚠ " + r.Error().Error() + " — see Diagnostics → Error Log")
		appendOutput(output, "ERROR: "+r.Error().Error())
		return
	}
	rep := r.Value()
	status.SetText(fmt.Sprintf("✓ wrote %d row(s)", rep.RowCount))
	appendOutput(output, "")
	appendOutput(output, "output: "+rep.OutPath)
}

func drainExportProgress(ch <-chan core.ExportProgress, output *widget.Entry, done chan<- struct{}) {
	for ev := range ch {
		appendOutput(output, fmt.Sprintf("[%s] rows=%d total=%d",
			ev.Phase, ev.RowsWritten, ev.TotalRows))
	}
	close(done)
}
