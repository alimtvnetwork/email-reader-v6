// shims_test.go covers the typed query shims introduced to keep
// `internal/core/*` free of `database/sql`. We test SQL composition
// at the unit level and round-trip the streaming methods through a
// real (in-memory-ish, t.TempDir-backed) store to lock the column
// order down.
package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestBuildEmailExportQuery_Composition is the SQL-composition twin of
// the (now-deleted) `core.TestBuildExportQuery_Composition`. Lives here
// because the builder moved into store/shims.go (AC-DB-52).
func TestBuildEmailExportQuery_Composition(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		f          EmailExportFilter
		wantClause string
		wantArgs   int
	}{
		{"empty", EmailExportFilter{}, "", 0},
		{"alias", EmailExportFilter{Alias: "a"}, "Alias = ?", 1},
		{"since", EmailExportFilter{Since: t0}, "ReceivedAt >= ?", 1},
		{"until", EmailExportFilter{Until: t1}, "ReceivedAt < ?", 1},
		{"all", EmailExportFilter{Alias: "a", Since: t0, Until: t1},
			"Alias = ? AND ReceivedAt >= ? AND ReceivedAt < ?", 3},
	}
	for _, c := range cases {
		q, args := buildEmailExportQuery(c.f, false)
		if c.wantClause == "" {
			if strings.Contains(q, "WHERE") {
				t.Errorf("%s: expected no WHERE, got %q", c.name, q)
			}
		} else if !strings.Contains(q, c.wantClause) {
			t.Errorf("%s: expected clause %q in %q", c.name, c.wantClause, q)
		}
		if len(args) != c.wantArgs {
			t.Errorf("%s: want %d args, got %d", c.name, c.wantArgs, len(args))
		}
		if !strings.Contains(q, "ORDER BY Id ASC") {
			t.Errorf("%s: missing ORDER BY: %q", c.name, q)
		}
	}
}

// TestBuildEmailExportQuery_Count flips the count flag and asserts the
// projection swaps to COUNT(*) and the ORDER BY drops out.
func TestBuildEmailExportQuery_Count(t *testing.T) {
	t.Parallel()
	q, _ := buildEmailExportQuery(EmailExportFilter{Alias: "x"}, true)
	if !strings.Contains(q, "SELECT COUNT(*)") {
		t.Errorf("count form should project COUNT(*): %q", q)
	}
	if strings.Contains(q, "ORDER BY") {
		t.Errorf("count form should not ORDER BY: %q", q)
	}
	if !strings.Contains(q, "WHERE Alias = ?") {
		t.Errorf("count form should still honour WHERE: %q", q)
	}
}

// TestBuildOpenedUrlsQuery is the SQL-composition twin of the
// (now-deleted) `core.TestBuildOpenedUrlsQuery`.
func TestBuildOpenedUrlsQuery(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		name     string
		f        OpenedUrlListFilter
		wantArgs int
		wantSubs []string
	}{
		{"baseline",
			OpenedUrlListFilter{Limit: 50, Before: now},
			2, []string{"OpenedAt < ?", "LIMIT ?"}},
		{"alias",
			OpenedUrlListFilter{Limit: 50, Before: now, Alias: "work"},
			3, []string{"Alias = ?"}},
		{"origin",
			OpenedUrlListFilter{Limit: 50, Before: now, Origin: "rule"},
			3, []string{"Origin = ?"}},
		{"both",
			OpenedUrlListFilter{Limit: 50, Before: now, Alias: "w", Origin: "manual"},
			4, []string{"Alias = ?", "Origin = ?"}},
	}
	for _, c := range cases {
		q, args := buildOpenedUrlsQuery(c.f)
		if len(args) != c.wantArgs {
			t.Errorf("%s: want %d args, got %d (%v)", c.name, c.wantArgs, len(args), args)
		}
		for _, sub := range c.wantSubs {
			if !strings.Contains(q, sub) {
				t.Errorf("%s: query missing %q in %s", c.name, sub, q)
			}
		}
	}
}

// TestQueryEmailExportRows_RoundTrip inserts two rows, streams them
// back through the typed shim, and confirms the column order matches
// the reader contract used by `internal/exporter` and
// `internal/core.writeFilteredRows`.
func TestQueryEmailExportRows_RoundTrip(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	for i, alias := range []string{"a", "b"} {
		if _, _, err := st.UpsertEmail(ctx, &Email{
			Alias:      alias,
			MessageId:  string(rune('A'+i)) + "@x",
			Uid:        uint32(i + 1),
			FromAddr:   "f@x",
			Subject:    "s",
			ReceivedAt: time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("UpsertEmail: %v", err)
		}
	}

	rows, err := st.QueryEmailExportRows(ctx, EmailExportFilter{Alias: "b"})
	if err != nil {
		t.Fatalf("QueryEmailExportRows: %v", err)
	}
	defer rows.Close()

	var n int
	for rows.Next() {
		var (
			id, uid                                         int64
			alias, msgId, fromA, toA, ccA, subj, bt, bh, fp string
			received, created                               any
		)
		if err := rows.Scan(&id, &alias, &msgId, &uid, &fromA, &toA, &ccA,
			&subj, &bt, &bh, &received, &fp, &created); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if alias != "b" {
			t.Errorf("filter not applied: got alias=%q", alias)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 row, got %d", n)
	}

	got, err := st.CountEmailsFiltered(ctx, EmailExportFilter{})
	if err != nil {
		t.Fatalf("CountEmails: %v", err)
	}
	if got != 2 {
		t.Errorf("CountEmails: want 2, got %d", got)
	}
}
