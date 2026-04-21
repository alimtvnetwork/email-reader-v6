package mailclient

import (
	"strings"
	"testing"
)

func TestParseRawSimple(t *testing.T) {
	raw := []byte("From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Hello\r\n" +
		"Message-Id: <abc@example.com>\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"hi there\r\n")
	m, err := parseRaw(raw)
	if err != nil {
		t.Fatalf("parseRaw: %v", err)
	}
	if !strings.Contains(m.From, "alice@example.com") {
		t.Errorf("from = %q", m.From)
	}
	if m.Subject != "Hello" {
		t.Errorf("subject = %q", m.Subject)
	}
	if !strings.Contains(m.BodyText, "hi there") {
		t.Errorf("body = %q", m.BodyText)
	}
}

func TestStripHtml(t *testing.T) {
	got := stripHtml("<p>hello <b>world</b></p>")
	want := "hello world"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSanitize(t *testing.T) {
	if got := sanitize("abc/def<weird>?id"); got != "abc_def_weird_id" {
		t.Errorf("sanitize = %q", got)
	}
}
