// Package core holds framework-agnostic operations shared by the CLI and the
// Fyne UI. Functions here perform business logic only — they never print,
// prompt, or touch cobra/survey. Callers (CLI / UI) handle I/O.
//
// Per spec/21-app/04-coding-standards.md §4.2 every exported core API in
// this file returns errtrace.Result[T] (not (T, error)). Lower-level
// adapters (internal/config, internal/imapdef) still return raw error and
// are wrapped with a stable error code at this boundary via WrapCode +
// WithContext.
package core

import (
	"strings"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/imapdef"
)

// AccountInput captures everything needed to create or update an account.
// PlainPassword is the unencoded password as the user typed it; it will be
// sanitized + base64-encoded before being persisted.
type AccountInput struct {
	Alias          string
	Email          string
	PlainPassword  string
	ImapHost       string // optional — auto-derived from email domain when empty
	ImapPort       int    // optional — defaults to imapdef lookup or 993
	UseTLS         bool
	UseTLSExplicit bool   // true when the caller explicitly set UseTLS
	Mailbox        string // optional — defaults to "INBOX"
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
// by Diagnose.
func AddAccount(in AccountInput) errtrace.Result[*AddAccountResult] {
	clean, hidden, vErr := validateAndSanitize(&in)
	if vErr != nil {
		return errtrace.Err[*AddAccountResult](
			errtrace.WrapCode(vErr, errtrace.ErrConfigValidate, "validate account input").
				WithContext("Alias", in.Alias).
				WithContext("Email", in.Email),
		)
	}

	cfg, err := config.Load()
	if err != nil {
		return errtrace.Err[*AddAccountResult](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen, "load config").
				WithContext("Alias", in.Alias),
		)
	}

	acct := buildAccount(in, clean)
	existed := cfg.FindAccount(acct.Alias) != nil
	cfg.UpsertAccount(acct)
	if err := config.Save(cfg); err != nil {
		return errtrace.Err[*AddAccountResult](
			errtrace.WrapCode(err, errtrace.ErrConfigEncode, "save config").
				WithContext("Alias", in.Alias),
		)
	}
	// Publish AFTER persistence so consumers (e.g. Tools.diagCache)
	// only invalidate on a state that actually exists on disk.
	if existed {
		publishAccountEvent(AccountUpdated, acct.Alias)
	} else {
		publishAccountEvent(AccountAdded, acct.Alias)
	}
	p, _ := config.Path()
	return errtrace.Ok(&AddAccountResult{
		Account:        acct,
		HiddenCharsRem: hidden,
		ConfigPath:     p,
	})
}

// buildAccount constructs the persisted config.Account from validated input
// plus the cleaned password. Splitting this out keeps AddAccount under the
// 15-statement linter limit (AC-PROJ-20).
func buildAccount(in AccountInput, cleanPassword string) config.Account {
	host, port, useTLS, mailbox := resolveImapDefaults(in)
	return config.Account{
		Alias:       in.Alias,
		Email:       in.Email,
		PasswordB64: config.EncodePassword(cleanPassword),
		ImapHost:    host,
		ImapPort:    port,
		UseTLS:      useTLS,
		Mailbox:     mailbox,
	}
}

// validateAndSanitize trims required fields on `in` (in place), then sanitizes
// the password. Returns the cleaned password and the count of hidden chars
// removed, or an error when required fields are missing. Returns a raw error
// (not a *Coded) — AddAccount wraps it with the right code + context.
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
func ListAccounts() errtrace.Result[[]config.Account] {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Err[[]config.Account](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen, "load config"),
		)
	}
	out := make([]config.Account, len(cfg.Accounts))
	copy(out, cfg.Accounts)
	return errtrace.Ok(out)
}

// GetAccount returns the account with the given alias or an error.
func GetAccount(alias string) errtrace.Result[config.Account] {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Err[config.Account](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen, "load config").
				WithContext("Alias", alias),
		)
	}
	p := cfg.FindAccount(alias)
	if p == nil {
		return errtrace.Err[config.Account](
			errtrace.NewCoded(errtrace.ErrConfigAccountMissing, "account lookup").
				WithContext("Alias", alias),
		)
	}
	return errtrace.Ok(*p)
}

// RemoveAccount deletes the account with the given alias.
func RemoveAccount(alias string) errtrace.Result[struct{}] {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Err[struct{}](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen, "load config").
				WithContext("Alias", alias),
		)
	}
	if !cfg.RemoveAccount(alias) {
		return errtrace.Err[struct{}](
			errtrace.NewCoded(errtrace.ErrConfigAccountMissing, "remove account").
				WithContext("Alias", alias),
		)
	}
	if err := config.Save(cfg); err != nil {
		return errtrace.Err[struct{}](
			errtrace.WrapCode(err, errtrace.ErrConfigEncode, "save config").
				WithContext("Alias", alias),
		)
	}
	publishAccountEvent(AccountRemoved, alias)
	return errtrace.Ok(struct{}{})
}
