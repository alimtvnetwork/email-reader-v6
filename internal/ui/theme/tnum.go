// tnum.go exposes the tabular-numerics OpenType feature flag used by
// numeric columns (Email row count, OpenedUrl count, Watch counters)
// per spec/24-app-design-system-and-ui/01-tokens.md §3.2.
//
// Fyne v2.5.2 has no built-in feature-flag API on the canvas.Text widget,
// so this file ships only the OpenType feature tag string + a typed
// alias. The Watch/Dashboard counters consume `TabularNumFeatures()` and
// pass it through whatever rendering helper they choose. Once Fyne ships
// `text.NewWithFeatures` (post-v2.6), the wiring is a one-line swap.
//
// Pure Go (fyne-free) — keeps the contract testable under `-tags nofyne`.
package theme

// TabularNumFeature is the OpenType feature tag that enables fixed-width
// digit advance widths (so right-aligned counts don't jitter on update).
// Inter Variable ships a `tnum` glyph set.
const TabularNumFeature = "tnum"

// TabularNumFeatures returns the canonical slice of OpenType feature
// tags consumed by numeric-column widgets. v1 returns just `tnum`;
// future additions (e.g. `lnum` for lining figures) extend this slice
// without touching call sites.
func TabularNumFeatures() []string {
	return []string{TabularNumFeature}
}
