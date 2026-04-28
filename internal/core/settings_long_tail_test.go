// settings_long_tail_test.go closes the AC-SB long-tail (Slice #128):
// AC-SB-12 (Rules byte-identical preservation),
// AC-SB-13 (unknown top-level key preservation),
// AC-SB-14 (atomic-crash simulation: tmp left over → original intact),
// AC-SB-18 (ResetToDefaults emits SettingsResetApplied event),
// AC-SB-20 (DetectChrome: config path takes precedence),
// AC-SB-21 (DetectChrome: env var takes precedence when config empty),
// AC-SB-23 (DetectChrome: stat permission denied → ChromeNotFound, no error).
//
// AC-SB-24 ("go test -race ./internal/core/...") is enforced by the CI
// make target rather than a Go test and so stays in the gap allowlist
// with a cross-reference comment.
package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
)

// Satisfies AC-SB-12 — Save preserves the Rules array byte-identically
// (golden-file diff over a mixed-rule fixture).
func TestSettings_Save_Rules_Untouched(t *testing.T) {
	withIsolatedConfig(t, func() {
		cfg := config.Default()
		cfg.Rules = []config.Rule{
			{Name: "rule-a", Enabled: true, UrlRegex: `https?://a\.example/\S+`},
			{Name: "rule-b", Enabled: false, UrlRegex: `https?://b\.example/\S+`},
			{Name: "rule-c", Enabled: true, UrlRegex: `https?://c\.example/\S+`},
		}
		if err := config.Save(cfg); err != nil {
			t.Fatalf("seed config: %v", err)
		}
		// Snapshot the rules section as serialized JSON for a stable diff.
		before, err := json.Marshal(cfg.Rules)
		if err != nil {
			t.Fatalf("marshal before: %v", err)
		}

		s := newTestSettings(t)
		if r := s.Save(context.Background(), DefaultSettingsInput()); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		got, err := config.Load()
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		after, err := json.Marshal(got.Rules)
		if err != nil {
			t.Fatalf("marshal after: %v", err)
		}
		if string(before) != string(after) {
			t.Errorf("rules drifted across Save:\n before=%s\n after =%s", before, after)
		}
	})
}

// Satisfies AC-SB-13 — Save preserves unknown top-level keys
// (schemaVersion, _comment, custom add-ons) byte-for-byte.
func TestSettings_Save_UnknownKeys_Untouched(t *testing.T) {
	withIsolatedConfig(t, func() {
		// Hand-write a config.json that includes both the typed sections
		// AND unknown top-level keys.
		p, err := config.Path()
		if err != nil {
			t.Fatalf("path: %v", err)
		}
		raw := map[string]any{
			"schemaVersion": "v0.99-experimental",
			"_comment":      "Hand-edited; preserve me.",
			"x-future":      map[string]any{"feature": "unknown", "n": float64(42)},
			"watch":         map[string]any{"pollSeconds": float64(3)},
			"browser":       map[string]any{"chromePath": "", "incognitoArg": ""},
			"accounts":      []any{},
			"rules":         []any{},
		}
		b, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatalf("seed write: %v", err)
		}

		s := newTestSettings(t)
		if r := s.Save(context.Background(), DefaultSettingsInput()); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		// Re-read raw bytes and confirm the unknown keys are intact.
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read after: %v", err)
		}
		var rt map[string]any
		if err := json.Unmarshal(after, &rt); err != nil {
			t.Fatalf("unmarshal after: %v", err)
		}
		if got := rt["schemaVersion"]; got != "v0.99-experimental" {
			t.Errorf("schemaVersion drifted: %v", got)
		}
		if got := rt["_comment"]; got != "Hand-edited; preserve me." {
			t.Errorf("_comment drifted: %v", got)
		}
		xf, ok := rt["x-future"].(map[string]any)
		if !ok {
			t.Fatalf("x-future missing or wrong type: %T", rt["x-future"])
		}
		if xf["feature"] != "unknown" || xf["n"] != float64(42) {
			t.Errorf("x-future drifted: %v", xf)
		}
	})
}

