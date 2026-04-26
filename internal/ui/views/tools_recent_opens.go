// tools_recent_opens.go renders the "Recent opens" sub-tool tab inside
// Tools. It is the first production caller of `Tools.RecentOpenedUrls`
// and activates the Delta #1 Alias / Origin filters end-to-end:
// dropdown + alias entry → core.OpenedUrlListSpec → SQL WHERE clauses
// added by `buildOpenedUrlsQuery`.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (Recent opens).
//go:build !nofyne

package views

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// BuildRecentOpensTab returns the Recent-opens body: alias entry +
// origin dropdown + limit entry + Refresh button + scrolling result.
func BuildRecentOpensTab() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Recent opens — audit trail",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Read-only view over the OpenedUrls table. Filter by alias and origin (Delta #1).")
	subtitle.Wrapping = fyne.TextWrapWord

	aliasEntry := widget.NewEntry()
	aliasEntry.SetPlaceHolder("alias (empty = all)")
	originSelect := widget.NewSelect(OriginChoices, nil)
	originSelect.SetSelected("All")
	limitEntry := widget.NewEntry()
	limitEntry.SetPlaceHolder("limit (1..1000, default 100)")

	output := widget.NewMultiLineEntry()
	output.SetMinRowsVisible(18)
	output.Disable()
	status := widget.NewLabel("Click Refresh to query the audit table.")

	refreshBtn := widget.NewButton("Refresh", func() {
		runRecentOpensIntoUI(RecentOpensFilter{
			Alias:    aliasEntry.Text,
			Origin:   originSelect.Selected,
			LimitStr: limitEntry.Text,
		}, output, status)
	})
	refreshBtn.Importance = widget.HighImportance

	form := container.NewGridWithColumns(3,
		container.NewBorder(nil, nil, widget.NewLabel("Alias:"), nil, aliasEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Origin:"), nil, originSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Limit:"), refreshBtn, limitEntry),
	)
	header := container.NewVBox(heading, subtitle, form, status, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(output))
}

// runRecentOpensIntoUI builds the spec, calls core.Tools.RecentOpenedUrls,
// and renders the result lines + summary into the output entry.
func runRecentOpensIntoUI(f RecentOpensFilter, output *widget.Entry, status *widget.Label) {
	output.SetText("")
	status.SetText("Querying…")
	tools, err := buildReadTools()
	if err != nil {
		status.SetText("⚠ " + err.Error())
		return
	}
	spec := BuildRecentOpensSpec(f)
	start := time.Now()
	res := tools.RecentOpenedUrls(context.Background(), spec)
	elapsed := time.Since(start)
	if res.HasError() {
		status.SetText("⚠ " + res.Error().Error())
		appendOutput(output, "ERROR: "+res.Error().Error())
		return
	}
	rows := res.Value()
	for _, r := range rows {
		appendOutput(output, FormatRecentOpensRow(r))
	}
	if len(rows) == 0 {
		appendOutput(output, "(no rows match)")
	}
	status.SetText(FormatRecentOpensSummary(spec, len(rows), elapsed))
}
