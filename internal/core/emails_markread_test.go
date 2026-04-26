// emails_markread_test.go — Phase 4 (P4.2) coverage for
// (*EmailsService).MarkRead. Spec
// `spec/21-app/02-features/02-emails/01-backend.md` §2.3 lists these
// required cases:
//
//   1. MarkRead_500Uids_Under150ms        — perf gate (skipped under -short)
//   2. MarkRead_EmptyUids_NoSql            — assert zero `Exec` calls via fake
//   3. MarkRead_Idempotent                 — re-issuing same op leaves
//                                            state stable (behavioral)
//
// Plus boundary / error coverage that the spec implies but doesn't
// enumerate:
//
//   4. MarkRead_TooManyUids_Returns21221   — caller-bug guard (>1000)
//   5. MarkRead_OpenError_PropagatesErrDbOpen
//   6. MarkRead_StoreError_PropagatesErrDbInsertEmail (with alias ctx)
//   7. MarkRead_HappyPath_ForwardsAliasUidsAndReadFlag
//
// All tests use the existing `fakeEmailsStore` + `makeOpener` helpers
// from `emails_service_test.go`.
package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// -----------------------------------------------------------------------------
// 1. perf gate (spec §2.3: ≤ 150 ms for 500 UIDs)
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_500Uids_Under150ms(t *testing.T) {
	if testing.Short() {
		t.Skip("perf gate skipped under -short")
	}
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	uids := make([]uint32, 500)
	for i := range uids {
		uids[i] = uint32(i + 1)
	}
	start := time.Now()
	res := svc.MarkRead(context.Background(), "alias@x", uids, true)
	elapsed := time.Since(start)
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	const budget = 150 * time.Millisecond
	if elapsed > budget {
		t.Fatalf("MarkRead 500 uids: %v exceeds spec budget %v", elapsed, budget)
	}
	t.Logf("MarkRead 500 uids: %v (budget %v)", elapsed, budget)
}

// -----------------------------------------------------------------------------
// 2. empty uids → no SQL (spec §2.3)
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_EmptyUids_NoSql(t *testing.T) {
	fake := &fakeEmailsStore{}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.MarkRead(context.Background(), "alias@x", nil, true)
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	if got := atomic.LoadInt32(&fake.setReadCalls); got != 0 {
		t.Errorf("expected 0 SetEmailRead calls, got %d", got)
	}
	// Spec requires *no SQL* — the service should not even open a
	// store handle. closes counter is bumped only when defer fires
	// after a successful open; with the fast-path branch it should
	// remain 0.
	if got := atomic.LoadInt32(closes); got != 0 {
		t.Errorf("expected store opener untouched, got %d closes", got)
	}
}

// -----------------------------------------------------------------------------
// 3. idempotency (spec §2.3 — re-issuing same op is a no-op)
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_Idempotent(t *testing.T) {
	// The fake's "store state" is a simple per-uid map. SetEmailRead
	// writes set the value; we then assert that re-applying the same
	// (alias, uids, read) leaves the map unchanged.
	state := map[uint32]bool{1: false, 2: false, 3: false}
	fake := &fakeEmailsStoreWithState{state: state}
	opener := func() (emailsStore, func() error, error) {
		return fake, func() error { return nil }, nil
	}
	svc := NewEmailsService(opener).Value()

	uids := []uint32{1, 2, 3}
	if r := svc.MarkRead(context.Background(), "a", uids, true); r.HasError() {
		t.Fatalf("first MarkRead: %v", r.Error())
	}
	snapshot := copyMap(state)
	if r := svc.MarkRead(context.Background(), "a", uids, true); r.HasError() {
		t.Fatalf("second MarkRead: %v", r.Error())
	}
	if !equalMaps(snapshot, state) {
		t.Errorf("idempotency broken: state changed on repeat call\nbefore=%v\nafter =%v", snapshot, state)
	}
	for _, u := range uids {
		if !state[u] {
			t.Errorf("uid %d expected read=true after MarkRead, got false", u)
		}
	}
}

