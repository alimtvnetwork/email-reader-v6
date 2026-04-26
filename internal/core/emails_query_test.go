// emails_query_test.go — Phase 4.6 tests for `EmailQuery` /
// `EmailPage` / `EmailSortKey` and `(*EmailsService).ListPage`.
//
// Shares the `fakeEmailsStore` fake from `emails_service_test.go`.
//
// Coverage matrix:
//   - HappyPath_DefaultSort         → newest-first preserved from store
//   - SortReceivedAsc               → in-place reorder
//   - SortSubjectAsc                → case-insensitive subject sort
//   - SinceUntilWindow              → drops out-of-range rows
//   - PaginationSlicesItems         → Limit/Offset window correct,
//                                     NextOffset matches lo+len(page)
//   - PaginationOffsetOverflow      → graceful clamp to Total
//   - DeferredFlags_AreNoOps        → OnlyUnread/IncludeDeleted do
//                                     not drop rows in this slice
//                                     (tripwire — flips when the
//                                     follow-up slices land)
//   - PropagatesOpenError           → ErrDbOpen scope
//   - PropagatesQueryError          → ErrDbQueryEmail + alias ctx

package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// mkRow is a tiny builder for store.Email rows used by these tests.
func mkRow(uid uint32, alias, subj string, recv time.Time) store.Email {
	return store.Email{
		Id: int64(uid), Alias: alias, Uid: uid, FromAddr: alias + "@x",
		Subject: subj, ReceivedAt: recv,
	}
}

// newTestSvcWithRows builds an EmailsService whose store hands back
// the given rows on ListEmails. Returns the fake too for assertions.
func newTestSvcWithRows(t *testing.T, rows []store.Email) (*EmailsService, *fakeEmailsStore) {
	t.Helper()
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	res := NewEmailsService(opener)
	if res.HasError() {
		t.Fatalf("NewEmailsService: %v", res.Error())
	}
	return res.Value(), fake
}

func TestEmailsService_ListPage_HappyPath_DefaultSort(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	// Store returns newest-first per its ORDER BY contract.
	rows := []store.Email{
		mkRow(3, "a", "Newest", t0.Add(2*time.Hour)),
		mkRow(2, "a", "Middle", t0.Add(1*time.Hour)),
		mkRow(1, "a", "Oldest", t0),
	}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{Alias: "a"})
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	got := res.Value()
	if got.Total != 3 || len(got.Items) != 3 || got.NextOffset != 3 {
		t.Fatalf("page = %+v, want Total=3 len=3 NextOffset=3", got)
	}
	if got.Items[0].Subject != "Newest" {
		t.Errorf("default sort lost: items[0]=%q want Newest", got.Items[0].Subject)
	}
}

func TestEmailsService_ListPage_SortReceivedAsc(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := []store.Email{
		mkRow(3, "a", "Newest", t0.Add(2*time.Hour)),
		mkRow(1, "a", "Oldest", t0),
	}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", SortBy: EmailSortReceivedAsc,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	got := res.Value()
	if got.Items[0].Subject != "Oldest" {
		t.Errorf("asc sort failed: items[0]=%q want Oldest", got.Items[0].Subject)
	}
}

func TestEmailsService_ListPage_SortSubjectAsc_CaseFolded(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := []store.Email{
		mkRow(1, "a", "banana", t0),
		mkRow(2, "a", "Apple", t0),
		mkRow(3, "a", "cherry", t0),
	}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", SortBy: EmailSortSubjectAsc,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	want := []string{"Apple", "banana", "cherry"}
	for i, w := range want {
		if !strings.EqualFold(res.Value().Items[i].Subject, w) {
			t.Errorf("items[%d]=%q want %q", i, res.Value().Items[i].Subject, w)
		}
	}
}

func TestEmailsService_ListPage_SinceUntilWindow(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := []store.Email{
		mkRow(1, "a", "before", t0.Add(-time.Hour)),
		mkRow(2, "a", "inside", t0.Add(time.Hour)),
		mkRow(3, "a", "after", t0.Add(5*time.Hour)),
	}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", SinceAt: t0, UntilAt: t0.Add(2 * time.Hour),
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	got := res.Value()
	if got.Total != 1 || got.Items[0].Subject != "inside" {
		t.Fatalf("window filter wrong: %+v", got)
	}
}

