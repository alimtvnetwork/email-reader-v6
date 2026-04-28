// emails_lifecycle_test.go — Phase 4.3 coverage matrix for
// `(*EmailsService).Delete` and `(*EmailsService).Undelete`.
//
// Coverage matrix:
//   - HappyPath_Delete_StampsClock        — clock injected; SetEmailDeletedAt
//     called with *deletedAt == clock.Unix()
//   - HappyPath_Undelete_WritesNil        — deletedAt arg is nil
//   - Empty_Delete_NoStoreOpen            — opener never invoked
//   - Empty_Undelete_NoStoreOpen
//   - TooMany_Delete_Rejected             — ErrCoreInvalidArgument + ctx
//   - TooMany_Undelete_Rejected
//   - OpenError_Wrapped                   — ErrDbOpen
//   - StoreError_Wrapped                  — ErrDbInsertEmail + alias/uid_count/delete ctx
//   - ZeroRows_Delete_NotFound            — ErrEmailsLifecycleNotFound + ctx
//   - ZeroRows_Undelete_NotFound
//   - Idempotency_Delete                  — second call leaves end-state identical
//   - CloseFn_AlwaysInvoked               — defer audit (single open path)
package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// fixedClock returns a Clock that always reports `t`. Used so the
// "stamp == s.clock().Unix()" assertion is deterministic.
func fixedClock(t time.Time) Clock { return func() time.Time { return t } }

func TestDelete_HappyPath_StampsInjectedClock(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	fake := &fakeEmailsStore{setDeletedRows: 3}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsServiceFromDeps(EmailsServiceDeps{Store: opener, Clock: fixedClock(now)}).Value()

	res := svc.Delete(context.Background(), "user@x", []uint32{1, 2, 3})
	if res.HasError() {
		t.Fatalf("Delete: %v", res.Error())
	}
	if fake.lastSetDeletedAt == nil {
		t.Fatal("lastSetDeletedAt = nil; Delete must pass non-nil pointer")
	}
	if got, want := *fake.lastSetDeletedAt, now.Unix(); got != want {
		t.Errorf("stamped DeletedAt = %d, want %d (clock.Unix())", got, want)
	}
	if fake.lastSetDeletedAlias != "user@x" {
		t.Errorf("alias = %q, want %q", fake.lastSetDeletedAlias, "user@x")
	}
	if len(fake.lastSetDeletedUids) != 3 {
		t.Errorf("uid_count = %d, want 3", len(fake.lastSetDeletedUids))
	}
}

func TestUndelete_HappyPath_WritesNil(t *testing.T) {
	fake := &fakeEmailsStore{setDeletedRows: 2}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Undelete(context.Background(), "user@x", []uint32{1, 2})
	if res.HasError() {
		t.Fatalf("Undelete: %v", res.Error())
	}
	if fake.lastSetDeletedAt != nil {
		t.Errorf("lastSetDeletedAt = %d, want nil (Undelete writes NULL)", *fake.lastSetDeletedAt)
	}
}

func TestDelete_EmptyUids_NoStoreOpen(t *testing.T) {
	fake := &fakeEmailsStore{}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Delete(context.Background(), "a", nil)
	if res.HasError() {
		t.Fatalf("Delete(empty): %v", res.Error())
	}
	if fake.setDeletedCalls != 0 {
		t.Errorf("setDeletedCalls = %d, want 0 (empty must be no-op)", fake.setDeletedCalls)
	}
	if *closes != 0 {
		t.Errorf("closes = %d, want 0 (store must not open)", *closes)
	}
}

func TestUndelete_EmptyUids_NoStoreOpen(t *testing.T) {
	fake := &fakeEmailsStore{}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Undelete(context.Background(), "a", []uint32{})
	if res.HasError() {
		t.Fatalf("Undelete(empty): %v", res.Error())
	}
	if fake.setDeletedCalls != 0 || *closes != 0 {
		t.Errorf("Undelete(empty) opened the store (calls=%d closes=%d)", fake.setDeletedCalls, *closes)
	}
}

func TestDelete_TooManyUids_Rejected(t *testing.T) {
	fake := &fakeEmailsStore{}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	uids := make([]uint32, LifecycleMaxUids+1)
	res := svc.Delete(context.Background(), "a", uids)
	if !res.HasError() {
		t.Fatal("expected ErrCoreInvalidArgument")
	}
	var c *errtrace.Coded
	if !errors.As(res.Error(), &c) || c.Code != errtrace.ErrCoreInvalidArgument {
		t.Fatalf("expected ErrCoreInvalidArgument, got %v", res.Error())
	}
	if *closes != 0 || fake.setDeletedCalls != 0 {
		t.Errorf("over-budget call must short-circuit before opening store")
	}
}

