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

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// AddAccountFormOptions wires the form to its side effects. Save defaults
// to core.AddAccount and TestConn defaults to core.TestAccountConnection;
// tests inject stubs. OnSaved is called after a successful save so the
// shell can refresh the sidebar account picker.
//
// Edit mode: when Initial is non-nil the form starts pre-filled with that
// account, the Alias entry is locked (alias is the immutable key), the
// password placeholder explains "leave blank to keep current", and Save
// defaults to core.UpdateAccount instead of core.AddAccount. Callers
// using edit mode typically also override OnSaved to close their dialog.
type AddAccountFormOptions struct {
	Save     func(in core.AccountInput) errtrace.Result[*core.AddAccountResult]
	TestConn func(in core.AccountInput) errtrace.Result[core.TestConnectionResult]
	OnSaved  func()
	Initial  *config.Account // nil ⇒ Add mode; non-nil ⇒ Edit mode
}

// accountFormEntries holds the Fyne widgets that make up the Add Account
// form. Grouping them keeps BuildAddAccountForm under the 15-statement
// limit (AC-PROJ-20) by hiding a long sequence of NewEntry / NewCheck
// calls behind one helper.
type accountFormEntries struct {
	provider    *widget.Select
	alias       *widget.Entry
	email       *widget.Entry
	displayName *widget.Entry
	password    *widget.Entry
	host        *widget.Entry
	port        *widget.Entry
	useTLS      *widget.Check
	mailbox     *widget.Entry
}

// BuildAddAccountForm returns the inline Add Account form widget.
// In Edit mode (opts.Initial != nil) the same widget is used but
// pre-filled and bound to UpdateAccount.
func BuildAddAccountForm(opts AddAccountFormOptions) fyne.CanvasObject {
	editing := opts.Initial != nil
	opts = applyFormDefaults(opts, editing)
	e := newAccountFormEntries()
	status := newStatusLabel()
	autodiscover := newAutodiscoverButton(e, status)
	revealPw := newPasswordRevealButton(e.password)
	form := buildAccountForm(e, autodiscover, revealPw)
	if editing {
		applyInitialToEntries(*opts.Initial, e)
	}
	clear := func() {
		resetAccountEntries(e)
		revealPw.SetText("Show")
		e.password.Password = true
		e.password.Refresh()
		if editing {
			applyInitialToEntries(*opts.Initial, e)
		}
	}
	submit := newAccountSubmitButton(opts, e, status, clear, editing)
	testBtn := newTestConnectionButton(opts, e, status)
	clearBtn := widget.NewButton(clearLabel(editing), func() { clear(); status.SetText("") })
	actions := container.NewHBox(submit, testBtn, clearBtn)
	return container.NewPadded(container.NewVBox(form, widget.NewSeparator(), actions, status))
}

// applyFormDefaults fills in Save/TestConn defaults. In edit mode Save
// routes to UpdateAccount; in add mode it routes to AddAccount.
func applyFormDefaults(opts AddAccountFormOptions, editing bool) AddAccountFormOptions {
	if opts.Save == nil {
		if editing {
			opts.Save = core.UpdateAccount
		} else {
			opts.Save = core.AddAccount
		}
	}
	if opts.TestConn == nil {
		opts.TestConn = func(in core.AccountInput) errtrace.Result[core.TestConnectionResult] {
			return core.TestAccountConnection(in, 0)
		}
	}
	return opts
}

// applyInitialToEntries pre-fills the form widgets from an existing
// account. Locks the Alias entry (alias is the immutable key) and
// updates the password placeholder so the user knows blank == keep.
func applyInitialToEntries(a config.Account, e *accountFormEntries) {
	e.alias.SetText(a.Alias)
	e.alias.Disable()
	e.email.SetText(a.Email)
	e.displayName.SetText(a.DisplayName)
	e.password.SetText("")
	e.password.SetPlaceHolder("(leave blank to keep current password)")
	e.host.SetText(a.ImapHost)
	if a.ImapPort > 0 {
		e.port.SetText(strconv.Itoa(a.ImapPort))
	}
	e.useTLS.SetChecked(a.UseTLS)
	e.mailbox.SetText(a.Mailbox)
}

// clearLabel returns the button label appropriate for the form mode.
// "Reset" reads better than "Clear" when editing because it implies
// reverting to the loaded account, not blanking out fields.
func clearLabel(editing bool) string {
	if editing {
		return "Reset"
	}
	return "Clear"
}

// newAccountFormEntries constructs the entry widgets with their
// placeholders. Returning a struct keeps BuildAddAccountForm flat.
// The Provider Select is populated from PresetLabels() and starts on the
// "Custom (manual)" sentinel so existing keyboard-only users see no
// behaviour change until they pick a preset.
func newAccountFormEntries() *accountFormEntries {
	provider := widget.NewSelect(PresetLabels(), nil)
	provider.SetSelected(PresetLabels()[0])
	alias := widget.NewEntry()
	alias.SetPlaceHolder("e.g. work-gmail")
	email := widget.NewEntry()
	email.SetPlaceHolder("you@example.com")
	displayName := widget.NewEntry()
	displayName.SetPlaceHolder("optional — e.g. Work — Sales inbox")
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
	e := &accountFormEntries{provider, alias, email, displayName, password, host, port, useTLS, mailbox}
	provider.OnChanged = func(label string) { applyPresetToEntries(label, e) }
	return e
}

