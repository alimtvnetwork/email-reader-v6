// accounts_service.go — Slice #115 (Phase 6.1) typed service shell
// for the Accounts surface. Mirrors the Dashboard/Emails/Rules
// pattern (Phase 2/3): a stateless `*AccountsService` with method-
// bound versions of the free funcs in `accounts.go`, plus a
// `NewDefaultAccountsService()` constructor that returns a ready-to-
// use instance.
//
// **Why a shell (not a full rewrite)**: every existing call site in
// `internal/ui/`, `internal/cli/`, and `internal/core/tools_read.go`
// already calls the free funcs — `core.AddAccount`, `core.ListAccounts`,
// `core.GetAccount`, `core.RemoveAccount`. Adding a typed service
// shell that delegates to those free funcs gives us:
//
//   1. The spec-aligned shape (`*Service`-style API surface) so the
//      UI bootstrap can pass typed services around uniformly via
//      `Services` (`internal/ui/services.go`).
//   2. A future DI seam — `NewDefaultAccountsService` is a function
//      so a test can swap it for a stub that doesn't touch
//      `data/config.json`.
//   3. Zero behaviour change — the service methods are one-line
//      delegates, so the existing free-func tests (`accounts_test.go`)
//      still pin the exact same code paths.
//
// The free funcs stay (rip-and-replace would touch ~20 files for
// zero user-visible win). When a future slice is ready to migrate
// individual callers, it can do so one site at a time without
// blocking on a global rename.
package core

import (
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// AccountsService is the typed entry point for the Accounts surface.
// All methods are pure delegates to the free funcs in `accounts.go`
// — see that file for the canonical doc-comments on behaviour, error
// codes, and event publication.
//
// Stateless on purpose: same precedent as `*RulesService`. Future
// dependencies (e.g. an injected `*config.Loader` for tests) would
// be added as struct fields with a `WithX` setter, mirroring
// `(*EmailsService).WithRefresher`.
type AccountsService struct{}

// NewDefaultAccountsService returns the default zero-config
// AccountsService instance. Wraps in `errtrace.Result` for symmetry
// with `NewDefaultEmailsService` / `NewDefaultRulesService` even
// though construction can't currently fail — keeps the
// `BuildServices` boot path uniform across all 4 typed services.
func NewDefaultAccountsService() errtrace.Result[*AccountsService] {
	return errtrace.Ok(&AccountsService{})
}

// Add validates input and persists a new account. Delegates to the
// `AddAccount` free func — same validation, same atomic-write
// contract, same `AccountAdded`/`AccountUpdated` event publication.
func (s *AccountsService) Add(in AccountInput) errtrace.Result[*AddAccountResult] {
	return AddAccount(in)
}

// List returns all configured accounts (a copy — safe to mutate).
// Delegates to `ListAccounts`.
func (s *AccountsService) List() errtrace.Result[[]config.Account] {
	return ListAccounts()
}

// Get returns the account with the given alias or
// `ErrConfigAccountMissing`. Delegates to `GetAccount`.
func (s *AccountsService) Get(alias string) errtrace.Result[config.Account] {
	return GetAccount(alias)
}

// Remove deletes the account with the given alias and publishes
// `AccountRemoved`. Delegates to `RemoveAccount`.
func (s *AccountsService) Remove(alias string) errtrace.Result[struct{}] {
	return RemoveAccount(alias)
}
