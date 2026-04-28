// account_update.go — UpdateAccount mutates an EXISTING account by alias.
// Distinct from AddAccount in three ways:
//  1. Alias must already exist; otherwise ErrConfigAccountMissing.
//  2. PlainPassword may be blank — meaning "keep the current PasswordB64
//     unchanged". This lets the Edit form show the password field as
//     empty without forcing the user to re-type it.
//  3. Publishes AccountUpdated (never AccountAdded).
//
// Like AddAccount this function is the source of truth for write-lock
// atomicity (CF-A2): Load + Upsert + Save run inside config.WithWriteLock.
package core

import (
	"strings"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// UpdateAccount applies the supplied input to the existing account with
// the same alias. Returns ErrConfigAccountMissing if no such alias exists.
//
// Behaviour notes:
//   - PlainPassword == "" ⇒ keep the existing PasswordB64.
//   - DisplayName == ""   ⇒ clear the display name (it's optional).
//   - Host/Port/UseTLS/Mailbox follow the same imapdef-fallback rules as
//     AddAccount via resolveImapDefaults.
func UpdateAccount(in AccountInput) errtrace.Result[*AddAccountResult] {
	in.Alias = strings.TrimSpace(in.Alias)
	in.Email = strings.TrimSpace(in.Email)
	if in.Alias == "" || in.Email == "" {
		return errtrace.Err[*AddAccountResult](errtrace.NewCoded(
			errtrace.ErrConfigValidate, "alias and email are required").
			WithContext("Alias", in.Alias))
	}
	clean, hidden, err := sanitizeUpdatePassword(in.PlainPassword)
	if err != nil {
		return errtrace.Err[*AddAccountResult](errtrace.WrapCode(
			err, errtrace.ErrConfigValidate, "validate update password").
			WithContext("Alias", in.Alias))
	}
	result, acct, ok := persistUpdateAccount(in, clean, hidden)
	if result.HasError() {
		return result
	}
	if !ok {
		return result // missing-alias error already on result
	}
	publishAccountEvent(AccountUpdated, acct.Alias)
	return result
}

// sanitizeUpdatePassword returns ("", 0, nil) when the user left the
// password blank (signalling "keep existing"); otherwise behaves like
// validateAndSanitize's password branch.
func sanitizeUpdatePassword(plain string) (string, int, error) {
	if plain == "" {
		return "", 0, nil
	}
	clean := config.SanitizePassword(plain)
	hidden := len(plain) - len(clean)
	return clean, hidden, nil
}

// persistUpdateAccount runs the locked Load+Upsert+Save transaction.
// Returns the result, the saved account, and whether the alias existed
// (false ⇒ result is already set to a missing-alias error).
func persistUpdateAccount(in AccountInput, clean string, hidden int) (errtrace.Result[*AddAccountResult], config.Account, bool) {
	var (
		result errtrace.Result[*AddAccountResult]
		acct   config.Account
		found  bool
	)
	config.WithWriteLock(func() {
		cfg, err := config.Load()
		if err != nil {
			result = errtrace.Err[*AddAccountResult](errtrace.WrapCode(
				err, errtrace.ErrConfigOpen, "load config").
				WithContext("Alias", in.Alias))
			return
		}
		existing := cfg.FindAccount(in.Alias)
		if existing == nil {
			result = errtrace.Err[*AddAccountResult](errtrace.NewCoded(
				errtrace.ErrConfigAccountMissing, "update account").
				WithContext("Alias", in.Alias))
			return
		}
		acct = mergeAccountUpdate(*existing, in, clean)
		cfg.UpsertAccount(acct)
		if err := config.Save(cfg); err != nil {
			result = errtrace.Err[*AddAccountResult](errtrace.WrapCode(
				err, errtrace.ErrConfigEncode, "save config").
				WithContext("Alias", in.Alias))
			return
		}
		p, _ := config.Path()
		result = errtrace.Ok(&AddAccountResult{
			Account: acct, HiddenCharsRem: hidden, ConfigPath: p,
		})
		found = true
	})
	return result, acct, found
}

// mergeAccountUpdate applies non-blank fields from `in` over `existing`.
// PasswordB64 is preserved when `clean` is empty (user kept the password).
func mergeAccountUpdate(existing config.Account, in AccountInput, clean string) config.Account {
	host, port, useTLS, mailbox := resolveImapDefaults(in)
	out := config.Account{
		Alias:       existing.Alias, // alias is the immutable key
		Email:       in.Email,
		DisplayName: strings.TrimSpace(in.DisplayName),
		PasswordB64: existing.PasswordB64,
		ImapHost:    host,
		ImapPort:    port,
		UseTLS:      useTLS,
		Mailbox:     mailbox,
	}
	if clean != "" {
		out.PasswordB64 = config.EncodePassword(clean)
	}
	return out
}
