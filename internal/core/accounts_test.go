package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// withIsolatedConfig backs up data/config.json (if any), runs fn, and restores it.
// Uses the real on-disk path because config.Load/Save are not parameterized.
func withIsolatedConfig(t *testing.T, fn func()) {
	t.Helper()
	p, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	backup := p + ".testbackup"
	// Snapshot existing config (if any) so we don't clobber dev data.
	if data, err := os.ReadFile(p); err == nil {
		if werr := os.WriteFile(backup, data, 0o600); werr != nil {
			t.Fatalf("backup config: %v", werr)
		}
	}
	t.Cleanup(func() {
		if data, err := os.ReadFile(backup); err == nil {
			_ = os.WriteFile(p, data, 0o600)
			_ = os.Remove(backup)
		} else {
			_ = os.Remove(p)
		}
	})
	// Start from a clean slate for the test.
	_ = os.Remove(p)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	fn()
}

func TestAddAccount_RequiresFields(t *testing.T) {
	cases := []AccountInput{
		{Alias: "a", PlainPassword: "p"},             // missing email
		{Email: "a@x", PlainPassword: "p"},           // missing alias
		{Alias: "a", Email: "a@x"},                   // missing password
		{Alias: " ", Email: " ", PlainPassword: "p"}, // whitespace-only
	}
	for i, in := range cases {
		if r := AddAccount(in); !r.HasError() {
			t.Errorf("case %d: expected error for input %+v", i, in)
		}
	}
}

func TestAddAccount_PersistsAndDerivesDefaults(t *testing.T) {
	withIsolatedConfig(t, func() {
		r := AddAccount(AccountInput{
			Alias:         "test",
			Email:         "user@gmail.com",
			PlainPassword: "secret123",
		})
		if r.HasError() {
			t.Fatalf("AddAccount: %v", r.Error())
		}
		res := r.Value()
		if res.Account.Alias != "test" || res.Account.Email != "user@gmail.com" {
			t.Errorf("unexpected stored account: %+v", res.Account)
		}
		if res.Account.ImapPort == 0 {
			t.Errorf("expected derived imap port, got 0")
		}
		if res.Account.Mailbox != "INBOX" {
			t.Errorf("expected default mailbox INBOX, got %q", res.Account.Mailbox)
		}
		if res.Account.PasswordB64 == "secret123" {
			t.Errorf("password was not encoded")
		}
		// Round-trip via List.
		listR := ListAccounts()
		if listR.HasError() {
			t.Fatalf("ListAccounts: %v", listR.Error())
		}
		list := listR.Value()
		if len(list) != 1 || list[0].Alias != "test" {
			t.Errorf("ListAccounts returned %+v", list)
		}
	})
}

func TestAddAccount_StripsHiddenChars(t *testing.T) {
	withIsolatedConfig(t, func() {
		// U+200B zero-width space appended to the password.
		dirty := "secret\u200B"
		r := AddAccount(AccountInput{
			Alias:         "x",
			Email:         "x@example.com",
			PlainPassword: dirty,
			ImapHost:      "mail.example.com",
			ImapPort:      993,
			UseTLS:        true,
		})
		if r.HasError() {
			t.Fatalf("AddAccount: %v", r.Error())
		}
		res := r.Value()
		if res.HiddenCharsRem == 0 {
			t.Errorf("expected hidden char count > 0, got 0")
		}
		dec, err := config.DecodePassword(res.Account.PasswordB64)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if dec != "secret" {
			t.Errorf("expected sanitized %q, got %q", "secret", dec)
		}
	})
}

func TestAddAccount_RespectsExplicitFields(t *testing.T) {
	withIsolatedConfig(t, func() {
		r := AddAccount(AccountInput{
			Alias:          "y",
			Email:          "y@example.com",
			PlainPassword:  "p",
			ImapHost:       "imap.custom.tld",
			ImapPort:       143,
			UseTLS:         false,
			UseTLSExplicit: true,
			Mailbox:        "Archive",
		})
		if r.HasError() {
			t.Fatalf("AddAccount: %v", r.Error())
		}
		res := r.Value()
		if res.Account.ImapHost != "imap.custom.tld" || res.Account.ImapPort != 143 ||
			res.Account.UseTLS != false || res.Account.Mailbox != "Archive" {
			t.Errorf("explicit fields not preserved: %+v", res.Account)
		}
	})
}

func TestGetAndRemoveAccount(t *testing.T) {
	withIsolatedConfig(t, func() {
		addR := AddAccount(AccountInput{
			Alias: "a", Email: "a@x.com", PlainPassword: "p",
			ImapHost: "h", ImapPort: 993, UseTLS: true, UseTLSExplicit: true,
		})
		if addR.HasError() {
			t.Fatalf("AddAccount: %v", addR.Error())
		}
		gotR := GetAccount("a")
		if gotR.HasError() || gotR.Value().Alias != "a" {
			t.Fatalf("GetAccount: %v, %+v", gotR.Error(), gotR.Value())
		}
		if missR := GetAccount("missing"); !missR.HasError() {
			t.Error("expected error for missing alias")
		}
		if rmR := RemoveAccount("a"); rmR.HasError() {
			t.Fatalf("RemoveAccount: %v", rmR.Error())
		}
		if rmR := RemoveAccount("a"); !rmR.HasError() {
			t.Error("expected error removing missing alias")
		}
	})
}