// fakeEmailsStoreWithState extends fakeEmailsStore with a read-flag
// map so the idempotency test can observe behavior, not just call
// counts. Embedding gives us List/Get/Count/etc. for free.
type fakeEmailsStoreWithState struct {
	fakeEmailsStore
	state map[uint32]bool
}

func (f *fakeEmailsStoreWithState) SetEmailRead(_ context.Context, _ string, uids []uint32, read bool) (int64, error) {
	var n int64
	for _, u := range uids {
		if _, ok := f.state[u]; ok {
			f.state[u] = read
			n++
		}
	}
	return n, nil
}

func copyMap(m map[uint32]bool) map[uint32]bool {
	out := make(map[uint32]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
func equalMaps(a, b map[uint32]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// 4. caller-bug guard: too many UIDs
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_TooManyUids_ReturnsInvalidArgument(t *testing.T) {
	fake := &fakeEmailsStore{}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	uids := make([]uint32, MarkReadMaxUids+1)
	res := svc.MarkRead(context.Background(), "a", uids, true)
	if !res.HasError() {
		t.Fatal("expected invalid-argument error, got ok")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrCoreInvalidArgument {
		t.Fatalf("expected ErrCoreInvalidArgument, got %v", res.Error())
	}
	if got, ok := ctxValue(coded, "uid_count"); !ok || got.(int) != MarkReadMaxUids+1 {
		t.Errorf("expected uid_count=%d in ctx, got %v (ok=%v)", MarkReadMaxUids+1, got, ok)
	}
	if got := atomic.LoadInt32(closes); got != 0 {
		t.Errorf("over-budget call must not open the store; got %d closes", got)
	}
}

// -----------------------------------------------------------------------------
// 5. open error → ErrDbOpen
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_PropagatesOpenError(t *testing.T) {
	openErr := errors.New("disk gone")
	opener, _ := makeOpener(nil, openErr)
	svc := NewEmailsService(opener).Value()

	res := svc.MarkRead(context.Background(), "a", []uint32{1}, true)
	if !res.HasError() {
		t.Fatal("expected open error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbOpen {
		t.Fatalf("expected ErrDbOpen, got %v", res.Error())
	}
}

// -----------------------------------------------------------------------------
// 6. store error → ErrDbInsertEmail (with alias + uid_count ctx)
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_PropagatesStoreError(t *testing.T) {
	fake := &fakeEmailsStore{setReadErr: errors.New("disk full")}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.MarkRead(context.Background(), "alias@x", []uint32{1, 2, 3}, true)
	if !res.HasError() {
		t.Fatal("expected store error to surface")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrDbInsertEmail {
		t.Fatalf("expected ErrDbInsertEmail, got %v", res.Error())
	}
	if got, ok := ctxValue(coded, "alias"); !ok || got != "alias@x" {
		t.Errorf("expected alias context, got %v (ok=%v)", got, ok)
	}
	if got, ok := ctxValue(coded, "uid_count"); !ok || got.(int) != 3 {
		t.Errorf("expected uid_count=3, got %v (ok=%v)", got, ok)
	}
}

// -----------------------------------------------------------------------------
// 7. happy path: alias / uids / read flag forwarded verbatim
// -----------------------------------------------------------------------------

func TestEmailsService_MarkRead_ForwardsArgsToStore(t *testing.T) {
	fake := &fakeEmailsStore{setReadRows: 2}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	uids := []uint32{10, 20}
	res := svc.MarkRead(context.Background(), "alias@x", uids, false)
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	if fake.lastSetReadAlias != "alias@x" {
		t.Errorf("alias: want alias@x, got %q", fake.lastSetReadAlias)
	}
	if len(fake.lastSetReadUids) != 2 || fake.lastSetReadUids[0] != 10 || fake.lastSetReadUids[1] != 20 {
		t.Errorf("uids: want [10 20], got %v", fake.lastSetReadUids)
	}
	if fake.lastSetReadValue != false {
		t.Errorf("read flag: want false, got true")
	}
}
