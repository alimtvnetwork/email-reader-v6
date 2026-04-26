// emails_counts_test.go — Phase 4 (P4.5) + Slice #100 coverage for
// (*EmailsService).Counts. Spec
// `spec/21-app/02-features/02-emails/01-backend.md` §2.6 requires:
//
//   - Counts_MatchesDirectSql — the projection equals what three
//     independent COUNT queries return (we assert this against the
//     fake's programmed values; a follow-on golden test against a
//     real store is tracked under Phase 4 acceptance).
//   - alias = "" must aggregate across all accounts.
//   - alias = "ghost" with zero rows returns {0, 0, 0}.
//   - Open / count failures propagate with the expected error codes
//     and `alias` context (one case per COUNT projection).
//
// The historical `Deleted == 0` tripwire (pre-Slice #100) has flipped
// — see TestEmailsService_Counts_DeletedFieldPopulated below.
package core

import (
	"context"
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
)

func TestEmailsService_Counts_HappyPath(t *testing.T) {
	fake := &fakeEmailsStore{count: 42, unread: 7, deleted: 3}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "alias@x")
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	got := res.Value()
	want := EmailCounts{Total: 42, Unread: 7, Deleted: 3}
	if got != want {
		t.Errorf("Counts: want %+v, got %+v", want, got)
	}
	if fake.lastCountAls != "alias@x" || fake.lastUnreadAls != "alias@x" || fake.lastDeletedAls != "alias@x" {
		t.Errorf("alias not forwarded: total=%q unread=%q deleted=%q",
			fake.lastCountAls, fake.lastUnreadAls, fake.lastDeletedAls)
	}
}

func TestEmailsService_Counts_EmptyAlias_AggregatesAll(t *testing.T) {
	fake := &fakeEmailsStore{count: 1234, unread: 56}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "")
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	if got := res.Value(); got.Total != 1234 || got.Unread != 56 {
		t.Errorf("aggregate: want {1234, 56}, got %+v", got)
	}
	if fake.lastCountAls != "" || fake.lastUnreadAls != "" {
		t.Errorf("empty alias should pass through unchanged: total=%q unread=%q",
			fake.lastCountAls, fake.lastUnreadAls)
	}
}

func TestEmailsService_Counts_GhostAlias_ReturnsZeros(t *testing.T) {
	fake := &fakeEmailsStore{count: 0, unread: 0}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "ghost")
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	if got := res.Value(); got != (EmailCounts{}) {
		t.Errorf("ghost alias: want zero-value EmailCounts, got %+v", got)
	}
}

func TestEmailsService_Counts_PropagatesOpenError(t *testing.T) {
	opener, _ := makeOpener(nil, errors.New("disk gone"))
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "a")
	if !res.HasError() {
		t.Fatal("expected open error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbOpen {
		t.Fatalf("expected ErrDbOpen, got %v", res.Error())
	}
}

func TestEmailsService_Counts_PropagatesTotalError(t *testing.T) {
	fake := &fakeEmailsStore{countErr: errors.New("syntax err")}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "alias@x")
	if !res.HasError() {
		t.Fatal("expected query error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbQueryEmail {
		t.Fatalf("expected ErrDbQueryEmail, got %v", res.Error())
	}
	if got, ok := ctxValue(coded, "alias"); !ok || got != "alias@x" {
		t.Errorf("expected alias ctx, got %v (ok=%v)", got, ok)
	}
}

func TestEmailsService_Counts_PropagatesUnreadError(t *testing.T) {
	fake := &fakeEmailsStore{count: 10, unreadErr: errors.New("disk error")}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "alias@x")
	if !res.HasError() {
		t.Fatal("expected unread query error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbQueryEmail {
		t.Fatalf("expected ErrDbQueryEmail, got %v", res.Error())
	}
	if got, ok := ctxValue(coded, "alias"); !ok || got != "alias@x" {
		t.Errorf("expected alias ctx, got %v (ok=%v)", got, ok)
	}
}

func TestEmailsService_Counts_PropagatesDeletedError(t *testing.T) {
	fake := &fakeEmailsStore{count: 10, unread: 5, deletedErr: errors.New("disk error")}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Counts(context.Background(), "alias@x")
	if !res.HasError() {
		t.Fatal("expected deleted query error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbQueryEmail {
		t.Fatalf("expected ErrDbQueryEmail, got %v", res.Error())
	}
	if got, ok := ctxValue(coded, "alias"); !ok || got != "alias@x" {
		t.Errorf("expected alias ctx, got %v (ok=%v)", got, ok)
	}
}

// TestEmailsService_Counts_DeletedFieldPopulated is the post-Slice-#100
// successor to the historical "Deleted is zero-pinned" tripwire. It
// asserts that a programmed `deleted` count flows through to
// `EmailCounts.Deleted` unchanged, locking in the populator wired up
// against m0012's `DeletedAt` column.
func TestEmailsService_Counts_DeletedFieldPopulated(t *testing.T) {
	fake := &fakeEmailsStore{count: 100, unread: 50, deleted: 17}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	got := svc.Counts(context.Background(), "x").Value()
	if got.Deleted != 17 {
		t.Fatalf("Deleted: want 17 (forwarded from CountDeletedEmails), got %d", got.Deleted)
	}
}
