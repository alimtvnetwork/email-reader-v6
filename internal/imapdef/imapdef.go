// Package imapdef holds built-in IMAP server defaults keyed by email domain.
// Used by `email-read add` to auto-suggest host/port/TLS for common providers.
package imapdef

import "strings"

// Server describes one IMAP endpoint suggestion.
type Server struct {
	Host   string
	Port   int
	UseTLS bool
}

// builtin maps the lowercase domain (after the @) to a known IMAP server.
var builtin = map[string]Server{
	"gmail.com":       {"imap.gmail.com", 993, true},
	"googlemail.com":  {"imap.gmail.com", 993, true},
	"outlook.com":     {"outlook.office365.com", 993, true},
	"hotmail.com":     {"outlook.office365.com", 993, true},
	"live.com":        {"outlook.office365.com", 993, true},
	"office365.com":   {"outlook.office365.com", 993, true},
	"yahoo.com":       {"imap.mail.yahoo.com", 993, true},
	"icloud.com":      {"imap.mail.me.com", 993, true},
	"me.com":          {"imap.mail.me.com", 993, true},
	"fastmail.com":    {"imap.fastmail.com", 993, true},
	"protonmail.com":  {"127.0.0.1", 1143, false}, // requires Proton Bridge
	"zoho.com":        {"imap.zoho.com", 993, true},
}

// Lookup returns the suggested server for the given email address.
// If no built-in match exists, it falls back to `mail.<domain>` (cPanel-style)
// as the primary guess, with `imap.<domain>` returned as the secondary guess.
// Both fallbacks default to port 993 + TLS.
func Lookup(email string) (primary Server, secondary Server, knownProvider bool) {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return Server{}, Server{}, false
	}
	domain := strings.ToLower(email[at+1:])
	if s, ok := builtin[domain]; ok {
		return s, Server{}, true
	}
	return Server{Host: "mail." + domain, Port: 993, UseTLS: true},
		Server{Host: "imap." + domain, Port: 993, UseTLS: true},
		false
}

// SeedAccount returns the dev/test seed account suggested by the spec.
// The password is intentionally NOT included — the user must supply it.
func SeedAccount() (alias, email string, srv Server) {
	return "atto",
		"lovable.admin@attobondcleaning.store",
		Server{Host: "mail.attobondcleaning.store", Port: 993, UseTLS: true}
}
