// add_account_form.go renders the Add Account inline form: alias / email /
// password / host (with Autodiscover) / port / TLS / mailbox. On submit it
// validates via ValidateAccountForm (pure Go, tested in account_form_test)
// then calls core.AddAccount. Errors render inline; success shows a
// status banner with the saved alias and config path.
//
// Behind the !nofyne build tag because it imports the Fyne widget set.
//go:build !nofyne

package views

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// AddAccountFormOptions wires the form to its side effects. Save defaults
// to core.AddAccount; tests inject a stub. OnSaved is called after a
// successful save so the shell can refresh the sidebar account picker.
type AddAccountFormOptions struct {
	Save    func(in core.AccountInput) (*core.AddAccountResult, error)
	OnSaved func()
}

// accountFormEntries holds the seven Fyne widgets that make up the Add
// Account form. Grouping them keeps BuildAddAccountForm under the
// 15-statement limit (AC-PROJ-20) by hiding a long sequence of NewEntry /
// NewCheck calls behind one helper.
type accountFormEntries struct {
	alias    *widget.Entry
	email    *widget.Entry
	password *widget.Entry
	host     *widget.Entry
	port     *widget.Entry
	useTLS   *widget.Check
	mailbox  *widget.Entry
}

// BuildAddAccountForm returns the inline Add Account form widget.
func BuildAddAccountForm(opts AddAccountFormOptions) fyne.CanvasObject {
	if opts.Save == nil {
		opts.Save = core.AddAccount
	}
	e := newAccountFormEntries()
	status := newStatusLabel()
	autodiscover := newAutodiscoverButton(e, status)
	form := buildAccountForm(e, autodiscover)
	clear := func() { resetAccountEntries(e) }
	submit := newAccountSubmitButton(opts, e, status, clear)
	clearBtn := widget.NewButton("Clear", func() { clear(); status.SetText("") })
	actions := container.NewHBox(submit, clearBtn)
	return container.NewPadded(container.NewVBox(form, widget.NewSeparator(), actions, status))
}

// newAccountFormEntries constructs the seven entry widgets with their
// placeholders. Returning a struct keeps BuildAddAccountForm flat.
func newAccountFormEntries() *accountFormEntries {
	alias := widget.NewEntry()
	alias.SetPlaceHolder("e.g. work-gmail")
	email := widget.NewEntry()
	email.SetPlaceHolder("you@example.com")
	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("IMAP password or app-specific token")
	host := widget.NewEntry()
	host.SetPlaceHolder("auto-detect from email — leave blank to use imapdef")
	port := widget.NewEntry()
	port.SetPlaceHolder("993")
	useTLS := widget.NewCheck("Use TLS (recommended)", nil)
	useTLS.SetChecked(true)
	mailbox := widget.NewEntry()
	mailbox.SetPlaceHolder("INBOX")
	return &accountFormEntries{alias, email, password, host, port, useTLS, mailbox}
}

// newStatusLabel returns a wrap-on-word label used to show inline form
// status messages (errors, success banners).
func newStatusLabel() *widget.Label {
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	return status
}

// newAutodiscoverButton wires the Autodiscover button to SuggestServer
// and pushes the suggested host/port/TLS into the matching entries.
func newAutodiscoverButton(e *accountFormEntries, status *widget.Label) *widget.Button {
	return widget.NewButton("Autodiscover", func() {
		s := SuggestServer(e.email.Text)
		if s.Host == "" {
			status.SetText("⚠ Enter an email first to autodiscover.")
			return
		}
		e.host.SetText(s.Host)
		e.port.SetText(strconv.Itoa(s.Port))
		e.useTLS.SetChecked(s.UseTLS)
		status.SetText(fmt.Sprintf("Suggested %s:%d (TLS=%v).", s.Host, s.Port, s.UseTLS))
	})
}

// buildAccountForm composes the widget.Form rows from the seven entries.
func buildAccountForm(e *accountFormEntries, autodiscover *widget.Button) *widget.Form {
	return widget.NewForm(
		widget.NewFormItem("Alias", e.alias),
		widget.NewFormItem("Email", e.email),
		widget.NewFormItem("Password", e.password),
		widget.NewFormItem("IMAP host", container.NewBorder(nil, nil, nil, autodiscover, e.host)),
		widget.NewFormItem("Port", e.port),
		widget.NewFormItem("TLS", e.useTLS),
		widget.NewFormItem("Mailbox", e.mailbox),
	)
}

// resetAccountEntries clears every input back to its initial state.
func resetAccountEntries(e *accountFormEntries) {
	e.alias.SetText("")
	e.email.SetText("")
	e.password.SetText("")
	e.host.SetText("")
	e.port.SetText("")
	e.useTLS.SetChecked(true)
	e.mailbox.SetText("")
}

// newAccountSubmitButton wires the primary "Save account" button: validate
// → call opts.Save → render status → run OnSaved hook on success.
func newAccountSubmitButton(opts AddAccountFormOptions, e *accountFormEntries, status *widget.Label, clear func()) *widget.Button {
	submit := widget.NewButton("Save account", func() {
		v := ValidateAccountForm(accountFormInputFromEntries(e))
		if !v.Valid {
			status.SetText("⚠ " + strings.Join(v.Errors, " · "))
			return
		}
		res, err := opts.Save(accountInputFromValid(v))
		if err != nil {
			status.SetText("⚠ Save failed: " + err.Error())
			return
		}
		status.SetText(formatAccountSavedMessage(res))
		clear()
		if opts.OnSaved != nil {
			opts.OnSaved()
		}
	})
	submit.Importance = widget.HighImportance
	return submit
}

// accountFormInputFromEntries pulls the current entry values into the
// validation input struct.
func accountFormInputFromEntries(e *accountFormEntries) AccountFormInput {
	return AccountFormInput{
		Alias:    e.alias.Text,
		Email:    e.email.Text,
		Password: e.password.Text,
		Host:     e.host.Text,
		Port:     e.port.Text,
		UseTLS:   e.useTLS.Checked,
		Mailbox:  e.mailbox.Text,
	}
}

// accountInputFromValid maps a validated form into the core.AccountInput
// the Save side effect expects.
func accountInputFromValid(v AccountFormValidation) core.AccountInput {
	return core.AccountInput{
		Alias:          v.Alias,
		Email:          v.Email,
		PlainPassword:  v.Password,
		ImapHost:       v.Host,
		ImapPort:       v.Port,
		UseTLS:         v.UseTLS,
		UseTLSExplicit: true,
		Mailbox:        v.Mailbox,
	}
}

// formatAccountSavedMessage renders the success banner including the
// HiddenCharsRem count when SanitizePassword stripped anything.
func formatAccountSavedMessage(res *core.AddAccountResult) string {
	msg := fmt.Sprintf("✓ Saved %q → %s:%d (TLS=%v). Config: %s",
		res.Account.Alias, res.Account.ImapHost, res.Account.ImapPort,
		res.Account.UseTLS, res.ConfigPath)
	if res.HiddenCharsRem > 0 {
		msg += fmt.Sprintf(" · Stripped %d hidden char(s) from password.", res.HiddenCharsRem)
	}
	return msg
}
