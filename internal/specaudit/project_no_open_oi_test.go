// project_no_open_oi_test.go — Slice #144 / Task #3 burn-down.
//
// Closes AC-PROJ-35:
//
//	"All open issues (`OI-N`) referenced in any feature 99-report are
//	 resolved (✅ Closed) before merge."
//
// Implementation: walks every
// `spec/21-app/02-features/*/99-consistency-report.md`, finds every
// markdown table row whose first cell matches `| OI-<digits> |`, and
// asserts the row contains a "✅" closure marker. Flags any row that:
//
//   - lacks the ✅ glyph entirely (open / forgotten), or
//   - explicitly says `🟡` / `⚠` / `TODO` / `FIXME` (in-flight).
//
// The test is intentionally lenient about HOW closure is phrased — any
// of "✅ Closed", "✅ **Closed**", "✅ Resolved", "✅ Done" all pass —
// because the existing 99-reports vary in wording. The only hard
// requirement is the ✅ glyph + absence of an open-status marker on
// the same row.
//
// Reserved test name from spec/21-app/97-acceptance-criteria.md
// row AC-PROJ-35: `Project_NoOpenOiReferencesAtMergeTime`. Matches via
// the substring AC-PROJ-35 below so the coverage audit (slice #119)
// resolves the citation without depending on the Go function name.
package specaudit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// oiRowRe matches a markdown table row whose first cell is `OI-<digits>`.
// We anchor on the leading `|` + spaces + `OI-` so prose mentions of
// `OI-2` inside paragraph text do NOT trip the audit.
var oiRowRe = regexp.MustCompile(`^\|\s*OI-\d+\s*\|`)

// inFlightMarkers are status glyphs/words that mean "still open" even
// when a ✅ also appears on the same row (defensive — should never
// happen in practice, but pins the spec author's intent).
var inFlightMarkers = []string{"🟡", "⚠", "TODO", "FIXME"}

// Test_Project_NoOpenOiReferencesAtMergeTime closes AC-PROJ-35.
func Test_Project_NoOpenOiReferencesAtMergeTime(t *testing.T) {
	root := repoRoot(t)
	featuresDir := filepath.Join(root, "spec", "21-app", "02-features")
	if _, err := os.Stat(featuresDir); err != nil {
		t.Skipf("features dir not present in this repo layout: %v", err)
	}

	var (
		violations []string
		scanned    int
	)
	walkErr := filepath.Walk(featuresDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) != "99-consistency-report.md" {
			return nil
		}
		scanned++
		violations = append(violations, scanReportForOpenOI(t, root, path)...)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", featuresDir, walkErr)
	}
	if scanned == 0 {
		t.Fatalf("no 99-consistency-report.md files found under %s", featuresDir)
	}
	if len(violations) > 0 {
		t.Fatalf("AC-PROJ-35: %d open OI row(s) across %d 99-report(s):\n  %s",
			len(violations), scanned, strings.Join(violations, "\n  "))
	}
}

// scanReportForOpenOI returns one violation message per OI row in the
// file at path that lacks ✅ or carries an in-flight marker.
// `root` is used to render relative paths in the violation messages.
func scanReportForOpenOI(t *testing.T, root, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	rel, _ := filepath.Rel(root, path)
	var out []string
	for i, line := range strings.Split(string(b), "\n") {
		if !oiRowRe.MatchString(line) {
			continue
		}
		if reason := classifyOIRow(line); reason != "" {
			out = append(out, rel+":"+itoaPlus1(i)+": "+reason+" — "+truncate(line, 140))
		}
	}
	return out
}

// classifyOIRow returns "" when the row is closed cleanly, otherwise
// a short diagnostic. Pulled out so the walker stays compact and the
// rule is unit-test-friendly without spinning up the file scan.
func classifyOIRow(row string) string {
	if !strings.Contains(row, "✅") {
		return "missing ✅ closure marker"
	}
	for _, m := range inFlightMarkers {
		if strings.Contains(row, m) {
			return "carries in-flight marker " + m + " on a ✅ row"
		}
	}
	return ""
}

// itoaPlus1 returns the human-readable line number (1-indexed).
// Avoids importing `strconv` for one call site at the cost of a
// 2-line helper — keeps this file's import block tight.
func itoaPlus1(i int) string {
	// Small enough that `fmt.Sprintf("%d", i+1)` would also be fine;
	// inlined to avoid pulling fmt for one call.
	return formatInt(i + 1)
}

// formatInt is a 32-bit-safe stringifier for non-negative ints.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// truncate caps s at max runes with an ellipsis. Used for readable
// failure output when an offending row is several hundred chars long
// (consistency reports often pack a Why/How explanation into one cell).
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
