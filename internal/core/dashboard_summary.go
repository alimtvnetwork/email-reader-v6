// dashboard_summary.go — P3.2 spec-aligned façade.
//
// The roadmap (mem://workflow/roadmap-phases.md, P3.1/P3.2) calls for
// a typed `*Dashboard` service exposing `Summary(ctx) (DashboardSummary, error)`.
// Phase 2 already shipped the underlying machinery as
// `*DashboardService.LoadStats` returning `DashboardStats`.
//
// Rather than fork a parallel API (which would violate the
// "rip-and-replace, no parallel APIs" rule from the roadmap
// conventions), we land P3.2 as a thin spec-named façade:
//
//   - `DashboardSummary` is a type alias for `DashboardStats` so spec
//     code and existing call sites can use either name interchangeably
//     without conversion.
//   - `(*DashboardService).Summary` delegates to `LoadStats`. It is the
//     name backend specs (spec/21-app/02-features/dashboard/*) use, and
//     becomes the canonical entry point going forward.
//
// `LoadStats` stays for one more slice (P3.5) so we can update callers
// in a single rip-and-replace commit, then it is deleted.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/errtrace"
)

// DashboardSummary is the spec-named projection. Identical layout to
// DashboardStats — an alias, not a separate type, so no conversion is
// needed at any boundary.
type DashboardSummary = DashboardStats

// Summary is the spec-aligned entry point. Behaviourally identical to
// LoadStats; the rename moves the public surface to match
// `spec/21-app/02-features/dashboard` terminology.
//
// Pass `alias == ""` for global totals only; non-empty alias also
// populates `EmailsForAlias`.
func (s *DashboardService) Summary(ctx context.Context, alias string) errtrace.Result[DashboardSummary] {
	return s.LoadStats(ctx, alias)
}
