// Command email-read-ui is the Fyne desktop frontend for email-read.
//
// Phase 2 / Step 8 milestone: this binary just opens an empty window so we
// can confirm the Fyne toolchain links on macOS / Windows / Linux. Layout,
// sidebar, and views land in subsequent steps (9, 10, …).
//
// Run locally:
//
//	go run ./cmd/email-read-ui
//
// Cross-compile single-file binaries:
//
//	fyne package -os darwin  -src ./cmd/email-read-ui
//	fyne package -os windows -src ./cmd/email-read-ui
//	fyne package -os linux   -src ./cmd/email-read-ui
package main

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Version mirrors the CLI version so both binaries report the same release.
// Bumped per release in lockstep with cmd/email-read/main.go.
const Version = "0.19.0"

func main() {
	a := app.NewWithID("dev.lovable.email-read")
	w := a.NewWindow("email-read · v" + Version)

	// Placeholder content for Step 8. Replaced by sidebar+detail in Step 9.
	w.SetContent(container.NewCenter(widget.NewLabel(
		"email-read UI — Phase 2 scaffold\n(empty window; sidebar lands in Step 9)",
	)))

	w.Resize(fyneSize(960, 640))
	w.CenterOnScreen()
	w.ShowAndRun()
}
