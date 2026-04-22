package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/mailclient"
)

func TestDiagnose_NoAccounts(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	// Pre-create an empty data dir so config.Load creates a fresh empty config.
	_ = os.MkdirAll(filepath.Join(tmp, "data"), 0o755)

	if err := Diagnose("", nil); err == nil {
		t.Fatal("expected error for no configured accounts")
	} else if !strings.Contains(err.Error(), "no accounts configured") {
		t.Fatalf("unexpected error: %v", err)
	}
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
