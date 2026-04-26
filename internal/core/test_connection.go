// test_connection.go — TestAccountConnection probes an IMAP endpoint with
// the supplied credentials WITHOUT persisting anything. Used by the Add
// Account form's "Test connection" button so users get fast feedback
// before committing the account to config.json.
package core

import (
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
)

// DefaultTestConnectionTimeout caps a single probe attempt.
const DefaultTestConnectionTimeout = 8 * time.Second

// TestAccountConnection resolves missing host/port/TLS from imapdef (so
// the user can leave host blank and still test) and dials + logs in via
// mailclient.DialPlain. Returns a *Coded error on failure or the resolved
// host/port/TLS on success so the form can report what it actually hit.
//
// The function does NOT touch config.json — it is a pure probe.
func TestAccountConnection(in AccountInput, timeout time.Duration) errtrace.Result[TestConnectionResult] {
	if in.Email == "" || in.PlainPassword == "" {
		return errtrace.Err[TestConnectionResult](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"email and password are required to test a connection"))
	}
	host, port, useTLS, _ := resolveImapDefaults(in)
	if host == "" || port == 0 {
		return errtrace.Err[TestConnectionResult](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"could not resolve IMAP host/port — pick a Provider or fill them in").
			WithContext("Email", in.Email))
	}
	if timeout <= 0 {
		timeout = DefaultTestConnectionTimeout
	}
	if err := mailclient.DialPlain(mailclient.PlainDialInput{
		Host: host, Port: port, UseTLS: useTLS,
		Email: in.Email, Password: in.PlainPassword, Timeout: timeout,
	}); err != nil {
		return errtrace.Err[TestConnectionResult](err)
	}
	return errtrace.Ok(TestConnectionResult{Host: host, Port: port, UseTLS: useTLS})
}

// TestConnectionResult reports the resolved endpoint that was successfully
// dialled — handy for showing the user "Connected to imap.gmail.com:993
// (TLS)" rather than echoing their possibly-blank inputs back.
type TestConnectionResult struct {
	Host   string
	Port   int
	UseTLS bool
}
