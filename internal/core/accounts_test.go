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
		{Alias: "a", PlainPassword: "p"},                  // missing email
		{Email: "a@x", PlainPassword: "p"},                // missing alias
		{Alias: "a", Email: "a@x"},                        // missing password
		{Alias: " ", Email: " ", PlainPassword: "p"},      // whitespace-only
	}
	for i, in := range cases {
		if _, err := AddAccount(in); err == nil {
			t.Errorf("case %d: expected error for input %+v", i, in)
		}
	}
}

func TestAddAccount_PersistsAndDerivesDefaults(t *testing.T) {
	withIsolatedConfig(t, func() {
		res, err := AddAccount(AccountInput{
			Alias:         "test",
			Email:         "user@gmail.com",
			PlainPassword: "secret123",
		})
		if err != nil {
			t.Fatalf("AddAccount: %v", err)
		}
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
		list, err := ListAccounts()
		if err != nil {
			t.Fatalf("ListAccounts: %v", err)
		}
		if len(list) != 1 || list[0].Alias != "test" {
			t.Errorf("ListAccounts returned %+v", list)
		}
	})
}

func TestAddAccount_StripsHiddenChars(t *testing.T) {
	withIsolatedConfig(t, func() {
		// U+200B zero-width space appended to the password.
		dirty := "secret\u200B"
		res, err := AddAccount(AccountInput{
			Alias:         "x",
			Email:         "x@example.com",
			PlainPassword: dirty,
			ImapHost:      "mail.example.com",
			ImapPort:      993,
			UseTLS:        true,
		})
		if err != nil {
			t.Fatalf("AddAccount: %v", err)
		}
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
		res, err := AddAccount(AccountInput{
			Alias:          "y",
			Email:          "y@example.com",
			PlainPassword:  "p",
			ImapHost:       "imap.custom.tld",
			ImapPort:       143,
			UseTLS:         false,
			UseTLSExplicit: true,
			Mailbox:        "Archive",
		})
		if err != nil {
			t.Fatalf("AddAccount: %v", err)
		}
		if res.Account.ImapHost != "imap.custom.tld" || res.Account.ImapPort != 143 ||
			res.Account.UseTLS != false || res.Account.Mailbox != "Archive" {
			t.Errorf("explicit fields not preserved: %+v", res.Account)
		}
	})
}

func TestGetAndRemoveAccount(t *testing.T) {
	withIsolatedConfig(t, func() {
		_, err := AddAccount(AccountInput{
			Alias: "a", Email: "a@x.com", PlainPassword: "p",
			ImapHost: "h", ImapPort: 993, UseTLS: true, UseTLSExplicit: true,
		})
		if err != nil {
			t.Fatalf("AddAccount: %v", err)
		}
		got, err := GetAccount("a")
		if err != nil || got.Alias != "a" {
			t.Fatalf("GetAccount: %v, %+v", err, got)
		}
		if _, err := GetAccount("missing"); err == nil {
			t.Error("expected error for missing alias")
		}
		if err := RemoveAccount("a"); err != nil {
			t.Fatalf("RemoveAccount: %v", err)
		}
		if err := RemoveAccount("a"); err == nil {
			t.Error("expected error removing missing alias")
		}
	})
}