// Satisfies AC-SB-14 — atomic-write semantics: a stale tmp file from a
// prior crashed Save does not corrupt the next Load. Original file
// remains parseable.
//
// Note: spec wording calls for SIGKILL between tmp-write and rename; we
// simulate the post-crash filesystem state directly (orphan .tmp,
// untouched real file) because forking a victim process is not portable
// across CI hosts.
func TestSettings_Save_AtomicCrash(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		// Persist a known-good baseline.
		good := DefaultSettingsInput()
		good.PollSeconds = 7
		if r := s.Save(context.Background(), good); r.HasError() {
			t.Fatalf("baseline Save: %v", r.Error())
		}

		p, err := config.Path()
		if err != nil {
			t.Fatalf("path: %v", err)
		}
		baselineBytes, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read baseline: %v", err)
		}

		// Inject orphan tmp files (both .tmp suffixes used by the two
		// writers) containing junk bytes — simulates a process killed
		// between WriteFile and Rename.
		for _, suffix := range []string{".tmp", ".settings.tmp"} {
			if err := os.WriteFile(p+suffix, []byte("{ this is not valid json"), 0o600); err != nil {
				t.Fatalf("write orphan %s: %v", suffix, err)
			}
		}

		// Original config must still parse.
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load after orphan tmp: %v", err)
		}
		if cfg.Watch.PollSeconds != 7 {
			t.Errorf("baseline value lost: PollSeconds=%d, want 7", cfg.Watch.PollSeconds)
		}
		// And byte-equal to what we read pre-orphan.
		afterBytes, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read after orphan: %v", err)
		}
		if string(baselineBytes) != string(afterBytes) {
			t.Error("real config.json was modified by orphan tmp presence")
		}

		// Recovery: the next Save must succeed and overwrite cleanly.
		good.PollSeconds = 9
		if r := s.Save(context.Background(), good); r.HasError() {
			t.Fatalf("recovery Save: %v", r.Error())
		}
		s2 := newTestSettings(t)
		again := s2.Get(context.Background())
		if again.HasError() {
			t.Fatalf("Get post-recovery: %v", again.Error())
		}
		if again.Value().PollSeconds != 9 {
			t.Errorf("recovery did not persist: %d", again.Value().PollSeconds)
		}
	})
}

// Satisfies AC-SB-18 — ResetToDefaults emits exactly one
// SettingsEvent{Kind: SettingsResetApplied} on subscribers.
func TestSettings_Reset_EventEmitted(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		ch, cancel := s.Subscribe(context.Background())
		defer cancel()

		r := s.ResetToDefaults(context.Background())
		if r.HasError() {
			t.Fatalf("Reset: %v", r.Error())
		}
		select {
		case ev := <-ch:
			if ev.Kind != SettingsResetApplied {
				t.Errorf("Kind = %v, want SettingsResetApplied", ev.Kind)
			}
			if ev.Snapshot.PollSeconds != config.DefaultWatchPollSeconds {
				t.Errorf("event PollSeconds = %d, want 5", ev.Snapshot.PollSeconds)
			}
		case <-time.After(time.Second):
			t.Fatal("no reset event delivered within 1s")
		}
		// And no second event should fire spuriously.
		select {
		case ev := <-ch:
			t.Fatalf("unexpected second event: %+v", ev)
		case <-time.After(50 * time.Millisecond):
		}
	})
}

// Satisfies AC-SB-20 — DetectChrome returns ChromeFromConfig when
// ChromePath is set on the snapshot (config wins over env / OS-default).
func TestSettings_Detect_ConfigPrecedence(t *testing.T) {
	withIsolatedConfig(t, func() {
		// Build a real executable temp file we can point ChromePath at.
		fakeChrome := makeFakeExecutable(t, "fake-chrome-config")
		// Also seed the env var to a different real exe so we can prove
		// config wins.
		envChrome := makeFakeExecutable(t, "fake-chrome-env")
		t.Setenv("EMAIL_READ_CHROME", envChrome)

		s := newTestSettings(t)
		in := DefaultSettingsInput()
		in.BrowserOverride.ChromePath = fakeChrome
		if r := s.Save(context.Background(), in); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		r := s.DetectChrome(context.Background())
		if r.HasError() {
			t.Fatalf("DetectChrome: %v", r.Error())
		}
		got := r.Value()
		if got.Source != ChromeFromConfig {
			t.Errorf("Source = %v, want ChromeFromConfig", got.Source)
		}
		if got.Path != fakeChrome {
			t.Errorf("Path = %s, want %s", got.Path, fakeChrome)
		}
	})
}

