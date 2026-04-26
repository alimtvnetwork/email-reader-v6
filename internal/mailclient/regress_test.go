package mailclient

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// Regress_Issue06_FilenameSchemeAndReadCmdParity — Issue 06 (confusing .eml
// filenames). The scheme is:
//
//	email/<alias>/<YYYY-MM-DD>/HH.MM.SS__<from>__<subject>__uid<N>.eml
//
// We exercise the helpers and the filename builder directly (no FS), so the
// test is fast and deterministic regardless of the executable's location.
//
// Maps to AC-PROJ-28.
func Regress_Issue06_FilenameSchemeAndReadCmdParity(t *testing.T) {
	// 1. Helpers — guard the underlying transformations.
	if got := sanitizeReadable("Re: Check"); got != "re-check" {
		t.Errorf(`sanitizeReadable("Re: Check") = %q, want "re-check"`, got)
	}
	if got := sanitizeReadable("Abdullah.Mahin.Rasia@gmail.com"); got != "abdullah-mahin-rasia-gmail-com" {
		t.Errorf("sanitizeReadable email = %q", got)
	}
	if got := extractEmailAddr(`"Abdullah" <a.b@c.com>`); got != "a.b@c.com" {
		t.Errorf(`extractEmailAddr = %q, want "a.b@c.com"`, got)
	}
	if got := extractEmailAddr("plain@x.com"); got != "plain@x.com" {
		t.Errorf("extractEmailAddr fallback = %q", got)
	}

	// 2. Filename composition — exercise buildRawFilename directly.
	when := time.Date(2026, 4, 22, 19, 17, 14, 0, time.UTC)
	m := &Message{
		Uid:        12,
		From:       `"Abdullah Al Mahin" <abdullah.mahin.rasia@gmail.com>`,
		Subject:    "Re: Check",
		ReceivedAt: when,
	}
	name := buildRawFilename(when, m)

	wantParts := []string{
		"19.17.14",                       // HH.MM.SS prefix → chronological sort
		"abdullah-mahin-rasia-gmail-com", // sanitized sender
		"re-check",                       // sanitized subject
		"__uid12",                        // dedup suffix
	}
	for _, p := range wantParts {
		if !strings.Contains(name, p) {
			t.Errorf("filename %q missing required fragment %q — issue 06 regresses", name, p)
		}
	}
	if !strings.HasSuffix(name, ".eml") {
		t.Errorf("filename %q lacks .eml extension", name)
	}

	// 3. The HH.MM.SS prefix MUST be sortable: enforce digit shape.
	if !regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}__`).MatchString(name) {
		t.Errorf("filename %q does not start with HH.MM.SS__", name)
	}

	// 4. Empty subject / sender fall through to known sentinels (so files
	// don't collide as "..eml" or "__.eml").
	mEmpty := &Message{Uid: 7, From: "", Subject: "", ReceivedAt: when}
	nameEmpty := buildRawFilename(when, mEmpty)
	if !strings.Contains(nameEmpty, "unknown-sender") {
		t.Errorf("empty From should yield unknown-sender; got %q", nameEmpty)
	}
	if !strings.Contains(nameEmpty, "no-subject") {
		t.Errorf("empty Subject should yield no-subject; got %q", nameEmpty)
	}
}
