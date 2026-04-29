// sidebar_render.go computes the text label for one rendered sidebar
// row. Pure / fyne-free so headless CI can lock the formatting rules
// (header inset, active-caret prefix, badge suffix) without spinning
// up an OpenGL context.
//
// Slice #213 (sidebar hit-area & visual polish) factored this out of
// the previously-inline binder closure in sidebar.go so the rules
// have one canonical implementation + matching headless test
// (sidebar_render_test.go).
package ui

// SidebarRowText returns the rendered text for one row.
//
//   - Header rows: prefixed with "  " (two-space inset) so the
//     italic+bold group label visually nests *inside* the sidebar
//     rather than competing with a nav row's left-edge alignment.
//     This separation is the visual half of the misclick fix —
//     the existing logical fallback in sidebar.go (clicking a
//     header bounces to the next nav row) handles the click side.
//   - Nav rows (active): prefixed with "▸ " caret so the user can
//     see which row is currently selected before they click another.
//     Catches the "Diagnose vs Error Log" misclick by giving an
//     unambiguous "you are here" cue rather than relying on the
//     theme's subtle row-highlight tint.
//   - Nav rows (inactive): plain title with the existing badge
//     suffix from formatNavRowLabel.
//
// All inputs are required. Out-of-range navIdx returns "" — defensive
// fallback so a stale row slice doesn't panic the renderer.
func SidebarRowText(
	row sidebarRow,
	items []NavItem,
	badgeFor func(NavKind) int64,
	active bool,
) string {
	if row.header != "" {
		return "  " + row.header
	}
	if row.navIdx < 0 || row.navIdx >= len(items) {
		return ""
	}
	item := items[row.navIdx]
	var badge int64
	if badgeFor != nil {
		badge = badgeFor(item.Kind)
	}
	base := formatNavRowLabel(item.Title, badge)
	if active {
		return "▸ " + base
	}
	return "  " + base // 2-space indent so active/inactive rows have the same left-edge column
}