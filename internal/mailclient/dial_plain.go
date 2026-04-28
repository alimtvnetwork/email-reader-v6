// dial_plain.go — DialPlain opens an IMAP connection using the supplied
// host/port/TLS/credentials directly, without needing a saved
// config.Account. Used by the Add Account form's "Test connection"
// button to verify credentials BEFORE persisting them.
//
// This is a separate file from mailclient.go to keep the surface area
// of the change minimal and reviewable: no edits to existing Dial.
package mailclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/emersion/go-imap/client"

	"github.com/lovable/email-read/internal/errtrace"
)

// PlainDialInput is everything DialPlain needs. Timeout caps both the
// TCP/TLS handshake and the LOGIN round-trip so a hung server can't
// freeze the UI button.
type PlainDialInput struct {
	Host     string
	Port     int
	UseTLS   bool
	Email    string
	Password string        // plaintext — NEVER persisted by DialPlain
	Timeout  time.Duration // 0 ⇒ DefaultPlainDialTimeout
}

// DefaultPlainDialTimeout is the upper bound on a single Test Connection
// attempt. Generous enough for slow/shared-hosting IMAPs but still short
// enough to keep the UI responsive.
const DefaultPlainDialTimeout = 30 * time.Second

// DialPlain dials the given IMAP endpoint, logs in, and immediately logs
// out. Returns a *Coded error on dial / handshake / login failure so
// callers can branch on the code.
//
// On success the connection is fully closed before returning — this is a
// probe, not a session.
func DialPlain(in PlainDialInput) error {
	if in.Host == "" || in.Port <= 0 || in.Email == "" || in.Password == "" {
		return errtrace.NewCoded(errtrace.ErrCoreInvalidArgument,
			"DialPlain requires host, port, email, and password")
	}
	timeout := in.Timeout
	if timeout <= 0 {
		timeout = DefaultPlainDialTimeout
	}

	c, err := dialPlainConn(in.Host, in.Port, in.UseTLS, timeout)
	if err != nil {
		return wrapDialError(err, in.Host, in.Port)
	}
	defer func() { _ = c.Logout() }()

	c.Timeout = timeout
	if err := c.Login(in.Email, in.Password); err != nil {
		return errtrace.WrapCode(err, errtrace.ErrMailLogin, "imap login").
			WithContext("Email", in.Email).
			WithContext("Host", in.Host)
	}
	return nil
}

// dialPlainConn performs the TCP (or TLS) dial with the supplied timeout.
func dialPlainConn(host string, port int, useTLS bool, timeout time.Duration) (*client.Client, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	dialer := &net.Dialer{Timeout: timeout}
	if useTLS {
		return client.DialWithDialerTLS(dialer, addr, &tls.Config{ServerName: host})
	}
	return client.DialWithDialer(dialer, addr)
}

// wrapDialError tags timeouts vs generic dial failures with the right code.
func wrapDialError(err error, host string, port int) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errtrace.WrapCode(err, errtrace.ErrMailTimeout, "imap dial timed out").
			WithContext("Host", host).
			WithContext("Port", fmt.Sprintf("%d", port))
	}
	return errtrace.WrapCode(err, errtrace.ErrMailDial, "imap dial failed").
		WithContext("Host", host).
		WithContext("Port", fmt.Sprintf("%d", port))
}