func TestEmailsService_ListPage_PaginationSlicesItems(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := make([]store.Email, 0, 10)
	for i := 0; i < 10; i++ {
		rows = append(rows, mkRow(uint32(10-i), "a", "s",
			t0.Add(time.Duration(10-i)*time.Hour)))
	}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", Limit: 3, Offset: 4,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	got := res.Value()
	if got.Total != 10 {
		t.Errorf("Total=%d want 10 (must ignore Limit)", got.Total)
	}
	if len(got.Items) != 3 {
		t.Errorf("len(Items)=%d want 3", len(got.Items))
	}
	if got.NextOffset != 7 {
		t.Errorf("NextOffset=%d want 7", got.NextOffset)
	}
}

func TestEmailsService_ListPage_PaginationOffsetOverflow(t *testing.T) {
	rows := []store.Email{mkRow(1, "a", "only", time.Now())}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", Offset: 999, Limit: 10,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	got := res.Value()
	if got.Total != 1 || len(got.Items) != 0 || got.NextOffset != 1 {
		t.Fatalf("overflow page = %+v, want Total=1 len=0 NextOffset=1", got)
	}
}

// OnlyUnread is now wired to the freshly-exposed `store.Email.IsRead`
// field. Mixed read/unread input must yield only the unread rows; Total
// must reflect the post-filter count.
func TestEmailsService_ListPage_OnlyUnread_FiltersReadRows(t *testing.T) {
	t0 := time.Now()
	rows := []store.Email{
		mkRow(3, "a", "fresh", t0),
		mkRow(2, "a", "already-read", t0),
		mkRow(1, "a", "also-fresh", t0),
	}
	rows[1].IsRead = true // middle row is read
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", OnlyUnread: true,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	got := res.Value()
	if got.Total != 2 {
		t.Fatalf("Total=%d want 2 (one read row dropped)", got.Total)
	}
	for _, it := range got.Items {
		if it.Subject == "already-read" {
			t.Errorf("read row leaked into OnlyUnread results: %+v", it)
		}
	}
}

// OnlyUnread=false must leave read rows in place — no accidental filter.
func TestEmailsService_ListPage_OnlyUnread_OffKeepsReadRows(t *testing.T) {
	rows := []store.Email{
		mkRow(2, "a", "read", time.Now()),
		mkRow(1, "a", "unread", time.Now()),
	}
	rows[0].IsRead = true
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{Alias: "a"})
	if res.HasError() || res.Value().Total != 2 {
		t.Fatalf("Total=%d want 2; err=%v", res.Value().Total, res.Error())
	}
}

// Residual tripwire: IncludeDeleted is still a no-op until P4.3 lands
// the DeletedAt column. When that slice ships, this test should fail
// and force the no-op branch to become a real filter.
func TestEmailsService_ListPage_IncludeDeleted_StillNoOp(t *testing.T) {
	rows := []store.Email{mkRow(1, "a", "s", time.Now())}
	s, _ := newTestSvcWithRows(t, rows)

	res := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", IncludeDeleted: true,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	if res.Value().Total != 1 {
		t.Fatalf("IncludeDeleted altered Total in P4.6 follow-up (Total=%d). "+
			"P4.3 likely landed — replace this tripwire with real filter coverage.",
			res.Value().Total)
	}
}

func TestEmailsService_ListPage_PropagatesOpenError(t *testing.T) {
	openErr := errors.New("disk gone")
	opener, _ := makeOpener(nil, openErr)
	svcRes := NewEmailsService(opener)
	if svcRes.HasError() {
		t.Fatalf("NewEmailsService: %v", svcRes.Error())
	}

	res := svcRes.Value().ListPage(context.Background(), EmailQuery{})
	if !res.HasError() {
		t.Fatal("want error, got nil")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbOpen {
		t.Errorf("code = %v, want ErrDbOpen", coded)
	}
}

func TestEmailsService_ListPage_PropagatesQueryError(t *testing.T) {
	qErr := errors.New("query boom")
	fake := &fakeEmailsStore{listErr: qErr}
	opener, _ := makeOpener(fake, nil)
	svcRes := NewEmailsService(opener)
	if svcRes.HasError() {
		t.Fatalf("NewEmailsService: %v", svcRes.Error())
	}

	res := svcRes.Value().ListPage(context.Background(),
		EmailQuery{Alias: "primary"})
	if !res.HasError() {
		t.Fatal("want error, got nil")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbQueryEmail {
		t.Fatalf("code = %v, want ErrDbQueryEmail", coded)
	}
	// Context is []ContextField — scan for the alias key.
	var aliasOK bool
	for _, f := range coded.Context {
		if f.Key == "alias" && f.Value == "primary" {
			aliasOK = true
			break
		}
	}
	if !aliasOK {
		t.Errorf("alias ctx missing or wrong: %+v", coded.Context)
	}
}