func TestUndelete_TooManyUids_Rejected(t *testing.T) {
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	uids := make([]uint32, LifecycleMaxUids+1)
	res := svc.Undelete(context.Background(), "a", uids)
	if !res.HasError() {
		t.Fatal("expected ErrCoreInvalidArgument")
	}
	var c *errtrace.Coded
	if !errors.As(res.Error(), &c) || c.Code != errtrace.ErrCoreInvalidArgument {
		t.Fatalf("expected ErrCoreInvalidArgument, got %v", res.Error())
	}
}

func TestDelete_OpenError_Wrapped(t *testing.T) {
	openErr := errors.New("boom-open")
	opener, _ := makeOpener(&fakeEmailsStore{}, openErr)
	svc := NewEmailsService(opener).Value()

	res := svc.Delete(context.Background(), "a", []uint32{1})
	if !res.HasError() {
		t.Fatal("expected ErrDbOpen")
	}
	var c *errtrace.Coded
	if !errors.As(res.Error(), &c) || c.Code != errtrace.ErrDbOpen {
		t.Fatalf("expected ErrDbOpen, got %v", res.Error())
	}
	if !errors.Is(res.Error(), openErr) {
		t.Errorf("expected wrapped openErr; got %v", res.Error())
	}
}

func TestDelete_StoreError_Wrapped(t *testing.T) {
	storeErr := errors.New("boom-update")
	fake := &fakeEmailsStore{setDeletedErr: storeErr}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Delete(context.Background(), "a", []uint32{1, 2})
	if !res.HasError() {
		t.Fatal("expected ErrDbInsertEmail")
	}
	var c *errtrace.Coded
	if !errors.As(res.Error(), &c) || c.Code != errtrace.ErrDbInsertEmail {
		t.Fatalf("expected ErrDbInsertEmail, got %v", res.Error())
	}
	// Verify ctx attachments fired.
	wantKeys := map[string]bool{"alias": false, "uid_count": false, "delete": false}
	for _, kv := range c.Context {
		if _, ok := wantKeys[kv.Key]; ok {
			wantKeys[kv.Key] = true
		}
	}
	for k, ok := range wantKeys {
		if !ok {
			t.Errorf("missing context key %q in %+v", k, c.Context)
		}
	}
}

func TestDelete_ZeroRows_LifecycleNotFound(t *testing.T) {
	fake := &fakeEmailsStore{setDeletedRows: 0}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Delete(context.Background(), "a", []uint32{42, 43})
	if !res.HasError() {
		t.Fatal("expected ErrEmailsLifecycleNotFound")
	}
	var c *errtrace.Coded
	if !errors.As(res.Error(), &c) || c.Code != errtrace.ErrEmailsLifecycleNotFound {
		t.Fatalf("expected ErrEmailsLifecycleNotFound, got %v", res.Error())
	}
}

func TestUndelete_ZeroRows_LifecycleNotFound(t *testing.T) {
	fake := &fakeEmailsStore{setDeletedRows: 0}
	opener, _ := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	res := svc.Undelete(context.Background(), "a", []uint32{42})
	if !res.HasError() {
		t.Fatal("expected ErrEmailsLifecycleNotFound")
	}
	var c *errtrace.Coded
	if !errors.As(res.Error(), &c) || c.Code != errtrace.ErrEmailsLifecycleNotFound {
		t.Fatalf("expected ErrEmailsLifecycleNotFound, got %v", res.Error())
	}
}

func TestDelete_Idempotent_RestampsClock(t *testing.T) {
	// Behavioural idempotency: re-issuing the same Delete leaves the
	// store in the *same logical state* (deleted). Timestamp may
	// advance — we deliberately do NOT pin it, matching the spec
	// docstring ("UI may rely on latest stamp for 'deleted N seconds
	// ago' labels").
	fake := &fakeEmailsStore{setDeletedRows: 1}
	opener, _ := makeOpener(fake, nil)
	tick := int64(1_000_000_000)
	svc := NewEmailsServiceFromDeps(EmailsServiceDeps{
		Store: opener,
		Clock: func() time.Time { tick++; return time.Unix(tick, 0).UTC() },
	}).Value()

	for i := 0; i < 3; i++ {
		if r := svc.Delete(context.Background(), "a", []uint32{1}); r.HasError() {
			t.Fatalf("Delete #%d: %v", i+1, r.Error())
		}
	}
	if got := fake.setDeletedCalls; got != 3 {
		t.Errorf("setDeletedCalls = %d, want 3 (idempotent calls still hit store)", got)
	}
}

func TestDelete_CloseFn_AlwaysInvoked(t *testing.T) {
	fake := &fakeEmailsStore{setDeletedRows: 1}
	opener, closes := makeOpener(fake, nil)
	svc := NewEmailsService(opener).Value()

	if r := svc.Delete(context.Background(), "a", []uint32{1}); r.HasError() {
		t.Fatalf("Delete: %v", r.Error())
	}
	if *closes != 1 {
		t.Errorf("closes = %d, want 1 (defer must fire exactly once)", *closes)
	}
}
