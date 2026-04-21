package watcher

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/store"
)

// TestStoreFlow exercises the store hooks the watcher relies on:
// upsert dedup + url dedup. The IMAP/network parts are covered separately
// by mailclient_test.go.
func TestStoreFlow(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenAt(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	e := &store.Email{
		Alias: "a", MessageId: "<m1@x>", Uid: 5,
		Subject: "hi", ReceivedAt: time.Now(),
	}
	id1, ins1, err := st.UpsertEmail(ctx, e)
	if err != nil || !ins1 || id1 == 0 {
		t.Fatalf("first upsert: id=%d ins=%v err=%v", id1, ins1, err)
	}
	id2, ins2, err := st.UpsertEmail(ctx, e)
	if err != nil || ins2 || id2 != id1 {
		t.Fatalf("dedup upsert: id=%d ins=%v err=%v (want id=%d ins=false)", id2, ins2, err, id1)
	}

	ok, err := st.RecordOpenedUrl(ctx, id1, "r", "https://x/y")
	if err != nil || !ok {
		t.Fatalf("first record: ok=%v err=%v", ok, err)
	}
	ok, err = st.RecordOpenedUrl(ctx, id1, "r", "https://x/y")
	if err != nil || ok {
		t.Fatalf("dedup record: ok=%v err=%v", ok, err)
	}
}
