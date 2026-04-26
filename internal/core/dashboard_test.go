package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// snapshotAndRestore preserves a file across the test so sibling tests
// (which load the same global config.json / emails.db next to the
// executable) don't see our seeded fixtures.
func snapshotAndRestore(t *testing.T, p string) {
	t.Helper()
	orig, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		t.Cleanup(func() { _ = os.Remove(p) })
		return
	}
	if err != nil {
		t.Fatalf("snapshot %s: %v", p, err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(p, orig, 0o600); err != nil {
			t.Errorf("restore %s: %v", p, err)
		}
	})
}

func TestLoadDashboardStats(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })

	cfgPath, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(filepath.Dir(cfgPath), "emails.db")
	snapshotAndRestore(t, cfgPath)
	snapshotAndRestore(t, dbPath)

	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Accounts: []config.Account{
			{Alias: "a", Email: "a@x", ImapHost: "h", ImapPort: 993, UseTLS: true, Mailbox: "INBOX"},
			{Alias: "b", Email: "b@x", ImapHost: "h", ImapPort: 993, UseTLS: true, Mailbox: "INBOX"},
		},
		Rules: []config.Rule{
			{Name: "r1", Enabled: true, UrlRegex: "https?://.+"},
			{Name: "r2", Enabled: false, UrlRegex: "https?://.+"},
		},
		Watch: config.Watch{PollSeconds: 3},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save cfg: %v", err)
	}

	res := LoadDashboardStats(context.Background(), "a")
	if res.HasError() {
		t.Fatalf("LoadDashboardStats: %v", res.Error())
	}
	stats := res.Value()
	if stats.Accounts != 2 {
		t.Errorf("Accounts = %d, want 2", stats.Accounts)
	}
	if stats.RulesTotal != 2 || stats.RulesEnabled != 1 {
		t.Errorf("rules = %d/%d, want 1/2", stats.RulesEnabled, stats.RulesTotal)
	}
	if stats.Alias != "a" {
		t.Errorf("Alias = %q, want %q", stats.Alias, "a")
	}
}