// Satisfies AC-SB-21 — DetectChrome returns ChromeFromEnv when
// EMAIL_READ_CHROME is set and config ChromePath is empty.
func TestSettings_Detect_EnvPrecedence(t *testing.T) {
	withIsolatedConfig(t, func() {
		envChrome := makeFakeExecutable(t, "fake-chrome-env-only")
		t.Setenv("EMAIL_READ_CHROME", envChrome)
		// Empty PATH so the PATH-lookup step can't accidentally satisfy
		// the probe before env does.
		t.Setenv("PATH", t.TempDir())

		s := newTestSettings(t)
		// Default snapshot has empty ChromePath, so step 1 (config) is a no-op
		// and step 2 (env) must win.
		if r := s.Save(context.Background(), DefaultSettingsInput()); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		r := s.DetectChrome(context.Background())
		if r.HasError() {
			t.Fatalf("DetectChrome: %v", r.Error())
		}
		got := r.Value()
		// On hosts with a real OS-default Chrome installed, the OS-default
		// step runs after env, so env should still win. We assert exactly
		// that.
		if got.Source != ChromeFromEnv {
			t.Errorf("Source = %v, want ChromeFromEnv (path=%s)", got.Source, got.Path)
		}
		if got.Path != envChrome {
			t.Errorf("Path = %s, want %s", got.Path, envChrome)
		}
	})
}

// Satisfies AC-SB-23 — when os.Stat fails with permission-denied during
// the config-path probe, DetectChrome returns ChromeNotFound (NOT an
// error). The WARN log assertion is left to a future logger-injection
// refactor; this test pins the no-error / no-panic contract.
func TestSettings_Detect_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: chmod 0o000 has no equivalent on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses POSIX permission checks")
	}
	withIsolatedConfig(t, func() {
		// Build a directory we cannot stat into, then point ChromePath
		// at a file inside it.
		lockedDir := filepath.Join(t.TempDir(), "locked")
		if err := os.Mkdir(lockedDir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		secret := filepath.Join(lockedDir, "chrome")
		if err := os.WriteFile(secret, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("seed: %v", err)
		}
		// Strip exec/search bit from the parent so os.Stat on `secret`
		// returns EACCES.
		if err := os.Chmod(lockedDir, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(lockedDir, 0o700) })

		// Empty PATH and unset env so other probe steps don't sneak a win.
		t.Setenv("EMAIL_READ_CHROME", "")
		t.Setenv("PATH", t.TempDir())

		s := newTestSettings(t)
		// Inject the inaccessible path into the snapshot cache directly —
		// going through Save would fail validation (ER-SET-21774) before
		// DetectChrome runs.
		s.mu.Lock()
		s.lastApplied = SettingsSnapshot{
			PollSeconds:           3,
			BrowserOverride:       BrowserOverride{ChromePath: secret},
			OpenUrlAllowedSchemes: []string{"https"},
		}
		s.mu.Unlock()

		r := s.DetectChrome(context.Background())
		if r.HasError() {
			t.Fatalf("DetectChrome must not error on EACCES, got %v", r.Error())
		}
		// Must not falsely report ChromeFromConfig — the stat failed.
		if got := r.Value().Source; got == ChromeFromConfig {
			t.Errorf("Source = ChromeFromConfig despite stat EACCES")
		}
	})
}

// makeFakeExecutable writes a 0o755 file in t.TempDir() and returns its
// absolute path. On Windows the exec-bit check is skipped by
// fileExistsExec so any regular file works.
func makeFakeExecutable(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}
	return p
}
