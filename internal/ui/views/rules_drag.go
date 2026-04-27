// rules_drag.go — Slice #114 drag-handle reorder for the Rules view.
//
// Two layers, deliberately split for testability:
//
//   - Pure-Go: `computeDragTargetIndex` translates a cumulative
//     vertical drag (in pixels) plus a row-height estimate into the
//     target row index. Clamped to `[0, totalRows-1]`. Zero Fyne
//     dependency — table-tested without spinning the driver.
//
//   - Fyne widget: `dragHandle` is a tiny `widget.BaseWidget` that
//     implements `fyne.Draggable` (Dragged/DragEnd) + renders a "⋮⋮"
//     glyph. It accumulates `DragEvent.Dragged.DY` then on DragEnd
//     calls back with `computeDragTargetIndex(...)`. The handle owns
//     no state about the rest of the table — the callback closure
//     does the `Reorder` plumbing, so the handle stays composable
//     and reusable for future tables (Accounts, Watch).
//
// Why a custom handle (not `container.NewDraggable` or similar):
// Fyne v2.5 does not ship a built-in row-drag affordance for VBox-
// based tables; `widget.List` has experimental drag support but the
// Rules view uses `container.NewVBox` (so each row is a 5-col grid
// with rich child widgets, not a single label). Implementing
// `fyne.Draggable` on a per-row handle is the smallest surface that
// gets us a real drag gesture without rewriting the table.
//
//go:build !nofyne

package views

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// defaultRuleRowHeightPx is the pixel height we assume for one Rules
// table row when translating a cumulative drag into a target index.
// Picked empirically from the current Fyne theme metrics — one row of
// 5-col `GridWithColumns` with a single-line label averages ~36px;
// rounding to 40 gives a small dead-zone before the first row swap,
// which feels less twitchy in manual testing than a tight match.
//
// If this proves wrong on dense themes (e.g. a future small-text
// preset), the handle constructor accepts an override. The default
// keeps existing call-sites zero-config.
const defaultRuleRowHeightPx float32 = 40

// computeDragTargetIndex translates a cumulative vertical drag
// (positive = downward) into the destination row index for a row
// currently sitting at `currentIndex`. Clamped to `[0, totalRows-1]`
// so a drag past either end snaps to the corresponding terminal row.
//
// Pure function — no I/O, no Fyne types. The Fyne handle accumulates
// the drag and calls this once on `DragEnd`.
//
// Edge cases (table-tested):
//   - totalRows ≤ 1            → always returns 0 (nothing to swap).
//   - rowHeightPx ≤ 0          → returns currentIndex (defensive;
//     rowHeightPx coming from a misconfigured theme would otherwise
//     divide-by-zero or invert the sign).
//   - small drag (< rowHeightPx/2) → returns currentIndex (no move).
//   - large drag past last row → clamps to totalRows-1.
//   - large drag past first row → clamps to 0.
func computeDragTargetIndex(currentIndex, totalRows int, dragYPx, rowHeightPx float32) int {
	if totalRows <= 1 {
		return 0
	}
	if currentIndex < 0 {
		currentIndex = 0
	}
	if currentIndex >= totalRows {
		currentIndex = totalRows - 1
	}
	if rowHeightPx <= 0 {
		return currentIndex
	}
	// Snap-to-row: divide by row height, round to nearest integer.
	// We bias toward "no move" by requiring at least half a row's
	// worth of drag before counting one position — matches what
	// users intuit as "I dragged it past the next row".
	steps := dragYPx / rowHeightPx
	// Round half-away-from-zero so a drag of exactly 0.5*rowHeight
	// down moves one row down (and -0.5*rowHeight moves one up),
	// rather than a banker's-rounding tie that maps to no move.
	if steps >= 0 {
		steps += 0.5
	} else {
		steps -= 0.5
	}
	target := currentIndex + int(steps)
	if target < 0 {
		target = 0
	}
	if target >= totalRows {
		target = totalRows - 1
	}
	return target
}

// dragHandle is a Fyne widget that renders a "⋮⋮" grab affordance
// and reports cumulative vertical drags via `OnDragEnd(targetIndex)`.
//
// State kept minimal:
//   - `accumDY` is the running sum of `DragEvent.Dragged.DY` since
//     the most recent DragEnd. Reset to 0 on every DragEnd so a
//     subsequent drag starts fresh.
//
// The handle does NOT mutate the rules slice itself — it just
// reports the target index. The owning `ruleRow` closure is
// responsible for calling `opts.Reorder` and refreshing.
type dragHandle struct {
	widget.BaseWidget
	currentIndex int
	totalRows    int
	rowHeightPx  float32
	accumDY      float32
	OnDragEnd    func(targetIndex int)
}

// newDragHandle constructs a handle bound to `currentIndex` within a
// table of `totalRows`. Pass `0` for `rowHeightPx` to use
// `defaultRuleRowHeightPx`. The `onDragEnd` callback fires only
// when the target index DIFFERS from currentIndex — saves the
// caller a no-op `Reorder` round-trip.
func newDragHandle(currentIndex, totalRows int, rowHeightPx float32,
	onDragEnd func(targetIndex int)) *dragHandle {
	if rowHeightPx <= 0 {
		rowHeightPx = defaultRuleRowHeightPx
	}
	h := &dragHandle{
		currentIndex: currentIndex,
		totalRows:    totalRows,
		rowHeightPx:  rowHeightPx,
		OnDragEnd: func(target int) {
			if target == currentIndex {
				return
			}
			if onDragEnd != nil {
				onDragEnd(target)
			}
		},
	}
	h.ExtendBaseWidget(h)
	return h
}

// Dragged accumulates the per-event vertical delta. Horizontal
// motion is intentionally ignored — the table only reorders along
// the Y axis.
func (h *dragHandle) Dragged(ev *fyne.DragEvent) {
	if ev == nil {
		return
	}
	h.accumDY += ev.Dragged.DY
}

// DragEnd computes the target index, fires the callback (if the
// target differs from the start), and resets state. Safe to call
// without a preceding Dragged — accumDY stays 0 → target ==
// currentIndex → callback gated to no-op.
func (h *dragHandle) DragEnd() {
	target := computeDragTargetIndex(h.currentIndex, h.totalRows, h.accumDY, h.rowHeightPx)
	h.accumDY = 0
	if h.OnDragEnd != nil {
		h.OnDragEnd(target)
	}
}

// CreateRenderer renders the handle as a centred "⋮⋮" glyph. We use
// `canvas.Text` (not `widget.Label`) so the glyph stays compact and
// doesn't inherit the Label's vertical padding — a tighter handle
// hits better with cursor precision during a drag.
func (h *dragHandle) CreateRenderer() fyne.WidgetRenderer {
	txt := canvas.NewText("⋮⋮", fyne.CurrentApp().Settings().Theme().Color(
		"foreground", fyne.CurrentApp().Settings().ThemeVariant()))
	txt.TextStyle = fyne.TextStyle{Bold: true}
	txt.Alignment = fyne.TextAlignCenter
	return widget.NewSimpleRenderer(container.NewCenter(txt))
}
