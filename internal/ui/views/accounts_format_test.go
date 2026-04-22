package views

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
)

func TestAccountServer(t *testing.T) {
	cases := []struct {
		name string
		in   config.Account
		want string
	}{
		{"tls default port", config.Account{ImapHost: "imap.x", UseTLS: true}, "imap.x:993 (TLS)"},
		{"plain explicit port", config.Account{ImapHost: "imap.x", ImapPort: 143}, "imap.x:143 (PLAIN)"},
		{"unset host", config.Account{}, "(unset):993 (PLAIN)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AccountServer(tc.in); got != tc.want {
				t.Errorf("AccountServer(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLastSeenLabel(t *testing.T) {
	if LastSeenLabel(0) != "(never watched)" || LastSeenLabel(42) != "42" {
		t.Error("labels drifted")
	}
}
