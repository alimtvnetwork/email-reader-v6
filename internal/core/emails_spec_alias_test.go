// emails_spec_alias_test.go — locks the spec-name alias contract.
//
// **Phase 4.1 lock-in.** Three contracts:
//
//  1. `core.Emails` resolves to the same underlying type as
//     `core.EmailsService` (the alias is real, not a wrapper struct).
//  2. `core.NewEmails()` returns a non-nil `*core.Emails` via the
//     production opener (same wiring as `NewDefaultEmailsService`).
//  3. The existing `List`/`Get`/`Count` methods are reachable through
//     the `*Emails` name with no adapter or signature drift — i.e.
//     P4.2+ slices can author method bodies on `*EmailsService` and
//     reference them as `*Emails` interchangeably.
//
// If a future slice ever splits `Emails` from `EmailsService` (e.g.
// new struct with `store.Store` injection per spec §1), this test
// must be revised in the same commit so the rename is intentional.

package core

import (
	"context"
	"reflect"
	"testing"
)

func TestEmails_AliasResolvesToEmailsService(t *testing.T) {
	// Contract 1: identical underlying type via reflection.
	wantType := reflect.TypeOf((*EmailsService)(nil))
	gotType := reflect.TypeOf((*Emails)(nil))
	if wantType != gotType {
		t.Fatalf("core.Emails alias drift: want %v, got %v", wantType, gotType)
	}
}

func TestNewEmails_ReturnsWiredService(t *testing.T) {
	// Contract 2: spec-canonical constructor name resolves.
	res := NewEmails()
	if res.HasError() {
		t.Fatalf("NewEmails: unexpected err: %v", res.Error())
	}
	svc := res.Value()
	if svc == nil {
		t.Fatal("NewEmails: returned nil *Emails")
	}
	if svc.openStore == nil {
		t.Fatal("NewEmails: openStore dep not wired (NewDefaultEmailsService should inject defaultStoreOpener)")
	}
}

func TestEmails_MethodsReachableViaSpecName(t *testing.T) {
	// Contract 3: every method defined on *EmailsService is callable
	// through *Emails with no signature drift. We bind method values
	// via the alias name — compile-time proof the surface is shared.
	// Production wiring isn't exercised; we only need the binding to
	// type-check and the bound values to be non-nil.
	res := NewEmails()
	if res.HasError() {
		t.Fatalf("NewEmails: %v", res.Error())
	}
	svc := res.Value()
	var e *Emails = svc

	listFn := e.List
	getFn := e.Get
	countFn := e.Count
	if listFn == nil || getFn == nil || countFn == nil {
		t.Fatal("method bindings via *Emails returned nil")
	}

	// Sanity-only: confirm List has the expected signature shape by
	// invoking with a canceled ctx + empty opts — we expect a non-nil
	// Result either way; we don't assert on the err since the real
	// store may or may not be openable in the test sandbox. The point
	// is the call type-checks.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = listFn(ctx, ListEmailsOptions{})
}
