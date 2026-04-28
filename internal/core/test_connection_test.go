// test_connection_test.go covers the input-validation paths of
// TestAccountConnection. The real IMAP dial is exercised by integration
// tests; here we only assert that the function rejects bad input with
// the expected error codes BEFORE attempting any network I/O.
package core

import (
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
)

func TestTestAccountConnection_RejectsMissingEmail(t *testing.T) {
	r := TestAccountConnection(AccountInput{PlainPassword: "p"}, 0)
	if !r.HasError() {
		t.Fatal("expected error for missing email")
	}
	var c *errtrace.Coded
	if !errors.As(r.Error(), &c) || c.Code != errtrace.ErrCoreInvalidArgument {
		t.Errorf("expected ErrCoreInvalidArgument, got %v", r.Error())
	}
}

func TestTestAccountConnection_RejectsMissingPassword(t *testing.T) {
	r := TestAccountConnection(AccountInput{Email: "u@x.com"}, 0)
	if !r.HasError() {
		t.Fatal("expected error for missing password")
	}
	var c *errtrace.Coded
	if !errors.As(r.Error(), &c) || c.Code != errtrace.ErrCoreInvalidArgument {
		t.Errorf("expected ErrCoreInvalidArgument, got %v", r.Error())
	}
}

func TestTestAccountConnection_RejectsUnresolvableEndpoint(t *testing.T) {
	// No host hint, no @-sign in email ⇒ resolveImapDefaults can't fill
	// host/port and we must bail BEFORE dialing.
	r := TestAccountConnection(AccountInput{
		Email:         "noatsign",
		PlainPassword: "p",
	}, 0)
	if !r.HasError() {
		t.Fatal("expected error for unresolvable endpoint")
	}
	var c *errtrace.Coded
	if !errors.As(r.Error(), &c) || c.Code != errtrace.ErrCoreInvalidArgument {
		t.Errorf("expected ErrCoreInvalidArgument, got %v", r.Error())
	}
}

func TestTestAccountConnection_SanitizesPasswordAndWrapsLoginFailure(t *testing.T) {
	oldDial := testAccountConnectionDial
	t.Cleanup(func() { testAccountConnectionDial = oldDial })
	testAccountConnectionDial = func(in mailclient.PlainDialInput) error {
		if in.Password != "secret" {
			t.Fatalf("password passed to DialPlain = %q, want sanitized secret", in.Password)
		}
		return errtrace.NewCoded(errtrace.ErrMailLogin, "imap login")
	}

	r := TestAccountConnection(AccountInput{
		Alias: "Admin", Email: "user@example.com", PlainPassword: "\u200bsecret\u2060",
		ImapHost: "mail.example.com", ImapPort: 993, UseTLS: true,
	}, time.Second)
	if !r.HasError() {
		t.Fatal("expected login failure")
	}
	var c *errtrace.Coded
	if !errors.As(r.Error(), &c) || c.Code != errtrace.ErrAccountTestFailed {
		t.Fatalf("expected ErrAccountTestFailed, got %v", r.Error())
	}
	if !errors.As(errors.Unwrap(c), &c) || c.Code != errtrace.ErrMailLogin {
		t.Fatalf("expected wrapped ErrMailLogin, got %v", r.Error())
	}
}
