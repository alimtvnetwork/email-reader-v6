package core

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

func Test_Doctor_NoAccounts_ReturnsCodedError(t *testing.T) {
	withTempConfig(t, func() {
		_ = os.MkdirAll("data", 0o755)
		r := Doctor("")
		if !r.HasError() {
			t.Fatal("Doctor() with no accounts: want error, got nil")
		}
		// Don't assert exact code text — just confirm it surfaces SOMETHING.
		if r.Error().Error() == "" {
			t.Fatal("Doctor() error has empty message")
		}
	})
}

func Test_Doctor_HiddenChars_FlaggedAndDumped(t *testing.T) {
	withTempConfig(t, func() {
		// Password with a U+200B (zero-width space) — sanitization will strip it.
		raw := "pa\u200bss"
		writeOneAccount(t, "atto", "atto@example.com", raw)

		r := Doctor("")
		if r.HasError() {
			t.Fatalf("Doctor() unexpected error: %v", r.Error())
		}
		reps := r.Value()
		if len(reps) != 1 {
			t.Fatalf("got %d reports, want 1", len(reps))
		}
		rep := reps[0]
		if rep.Alias != "atto" || rep.Email != "atto@example.com" {
			t.Errorf("alias/email mismatch: %+v", rep)
		}
		if !rep.Hidden {
			t.Error("Hidden = false; want true (raw contains U+200B)")
		}
		if rep.RuneCount != 4 || rep.StoredBytes != len(raw) {
			t.Errorf("counts: stored=%d (want %d), runes=%d (want 4)",
				rep.StoredBytes, len(raw), rep.RuneCount)
		}
		if len(rep.Sanitized) != 4 {
			t.Errorf("sanitized dump len=%d, want 4", len(rep.Sanitized))
		}
		if len(rep.Raw) == 0 {
			t.Error("Raw dump empty — should be populated when Hidden=true")
		}
	})
}

func Test_Doctor_TargetFilter_NotFound(t *testing.T) {
	withTempConfig(t, func() {
		writeOneAccount(t, "atto", "atto@example.com", "secret")
		r := Doctor("nope")
		if !r.HasError() {
			t.Fatal("want error for missing alias, got nil")
		}
	})
}

func Test_Doctor_TargetFilter_Match(t *testing.T) {
	withTempConfig(t, func() {
		writeOneAccount(t, "atto", "atto@example.com", "secret")
		r := Doctor("atto")
		if r.HasError() {
			t.Fatalf("Doctor(\"atto\"): %v", r.Error())
		}
		if len(r.Value()) != 1 {
			t.Errorf("got %d reports for matching alias, want 1", len(r.Value()))
		}
	})
}

// withTempConfig points config.Load at a temp dir and restores cwd on exit.
func withTempConfig(t *testing.T, fn func()) {
	t.Helper()
	tmp := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(tmp, "data"), 0o755)
	fn()
}

// writeOneAccount drops a config.json with one account whose password is
// the raw (un-sanitized) `pw` string, base64-encoded directly.
func writeOneAccount(t *testing.T, alias, email, pw string) {
	t.Helper()
	cfg := &config.Config{Accounts: []config.Account{{
		Alias:       alias,
		Email:       email,
		PasswordB64: base64.StdEncoding.EncodeToString([]byte(pw)),
		ImapHost:    "imap.example.com",
		ImapPort:    993,
		UseTLS:      true,
		Mailbox:     "INBOX",
	}}}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}
