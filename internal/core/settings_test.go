package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func newTestSettings(t *testing.T) *Settings {
	t.Helper()
	r := NewSettings(func() time.Time { return time.Unix(1700000000, 0).UTC() })
	if r.HasError() {
		t.Fatalf("NewSettings: %v", r.Error())
	}
	return r.Value()
}

func TestSettings_GetReturnsDefaults(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		r := s.Get(context.Background())
		if r.HasError() {
			t.Fatalf("Get: %v", r.Error())
		}
		snap := r.Value()
		if snap.PollSeconds != 3 {
			t.Errorf("PollSeconds = %d, want 3", snap.PollSeconds)
		}
		if snap.Theme != ThemeDark {
			t.Errorf("Theme = %v, want ThemeDark", snap.Theme)
		}
		if !snap.AutoStartWatch {
			t.Error("AutoStartWatch should default to true")
		}
		if len(snap.OpenUrlAllowedSchemes) != 1 || snap.OpenUrlAllowedSchemes[0] != "https" {
			t.Errorf("OpenUrlAllowedSchemes = %v, want [https]", snap.OpenUrlAllowedSchemes)
		}
		if snap.ConfigPath == "" || snap.DataDir == "" || snap.EmailArchiveDir == "" {
			t.Error("path fields must be populated")
		}
	})
}

func TestSettings_SaveAndRoundTrip(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		in := SettingsInput{
			PollSeconds:           10,
			Theme:                 ThemeLight,
			OpenUrlAllowedSchemes: []string{"HTTPS", "http", "https"}, // dup + uppercase
			AllowLocalhostUrls:    true,
			AutoStartWatch:        false,
		}
		r := s.Save(context.Background(), in)
		if r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}
		got := r.Value()
		if got.PollSeconds != 10 {
			t.Errorf("PollSeconds = %d, want 10", got.PollSeconds)
		}
		if got.Theme != ThemeLight {
			t.Errorf("Theme = %v, want Light", got.Theme)
		}
		if len(got.OpenUrlAllowedSchemes) != 2 ||
			got.OpenUrlAllowedSchemes[0] != "http" ||
			got.OpenUrlAllowedSchemes[1] != "https" {
			t.Errorf("schemes not normalized: %v", got.OpenUrlAllowedSchemes)
		}
		if got.AutoStartWatch {
			t.Error("AutoStartWatch should be false")
		}
		// Re-Get from disk to confirm persistence.
		s2 := newTestSettings(t)
		again := s2.Get(context.Background())
		if again.HasError() {
			t.Fatalf("Get after Save: %v", again.Error())
		}
		if again.Value().PollSeconds != 10 {
			t.Errorf("persisted PollSeconds = %d", again.Value().PollSeconds)
		}
	})
}

func TestSettings_SavePreservesAccountsAndRules(t *testing.T) {
	withIsolatedConfig(t, func() {
		// Seed an account + rule via the existing config layer.
		cfg := config.Default()
		cfg.Accounts = append(cfg.Accounts, config.Account{
			Alias: "primary", Email: "u@example.com",
			ImapHost: "imap.example.com", ImapPort: 993, UseTLS: true,
			Mailbox: "INBOX",
		})
		cfg.Rules = append(cfg.Rules, config.Rule{
			Name: "auto-open", Enabled: true, UrlRegex: `https?://\S+`,
		})
		if err := config.Save(cfg); err != nil {
			t.Fatalf("seed config: %v", err)
		}
		s := newTestSettings(t)
		r := s.Save(context.Background(), DefaultSettingsInput())
		if r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}
		got, err := config.Load()
		if err != nil {
			t.Fatalf("reload config: %v", err)
		}
		if len(got.Accounts) != 1 || got.Accounts[0].Alias != "primary" {
			t.Errorf("account lost: %+v", got.Accounts)
		}
		if len(got.Rules) != 1 || got.Rules[0].Name != "auto-open" {
			t.Errorf("rule lost: %+v", got.Rules)
		}
	})
}

func TestSettings_ValidationErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		cases := []struct {
			name string
			in   SettingsInput
			code errtrace.Code
		}{
			{
				name: "poll too small",
				in:   SettingsInput{PollSeconds: 0, Theme: ThemeDark, OpenUrlAllowedSchemes: []string{"https"}},
				code: errtrace.ErrSettingsPollSeconds,
			},
			{
				name: "poll too large",
				in:   SettingsInput{PollSeconds: 999, Theme: ThemeDark, OpenUrlAllowedSchemes: []string{"https"}},
				code: errtrace.ErrSettingsPollSeconds,
			},
			{
				name: "bad theme",
				in:   SettingsInput{PollSeconds: 3, Theme: ThemeMode(99), OpenUrlAllowedSchemes: []string{"https"}},
				code: errtrace.ErrSettingsTheme,
			},
			{
				name: "javascript scheme",
				in:   SettingsInput{PollSeconds: 3, Theme: ThemeDark, OpenUrlAllowedSchemes: []string{"javascript"}},
				code: errtrace.ErrSettingsUrlScheme,
			},
			{
				name: "bad incognito arg",
				in: SettingsInput{
					PollSeconds: 3, Theme: ThemeDark,
					OpenUrlAllowedSchemes: []string{"https"},
					BrowserOverride:       BrowserOverride{IncognitoArg: "rm -rf /"},
				},
				code: errtrace.ErrSettingsIncognitoArg,
			},
			{
				name: "localhost without http",
				in: SettingsInput{
					PollSeconds: 3, Theme: ThemeDark,
					OpenUrlAllowedSchemes: []string{"https"},
					AllowLocalhostUrls:    true,
				},
				code: errtrace.ErrSettingsCompositeRule,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				// Bypass normalize default-substitution by leaving PollSeconds
				// > 0 except for the dedicated case.
				in := tc.in
				if tc.name != "poll too small" && in.PollSeconds == 0 {
					in.PollSeconds = 3
				}
				r := s.Save(context.Background(), in)
				if !r.HasError() {
					t.Fatalf("expected error %s, got none", tc.code)
				}
				if !contains(r.Error().Error(), string(tc.code)) {
					t.Errorf("error %v does not carry code %s", r.Error(), tc.code)
				}
			})
		}
	})
}

