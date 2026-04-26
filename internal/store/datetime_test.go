// datetime_test.go enforces AC-DB-53:
//
//	"All datetime values in `Email`, `WatchState`, `OpenedUrl` are
//	 stored as RFC 3339 UTC strings (regex on a sample fetch)."
//
// The test opens a fresh temp DB, inserts one representative row in
// each of the three tables (driving Go-side parameter binding for
// `ReceivedAt` / `LastReceivedAt` and SQLite-side defaults for
// `CreatedAt` / `UpdatedAt` / `OpenedAt`), then SELECTs the raw TEXT
// representation of every datetime column and matches each against
// the RFC 3339 UTC regex with optional fractional seconds and a
// mandatory `Z` suffix.
//
// Spec: spec/23-app-database/01-schema.md X-4 + 97 AC-DB-53.
package store

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// rfc3339UTCRegexp matches `YYYY-MM-DDTHH:MM:SS[.fff…]Z` — the format
// produced by both `formatRFC3339UTC` and the SQLite expression
// `strftime('%Y-%m-%dT%H:%M:%fZ','now')`.
var rfc3339UTCRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$`)

// datetimeColumn lists the (table, column) pairs we audit. Kept as a
// package-level var so the schema-evolution case (a new `*At` column
// added later) is a one-line update here.
var datetimeColumns = []struct{ table, column string }{
	{"Emails", "ReceivedAt"},
	{"Emails", "CreatedAt"},
	{"WatchState", "LastReceivedAt"},
	{"WatchState", "UpdatedAt"},
	{"OpenedUrls", "OpenedAt"},
}

func Test_DateTime_FormatUtc(t *testing.T) {
	s := openTempStoreForDateTime(t)
	defer s.Close()
	id := seedDateTimeFixtures(t, s)
	_ = id
	for _, dc := range datetimeColumns {
		assertColumnIsRFC3339UTC(t, s, dc.table, dc.column)
	}
}

// openTempStoreForDateTime spins up a fresh on-disk DB in t.TempDir()
// and returns the opened *Store with t.Cleanup wired.
func openTempStoreForDateTime(t *testing.T) *Store {
	t.Helper()
	s, err := OpenAt(filepath.Join(t.TempDir(), "dt.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	return s
}

// seedDateTimeFixtures inserts one row per audited table, exercising
// both Go-bound datetimes (ReceivedAt / LastReceivedAt) and
// SQLite-defaulted ones (CreatedAt / UpdatedAt / OpenedAt). Returns
// the inserted Email Id so OpenedUrls has a valid FK target.
func seedDateTimeFixtures(t *testing.T, s *Store) int64 {
	t.Helper()
	ctx := context.Background()
	rec := time.Date(2026, 1, 2, 3, 4, 5, 600_000_000, time.UTC)
	id, _, err := s.UpsertEmail(ctx, &Email{
		Alias: "ac-db-53", MessageId: "m@dt", Uid: 1,
		Subject: "s", BodyText: "b", ReceivedAt: rec,
	})
	if err != nil {
		t.Fatalf("UpsertEmail: %v", err)
	}
	if err := s.UpsertWatchState(ctx, WatchState{
		Alias: "ac-db-53", LastUid: 1, LastSubject: "s",
		LastReceivedAt: rec,
	}); err != nil {
		t.Fatalf("UpsertWatchState: %v", err)
	}
	if _, err := s.RecordOpenedUrl(ctx, id, "rule", "https://x.test/p"); err != nil {
		t.Fatalf("RecordOpenedUrl: %v", err)
	}
	return id
}

// assertColumnIsRFC3339UTC SELECTs the column as raw TEXT (so we see
// the bytes SQLite stores, not a Go-driver coercion) and matches it
// against rfc3339UTCRegexp.
func assertColumnIsRFC3339UTC(t *testing.T, s *Store, table, column string) {
	t.Helper()
	q := fmt.Sprintf(`SELECT CAST(%s AS TEXT) FROM %s LIMIT 1`, column, table)
	var raw string
	if err := s.DB.QueryRowContext(context.Background(), q).Scan(&raw); err != nil {
		t.Fatalf("scan %s.%s: %v", table, column, err)
	}
	if !rfc3339UTCRegexp.MatchString(raw) {
		t.Errorf("AC-DB-53 violation: %s.%s stored as %q (want RFC 3339 UTC)", table, column, raw)
	}
}
