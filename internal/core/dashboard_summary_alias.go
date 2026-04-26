// dashboard_summary_alias.go — `DashboardSummary` type alias.
//
// Spec uses `DashboardSummary` (spec/21-app/02-features/01-dashboard);
// the historical Go field name was `DashboardStats`. After the P3.5
// rip-and-replace, `Summary` is the canonical method (in dashboard.go)
// and `DashboardSummary` is the spec-aligned name. This file keeps the
// alias so the older `DashboardStats` identifier — still used by tests
// and by `DashboardOptions.Stats` callers — remains valid without
// conversion.
package core

// DashboardSummary is the spec-named projection. Identical layout to
// DashboardStats — an alias, not a separate type, so no conversion is
// needed at any boundary. The two names are interchangeable.
type DashboardSummary = DashboardStats
