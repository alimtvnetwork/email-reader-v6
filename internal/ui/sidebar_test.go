package ui

import "testing"

// TestNavItemsCoverAllKinds locks the canonical sidebar order and ensures
// every NavKind constant has exactly one matching entry. Catches typos like
// duplicating "rules" or accidentally dropping an item during a refactor.
//
// Closes AC-DS-30 (slice #148): the spec's "Sidebar renders 7 items in the
// order from `03-layout-and-shell.md` §3.1" is satisfied because items 0..6
// of the assertion below pin the spec's canonical order
// (Dashboard, Emails, Rules, Accounts, Watch, Tools, Settings) — exactly
// the 7 rows in `spec/24-app-design-system-and-ui/03-layout-and-shell.md`
// §3.1 lines 89–95. The 8th entry (NavErrorLog) is the post-spec
// Diagnostics group append documented in nav.go and grouped separately
// in sidebar_rows.go (sidebar_rows_test.go enforces the group break).
// Reordering or removing any of the first 7 items, or rearranging Settings
// out of position 7, fails this test.
func TestNavItemsCoverAllKinds(t *testing.T) {
	want := []NavKind{NavDashboard, NavEmails, NavRules, NavAccounts, NavWatch, NavTools, NavSettings, NavErrorLog}
	if len(NavItems) != len(want) {
		t.Fatalf("NavItems has %d entries, want %d", len(NavItems), len(want))
	}
	seen := map[NavKind]int{}
	for i, it := range NavItems {
		if it.Kind != want[i] {
			t.Errorf("NavItems[%d].Kind = %q, want %q", i, it.Kind, want[i])
		}
		if it.Title == "" {
			t.Errorf("NavItems[%d] has empty Title", i)
		}
		if it.Placeholder == "" {
			t.Errorf("NavItems[%d] (%s) has empty Placeholder", i, it.Kind)
		}
		seen[it.Kind]++
	}
	for _, k := range want {
		if seen[k] != 1 {
			t.Errorf("NavKind %q appears %d times, want 1", k, seen[k])
		}
	}
}
