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

// BuildAddAccountForm returns the inline Add Account form widget.
func BuildAddAccountForm(opts AddAccountFormOptions) fyne.CanvasObject {
	if opts.Save == nil {
		opts.Save = core.AddAccount
	}

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

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	autodiscover := widget.NewButton("Autodiscover", func() {
		s := SuggestServer(email.Text)
		if s.Host == "" {
			status.SetText("⚠ Enter an email first to autodiscover.")
			return
		}
		host.SetText(s.Host)
		port.SetText(strconv.Itoa(s.Port))
		useTLS.SetChecked(s.UseTLS)
		status.SetText(fmt.Sprintf("Suggested %s:%d (TLS=%v).", s.Host, s.Port, s.UseTLS))
	})

	form := widget.NewForm(
		widget.NewFormItem("Alias", alias),
		widget.NewFormItem("Email", email),
		widget.NewFormItem("Password", password),
		widget.NewFormItem("IMAP host", container.NewBorder(nil, nil, nil, autodiscover, host)),
		widget.NewFormItem("Port", port),
		widget.NewFormItem("TLS", useTLS),
		widget.NewFormItem("Mailbox", mailbox),
	)

	clear := func() {
		alias.SetText("")
		email.SetText("")
		password.SetText("")
		host.SetText("")
		port.SetText("")
		useTLS.SetChecked(true)
		mailbox.SetText("")
	}

	submit := widget.NewButton("Save account", func() {
		v := ValidateAccountForm(AccountFormInput{
			Alias:    alias.Text,
			Email:    email.Text,
			Password: password.Text,
			Host:     host.Text,
			Port:     port.Text,
			UseTLS:   useTLS.Checked,
			Mailbox:  mailbox.Text,
		})
		if !v.Valid {
			status.SetText("⚠ " + strings.Join(v.Errors, " · "))
			return
		}
		res, err := opts.Save(core.AccountInput{
			Alias:          v.Alias,
			Email:          v.Email,
			PlainPassword:  v.Password,
			ImapHost:       v.Host,
			ImapPort:       v.Port,
			UseTLS:         v.UseTLS,
			UseTLSExplicit: true,
			Mailbox:        v.Mailbox,
		})
		if err != nil {
			status.SetText("⚠ Save failed: " + err.Error())
			return
		}
		msg := fmt.Sprintf("✓ Saved %q → %s:%d (TLS=%v). Config: %s",
			res.Account.Alias, res.Account.ImapHost, res.Account.ImapPort,
			res.Account.UseTLS, res.ConfigPath)
		if res.HiddenCharsRem > 0 {
			msg += fmt.Sprintf(" · Stripped %d hidden char(s) from password.", res.HiddenCharsRem)
		}
		status.SetText(msg)
		clear()
		if opts.OnSaved != nil {
			opts.OnSaved()
		}
	})
	submit.Importance = widget.HighImportance

	clearBtn := widget.NewButton("Clear", func() {
		clear()
		status.SetText("")
	})

	actions := container.NewHBox(submit, clearBtn)

	return container.NewPadded(container.NewVBox(
		form,
		widget.NewSeparator(),
		actions,
		status,
	))
}
