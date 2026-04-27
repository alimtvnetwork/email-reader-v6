// sidebar_rows.go computes the flat row layout for the sidebar list:
// each row is either an italic group header (for items that declare a
// non-empty Group, on the first occurrence of that group) or a real
// nav row pointing back at NavItems by index.
//
// Lives here (not in sidebar.go) so it stays fyne-free and headless
// CI can lock the row order — including the new "Diagnostics" header
// added in Phase 3.4 of the error-trace logging upgrade.
package ui

// sidebarRow is one entry in the rendered sidebar list. Exactly one
// of header / navIdx is meaningful per row.
type sidebarRow struct {
	// header is the italic group label rendered above the first
	// item in a group (e.g. "Diagnostics"). Empty for nav rows.
	header string
	// navIdx is the index into NavItems for selectable nav rows.
	// Ignored when header != "".
	navIdx int
}

// buildSidebarRows expands NavItems into a row slice with group
// headers inserted before the first item of each new group. Items
// without a Group are emitted as-is. Group headers are emitted only
// once per consecutive run of the same group (so two adjacent
// Diagnostics items share one header). The original NavItems order
// is preserved 1:1 — this is layout, not sort.
func buildSidebarRows(items []NavItem) []sidebarRow {
	rows := make([]sidebarRow, 0, len(items)+2)
	lastGroup := ""
	for i, it := range items {
		if it.Group != "" && it.Group != lastGroup {
			rows = append(rows, sidebarRow{header: it.Group})
			lastGroup = it.Group
		} else if it.Group == "" {
			lastGroup = ""
		}
		rows = append(rows, sidebarRow{navIdx: i})
	}
	return rows
}

// firstNavRow returns the index of the first selectable (non-header)
// row, or 0 when rows is empty / all-headers (defensive — should not
// happen with the canonical NavItems list).
func firstNavRow(rows []sidebarRow) int {
	for i, r := range rows {
		if r.header == "" {
			return i
		}
	}
	return 0
}
