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

// TestOpenedUrlsDelta1_Migration_Idempotent locks the contract that a
// freshly-opened DB has the 6 PascalCase Delta-#1 columns AND that a
// subsequent migrate() call (re-opening the same path) is a no-op.
func TestOpenedUrlsDelta1_Migration_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delta1.db")
	s, err := OpenAt(path)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	cols, err := s.openedUrlsColumns()
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	for _, want := range []string{"Alias", "Origin", "OriginalUrl", "IsDeduped", "IsIncognito", "TraceId"} {
		if !cols[want] {
			t.Errorf("missing Delta-#1 column %q", want)
		}
	}
	_ = s.Close()
	// Re-open: migrate() must not error on already-applied ALTERs.
	s2, err := OpenAt(path)
	if err != nil {
		t.Fatalf("open2 (idempotence): %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
}

// TestRecordOpenedUrlExt_RoundTrip asserts every Delta-#1 field flows
// through the Ext insert and is readable via a direct SELECT.
func TestRecordOpenedUrlExt_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, _, err := s.UpsertEmail(ctx, &Email{Alias: "a", MessageId: "<m@x>", Uid: 1})
	if err != nil {
		t.Fatal(err)
	}
	in := OpenedUrlInsert{
		EmailId: id, RuleName: "rule-A", Url: "https://x.test/canon",
		Alias: "work", Origin: "manual", OriginalUrl: "https://x.test/canon?token=secret",
		IsDeduped: false, IsIncognito: true, TraceId: "abc123def456",
	}
	ok, err := s.RecordOpenedUrlExt(ctx, in)
	if err != nil || !ok {
		t.Fatalf("Ext insert: ok=%v err=%v", ok, err)
	}
	var (
		alias, origin, orig, trace string
		dedup, incog               int
	)
	row := s.DB.QueryRowContext(ctx,
		`SELECT Alias, Origin, OriginalUrl, IsDeduped, IsIncognito, TraceId
		   FROM OpenedUrls WHERE EmailId = ? AND Url = ?`,
		id, "https://x.test/canon")
	if err := row.Scan(&alias, &origin, &orig, &dedup, &incog, &trace); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if alias != "work" || origin != "manual" || orig == "" || dedup != 0 || incog != 1 || trace != "abc123def456" {
		t.Fatalf("round-trip mismatch: alias=%q origin=%q orig=%q dedup=%d incog=%d trace=%q",
			alias, origin, orig, dedup, incog, trace)
	}
}

// TestRecordOpenedUrl_LegacyShim asserts the slim form still works and
// leaves the Delta-#1 columns at their default values.
func TestRecordOpenedUrl_LegacyShim(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, _, _ := s.UpsertEmail(ctx, &Email{Alias: "a", MessageId: "<legacy@x>", Uid: 2})
	if ok, err := s.RecordOpenedUrl(ctx, id, "rule-X", "https://legacy.test/p"); err != nil || !ok {
		t.Fatalf("legacy insert: ok=%v err=%v", ok, err)
	}
	var alias, trace string
	var incog int
	err := s.DB.QueryRowContext(ctx,
		`SELECT Alias, IsIncognito, TraceId FROM OpenedUrls WHERE EmailId = ?`, id,
	).Scan(&alias, &incog, &trace)
	if err != nil {
		t.Fatal(err)
	}
	if alias != "" || incog != 0 || trace != "" {
		t.Fatalf("legacy insert must default Delta-#1 cols, got alias=%q incog=%d trace=%q", alias, incog, trace)
	}
}
