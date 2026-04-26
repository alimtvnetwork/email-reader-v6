// emails_deps.go — Phase 4.8: wider DI on `*EmailsService`.
//
// Why this exists
//
//   The pre-P4.8 surface offered three ways to construct an
//   EmailsService:
//
//     1. `NewEmailsService(openStore)`             — required dep only
//     2. `NewDefaultEmailsService()`               — production wiring
//     3. `(*EmailsService).WithRefresher(r)`       — optional dep, fluent
//
//   That worked for P4.2..P4.6 because the only deps were `openStore`
//   and (later) `Refresher`. P4.3 (Delete/Undelete + DeleteReceipt)
//   and the rest of Phase 5 (Rules CRUD that EmailsService consults
//   for filter-time rule eval) introduce two more required deps:
//
//     - `Rules` (rules-store reader for inline filter eval)
//     - `Clock` (for DeletedAt timestamps; injectable so unit tests
//                don't depend on wall-clock instability)
//
//   Adding two more positional constructor params + two more fluent
//   setters would balloon the call-site shape and make it easy to
//   forget a required wire-up at bootstrap. The spec §1
//   ("dependency injection per service") prescribes a single deps
//   struct so:
//
//     - All deps are named — no positional confusion.
//     - Optional vs. required is documented at the field, in one
//       place, instead of scattered across constructor variants.
//     - Adding a future dep (P5 Rules, P6 Accounts, …) is one struct
//       field + one validation line — no new constructor needed.
//
// Backwards compatibility
//
//   `NewEmailsService` and `WithRefresher` are kept as thin shims
//   that internally build a deps struct. Every existing call site
//   (UI bootstrap, all in-package tests, the deprecated
//   package-level wrappers in emails.go) continues to work
//   unchanged. The deps struct is purely additive.
//
// Clock injection rationale
//
//   Production `*EmailsService` methods today have zero `time.Now()`
//   calls (verified at slice authoring time). `Clock` is forward
//   infrastructure for P4.3 (`DeletedAt = clock()`) and any future
//   "now"-stamped audit field. It's wired in this slice — not
//   later — because retrofitting Clock through the constructor
//   *after* P4.3 lands would force a second touchpoint on every
//   call site that already passed deps; doing it here keeps the
//   diff localized to one slice.
//
// `Rules` placeholder
//
//   The `Rules` field is typed as `RulesReader` — an interface
//   defined in this file with zero methods today. P5 will widen it
//   (List/Get/Match) without breaking the constructor surface. The
//   field is documented as optional until P5.x lands the consumers.

package core

import (
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// Clock is the injectable wall-clock seam. Production wires
// `time.Now`; tests pass a deterministic stub. Defined as a
// function (not an interface) to keep call sites trivial:
// `now := s.clock()`.
type Clock func() time.Time

// RulesReader is the forward-compat seam for the rules service.
// Empty for now (P4.8 introduces the wire path; P5 lands the
// methods). Kept as an interface — not a concrete pointer — so the
// Rules service implementation can evolve independently and tests
// can pass a fake without dragging the full rules engine in.
type RulesReader interface{}

// EmailsServiceDeps bundles every dependency `*EmailsService`
// consumes. Fields are split into "required" and "optional" by
// comment; `NewEmailsServiceFromDeps` validates required fields and
// substitutes safe defaults for the optional ones.
type EmailsServiceDeps struct {
	// Store is required. Returns an open emailsStore + close
	// callback. See `storeOpener` for the contract.
	Store storeOpener

	// Refresher is optional. When nil, `Refresh(ctx, alias)` returns
	// `ErrCoreInvalidArgument` (the existing P4.4 behavior). When
	// set, `Refresh` delegates to `Refresher.PollOnce`.
	Refresher Refresher

	// Rules is optional today. P5 widens `RulesReader` and the
	// EmailsService methods that need it (filter-time rule eval).
	// Until then, holding the field reserves the slot so P5 doesn't
	// reshape the deps struct.
	Rules RulesReader

	// Clock is optional. Defaults to `time.Now` when nil. Production
	// callers should leave nil; tests inject a deterministic stub.
	Clock Clock
}

// NewEmailsServiceFromDeps is the spec-shape constructor. Validates
// required fields, applies optional-field defaults, and returns a
// fully-configured `*EmailsService`.
//
// Validation rules:
//   - deps.Store == nil           → ErrCoreInvalidArgument
//   - everything else is optional and gets a documented default.
//
// Returns Result[*EmailsService] for parity with the rest of core's
// constructor surface.
func NewEmailsServiceFromDeps(deps EmailsServiceDeps) errtrace.Result[*EmailsService] {
	if deps.Store == nil {
		return errtrace.Err[*EmailsService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"NewEmailsServiceFromDeps: deps.Store is nil"))
	}
	clk := deps.Clock
	if clk == nil {
		clk = time.Now
	}
	return errtrace.Ok(&EmailsService{
		openStore: deps.Store,
		refresher: deps.Refresher,
		rules:     deps.Rules,
		clock:     clk,
	})
}
