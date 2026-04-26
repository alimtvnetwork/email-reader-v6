package queries

import (
	"strings"
	"testing"
)

func TestEmailByUid_Static(t *testing.T) {
	if !strings.Contains(EmailByUid, "FROM Emails") {
		t.Fatalf("EmailByUid missing FROM Emails: %q", EmailByUid)
	}
	if !strings.Contains(EmailByUid, "Alias = ? AND Uid = ?") {
		t.Fatalf("EmailByUid missing alias/uid predicates: %q", EmailByUid)
	}
}

func TestEmailsList_NoFilters(t *testing.T) {
	sql, args := EmailsList(EmailsListInput{})
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
	if strings.Contains(sql, " WHERE ") {
		t.Fatalf("unexpected WHERE clause: %q", sql)
	}
	if !strings.Contains(sql, "ORDER BY Uid DESC, Id DESC") {
		t.Fatalf("missing canonical ORDER BY: %q", sql)
	}
}

func TestEmailsList_AllFiltersAndPaging(t *testing.T) {
	sql, args := EmailsList(EmailsListInput{
		Alias: "a@b", Search: "Hello", Limit: 50, Offset: 100,
	})
	if !strings.Contains(sql, "Alias = ?") {
		t.Fatalf("missing Alias predicate: %q", sql)
	}
	if !strings.Contains(sql, "LOWER(Subject) LIKE ? OR LOWER(FromAddr) LIKE ?") {
		t.Fatalf("missing search predicate: %q", sql)
	}
	if !strings.Contains(sql, "LIMIT ?") || !strings.Contains(sql, "OFFSET ?") {
		t.Fatalf("missing LIMIT/OFFSET: %q", sql)
	}
	// args order: alias, needle, needle, limit, offset
	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d (%v)", len(args), args)
	}
	if args[0] != "a@b" {
		t.Fatalf("arg[0] alias mismatch: %v", args[0])
	}
	if args[1] != "%hello%" {
		t.Fatalf("arg[1] needle expected lowercased %%hello%%, got %v", args[1])
	}
	if args[3] != 50 || args[4] != 100 {
		t.Fatalf("limit/offset args wrong: %v", args)
	}
}

func TestEmailsList_LimitWithoutOffset(t *testing.T) {
	sql, args := EmailsList(EmailsListInput{Limit: 10})
	if !strings.Contains(sql, "LIMIT ?") {
		t.Fatalf("expected LIMIT: %q", sql)
	}
	if strings.Contains(sql, "OFFSET ?") {
		t.Fatalf("did not expect OFFSET: %q", sql)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Fatalf("expected single arg [10], got %v", args)
	}
}
