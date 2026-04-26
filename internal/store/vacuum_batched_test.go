// vacuum_batched_test.go pins PruneOpenedUrlsBeforeBatched (spec
// §5 + AC-DB-43): chunked DELETE that stops once a partial batch is
// observed. Three cases: under-batch (single shot), exact-multiple
// (last batch is empty), and many batches (loop count = ceil(N/B)).
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func seedOldRows(t *testing.T, s *Store, n int) {
	t.Helper()
	ctx := context.Background()
	id, _, err := s.UpsertEmail(ctx, &Email{
		Alias: "a", MessageId: "m1", Uid: 1, Subject: "s", FilePath: "/tmp/x",
	})
	if err != nil {
		t.Fatalf("UpsertEmail: %v", err)
	}
	for i := 0; i < n; i++ {
		url := "https://old.example/" + time.Now().Format("150405.000000000") + "-" + intToStr(i)
		if _, err := s.RecordOpenedUrl(ctx, id, "rule", url); err != nil {
			t.Fatalf("RecordOpenedUrl[%d]: %v", i, err)
		}
	}
	old := time.Now().UTC().Add(-200 * 24 * time.Hour)
	if _, err := s.DB.Exec(`UPDATE OpenedUrls SET OpenedAt=?`, old); err != nil {
		t.Fatalf("backdate: %v", err)
	}
}

func intToStr(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = digits[i%10]
		i /= 10
	}
	return string(b[pos:])
}

func TestPruneOpenedUrlsBeforeBatched_UnderBatch_SingleShot(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "b1.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	seedOldRows(t, s, 3)
	cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour)
	n, batches, err := s.PruneOpenedUrlsBeforeBatched(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 3 || batches != 1 {
		t.Fatalf("got n=%d batches=%d, want 3,1", n, batches)
	}
}

func TestPruneOpenedUrlsBeforeBatched_ExactMultiple(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "b2.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	seedOldRows(t, s, 6)
	cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour)
	// batchSize=3 → first batch deletes 3 (full), second batch deletes 3
	// (also full → loop continues), third batch deletes 0 (terminator).
	n, batches, err := s.PruneOpenedUrlsBeforeBatched(context.Background(), cutoff, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 6 || batches != 3 {
		t.Fatalf("got n=%d batches=%d, want 6,3", n, batches)
	}
}

func TestPruneOpenedUrlsBeforeBatched_ManyBatches(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "b3.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	seedOldRows(t, s, 25)
	cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour)
	// batchSize=10 → 10, 10, 5 (partial → stop): 3 batches, 25 rows.
	n, batches, err := s.PruneOpenedUrlsBeforeBatched(context.Background(), cutoff, 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 25 || batches != 3 {
		t.Fatalf("got n=%d batches=%d, want 25,3", n, batches)
	}
}

func TestPruneOpenedUrlsBeforeBatched_ZeroCutoffNoop(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "b4.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	seedOldRows(t, s, 5)
	n, batches, err := s.PruneOpenedUrlsBeforeBatched(context.Background(), time.Time{}, 100)
	if err != nil || n != 0 || batches != 0 {
		t.Fatalf("got n=%d batches=%d err=%v, want 0,0,nil", n, batches, err)
	}
}

func TestPruneOpenedUrlsBeforeBatched_DefaultBatchSize(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "b5.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	seedOldRows(t, s, 2)
	cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour)
	// batchSize=0 / negative falls back to DefaultPruneBatchSize.
	n, batches, err := s.PruneOpenedUrlsBeforeBatched(context.Background(), cutoff, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 2 || batches != 1 {
		t.Fatalf("got n=%d batches=%d, want 2,1", n, batches)
	}
}