// applyPresetToEntries pushes a preset's host/port/TLS into the matching
// entries. The "Custom (manual)" sentinel is a no-op so users keep
// whatever they typed. Pure-ish helper (touches widgets only) — the
// preset table itself is tested in account_presets_test.go.
func applyPresetToEntries(label string, e *accountFormEntries) {
	p, ok := FindPreset(label)
	if !ok || IsCustomPreset(p) {
		return
	}
	e.host.SetText(p.Host)
	e.port.SetText(strconv.Itoa(p.Port))
	e.useTLS.SetChecked(p.UseTLS)
}

// newPasswordRevealButton toggles the password Entry between hidden
// (default) and visible. Re-creating the underlying Entry would lose the
// typed value, so we mutate Password in place and refresh.
func newPasswordRevealButton(pw *widget.Entry) *widget.Button {
	btn := widget.NewButton("Show", nil)
	btn.OnTapped = func() {
		pw.Password = !pw.Password
		if pw.Password {
			btn.SetText("Show")
		} else {
			btn.SetText("Hide")
		}
		pw.Refresh()
	}
	return btn
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

// buildAccountForm composes the widget.Form rows from the entries. The
// password row gets a trailing "Show/Hide" button so users can verify
// what they typed without committing it to clipboard. The Provider row
// goes first so picking a preset auto-fills host/port/TLS before the
// user reaches them.
func buildAccountForm(e *accountFormEntries, autodiscover, revealPw *widget.Button) *widget.Form {
	return widget.NewForm(
		widget.NewFormItem("Provider", e.provider),
		widget.NewFormItem("Alias", e.alias),
		widget.NewFormItem("Display name", e.displayName),
		widget.NewFormItem("Email", e.email),
		widget.NewFormItem("Password", container.NewBorder(nil, nil, nil, revealPw, e.password)),
		widget.NewFormItem("IMAP host", container.NewBorder(nil, nil, nil, autodiscover, e.host)),
		widget.NewFormItem("Port", e.port),
		widget.NewFormItem("TLS", e.useTLS),
		widget.NewFormItem("Mailbox", e.mailbox),
	)
}

// resetAccountEntries clears every input back to its initial state.
func resetAccountEntries(e *accountFormEntries) {
	e.provider.SetSelected(PresetLabels()[0])
	e.alias.SetText("")
	e.email.SetText("")
	e.displayName.SetText("")
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
		r := opts.Save(accountInputFromValid(v))
		if r.HasError() {
			status.SetText("⚠ Save failed: " + r.Error().Error())
			return
		}
		status.SetText(formatAccountSavedMessage(r.Value()))
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
		Alias:       e.alias.Text,
		Email:       e.email.Text,
		DisplayName: e.displayName.Text,
		Password:    e.password.Text,
		Host:        e.host.Text,
		Port:        e.port.Text,
		UseTLS:      e.useTLS.Checked,
		Mailbox:     e.mailbox.Text,
	}
}

// accountInputFromValid maps a validated form into the core.AccountInput
// the Save side effect expects.
func accountInputFromValid(v AccountFormResult) core.AccountInput {
	return core.AccountInput{
		Alias:          v.Alias,
		Email:          v.Email,
		DisplayName:    v.DisplayName,
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

// newTestConnectionButton wires the "Test connection" button: validate
// → call opts.TestConn → render status. Does NOT call Save and does NOT
// run OnSaved — this is purely a probe so the user can verify
// credentials before committing them.
//
// Disables itself for the duration of the probe so a hung server can't
// be re-triggered. The status banner reports the resolved endpoint on
// success ("Connected to imap.gmail.com:993 (TLS)") so the user knows
// exactly what was tested.
func newTestConnectionButton(opts AddAccountFormOptions, e *accountFormEntries, status *widget.Label) *widget.Button {
	btn := widget.NewButton("Test connection", nil)
	btn.OnTapped = func() {
		v := ValidateAccountForm(accountFormInputFromEntries(e))
		if !v.Valid {
			status.SetText("⚠ " + strings.Join(v.Errors, " · "))
			return
		}
		btn.Disable()
		status.SetText("⏳ Testing connection…")
		go runTestConnection(opts, accountInputFromValid(v), status, btn)
	}
	return btn
}

// runTestConnection performs the probe off the UI goroutine and renders
// the result. Split out so newTestConnectionButton stays under the
// 15-statement linter limit (AC-PROJ-20).
func runTestConnection(opts AddAccountFormOptions, in core.AccountInput, status *widget.Label, btn *widget.Button) {
	r := opts.TestConn(in)
	defer btn.Enable()
	if r.HasError() {
		status.SetText("⚠ Test failed: " + r.Error().Error())
		return
	}
	res := r.Value()
	status.SetText(fmt.Sprintf("✓ Connected to %s:%d (TLS=%v).", res.Host, res.Port, res.UseTLS))
}
