// emails_service_test.go — Phase 2.4 typed *EmailsService coverage.
//
// Goal: prove that the new typed service does what the old
// package-level funcs did, without ever touching SQLite, by injecting
// a fake `storeOpener` that returns a programmable in-memory
// `emailsStore`.
//
// Coverage matrix:
//   - constructor rejects nil opener (ErrCoreInvalidArgument)
//   - List propagates ErrDbOpen on opener failure
//   - List propagates ErrDbQueryEmail on query failure (with alias ctx)
//   - List happy-path projects rows → summaries with snippet collapse
//   - Get happy-path populates EmailDetail body fields
//   - Count returns the underlying int unchanged
//   - close callback is invoked exactly once per call (lifetime audit)
package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// fakeEmailsStore is the minimal in-memory implementation of
// emailsStore. Each method returns a programmed value or programmed
// error; nothing is persisted between method calls.
type fakeEmailsStore struct {
	listRows []store.Email
	listErr  error
	getRow   *store.Email
	getErr   error
	count    int
	countErr error

	// CountUnreadEmails programming + observation (Phase 4 P4.5)
	unread        int
	unreadErr     error
	lastUnreadAls string

	// CountDeletedEmails programming + observation (Slice #100)
	deleted        int
	deletedErr     error
	lastDeletedAls string

	// MarkRead programming + observation (Phase 4 P4.2)
	setReadRows      int64
	setReadErr       error
	setReadCalls     int32
	lastSetReadAlias string
	lastSetReadUids  []uint32
	lastSetReadValue bool

	// SetEmailDeletedAt programming + observation (Phase 4 P4.3)
	setDeletedRows      int64
	setDeletedErr       error
	setDeletedCalls     int32
	lastSetDeletedAlias string
	lastSetDeletedUids  []uint32
	lastSetDeletedAt    *int64 // copied so the caller can free its stamp

	// observed inputs — for assertions
	lastListQuery store.EmailQuery
	lastGetAlias  string
	lastGetUid    uint32
	lastCountAls  string
}

func (f *fakeEmailsStore) ListEmails(_ context.Context, q store.EmailQuery) ([]store.Email, error) {
	f.lastListQuery = q
	return f.listRows, f.listErr
}
func (f *fakeEmailsStore) GetEmailByUid(_ context.Context, alias string, uid uint32) (*store.Email, error) {
	f.lastGetAlias, f.lastGetUid = alias, uid
	return f.getRow, f.getErr
}
func (f *fakeEmailsStore) CountEmails(_ context.Context, alias string) (int, error) {
	f.lastCountAls = alias
	return f.count, f.countErr
}
func (f *fakeEmailsStore) SetEmailRead(_ context.Context, alias string, uids []uint32, read bool) (int64, error) {
	atomic.AddInt32(&f.setReadCalls, 1)
	f.lastSetReadAlias = alias
	f.lastSetReadUids = append([]uint32(nil), uids...)
	f.lastSetReadValue = read
	return f.setReadRows, f.setReadErr
}
func (f *fakeEmailsStore) CountUnreadEmails(_ context.Context, alias string) (int, error) {
	f.lastUnreadAls = alias
	return f.unread, f.unreadErr
}
func (f *fakeEmailsStore) CountDeletedEmails(_ context.Context, alias string) (int, error) {
	f.lastDeletedAls = alias
	return f.deleted, f.deletedErr
}
func (f *fakeEmailsStore) SetEmailDeletedAt(_ context.Context, alias string, uids []uint32, deletedAt *int64) (int64, error) {
	atomic.AddInt32(&f.setDeletedCalls, 1)
	f.lastSetDeletedAlias = alias
	f.lastSetDeletedUids = append([]uint32(nil), uids...)
	if deletedAt != nil {
		v := *deletedAt
		f.lastSetDeletedAt = &v
	} else {
		f.lastSetDeletedAt = nil
	}
	return f.setDeletedRows, f.setDeletedErr
}

// makeOpener returns a storeOpener that hands out the given fake and
// counts how many times the close callback fires. Useful for the
// "close called exactly once" lifetime audit.
func makeOpener(fake *fakeEmailsStore, openErr error) (storeOpener, *int32) {
	var closes int32
	return func() (emailsStore, func() error, error) {
		if openErr != nil {
			return nil, func() error { return nil }, openErr
		}
		return fake, func() error {
			atomic.AddInt32(&closes, 1)
			return nil
		}, nil
	}, &closes
}

func TestNewEmailsService_RejectsNilOpener(t *testing.T) {
	res := NewEmailsService(nil)
	if !res.HasError() {
		t.Fatal("expected error when opener is nil, got ok")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) {
		t.Fatalf("expected *errtrace.Coded, got %T", res.Error())
	}
	if coded.Code != errtrace.ErrCoreInvalidArgument {
		t.Fatalf("expected ErrCoreInvalidArgument, got %v", coded.Code)
	}
}

