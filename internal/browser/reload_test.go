package browser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// Test_Reload_RearmsResolver verifies CF-T1: after Reload the next Path
// call re-runs the resolver against the new cfg, so a Settings update
// flows through without restarting the process.
func Test_Reload_RearmsResolver(t *testing.T) {
	dir := t.TempDir()
	a := writeFakeBrowser(t, dir, "alpha")
	b := writeFakeBrowser(t, dir, "beta")

	l := New(config.Browser{ChromePath: a})
	got, err := l.Path()
	if err != nil || got != a {
		t.Fatalf("first Path = %q,%v want %q,nil", got, err, a)
	}

	// Reload with a different path; next Path must reflect it.
	l.Reload(config.Browser{ChromePath: b})
	got2, err := l.Path()
	if err != nil || got2 != b {
		t.Fatalf("post-Reload Path = %q,%v want %q,nil", got2, err, b)
	}

	// IncognitoArg also re-resolves.
	if arg := l.IncognitoArg(); arg == "" {
		t.Fatalf("IncognitoArg = empty after Reload; want non-empty")
	}
}

// Test_Reload_ConcurrentSafe is a smoke test: Reload + Path racing in
// parallel must never panic. Use `go test -race` to catch data races.
func Test_Reload_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	p := writeFakeBrowser(t, dir, "x")
	l := New(config.Browser{ChromePath: p})

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			l.Reload(config.Browser{ChromePath: p})
		}
		close(done)
	}()
	for i := 0; i < 200; i++ {
		_, _ = l.Path()
	}
	<-done
}

// writeFakeBrowser drops an executable file so fileExists() returns true.
func writeFakeBrowser(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake browser: %v", err)
	}
	return p
}
