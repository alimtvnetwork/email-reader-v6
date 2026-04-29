// test_connection.go — TestAccountConnection probes an IMAP endpoint with
// the supplied credentials WITHOUT persisting anything. Used by the Add
// Account form's "Test connection" button so users get fast feedback
// before committing the account to config.json.
package core

import (
	"errors"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
)

// DefaultTestConnectionTimeout caps a single probe attempt.
const DefaultTestConnectionTimeout = 8 * time.Second

var testAccountConnectionDial = mailclient.DialPlain

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
	cleanPassword := config.SanitizePassword(in.PlainPassword)
	if err := testAccountConnectionDial(mailclient.PlainDialInput{
		Host: host, Port: port, UseTLS: useTLS,
		Email: in.Email, Password: cleanPassword, Timeout: timeout,
	}); err != nil {
		return errtrace.Err[TestConnectionResult](wrapTestConnectionError(err, in, host, port, useTLS, cleanPassword))
	}
	return errtrace.Ok(TestConnectionResult{Host: host, Port: port, UseTLS: useTLS})
}

func wrapTestConnectionError(err error, in AccountInput, host string, port int, useTLS bool, cleanPassword string) error {
	return errtrace.WrapCode(err, errtrace.ErrAccountTestFailed, testConnectionFailureMessage(err, host, port, useTLS)).
		WithContext("Alias", in.Alias).
		WithContext("Email", in.Email).
		WithContext("Host", host).
		WithContext("Port", port).
		WithContext("UseTLS", useTLS).
		WithContext("PasswordSanitized", cleanPassword != in.PlainPassword)
}

func testConnectionFailureMessage(err error, host string, port int, useTLS bool) string {
	var coded *errtrace.Coded
	if errors.As(err, &coded) && coded.Code == errtrace.ErrMailLogin {
		return "test connection failed — IMAP server rejected the login; verify the same email/password in webmail or reset the mailbox password"
	}
	if errors.As(err, &coded) && (coded.Code == errtrace.ErrMailTimeout || coded.Code == errtrace.ErrMailDial) {
		return testConnectionReachabilityMessage(host, port, useTLS)
	}
	return "test connection failed — IMAP test could not complete; inspect the wrapped mail error for the exact cause"
}

func testConnectionReachabilityMessage(host string, port int, useTLS bool) string {
	tlsHint := "TLS on"
	if !useTLS {
		tlsHint = "TLS off"
	}
	return "test connection failed — TCP connection to IMAP endpoint " + host + ":" + fmt.Sprint(port) + " (" + tlsHint + ") did not reach login; verify with nc -zv, and if ports 993/143 time out ask hosting to enable Dovecot/open IMAP firewall and keep Cloudflare mail DNS-only"
}

// TestConnectionResult reports the resolved endpoint that was successfully
// dialled — handy for showing the user "Connected to imap.gmail.com:993
// (TLS)" rather than echoing their possibly-blank inputs back.
type TestConnectionResult struct {
	Host   string
	Port   int
	UseTLS bool
}
