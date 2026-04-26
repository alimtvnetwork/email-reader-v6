// pollonce_test.go — behavior tests for the exported Watcher
// adapter. Avoids real IMAP by exercising only the registry +
// dispatch logic; the pollOnce body itself is covered indirectly by
// the existing Run-loop tests.

package watcher

import (
	"context"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

func TestNewWatcher_StartsEmpty(t *testing.T) {
	t.Parallel()
	w := NewWatcher()
	if w == nil || w.accounts == nil {
		t.Fatal("NewWatcher returned nil or unmapped registry")
	}
	if len(w.accounts) != 0 {
		t.Errorf("fresh registry size = %d, want 0", len(w.accounts))
	}
}

func TestWatcher_Register_RejectsEmptyAlias(t *testing.T) {
	t.Parallel()
	w := NewWatcher()
	err := w.Register(Options{Account: config.Account{Alias: ""}})
	if err == nil {
		t.Fatal("Register should reject empty alias")
	}
	if !strings.Contains(err.Error(), "Alias") {
		t.Errorf("err = %v, want message mentioning Alias", err)
	}
}

func TestWatcher_PollOnce_UnknownAlias_ReturnsError(t *testing.T) {
	t.Parallel()
	w := NewWatcher()
	err := w.PollOnce(context.Background(), "ghost")
	if err == nil {
		t.Fatal("PollOnce on unregistered alias should error")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("err = %v, want alias %q in message", err, "ghost")
	}
}

func TestWatcher_Register_OverwritesPriorOptions(t *testing.T) {
	t.Parallel()
	w := NewWatcher()
	a1 := Options{Account: config.Account{Alias: "primary", Email: "v1@x"}}
	a2 := Options{Account: config.Account{Alias: "primary", Email: "v2@x"}}
	if err := w.Register(a1); err != nil {
		t.Fatalf("Register a1: %v", err)
	}
	if err := w.Register(a2); err != nil {
		t.Fatalf("Register a2: %v", err)
	}
	w.mu.RLock()
	got := w.accounts["primary"].Account.Email
	w.mu.RUnlock()
	if got != "v2@x" {
		t.Errorf("after overwrite Email = %q, want v2@x", got)
	}
	if len(w.accounts) != 1 {
		t.Errorf("registry size = %d, want 1 (overwrite, not append)", len(w.accounts))
	}
}
