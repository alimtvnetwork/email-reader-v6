//go:build !nofyne

package views

import "fyne.io/fyne/v2"

// fixedHeightLayout reserves a stable vertical slot for a single child
// widget. Width follows the parent (so it grows with the window), but
// the MinSize.Height is pinned to `Height` so a `widget.List` placed
// inside it never collapses to its 1-row default and never overflows
// past `Height` — which is what caused the dashboard's recent-activity
// rows to visually overlap the live-counters row beneath them.
//
// Why not use `container.NewVScroll`? VScroll computes its MinSize
// from the child, which for `widget.List` is a single row. We want a
// PRE-allocated vertical slot regardless of how many rows the list
// currently holds.
type fixedHeightLayout struct{ Height float32 }

func (l fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w := float32(0)
	for _, o := range objects {
		if m := o.MinSize(); m.Width > w {
			w = m.Width
		}
	}
	return fyne.NewSize(w, l.Height)
}

func (l fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(size.Width, l.Height))
		o.Move(fyne.NewPos(0, 0))
	}
}
