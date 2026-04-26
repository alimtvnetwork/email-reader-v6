// account_form.go — pure-Go validator for the Add Account form. Lives in
// the views package (no fyne imports here) so the validation rules are
// unit-testable on headless CI without cgo.
package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lovable/email-read/internal/imapdef"
)

// AccountFormInput is the raw user input from the Add Account form. All
// fields are strings because they come straight from the Fyne Entry
// widgets — coercion to int / bool happens in ValidateAccountForm so
// every error surfaces in one pass.
type AccountFormInput struct {
	Alias              string
	Email              string
	DisplayName        string // optional human label
	Password           string
	Host               string // optional — auto-suggested from email when blank
	Port               string // optional — defaults to 993
	UseTLS             bool
	Mailbox            string // optional — defaults to "INBOX"
	AllowBlankPassword bool   // edit mode: blank password ⇒ keep existing
}

// AccountFormResult holds the cleaned values ready to feed into
// core.AddAccount, plus a list of human-readable validation errors. When
// Errors is non-empty, Valid is false and the caller should NOT submit.
type AccountFormResult struct {
	Valid       bool
	Errors      []string
	Alias       string
	Email       string
	DisplayName string
	Password    string
	Host        string
	Port        int
	UseTLS      bool
	Mailbox     string
}

// ValidateAccountForm trims input, fills sensible defaults from imapdef
// when the host/port are blank, and reports every problem at once. The
// password field is NOT trimmed of internal whitespace (config.Sanitize
// handles that downstream) — we only check it's non-empty.
func ValidateAccountForm(in AccountFormInput) AccountFormResult {
	out := AccountFormResult{
		Alias:       strings.TrimSpace(in.Alias),
		Email:       strings.TrimSpace(in.Email),
		DisplayName: strings.TrimSpace(in.DisplayName),
		Password:    in.Password,
		Host:        strings.TrimSpace(in.Host),
		UseTLS:      in.UseTLS,
		Mailbox:     strings.TrimSpace(in.Mailbox),
	}
	var errs []string

	if out.Alias == "" {
		errs = append(errs, "alias is required")
	}
	if out.Email == "" {
		errs = append(errs, "email is required")
	} else if !strings.Contains(out.Email, "@") {
		errs = append(errs, "email must contain @")
	}
	if !in.AllowBlankPassword && strings.TrimSpace(in.Password) == "" {
		errs = append(errs, "password is required")
	}

	// Port: blank ⇒ default 993; otherwise must parse as 1..65535.
	portStr := strings.TrimSpace(in.Port)
	if portStr == "" {
		out.Port = 0 // sentinel — core.AddAccount fills the imapdef default
	} else {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			errs = append(errs, fmt.Sprintf("port %q must be a number 1..65535", portStr))
		} else {
			out.Port = p
		}
	}

	// Host: blank is allowed — core.AddAccount derives via imapdef.Lookup.
	if out.Mailbox == "" {
		out.Mailbox = "INBOX"
	}

	out.Errors = errs
	out.Valid = len(errs) == 0
	return out
}

// SuggestServer returns the imapdef primary suggestion for an email so the
// "Autodiscover" button can pre-fill the host / port / TLS fields without
// committing them. Empty/invalid email ⇒ zero-value Server.
func SuggestServer(email string) imapdef.Server {
	if !strings.Contains(email, "@") {
		return imapdef.Server{}
	}
	primary, _, _ := imapdef.Lookup(strings.TrimSpace(email))
	return primary
}
