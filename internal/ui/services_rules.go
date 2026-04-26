// services_rules.go — Phase 5.5 thin wrappers around the typed
// `*core.RulesService` for UI call sites that already hold a
// `*Services` pointer.
//
// Why thin wrappers (vs. callers grabbing `s.Rules.Reorder` directly)
//   1. **Nil-safe by contract.** A degraded bootstrap (config path
//      unwritable, etc.) leaves `s.Rules == nil`. The Rename/Reorder
//      drag-and-drop handlers in the Rules view should fail with a
//      coded error, not a nil-pointer panic. Wrapping centralises
//      the nil check so every UI handler gets the guarantee for free.
//   2. **Stable surface.** If P5.6 ever splits Rules into a
//      ReadService + WriteService, only this file changes — the
//      Rules view keeps calling `services.Rename(old, new)`.
//   3. **Mirror of `AttachRefresher`.** Same shape as the Emails-side
//      bootstrap helper that already exists in `services.go`, so a
//      reader scanning the Services type sees a uniform pattern for
//      typed-service access.
//
// Why a dedicated `ErrRulesServiceUnwired` (vs. reusing
// `ErrRuleNotFound` or `ErrCoreInvalidArgument`)
//   "I never built the service" is structurally different from
//   "the service ran and the rule wasn't there". Routing both
//   through `ErrRuleNotFound` would silently mask a bootstrap bug
//   as a data-not-found UI message. Reusing `ErrCoreInvalidArgument`
//   would conflate caller-side validation failures with shell-side
//   wiring failures. A dedicated code surfaces the bootstrap gap
//   in test logs and lets the UI render a "service unavailable"
//   banner distinct from "no rule named X".

//go:build !nofyne

package ui

import (
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// Rename is the UI-facing wrapper for `(*core.RulesService).Rename`.
// Delegates straight through when the typed service is wired, returns
// `ErrRulesServiceUnwired` otherwise.
func (s *Services) Rename(oldName, newName string) errtrace.Result[core.Unit] {
	if s == nil || s.Rules == nil {
		return errtrace.Err[core.Unit](errtrace.NewCoded(
			errtrace.ErrRulesServiceUnwired,
			"ui.Services.Rename: rules service is not wired").
			WithContext("oldName", oldName).
			WithContext("newName", newName))
	}
	return s.Rules.Rename(oldName, newName)
}

// Reorder is the UI-facing wrapper for `(*core.RulesService).Reorder`.
// Same nil-guard contract as Rename.
func (s *Services) Reorder(names []string) errtrace.Result[core.Unit] {
	if s == nil || s.Rules == nil {
		return errtrace.Err[core.Unit](errtrace.NewCoded(
			errtrace.ErrRulesServiceUnwired,
			"ui.Services.Reorder: rules service is not wired").
			WithContext("nameCount", lenAsString(names)))
	}
	return s.Rules.Reorder(names)
}

// lenAsString stringifies len(names) for the error context. Local
// helper kept package-private so it doesn't pollute the package
// surface; reused by future P5.6 wrappers if/when they need
// count-in-context.
func lenAsString(names []string) string {
	// Small fast path for the most common case (drag a rule but
	// drop it back in place — count of zero items).
	switch len(names) {
	case 0:
		return "0"
	case 1:
		return "1"
	}
	// itoa-equivalent without pulling strconv just for one call —
	// loops up to ~20 iterations for int max, well below any
	// reasonable rules-list size.
	n := len(names)
	buf := make([]byte, 0, 6)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
