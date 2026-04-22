package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/store"
)

// TestLoadDashboardStats sets up an isolated config + store under a tmp cwd,
// inserts two emails for two aliases, then verifies every counter.
func TestLoadDashboardStats(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Seed config with 2 accounts + 2 rules (one disabled).
	cfg := &config.Config{
		Accounts: []config.Account{
			{Alias: "a", Email: "a@x", ImapHost: "h", ImapPort: 993, UseTLS: true, Mailbox: "INBOX"},
			{Alias: "b", Email: "b@x", ImapHost: "h", ImapPort: 993, UseTLS: true, Mailbox: "INBOX"},
		},
		Rules: []config.Rule{
			{Name: "r1", Enabled: true, UrlRegex: "https?://.+"},
			{Name: "r2", Enabled: false, UrlRegex: "https?://.+"},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Seed store with 1 email for alias "a" and 2 for alias "b".
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for i, e := range []*store.Email{
		{Alias: "a", MessageId: "<m1@x>", Uid: 1, Subject: "s1", ReceivedAt: time.Now()},
		{Alias: "b", MessageId: "<m2@x>", Uid: 2, Subject: "s2", ReceivedAt: time.Now()},
		{Alias: "b", MessageId: "<m3@x>", Uid: 3, Subject: "s3", ReceivedAt: time.Now()},
	} {
		if _, _, err := st.UpsertEmail(ctx, e); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	got, err := LoadDashboardStats(ctx, "b")
	if err != nil {
		t.Fatalf("LoadDashboardStats: %v", err)
	}
	want := DashboardStats{
		Accounts: 2, RulesTotal: 2, RulesEnabled: 1,
		EmailsTotal: 3, EmailsForAlias: 2, Alias: "b",
	}
	if got != want {
		t.Fatalf("stats mismatch:\n got=%+v\nwant=%+v", got, want)
	}

	// With empty alias EmailsForAlias must be zero (not summed).
	got2, err := LoadDashboardStats(ctx, "")
	if err != nil {
		t.Fatalf("LoadDashboardStats(empty): %v", err)
	}
	if got2.EmailsForAlias != 0 {
		t.Errorf("EmailsForAlias = %d, want 0 for empty alias", got2.EmailsForAlias)
	}
	if got2.EmailsTotal != 3 {
		t.Errorf("EmailsTotal = %d, want 3", got2.EmailsTotal)
	}
}
