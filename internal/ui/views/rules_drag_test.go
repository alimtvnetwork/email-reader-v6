// rules_drag_test.go — Slice #114 unit tests for the pure-Go drag
// target-index helper + the dragHandle's Dragged/DragEnd contract.
//
//go:build !nofyne

package views

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestComputeDragTargetIndex_NoMoveOnSmallDrag(t *testing.T) {
	// Half-row threshold: anything under 20px (rowHeight=40) at
	// currentIndex=2 should stay at 2.
	cases := []float32{0, 5, 10, 19, -5, -19}
	for _, dy := range cases {
		if got := computeDragTargetIndex(2, 5, dy, 40); got != 2 {
			t.Errorf("dy=%v: got %d, want 2 (no-move zone)", dy, got)
		}
	}
}

func TestComputeDragTargetIndex_MovesByOneAtHalfRow(t *testing.T) {
	// 20px drag (== rowHeight/2) should snap by 1 row in the drag
	// direction. Round-half-away-from-zero.
	if got := computeDragTargetIndex(2, 5, 20, 40); got != 3 {
		t.Errorf("dy=20: got %d, want 3", got)
	}
	if got := computeDragTargetIndex(2, 5, -20, 40); got != 1 {
		t.Errorf("dy=-20: got %d, want 1", got)
	}
}

func TestComputeDragTargetIndex_MovesByMultipleRows(t *testing.T) {
	// 100px drag at rowHeight=40 = 2.5 rows → rounds to 3.
	if got := computeDragTargetIndex(0, 10, 100, 40); got != 3 {
		t.Errorf("dy=100 from 0: got %d, want 3", got)
	}
	if got := computeDragTargetIndex(7, 10, -100, 40); got != 4 {
		t.Errorf("dy=-100 from 7: got %d, want 4", got)
	}
}

func TestComputeDragTargetIndex_ClampsToBounds(t *testing.T) {
	// Drag past the last row clamps to totalRows-1.
	if got := computeDragTargetIndex(3, 5, 9999, 40); got != 4 {
		t.Errorf("over-drag down: got %d, want 4", got)
	}
	// Drag past the first row clamps to 0.
	if got := computeDragTargetIndex(1, 5, -9999, 40); got != 0 {
		t.Errorf("over-drag up: got %d, want 0", got)
	}
}

func TestComputeDragTargetIndex_DefensiveEdgeCases(t *testing.T) {
	// Single-row table: nothing to swap.
	if got := computeDragTargetIndex(0, 1, 200, 40); got != 0 {
		t.Errorf("single-row: got %d, want 0", got)
	}
	// Zero/negative rowHeight → no-op (return currentIndex).
	if got := computeDragTargetIndex(2, 5, 200, 0); got != 2 {
		t.Errorf("zero rowHeight: got %d, want 2", got)
	}
	if got := computeDragTargetIndex(2, 5, 200, -5); got != 2 {
		t.Errorf("negative rowHeight: got %d, want 2", got)
	}
	// Out-of-range currentIndex gets clamped before computing.
	if got := computeDragTargetIndex(99, 5, 0, 40); got != 4 {
		t.Errorf("oob currentIndex high: got %d, want 4", got)
	}
	if got := computeDragTargetIndex(-3, 5, 0, 40); got != 0 {
		t.Errorf("oob currentIndex low: got %d, want 0", got)
	}
}

func TestDragHandle_AccumulatesDeltaAndFiresOnDragEnd(t *testing.T) {
	var fired int
	var got int
	h := newDragHandle(1, 5, 40, func(target int) {
		fired++
		got = target
	})
	// Three drag events totalling +60px (1.5 rows) → target = 1+2 = 3
	// (round-half-away-from-zero on 1.5).
	h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: 20}})
	h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: 20}})
	h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: 20}})
	h.DragEnd()
	if fired != 1 || got != 3 {
		t.Fatalf("fired=%d target=%d, want fired=1 target=3", fired, got)
	}
	// State must reset after DragEnd: a subsequent zero-drag DragEnd
	// must NOT re-fire (target == currentIndex == 1 → gated).
	h.DragEnd()
	if fired != 1 {
		t.Fatalf("DragEnd re-fired after reset: fired=%d", fired)
	}
}

func TestDragHandle_NoFireOnZeroNetDrag(t *testing.T) {
	var fired int
	h := newDragHandle(2, 5, 40, func(_ int) { fired++ })
	// +5, -5 → net 0 → target == currentIndex → gated.
	h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: 5}})
	h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: -5}})
	h.DragEnd()
	if fired != 0 {
		t.Fatalf("zero net drag fired callback: %d", fired)
	}
}

func TestDragHandle_NilDragEventIgnored(t *testing.T) {
	// Defensive: a nil DragEvent must not panic.
	var fired int
	h := newDragHandle(0, 3, 40, func(_ int) { fired++ })
	h.Dragged(nil)
	h.DragEnd()
	if fired != 0 {
		t.Fatalf("nil drag fired: %d", fired)
	}
}
