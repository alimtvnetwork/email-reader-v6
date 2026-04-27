//go:build !nofyne

// accounts_layout.go provides a tiny fixed-width layout helper used by
// accounts.go to pin the Actions column to a stable pixel width so the
// Edit / Delete buttons render at their intrinsic size instead of
// stretching to fill 1/7 of the window. Pulled into its own file so the
// helper can be unit-tested without dragging in the full Accounts view
// graph.
package views

import "fyne.io/fyne/v2"

// actionsColumnWidth is the reserved width (in Fyne logical pixels) for
// the Accounts table's Actions column. Sized to comfortably fit a
// pencil-Edit button + a trash-Delete button + the inter-button padding
// at the default Comfortable density. Update if the icon set or button
// label changes.
const actionsColumnWidth float32 = 220

// fixedWidthLayout is a minimal fyne.Layout that pins the container to a
// caller-chosen width and lets height float to the child's MinSize. Used
// by the Accounts table to keep the Actions column from expanding into
// the available row width.
type fixedWidthLayout struct{ width float32 }

// Layout positions every child at (0, 0) with size (l.width, size.Height).
// Multi-child usage keeps every child stacked at the origin — callers
// always pass a single HBox / Label so this is fine in practice.
func (l *fixedWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(l.width, size.Height))
		o.Move(fyne.NewPos(0, 0))
	}
}

// MinSize reports (l.width, max(child.MinSize.Height)) so the Fyne
// container reserves exactly the requested width. Returning the
// child's intrinsic min-height keeps row heights stable across density
// changes (Comfortable vs Compact).
func (l *fixedWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objs {
		if m := o.MinSize(); m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(l.width, h)
}
