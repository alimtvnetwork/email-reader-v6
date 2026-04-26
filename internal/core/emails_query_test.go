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
//   - TotalIgnoresLimit             → Total = full filtered count
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

func TestEmailsService_ListPage_HappyPath_DefaultSort(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := []store.Email{
		mkRow(3, "a", "Newest", t0.Add(2*time.Hour)),
		mkRow(2, "a", "Middle", t0.Add(1*time.Hour)),
		mkRow(1, "a", "Oldest", t0),
	}
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, err := s.ListPage(context.Background(), EmailQuery{Alias: "a"}).Unwrap()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
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
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, _ := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", SortBy: EmailSortReceivedAsc,
	}).Unwrap()
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
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, _ := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", SortBy: EmailSortSubjectAsc,
	}).Unwrap()
	want := []string{"Apple", "banana", "cherry"}
	for i, w := range want {
		if !strings.EqualFold(got.Items[i].Subject, w) {
			t.Errorf("items[%d]=%q want %q", i, got.Items[i].Subject, w)
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
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, _ := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", SinceAt: t0, UntilAt: t0.Add(2 * time.Hour),
	}).Unwrap()
	if got.Total != 1 || got.Items[0].Subject != "inside" {
		t.Fatalf("window filter wrong: %+v", got)
	}
}

func TestEmailsService_ListPage_PaginationSlicesItems(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	rows := make([]store.Email, 0, 10)
	for i := 0; i < 10; i++ {
		rows = append(rows, mkRow(uint32(10-i), "a", "s", t0.Add(time.Duration(10-i)*time.Hour)))
	}
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, _ := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", Limit: 3, Offset: 4,
	}).Unwrap()
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
	fake := &fakeEmailsStore{listRows: []store.Email{
		mkRow(1, "a", "only", time.Now()),
	}}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, _ := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", Offset: 999, Limit: 10,
	}).Unwrap()
	if got.Total != 1 || len(got.Items) != 0 || got.NextOffset != 1 {
		t.Fatalf("overflow page = %+v, want Total=1 len=0 NextOffset=1", got)
	}
}

// Tripwire: when the deferred slices land (store.Email.IsRead +
// DeletedAt), this test fails and forces us to re-design the no-op
// branches into real filters.
func TestEmailsService_ListPage_DeferredFlags_AreNoOps(t *testing.T) {
	rows := []store.Email{mkRow(1, "a", "s", time.Now())}
	fake := &fakeEmailsStore{listRows: rows}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	got, _ := s.ListPage(context.Background(), EmailQuery{
		Alias: "a", OnlyUnread: true, IncludeDeleted: true,
	}).Unwrap()
	if got.Total != 1 {
		t.Fatalf("OnlyUnread/IncludeDeleted dropped rows in P4.6 (Total=%d). "+
			"Either the deferred slices landed (good — wire real filters and "+
			"replace this tripwire) or a regression slipped in.", got.Total)
	}
}

func TestEmailsService_ListPage_PropagatesOpenError(t *testing.T) {
	openErr := errors.New("disk gone")
	opener, _ := makeOpener(nil, openErr)
	s, _ := NewEmailsService(opener).Unwrap()

	_, err := s.ListPage(context.Background(), EmailQuery{}).Unwrap()
	if err == nil || !errors.Is(err, openErr) {
		t.Fatalf("err = %v, want wraps openErr", err)
	}
	var coded *errtrace.CodedError
	if !errors.As(err, &coded) || coded.Code != errtrace.ErrDbOpen {
		t.Errorf("code = %v, want ErrDbOpen", coded)
	}
}

func TestEmailsService_ListPage_PropagatesQueryError(t *testing.T) {
	qErr := errors.New("query boom")
	fake := &fakeEmailsStore{listErr: qErr}
	opener, _ := makeOpener(fake, nil)
	s, _ := NewEmailsService(opener).Unwrap()

	_, err := s.ListPage(context.Background(), EmailQuery{Alias: "primary"}).Unwrap()
	var coded *errtrace.CodedError
	if !errors.As(err, &coded) || coded.Code != errtrace.ErrDbQueryEmail {
		t.Fatalf("code = %v, want ErrDbQueryEmail", coded)
	}
	if got, _ := coded.Context["alias"].(string); got != "primary" {
		t.Errorf("alias ctx = %q, want primary", got)
	}
}
