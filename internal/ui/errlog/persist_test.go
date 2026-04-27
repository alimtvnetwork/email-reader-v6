// persist_test.go — disk-persistence tests (Phase 4.1).
package errlog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPersistence_WriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "error-log.jsonl")
	p, err := NewPersistence(path, 0)
	if err != nil {
		t.Fatalf("NewPersistence: %v", err)
	}
	defer p.Close()

	want := []Entry{
		{Seq: 1, Timestamp: time.Now().UTC(), Component: "watcher", Summary: "boom", Trace: "boom\n  at watcher.go:1"},
		{Seq: 2, Timestamp: time.Now().UTC(), Component: "emails", Summary: "fetch", Trace: "fetch\n  at emails.go:1"},
	}
	for _, e := range want {
		if err := p.Write(e); err != nil {
			t.Fatalf("Write seq=%d: %v", e.Seq, err)
		}
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Seq != want[i].Seq || got[i].Component != want[i].Component || got[i].Summary != want[i].Summary {
			t.Errorf("entry %d mismatch: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestLoadFromFile_MissingIsEmpty(t *testing.T) {
	got, err := LoadFromFile(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing file should yield 0 entries, got %d", len(got))
	}
}

func TestLoadFromFile_SkipsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	good := Entry{Seq: 1, Component: "a", Summary: "ok"}
	gb, _ := json.Marshal(good)
	body := string(gb) + "\n" +
		"{not valid json\n" +
		`{"Seq":2,"Component":"b","Summary":"also-ok"}` + "\n" +
		"" // trailing empty line
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 surviving entries, got %d (%+v)", len(got), got)
	}
	if got[0].Summary != "ok" || got[1].Summary != "also-ok" {
		t.Errorf("wrong survivors: %+v", got)
	}
}

func TestPersistence_RotatesAtSizeCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	// Tiny cap so 2-3 writes trigger rotation. Each entry is ~80 bytes.
	p, err := NewPersistence(path, 100)
	if err != nil {
		t.Fatalf("NewPersistence: %v", err)
	}
	defer p.Close()

	for i := 0; i < 5; i++ {
		if err := p.Write(Entry{Seq: uint64(i + 1), Component: "x", Summary: strings.Repeat("y", 40)}); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file at %s.1, got: %v", path, err)
	}
	// Active file must exist and be smaller than the cap (post-rotate).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("active file gone after rotate: %v", err)
	}
	if info.Size() >= 100 {
		t.Errorf("active file %d bytes >= cap 100 — rotation did not reset size", info.Size())
	}
}

func TestStore_EnablePersistence_SeedsAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	prior := []Entry{
		{Seq: 10, Component: "a", Summary: "old-1"},
		{Seq: 11, Component: "a", Summary: "old-2"},
	}
	p, err := NewPersistence(path, 0)
	if err != nil {
		t.Fatalf("NewPersistence: %v", err)
	}
	defer p.Close()

	s := NewStore(5)
	s.EnablePersistence(p, prior)

	// Snapshot must include the prior entries (newest last).
	snap := s.Snapshot()
	if len(snap) != 2 || snap[0].Summary != "old-1" || snap[1].Summary != "old-2" {
		t.Fatalf("seeded snapshot wrong: %+v", snap)
	}

	// New Append: Seq must continue from max(prior.Seq)+1 = 12, and a
	// matching line must hit disk.
	s.Append(Entry{Component: "live", Summary: "new-1"})
	snap = s.Snapshot()
	if len(snap) != 3 || snap[2].Seq != 12 {
		t.Fatalf("post-append snapshot wrong (Seq must be 12): %+v", snap)
	}

	// Reading the file back must show only the *new* entry — the
	// persister writes live appends, not the seeded prior.
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Summary != "new-1" || loaded[0].Seq != 12 {
		t.Fatalf("file should have exactly the live append, got %+v", loaded)
	}
}

func TestStore_EnablePersistence_RingCapTrimsPrior(t *testing.T) {
	dir := t.TempDir()
	p, err := NewPersistence(filepath.Join(dir, "log.jsonl"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	prior := make([]Entry, 10)
	for i := range prior {
		prior[i] = Entry{Seq: uint64(i + 1), Summary: "p"}
	}
	s := NewStore(3) // cap 3 — only the last 3 prior survive
	s.EnablePersistence(p, prior)

	snap := s.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("ring should hold 3, got %d", len(snap))
	}
	if snap[0].Seq != 8 || snap[2].Seq != 10 {
		t.Errorf("ring should hold last 3 (seqs 8/9/10), got %+v", snap)
	}
}

func TestEnableDefaultPersistence_RestoresAcrossInstances(t *testing.T) {
	// Use a fresh process-wide singleton state by Clearing first.
	Clear()
	// Restore singleton to a pristine state when this test exits so a
	// later test (or a `-count=2` re-run of this same binary) does not
	// observe a dangling persister pointing at a now-closed file —
	// that used to nil-deref bufio.Writer in Persistence.Write.
	t.Cleanup(resetSingletonForTest)
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "error-log.jsonl") // exercises MkdirAll

	p, err := EnableDefaultPersistence(path, 0)
	if err != nil {
		t.Fatalf("EnableDefaultPersistence: %v", err)
	}
	ReportError("emails", testErr("first"))
	ReportError("watcher", testErr("second"))
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate a restart: reset the singleton, re-enable, and confirm
	// the prior entries return.
	resetSingletonForTest()
	p2, err := EnableDefaultPersistence(path, 0)
	if err != nil {
		t.Fatalf("EnableDefaultPersistence (round 2): %v", err)
	}
	defer p2.Close()

	got := Snapshot()
	if len(got) != 2 {
		t.Fatalf("after restart expected 2 entries, got %d", len(got))
	}
	if got[0].Summary != "first" || got[1].Summary != "second" {
		t.Errorf("restored entries out of order or wrong: %+v", got)
	}
}

// resetSingletonForTest forces a fresh singleton — only used by the
// restore-across-instances test.
func resetSingletonForTest() {
	singleton = nil
	singletonOnce = onceForTest()
}

// onceForTest builds a fresh sync.Once. Wrapped so the test file does
// not have to import sync just for this one call.
func onceForTest() (o sync.Once) { return }

type testErr string

func (e testErr) Error() string { return string(e) }
