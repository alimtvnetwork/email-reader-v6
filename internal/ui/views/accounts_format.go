// accounts_format.go — pure-Go helpers for the Accounts view.
package views

import (
	"fmt"

	"github.com/lovable/email-read/internal/config"
)

func AccountServer(a config.Account) string {
	tag := "TLS"
	if !a.UseTLS {
		tag = "PLAIN"
	}
	port := a.ImapPort
	if port == 0 {
		port = 993
	}
	host := a.ImapHost
	if host == "" {
		host = "(unset)"
	}
	return fmt.Sprintf("%s:%d (%s)", host, port, tag)
}

func LastSeenLabel(uid uint32) string {
	if uid == 0 {
		return "(never watched)"
	}
	return fmt.Sprintf("%d", uid)
}
