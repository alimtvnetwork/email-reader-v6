// accounts_service_test.go — Slice #115 contract tests for the
// `*AccountsService` shell. We don't re-test the underlying behaviour
// (covered by `accounts_test.go` against the free funcs); we only
// pin the **shape** so a future refactor that breaks the delegate
// chain trips a test rather than a runtime panic.
package core

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func TestNewDefaultAccountsService_ReturnsNonNilService(t *testing.T) {
	r := NewDefaultAccountsService()
	if r.HasError() {
		t.Fatalf("NewDefaultAccountsService: unexpected error: %v", r.Error())
	}
	if r.Value() == nil {
		t.Fatal("NewDefaultAccountsService: returned nil service")
	}
}

// TestAccountsService_MethodShapes is a compile-time shape lock:
// every method on `*AccountsService` must be assignable to the
// free-func signature it delegates to. If a future refactor changes
// either side without updating the other, this test fails to compile.
func TestAccountsService_MethodShapes(t *testing.T) {
	svc := &AccountsService{}

	var addFn func(AccountInput) errtrace.Result[*AddAccountResult] = svc.Add
	var listFn func() errtrace.Result[[]config.Account] = svc.List
	var getFn func(string) errtrace.Result[config.Account] = svc.Get
	var rmFn func(string) errtrace.Result[struct{}] = svc.Remove

	// Reference each binding so the compiler can't dead-code them.
	if addFn == nil || listFn == nil || getFn == nil || rmFn == nil {
		t.Fatal("nil method binding")
	}

	// Compile-time: free funcs must have the same shape.
	var _ func(AccountInput) errtrace.Result[*AddAccountResult] = AddAccount
	var _ func() errtrace.Result[[]config.Account] = ListAccounts
	var _ func(string) errtrace.Result[config.Account] = GetAccount
	var _ func(string) errtrace.Result[struct{}] = RemoveAccount

	t.Log("AccountsService method shapes match free funcs")
}

func TestAccountsService_List_Delegates(t *testing.T) {
	// Tiny smoke test: calling List() through the service returns
	// the same Result envelope as the free func. Doesn't assert on
	// content (depends on user's data/config.json) — just that the
	// delegate doesn't panic and returns a non-error envelope when
	// the free func does.
	svc := &AccountsService{}
	r := svc.List()
	free := ListAccounts()
	if r.HasError() != free.HasError() {
		t.Fatalf("delegate divergence: svc.HasError=%v free.HasError=%v",
			r.HasError(), free.HasError())
	}
	if !r.HasError() && len(r.Value()) != len(free.Value()) {
		t.Fatalf("delegate length mismatch: svc=%d free=%d",
			len(r.Value()), len(free.Value()))
	}
}
