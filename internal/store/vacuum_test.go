// vacuum_test.go covers PruneOpenedUrlsBefore, Analyze, and ShouldAnalyze.
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestPruneOpenedUrlsBefore_DeletesOnlyOldRows contributes to
// AC-DB-40 (Q-OPEN-PRUNE-LAUNCHED only deletes Launched rows
// older than cutoff) — the time-cutoff half is locked here
// (back-dated row deletes, recent row survives). The
// "Decision='Launched' filter" half remains uncovered: today's
// `RecordOpenedUrl` path always writes Launched, so we cannot
// negative-test that a Blocked row at the same age survives. That
// branch stays on the AC backlog (kept in coverageGapAllowlist).
func TestPruneOpenedUrlsBefore_DeletesOnlyOldRows(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "ret.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// Seed an Email row so we can FK-reference it.
	id, _, err := s.UpsertEmail(ctx, &Email{
		Alias: "a", MessageId: "m1", Uid: 1, Subject: "s", FilePath: "/tmp/x",
	})
	if err != nil {
		t.Fatalf("UpsertEmail: %v", err)
	}

	// Insert two rows, then back-date one and leave the other recent.
	if _, err := s.RecordOpenedUrl(ctx, id, "rule", "https://old.example/"); err != nil {
		t.Fatalf("record old: %v", err)
	}
	if _, err := s.RecordOpenedUrl(ctx, id, "rule", "https://new.example/"); err != nil {
		t.Fatalf("record new: %v", err)
	}
	old := time.Now().UTC().Add(-200 * 24 * time.Hour)
	if _, err := s.DB.Exec(`UPDATE OpenedUrls SET OpenedAt=? WHERE Url=?`,
		old, "https://old.example/"); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour)
	n, err := s.PruneOpenedUrlsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("PruneOpenedUrlsBefore: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted = %d, want 1", n)
	}

	// Recent row survives.
	var remaining int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM OpenedUrls`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d, want 1", remaining)
	}
}

func TestPruneOpenedUrlsBefore_ZeroCutoffIsNoop(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "ret.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	id, _, _ := s.UpsertEmail(ctx, &Email{
		Alias: "a", MessageId: "m1", Uid: 1, Subject: "s",
	})
	_, _ = s.RecordOpenedUrl(ctx, id, "r", "https://example/")

	n, err := s.PruneOpenedUrlsBefore(ctx, time.Time{})
	if err != nil {
		t.Fatalf("PruneOpenedUrlsBefore: %v", err)
	}
	if n != 0 {
		t.Fatalf("deleted = %d, want 0 (no-op)", n)
	}
}

// TestShouldAnalyze_AtAndAroundThreshold pins the integer comparison —
// exactly AnalyzeThreshold triggers, one less does not, anything more does.
func TestShouldAnalyze_AtAndAroundThreshold(t *testing.T) {
	cases := []struct {
		cum  int64
		want bool
	}{
		{0, false},
		{1, false},
		{AnalyzeThreshold - 1, false},
		{AnalyzeThreshold, true},
		{AnalyzeThreshold + 1, true},
		{10 * AnalyzeThreshold, true},
	}
	for _, c := range cases {
		if got := ShouldAnalyze(c.cum); got != c.want {
			t.Fatalf("ShouldAnalyze(%d) = %v, want %v", c.cum, got, c.want)
		}
	}
}

// TestAnalyze_RunsCleanlyOnEmptyDB: ANALYZE is supposed to be safe to
// call any time. Smoke-test it on an empty DB so we know the SQL parses
// and does not error out on a fresh schema.
func TestAnalyze_RunsCleanlyOnEmptyDB(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "an.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer s.Close()
	if err := s.Analyze(context.Background()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
}
