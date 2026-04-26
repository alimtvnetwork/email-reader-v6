// tools_diagnose.go renders the Diagnose sub-tool tab inside Tools. It
// calls core.Diagnose (the same backend the CLI `diagnose` command uses)
// and renders a streaming output area so the user sees each step land
// without needing to drop to a terminal.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (Diagnose sub-tool).
//go:build !nofyne

package views

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// BuildDiagnoseTab returns the Diagnose sub-tool body: an alias input
// (free-text — empty means "first configured account"), a Run button,
// and a scrollable text output that fills as core.Diagnose emits events.
func BuildDiagnoseTab() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Diagnose — IMAP probe",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Connects once and prints folder list, mailbox stats, recent headers, and a diagnosis. No emails are saved or opened.")
	subtitle.Wrapping = fyne.TextWrapWord

	aliasEntry := widget.NewEntry()
	aliasEntry.SetPlaceHolder("alias (empty = first configured account)")
	output := widget.NewMultiLineEntry()
	output.SetMinRowsVisible(18)
	output.Disable()
	status := widget.NewLabel("Click Run to connect.")

	runBtn := widget.NewButton("Run", func() { runDiagnoseIntoUI(aliasEntry.Text, output, status) })
	runBtn.Importance = widget.HighImportance

	header := container.NewVBox(heading, subtitle,
		container.NewBorder(nil, nil, widget.NewLabel("Alias:"), runBtn, aliasEntry),
		status, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(output))
}

// runDiagnoseIntoUI invokes core.Diagnose and appends each event into
// the output entry. Errors land in `status` AND in the output trail.
func runDiagnoseIntoUI(alias string, output *widget.Entry, status *widget.Label) {
	output.SetText("")
	status.SetText("Connecting…")
	step := 0
	res := core.Diagnose(alias, func(ev core.DiagnoseEvent) {
		appendDiagnoseEvent(output, ev, &step)
	})
	if res.HasError() {
		status.SetText("⚠ " + res.Error().Error())
		appendOutput(output, "ERROR: "+res.Error().Error())
		return
	}
	status.SetText("Done.")
}

// appendDiagnoseEvent renders one DiagnoseEvent into the output entry.
// Mirrors the CLI's step-numbered format from cli.runDiagnose so users
// see identical output regardless of surface.
func appendDiagnoseEvent(output *widget.Entry, ev core.DiagnoseEvent, step *int) {
	switch ev.Kind {
	case core.DiagnoseEventStart:
		a := ev.Account
		appendOutput(output, fmt.Sprintf("Alias: %s\nAccount: %s\nServer: %s:%d tls=%v mailbox=%q",
			a.Alias, a.Email, a.ImapHost, a.ImapPort, a.UseTLS, a.Mailbox))
		*step = 1
		appendOutput(output, fmt.Sprintf("\n%d) Connecting and logging in...", *step))
	case core.DiagnoseEventLoginOK:
		appendOutput(output, "   OK: login succeeded")
		appendDiagnoseStepHeader(output, step, "Listing server folders...")
	case core.DiagnoseEventFolders:
		appendDiagnoseFolders(output, ev)
		appendDiagnoseStepHeader(output, step, "Selecting configured mailbox...")
	case core.DiagnoseEventInbox:
		s := ev.Stats
		appendOutput(output, fmt.Sprintf("   OK: %q messages=%d unseen=%d uidNext=%d",
			s.Name, s.Messages, s.Unseen, s.UidNext))
	case core.DiagnoseEventHeaders:
		appendDiagnoseHeaders(output, ev)
	case core.DiagnoseEventSummary:
		appendOutput(output, "\nDiagnosis:\n   "+ev.Message)
	}
}

// appendDiagnoseStepHeader bumps the step counter and appends the next
// numbered header line. Extracted so appendDiagnoseEvent stays ≤15 stmts.
func appendDiagnoseStepHeader(output *widget.Entry, step *int, title string) {
	*step++
	appendOutput(output, fmt.Sprintf("\n%d) %s", *step, title))
}

// appendDiagnoseFolders renders the list-folders event body.
func appendDiagnoseFolders(output *widget.Entry, ev core.DiagnoseEvent) {
	for _, f := range ev.Folders {
		appendOutput(output, fmt.Sprintf("   - %s %v", f.Name, f.Attributes))
	}
}

// appendDiagnoseHeaders renders the recent-headers event body.
func appendDiagnoseHeaders(output *widget.Entry, ev core.DiagnoseEvent) {
	if len(ev.Headers) == 0 {
		appendOutput(output, "   No messages returned by server for this mailbox.")
		return
	}
	for _, h := range ev.Headers {
		when := "unknown-time"
		if !h.ReceivedAt.IsZero() {
			when = h.ReceivedAt.Format("2006-01-02 15:04:05 MST")
		}
		appendOutput(output, fmt.Sprintf("   UID=%d at=%s from=%q subject=%q",
			h.Uid, when, h.From, h.Subject))
	}
}

// appendOutput tacks a line onto the multiline entry plus a newline.
func appendOutput(output *widget.Entry, line string) {
	output.SetText(output.Text + line + "\n")
}