// ctxValue scans a Coded's slice-shaped Context for the given key.
func ctxValue(c *errtrace.Coded, key string) (any, bool) {
	for _, f := range c.Context {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

func TestEmailsService_List_PropagatesOpenError(t *testing.T) {
	openErr := errors.New("disk gone")
	opener, _ := makeOpener(nil, openErr)
	svc := NewEmailsService(opener).Value()

	res := svc.List(context.Background(), ListEmailsOptions{Alias: "a@b"})
	if !res.HasError() {
		t.Fatal("expected open error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbOpen {
		t.Fatalf("expected ErrDbOpen, got %v", res.Error())
	}
}

func TestEmailsService_List_PropagatesQueryError(t *testing.T) {
	fake := &fakeEmailsStore{listErr: errors.New("syntax err")}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.List(context.Background(), ListEmailsOptions{Alias: "a@b", Limit: 10})
	if !res.HasError() {
		t.Fatal("expected query error to surface")
	}
	var coded2 *errtrace.Coded
	if !errors.As(res.Error(), &coded2) || coded2.Code != errtrace.ErrDbQueryEmail {
		t.Fatalf("expected ErrDbQueryEmail, got %v", res.Error())
	}
	if got, ok := ctxValue(coded2, "alias"); !ok || got != "a@b" {
		t.Errorf("expected alias context 'a@b', got %v (ok=%v)", got, ok)
	}
	if atomic.LoadInt32(closes) != 1 {
		t.Errorf("expected close called once, got %d", atomic.LoadInt32(closes))
	}
}

func TestEmailsService_List_HappyPathProjectsRows(t *testing.T) {
	when := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	fake := &fakeEmailsStore{
		listRows: []store.Email{{
			Id: 1, Alias: "a@b", Uid: 42,
			FromAddr: "x@y", Subject: "hello",
			BodyText:   "  multi\n\nline   text  ", // exercises snippet collapse
			ReceivedAt: when,
		}},
	}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.List(context.Background(), ListEmailsOptions{Alias: "a@b", Limit: 50, Offset: 5, Search: "hi"})
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	out := res.Value()
	if len(out) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(out))
	}
	if out[0].Snippet != "multi line text" {
		t.Errorf("snippet collapse wrong: %q", out[0].Snippet)
	}
	if out[0].ReceivedAt != "2026-04-01T12:00:00Z" {
		t.Errorf("ReceivedAt format wrong: %q", out[0].ReceivedAt)
	}
	// query was forwarded verbatim
	if fake.lastListQuery.Limit != 50 || fake.lastListQuery.Offset != 5 || fake.lastListQuery.Search != "hi" {
		t.Errorf("query not forwarded: %+v", fake.lastListQuery)
	}
	if atomic.LoadInt32(closes) != 1 {
		t.Errorf("expected close called once, got %d", atomic.LoadInt32(closes))
	}
}

func TestEmailsService_Get_HappyPathPopulatesBody(t *testing.T) {
	fake := &fakeEmailsStore{
		getRow: &store.Email{
			Id: 7, Alias: "a@b", Uid: 9,
			FromAddr: "x@y", ToAddr: "to@z", CcAddr: "cc@z",
			Subject:  "s",
			BodyText: "plain", BodyHtml: "<p>html</p>",
		},
	}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Get(context.Background(), "a@b", 9)
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	d := res.Value()
	if d.To != "to@z" || d.Cc != "cc@z" || d.BodyText != "plain" || d.BodyHtml != "<p>html</p>" {
		t.Errorf("detail body fields wrong: %+v", d)
	}
	if fake.lastGetAlias != "a@b" || fake.lastGetUid != 9 {
		t.Errorf("Get inputs not forwarded: %q / %d", fake.lastGetAlias, fake.lastGetUid)
	}
	if atomic.LoadInt32(closes) != 1 {
		t.Errorf("expected close called once, got %d", atomic.LoadInt32(closes))
	}
}

func TestEmailsService_Count_ReturnsUnderlyingInt(t *testing.T) {
	fake := &fakeEmailsStore{count: 1234}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Count(context.Background(), "alias@x")
	if res.HasError() {
		t.Fatalf("unexpected error: %v", res.Error())
	}
	if res.Value() != 1234 {
		t.Errorf("expected 1234, got %d", res.Value())
	}
	if fake.lastCountAls != "alias@x" {
		t.Errorf("alias not forwarded: %q", fake.lastCountAls)
	}
	if atomic.LoadInt32(closes) != 1 {
		t.Errorf("expected close called once, got %d", atomic.LoadInt32(closes))
	}
}
