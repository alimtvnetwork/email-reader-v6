package mailclient

import (
	"os"
	"path/filepath"
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
// Verifies:
//   - sanitizeReadable lowercases + hyphenates correctly
//   - extractEmailAddr pulls "addr@host" from "Display <addr@host>"
//   - SaveRaw produces a path that contains the timestamp prefix, the
//     sanitized sender, the sanitized subject AND the __uidN suffix
//   - the day folder uses the YYYY-MM-DD shape so files sort
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

	// 2. Filename scheme — exercise SaveRaw against a temp dir.
	tmp := t.TempDir()
	t.Setenv("EMAIL_READ_DATA_DIR", tmp) // honored by config.EmailDir if supported
	// SaveRaw resolves email dir via config.EmailDir(); to keep the test
	// independent of env-var support, also chdir into tmp.
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	defer os.Chdir(prev)

	when := time.Date(2026, 4, 22, 19, 17, 14, 0, time.UTC)
	m := &Message{
		Uid:        12,
		From:       `"Abdullah Al Mahin" <abdullah.mahin.rasia@gmail.com>`,
		Subject:    "Re: Check",
		ReceivedAt: when,
		Raw:        []byte("From: x\r\n\r\nbody"),
	}
	path, err := SaveRaw("admin", m)
	if err != nil {
		t.Fatalf("SaveRaw: %v", err)
	}

	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))

	if parent != "2026-04-22" {
		t.Errorf("day folder = %q, want %q", parent, "2026-04-22")
	}
	wantParts := []string{
		"19.17.14",                       // HH.MM.SS prefix → chronological sort
		"abdullah-mahin-rasia-gmail-com", // sanitized sender
		"re-check",                       // sanitized subject
		"__uid12",                        // dedup suffix
	}
	for _, p := range wantParts {
		if !strings.Contains(base, p) {
			t.Errorf("filename %q missing required fragment %q — issue 06 regresses", base, p)
		}
	}
	if !strings.HasSuffix(base, ".eml") {
		t.Errorf("filename %q lacks .eml extension", base)
	}

	// 3. The hh.mm.ss prefix MUST be sortable: enforce digit shape.
	if !regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}__`).MatchString(base) {
		t.Errorf("filename %q does not start with HH.MM.SS__", base)
	}
}
