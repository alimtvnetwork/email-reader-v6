package ui

import "testing"

// TestNavItemsCoverAllKinds locks the canonical sidebar order and ensures
// every NavKind constant has exactly one matching entry. Catches typos like
// duplicating "rules" or accidentally dropping an item during a refactor.
func TestNavItemsCoverAllKinds(t *testing.T) {
	want := []NavKind{NavDashboard, NavEmails, NavRules, NavAccounts, NavWatch, NavTools}
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