func TestSettings_ChromePathValidation(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		// Non-absolute path
		in := DefaultSettingsInput()
		in.BrowserOverride.ChromePath = "chrome"
		r := s.Save(context.Background(), in)
		if !r.HasError() || !contains(r.Error().Error(), string(errtrace.ErrSettingsChromePath)) {
			t.Fatalf("expected ChromePath error, got %v", r.Error())
		}
		// Absolute but missing.
		in.BrowserOverride.ChromePath = "/no/such/chrome"
		r = s.Save(context.Background(), in)
		if !r.HasError() {
			t.Fatal("expected stat error")
		}
		// Real file (a tempfile) → passes.
		f, err := os.CreateTemp(t.TempDir(), "chrome-*")
		if err != nil {
			t.Fatalf("tempfile: %v", err)
		}
		f.Close()
		in.BrowserOverride.ChromePath = f.Name()
		r = s.Save(context.Background(), in)
		if r.HasError() {
			t.Fatalf("expected success with real file, got %v", r.Error())
		}
	})
}

func TestSettings_ResetToDefaults(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		// Mutate first.
		_ = s.Save(context.Background(), SettingsInput{
			PollSeconds: 30, Theme: ThemeLight,
			OpenUrlAllowedSchemes: []string{"https"},
			AutoStartWatch:        false,
		})
		r := s.ResetToDefaults(context.Background())
		if r.HasError() {
			t.Fatalf("Reset: %v", r.Error())
		}
		snap := r.Value()
		if snap.PollSeconds != 3 || snap.Theme != ThemeDark || !snap.AutoStartWatch {
			t.Errorf("reset did not apply defaults: %+v", snap)
		}
	})
}

func TestSettings_SubscribeReceivesEvents(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		ch, cancel := s.Subscribe(context.Background())
		defer cancel()

		r := s.Save(context.Background(), SettingsInput{
			PollSeconds: 5, Theme: ThemeDark,
			OpenUrlAllowedSchemes: []string{"https"},
		})
		if r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}
		select {
		case ev := <-ch:
			if ev.Kind != SettingsSaved {
				t.Errorf("Kind = %v, want SettingsSaved", ev.Kind)
			}
			if ev.Snapshot.PollSeconds != 5 {
				t.Errorf("event PollSeconds = %d", ev.Snapshot.PollSeconds)
			}
		case <-time.After(time.Second):
			t.Fatal("no event delivered within 1s")
		}

		// Reset emits a different kind.
		_ = s.ResetToDefaults(context.Background())
		select {
		case ev := <-ch:
			if ev.Kind != SettingsResetApplied {
				t.Errorf("Kind = %v, want SettingsResetApplied", ev.Kind)
			}
		case <-time.After(time.Second):
			t.Fatal("no reset event")
		}
	})
}

func TestSettings_SubscribeCancelStopsDelivery(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		ch, cancel := s.Subscribe(context.Background())
		cancel()
		// Channel should be closed.
		_, ok := <-ch
		if ok {
			t.Error("expected channel to be closed after cancel")
		}
		// Saving must not panic.
		_ = s.Save(context.Background(), SettingsInput{
			PollSeconds: 7, Theme: ThemeDark,
			OpenUrlAllowedSchemes: []string{"https"},
		})
	})
}

func TestSettings_DetectChromeNotFoundIsNotError(t *testing.T) {
	withIsolatedConfig(t, func() {
		// Point env at a non-existent path; the probe should still succeed
		// (NotFound is documented as success-with-empty).
		t.Setenv("EMAIL_READ_CHROME", filepath.Join(t.TempDir(), "no-chrome"))
		// Also ensure PATH lookups won't accidentally hit a real browser by
		// pointing PATH at an empty dir.
		t.Setenv("PATH", t.TempDir())
		s := newTestSettings(t)
		r := s.DetectChrome(context.Background())
		if r.HasError() {
			t.Fatalf("DetectChrome should not error: %v", r.Error())
		}
		// On hosts that have a real OS-default Chrome installed, we may get
		// ChromeFromOsDefault; otherwise NotFound. Both are valid here —
		// we only assert no error and a known enum value.
		switch r.Value().Source {
		case ChromeNotFound, ChromeFromOsDefault, ChromeFromPath, ChromeFromConfig, ChromeFromEnv:
		default:
			t.Errorf("unknown source: %v", r.Value().Source)
		}
	})
}

func TestParseThemeMode(t *testing.T) {
	cases := []struct {
		in    string
		want  ThemeMode
		valid bool
	}{
		{"", ThemeDark, true},
		{"Dark", ThemeDark, true},
		{"Light", ThemeLight, true},
		{"System", ThemeSystem, true},
		{"dark", ThemeDark, false},
		{"bogus", ThemeDark, false},
	}
	for _, tc := range cases {
		got, ok := ParseThemeMode(tc.in)
		if ok != tc.valid || got != tc.want {
			t.Errorf("ParseThemeMode(%q) = (%v,%v), want (%v,%v)",
				tc.in, got, ok, tc.want, tc.valid)
		}
	}
}

// contains is a tiny helper to avoid pulling in strings just for one use.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
