// sidebar_render_test.go locks the SidebarRowText formatting rules
// added in Slice #213 (sidebar hit-area & visual polish). Pure
// fyne-free so it runs under -tags nofyne on CI.
package ui

import "testing"

func TestSidebarRowText_Header(t *testing.T) {
	got := SidebarRowText(sidebarRow{header: "Diagnostics"}, NavItems, nil, false)
	if got != "  Diagnostics" {
		t.Fatalf("header indent missing; got %q", got)
	}
}

func TestSidebarRowText_NavInactive(t *testing.T) {
	// NavItems[0] is Dashboard.
	got := SidebarRowText(sidebarRow{navIdx: 0}, NavItems, nil, false)
	if got != "  Dashboard" {
		t.Fatalf("inactive row should have 2-space indent + plain title; got %q", got)
	}
}

func TestSidebarRowText_NavActive(t *testing.T) {
	got := SidebarRowText(sidebarRow{navIdx: 0}, NavItems, nil, true)
	if got != "▸ Dashboard" {
		t.Fatalf("active row should have caret prefix; got %q", got)
	}
}

func TestSidebarRowText_NavWithBadge(t *testing.T) {
	// Find Error Log index dynamically so the test survives nav reorder.
	idx := -1
	for i, it := range NavItems {
		if it.Kind == NavErrorLog {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("NavErrorLog not present in NavItems")
	}
	badge := func(k NavKind) int64 {
		if k == NavErrorLog {
			return 7
		}
		return 0
	}
	got := SidebarRowText(sidebarRow{navIdx: idx}, NavItems, badge, false)
	want := "  Error Log  (7)"
	if got != want {
		t.Fatalf("badge suffix lost; got %q want %q", got, want)
	}
}

func TestSidebarRowText_OutOfRange(t *testing.T) {
	if got := SidebarRowText(sidebarRow{navIdx: 999}, NavItems, nil, false); got != "" {
		t.Fatalf("out-of-range navIdx should return empty; got %q", got)
	}
}