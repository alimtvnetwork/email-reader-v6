package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/mailclient"
)

func TestDiagnose_NoAccounts(t *testing.T) {
	// Use withIsolatedConfig so config.json is actually isolated —
	// the old inline chdir was a no-op (config.Path resolves via
	// os.Executable, not cwd) and let prior tests' atto account
	// leak in under `-count>1`.
	withIsolatedConfig(t, func() {
		tmp := t.TempDir()
		old, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(old) })
		if err := os.Chdir(tmp); err != nil {
			t.Fatal(err)
		}
		_ = os.MkdirAll(filepath.Join(tmp, "data"), 0o755)

		res := Diagnose("", nil)
		if !res.HasError() {
			t.Fatal("expected error for no configured accounts")
		}
		if !strings.Contains(res.Error().Error(), "no accounts configured") {
			t.Fatalf("unexpected error: %v", res.Error())
		}
	})
}

func TestSummarize(t *testing.T) {
	cases := []struct {
		name           string
		stats          mailclient.MailboxStats
		foundElsewhere bool
		want           string
	}{
		{"baseline-no-other", mailclient.MailboxStats{Messages: 1, UidNext: 2}, false, "routing/delivery before IMAP"},
		{"baseline-other", mailclient.MailboxStats{Messages: 1, UidNext: 2}, true, "Spam/Junk/Sent/All Mail"},
		{"has-mail", mailclient.MailboxStats{Messages: 5, UidNext: 10}, false, "more mail than the watcher baseline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarize(tc.stats, tc.foundElsewhere)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("summarize() = %q, want substring %q", got, tc.want)
			}
		})
	}
}
