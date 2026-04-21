package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertEmailAndDedup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	e := &Email{
		Alias:      "atto",
		MessageId:  "<abc@example.com>",
		Uid:        42,
		FromAddr:   "x@y",
		Subject:    "hello",
		ReceivedAt: time.Now(),
		FilePath:   "/tmp/x.eml",
	}
	id1, inserted, err := s.UpsertEmail(ctx, e)
	if err != nil || !inserted || id1 == 0 {
		t.Fatalf("first insert failed: id=%d ins=%v err=%v", id1, inserted, err)
	}
	id2, inserted, err := s.UpsertEmail(ctx, e)
	if err != nil {
		t.Fatalf("dedup err: %v", err)
	}
	if inserted {
		t.Fatal("second insert should be a dedup hit")
	}
	if id2 != id1 {
		t.Fatalf("dedup returned different id: %d vs %d", id2, id1)
	}
}

func TestWatchStateRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ws, err := s.GetWatchState(ctx, "atto")
	if err != nil || ws.LastUid != 0 {
		t.Fatalf("initial state: %+v err=%v", ws, err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.UpsertWatchState(ctx, WatchState{
		Alias: "atto", LastUid: 100, LastSubject: "hi", LastReceivedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.GetWatchState(ctx, "atto")
	if err != nil || got.LastUid != 100 || got.LastSubject != "hi" {
		t.Fatalf("readback bad: %+v err=%v", got, err)
	}
}

func TestOpenedUrlsDedup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, _, err := s.UpsertEmail(ctx, &Email{Alias: "a", MessageId: "<m@x>", Uid: 1})
	if err != nil {
		t.Fatal(err)
	}
	ins, err := s.RecordOpenedUrl(ctx, id, "r", "https://x.test/1")
	if err != nil || !ins {
		t.Fatalf("first: ins=%v err=%v", ins, err)
	}
	ins, err = s.RecordOpenedUrl(ctx, id, "r", "https://x.test/1")
	if err != nil || ins {
		t.Fatalf("dedup should skip: ins=%v err=%v", ins, err)
	}
	has, _ := s.HasOpenedUrl(ctx, id, "https://x.test/1")
	if !has {
		t.Fatal("HasOpenedUrl should be true")
	}
}
