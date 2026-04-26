// Package core holds framework-agnostic operations shared by the CLI and the
// upcoming Fyne UI. Functions here perform business logic only — they never
// print, prompt, or touch cobra/survey. Callers (CLI / UI) handle I/O.
package core

import (
	"fmt"
	"strings"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/imapdef"
)

// AccountInput captures everything needed to create or update an account.
// PlainPassword is the unencoded password as the user typed it; it will be
// sanitized + base64-encoded before being persisted.
type AccountInput struct {
	Alias         string
	Email         string
	PlainPassword string
	ImapHost      string // optional — auto-derived from email domain when empty
	ImapPort      int    // optional — defaults to imapdef lookup or 993
	UseTLS        bool
	UseTLSExplicit bool   // true when the caller explicitly set UseTLS
	Mailbox       string // optional — defaults to "INBOX"
}

// AddAccountResult reports the saved account plus any cleanup that happened.
type AddAccountResult struct {
	Account        config.Account
	HiddenCharsRem int    // number of invisible chars stripped from password
	ConfigPath     string // absolute path of data/config.json
}

// AddAccount validates input, derives missing IMAP defaults from the email
// domain, sanitizes the password, and persists the account to config.json.
// It does NOT verify the IMAP connection — that's a separate concern handled
// by Diagnose. Returns an error for missing required fields or persistence
// failures.
func AddAccount(in AccountInput) (*AddAccountResult, error) {
	clean, hidden, err := validateAndSanitize(&in)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, errtrace.Wrap(err, "load config")
	}

	host, port, useTLS, mailbox := resolveImapDefaults(in)
	acct := config.Account{
		Alias:       in.Alias,
		Email:       in.Email,
		PasswordB64: config.EncodePassword(clean),
		ImapHost:    host,
		ImapPort:    port,
		UseTLS:      useTLS,
		Mailbox:     mailbox,
	}
	cfg.UpsertAccount(acct)
	if err := config.Save(cfg); err != nil {
		return nil, errtrace.Wrap(err, "save config")
	}
	p, _ := config.Path()
	return &AddAccountResult{
		Account:        acct,
		HiddenCharsRem: hidden,
		ConfigPath:     p,
	}, nil
}

// validateAndSanitize trims required fields on `in` (in place), then sanitizes
// the password. Returns the cleaned password and the count of hidden chars
// removed, or an error when required fields are missing.
func validateAndSanitize(in *AccountInput) (string, int, error) {
	in.Email = strings.TrimSpace(in.Email)
	in.Alias = strings.TrimSpace(in.Alias)
	if in.Email == "" || in.Alias == "" || in.PlainPassword == "" {
		return "", 0, errtrace.New("email, alias and password are required")
	}
	clean := config.SanitizePassword(in.PlainPassword)
	hidden := len(in.PlainPassword) - len(clean)
	return clean, hidden, nil
}

// resolveImapDefaults derives missing host/port/TLS/mailbox values from the
// email domain via imapdef lookup, falling back to sensible defaults.
func resolveImapDefaults(in AccountInput) (host string, port int, useTLS bool, mailbox string) {
	host = in.ImapHost
	port = in.ImapPort
	useTLS = in.UseTLS
	if host == "" || port == 0 {
		primary, _, _ := imapdef.Lookup(in.Email)
		if host == "" {
			host = primary.Host
		}
		if port == 0 {
			port = primary.Port
			if port == 0 {
				port = 993
			}
		}
		if !in.UseTLSExplicit {
			useTLS = primary.UseTLS || useTLS
		}
	}
	mailbox = in.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	return host, port, useTLS, mailbox
}

// ListAccounts returns all configured accounts (a copy — safe to mutate).
func ListAccounts() ([]config.Account, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errtrace.Wrap(err, "load config")
	}
	out := make([]config.Account, len(cfg.Accounts))
	copy(out, cfg.Accounts)
	return out, nil
}

// GetAccount returns the account with the given alias or an error.
func GetAccount(alias string) (config.Account, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Account{}, errtrace.Wrap(err, "load config")
	}
	p := cfg.FindAccount(alias)
	if p == nil {
		return config.Account{}, errtrace.New(fmt.Sprintf("no account with alias %q", alias))
	}
	return *p, nil
}

// RemoveAccount deletes the account with the given alias.
func RemoveAccount(alias string) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if !cfg.RemoveAccount(alias) {
		return errtrace.New(fmt.Sprintf("no account with alias %q", alias))
	}
	if err := config.Save(cfg); err != nil {
		return errtrace.Wrap(err, "save config")
	}
	return nil
}
