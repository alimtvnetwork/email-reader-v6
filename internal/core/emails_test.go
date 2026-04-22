package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/store"
)

// withIsolatedStore points config.DataDir at a temp dir for the duration of
// the test by setting a fake EXE location via the OS env.
// We can't easily redirect config.Path without a refactor, so instead we
// open the store directly at a temp path and exercise core helpers that
// accept a path. For now, core.ListEmails uses store.Open() (real path),
// so these tests instead drive the lower-level store directly to verify
// the SQL we added, then call snippet/toSummary unit-only.

func TestSnippet_PrefersTextThenStripsHtml(t *testing.T) {
	if got := snippet("hello   world", ""); got != "hello world" {
		t.Fatalf("plain text snippet wrong: %q", got)
	}
	if got := snippet("", "<p>hi <b>there</b></p>"); got != "hi there" {
		t.Fatalf("html snippet wrong: %q", got)
	}
	long := ""
	for i := 0; i < 200; i++ {
		long += "a"
	}
	got := snippet(long, "")
	runes := []rune(got)
	if len(runes) > 140 || runes[len(runes)-1] != '…' {
		t.Fatalf("expected truncated snippet, got len=%d %q", len(runes), got)
	}
}

func TestStoreListEmails_FilterAndSearch(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenAt(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	rows := []store.Email{
		{Alias: "a", MessageId: "<1@x>", Uid: 1, FromAddr: "alice@x", Subject: "Hello world", ReceivedAt: time.Now()},
		{Alias: "a", MessageId: "<2@x>", Uid: 2, FromAddr: "bob@x", Subject: "Receipt #42", ReceivedAt: time.Now()},
		{Alias: "b", MessageId: "<3@x>", Uid: 1, FromAddr: "carol@y", Subject: "Other inbox", ReceivedAt: time.Now()},
	}
	for i := range rows {
		if _, _, err := st.UpsertEmail(ctx, &rows[i]); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	all, err := st.ListEmails(ctx, store.EmailQuery{})
	if err != nil || len(all) != 3 {
		t.Fatalf("list all: got %d err=%v", len(all), err)
	}
	if all[0].Uid != 2 {
		t.Fatalf("expected newest-first ordering, got first uid=%d", all[0].Uid)
	}
	byAlias, _ := st.ListEmails(ctx, store.EmailQuery{Alias: "a"})
	if len(byAlias) != 2 {
		t.Fatalf("alias filter expected 2, got %d", len(byAlias))
	}
	bySearch, _ := st.ListEmails(ctx, store.EmailQuery{Search: "receipt"})
	if len(bySearch) != 1 || bySearch[0].Uid != 2 {
		t.Fatalf("search filter unexpected: %+v", bySearch)
	}
	limited, _ := st.ListEmails(ctx, store.EmailQuery{Limit: 1})
	if len(limited) != 1 {
		t.Fatalf("limit failed: got %d", len(limited))
	}
	n, _ := st.CountEmails(ctx, "a")
	if n != 2 {
		t.Fatalf("count alias=a expected 2, got %d", n)
	}
	nAll, _ := st.CountEmails(ctx, "")
	if nAll != 3 {
		t.Fatalf("count all expected 3, got %d", nAll)
	}
}

// Make sure config import doesn't get pruned by goimports — referenced via
// store.OpenAt only, so explicitly use the package.
var _ = config.Default
