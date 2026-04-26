// fyne_apply.go installs the AppTheme on the Fyne app and triggers a
// refresh whenever the active mode changes. Built behind `!nofyne` so the
// pure-Go theme.go can still compile on headless CI.
//
// Bootstrap protocol (spec/24-…/02-theme-implementation.md §5):
//   1. Caller calls theme.Apply(mode) BEFORE any window construction.
//   2. Apply (fyne-free) updates the active mode atomically.
//   3. ApplyToFyne (this file) is invoked from cmd/email-read-ui/main or
//      ui.Run to install AppTheme + repaint. It is also called by the
//      Settings Subscribe consumer for live theme switches.
//go:build !nofyne

package theme

import (
	"fyne.io/fyne/v2"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// ApplyToFyne updates the active mode AND broadcasts the change to the
// running Fyne app (if any). Safe to call from any goroutine — the Fyne
// SetTheme call is marshalled onto the UI thread via fyne.Do.
//
// Behaves as a no-op-on-success when there is no current Fyne app yet
// (cmd-line bootstrap, tests). The mode is still updated so the next
// app.NewWindow picks up the right palette.
func ApplyToFyne(mode core.ThemeMode) errtrace.Result[struct{}] {
	if r := Apply(mode); r.HasError() {
		return r
	}
	a := fyne.CurrentApp()
	if a == nil {
		return errtrace.Ok(struct{}{})
	}
	fyne.Do(func() {
		a.Settings().SetTheme(NewAppTheme())
	})
	return errtrace.Ok(struct{}{})
}
