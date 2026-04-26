// tools_doctor.go renders the Doctor sub-tool tab inside the Tools view.
// It calls core.Doctor (the same backend the CLI `doctor` command uses)
// and renders one expandable card per account showing the rune-dump of
// the stored password — the smoking-gun diagnostic for the
// hidden-character bug class (.lovable/solved-issues/06-…).
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (Doctor sub-tool).
//go:build !nofyne

package views

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// BuildDoctorTab returns the Doctor sub-tool body. The tab is a single
// vertical scroll: header strip with a [Run] button + status, followed
// by one card per account inserted on Run.
func BuildDoctorTab() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Doctor — password rune-dump",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Inspects every stored account password for hidden / invisible characters (whitespace, zero-width joiners, etc.) that would silently break IMAP login.")
	subtitle.Wrapping = fyne.TextWrapWord

	results := container.NewVBox()
	status := widget.NewLabel("Click Run to inspect.")
	runBtn := widget.NewButton("Run", func() { runDoctorIntoUI(results, status) })
	runBtn.Importance = widget.HighImportance

	header := container.NewVBox(heading, subtitle, container.NewHBox(runBtn, status), widget.NewSeparator())
	scroll := container.NewVScroll(results)
	return container.NewBorder(header, nil, nil, nil, scroll)
}

// runDoctorIntoUI invokes core.Doctor and rebuilds the results column.
// Errors surface in `status`; per-account decode errors render as a card.
func runDoctorIntoUI(results *fyne.Container, status *widget.Label) {
	results.Objects = nil
	r := core.Doctor("")
	if r.HasError() {
		status.SetText("⚠ " + r.Error().Error())
		results.Refresh()
		return
	}
	reps := r.Value()
	for _, rep := range reps {
		results.Add(buildDoctorCard(rep))
	}
	status.SetText(fmt.Sprintf("Inspected %d account(s).", len(reps)))
	results.Refresh()
}

// buildDoctorCard renders one DoctorReport as a Fyne Card with summary +
// rune-dump table. Hidden==true cards are visually flagged.
func buildDoctorCard(rep core.DoctorReport) fyne.CanvasObject {
	if rep.DecodeError != "" {
		return widget.NewCard(rep.Alias, rep.Email,
			widget.NewLabel("⚠ decode error: "+rep.DecodeError))
	}
	body := container.NewVBox(buildDoctorSummary(rep))
	body.Add(buildRuneDumpLabel("rune dump (sanitized):", rep.Sanitized))
	if rep.Hidden {
		body.Add(buildRuneDumpLabel("rune dump (raw, BEFORE sanitization):", rep.Raw))
	}
	title := rep.Alias
	if rep.Hidden {
		title = "⚠ " + rep.Alias + " (hidden chars detected)"
	}
	return widget.NewCard(title, rep.Email, body)
}

// buildDoctorSummary formats the "stored bytes / sanitized rune count"
// summary line for one report.
func buildDoctorSummary(rep core.DoctorReport) *widget.Label {
	txt := fmt.Sprintf("stored bytes: %d   |   sanitized rune count: %d",
		rep.StoredBytes, rep.RuneCount)
	if rep.Hidden {
		txt += "\n⚠ stored password contains hidden chars; sanitized version is what we send to IMAP."
	}
	l := widget.NewLabel(txt)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// buildRuneDumpLabel renders the rune table as a monospace-ish label so
// columns align. (TextGrid would be nicer; Label keeps this MVP simple.)
func buildRuneDumpLabel(title string, runes []core.DoctorRune) fyne.CanvasObject {
	header := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	body := widget.NewLabel(formatRuneDump(runes))
	body.TextStyle = fyne.TextStyle{Monospace: true}
	return container.NewVBox(header, body)
}

// formatRuneDump turns a []DoctorRune into the same `[ N] U+XXXX "g"`
// table the CLI prints. Single source of formatting.
func formatRuneDump(runes []core.DoctorRune) string {
	out := ""
	for _, r := range runes {
		out += fmt.Sprintf("  [%2d] U+%04X %q\n", r.Index, r.Code, r.Glyph)
	}
	if out == "" {
		return "  (empty)"
	}
	return out
}
