package imapdef

import "testing"

func TestLookupKnown(t *testing.T) {
	p, _, known := Lookup("foo@gmail.com")
	if !known {
		t.Fatal("gmail should be known")
	}
	if p.Host != "imap.gmail.com" || p.Port != 993 || !p.UseTLS {
		t.Fatalf("bad gmail server: %+v", p)
	}
}

func TestLookupUnknownFallback(t *testing.T) {
	p, s, known := Lookup("admin@attobondcleaning.store")
	if known {
		t.Fatal("custom domain should not be known")
	}
	if p.Host != "mail.attobondcleaning.store" {
		t.Fatalf("primary should be mail.<domain>, got %s", p.Host)
	}
	if s.Host != "imap.attobondcleaning.store" {
		t.Fatalf("secondary should be imap.<domain>, got %s", s.Host)
	}
}

func TestLookupBadInput(t *testing.T) {
	if _, _, k := Lookup("not-an-email"); k {
		t.Fatal("invalid input should not be marked known")
	}
}
