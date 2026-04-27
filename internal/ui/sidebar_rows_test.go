// sidebar_rows_test.go locks the layout the sidebar list renders.
// Lives in package ui (not _test fyne-gated) so headless CI runs it.
package ui

import "testing"

func TestBuildSidebarRows_DiagnosticsHeaderInsertedOnce(t *testing.T) {
	rows := buildSidebarRows(NavItems)
	// Expect one header row for "Diagnostics" preceding NavErrorLog,
	// no other headers (other items have empty Group).
	headers := 0
	var headerBeforeErrorLog bool
	for i, r := range rows {
		if r.header == "" {
			continue
		}
		headers++
		if r.header == "Diagnostics" {
			// The next row must be a nav row pointing at NavErrorLog.
			if i+1 >= len(rows) || rows[i+1].header != "" {
				t.Fatalf("Diagnostics header not followed by a nav row")
			}
			if NavItems[rows[i+1].navIdx].Kind != NavErrorLog {
				t.Fatalf("Diagnostics header followed by %q, want %q",
					NavItems[rows[i+1].navIdx].Kind, NavErrorLog)
			}
			headerBeforeErrorLog = true
		}
	}
	if headers != 1 {
		t.Fatalf("got %d headers, want exactly 1 (Diagnostics)", headers)
	}
	if !headerBeforeErrorLog {
		t.Fatalf("Diagnostics header missing before NavErrorLog")
	}
}

func TestFirstNavRow_SkipsHeaders(t *testing.T) {
	rows := []sidebarRow{
		{header: "Diagnostics"},
		{navIdx: 7},
	}
	if got := firstNavRow(rows); got != 1 {
		t.Errorf("firstNavRow = %d, want 1", got)
	}
}
