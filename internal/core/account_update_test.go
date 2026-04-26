// account_update_test.go covers the three UpdateAccount behaviours that
// distinguish it from AddAccount:
//   - missing alias ⇒ ErrConfigAccountMissing (NOT silent insert)
//   - blank password ⇒ existing PasswordB64 preserved
//   - non-blank password ⇒ existing PasswordB64 replaced
package core

import (
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
)

func TestUpdateAccount_MissingAliasErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		r := UpdateAccount(AccountInput{
			Alias:         "ghost",
			Email:         "ghost@example.com",
			PlainPassword: "p",
			ImapHost:      "mail.example.com",
			ImapPort:      993,
			UseTLS:        true,
		})
		if !r.HasError() {
			t.Fatal("expected error updating non-existent alias")
		}
		var c *errtrace.Coded
		if !errors.As(r.Error(), &c) || c.Code != errtrace.ErrConfigAccountMissing {
			t.Errorf("expected ErrConfigAccountMissing, got %v", r.Error())
		}
	})
}

func TestUpdateAccount_BlankPasswordKeepsExisting(t *testing.T) {
	withIsolatedConfig(t, func() {
		// Seed an account first.
		add := AddAccount(AccountInput{
			Alias:         "work",
			Email:         "old@example.com",
			PlainPassword: "original",
			ImapHost:      "mail.example.com", ImapPort: 993, UseTLS: true,
		})
		if add.HasError() {
			t.Fatalf("seed: %v", add.Error())
		}
		oldB64 := add.Value().Account.PasswordB64

		// Update everything BUT the password.
		upd := UpdateAccount(AccountInput{
			Alias:    "work",
			Email:    "new@example.com",
			ImapHost: "mail.example.com", ImapPort: 993, UseTLS: true,
			Mailbox: "Archive",
		})
		if upd.HasError() {
			t.Fatalf("UpdateAccount: %v", upd.Error())
		}
		got := upd.Value().Account
		if got.PasswordB64 != oldB64 {
			t.Errorf("blank password should keep PasswordB64, got %q want %q", got.PasswordB64, oldB64)
		}
		if got.Email != "new@example.com" {
			t.Errorf("Email not updated: %q", got.Email)
		}
		if got.Mailbox != "Archive" {
			t.Errorf("Mailbox not updated: %q", got.Mailbox)
		}
	})
}

func TestUpdateAccount_NonBlankPasswordReplaces(t *testing.T) {
	withIsolatedConfig(t, func() {
		add := AddAccount(AccountInput{
			Alias: "a", Email: "a@example.com", PlainPassword: "first",
			ImapHost: "mail.example.com", ImapPort: 993, UseTLS: true,
		})
		if add.HasError() {
			t.Fatalf("seed: %v", add.Error())
		}
		oldB64 := add.Value().Account.PasswordB64

		upd := UpdateAccount(AccountInput{
			Alias: "a", Email: "a@example.com", PlainPassword: "second",
			ImapHost: "mail.example.com", ImapPort: 993, UseTLS: true,
		})
		if upd.HasError() {
			t.Fatalf("UpdateAccount: %v", upd.Error())
		}
		if upd.Value().Account.PasswordB64 == oldB64 {
			t.Error("non-blank password should replace PasswordB64")
		}
		if upd.Value().Account.PasswordB64 == "second" {
			t.Error("password must be base64-encoded, not stored raw")
		}
	})
}
