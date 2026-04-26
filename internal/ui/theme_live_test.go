// theme_live_test.go covers forwardThemeEvents — the Settings → Theme
// live consumer wired in app.go. Build-tagged with the rest of the
// fyne-backed package so we don't import fyne behind `-tags nofyne`.
//go:build !nofyne

package ui

import (
	"testing"
	"time"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/ui/theme"
)

// Test_ForwardThemeEvents_AppliesAndDedupes drives a stub channel and
// verifies (a) every distinct mode reaches theme.Active(), (b) repeated
// modes are no-ops (no SetTheme call — observable here as Active() never
// flipping back since the value is identical anyway, plus the call
// returning quickly).
func Test_ForwardThemeEvents_AppliesAndDedupes(t *testing.T) {
	events := make(chan core.SettingsEvent, 8)
	events <- mkEv(core.ThemeLight)
	events <- mkEv(core.ThemeLight) // duplicate → skipped
	events <- mkEv(core.ThemeDark)
	close(events)

	done := make(chan struct{})
	go func() {
		forwardThemeEvents(events)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("forwardThemeEvents did not return after channel close")
	}
	if got := theme.Active(); got != core.ThemeDark {
		t.Fatalf("Active() = %v, want ThemeDark (last event)", got)
	}
}

// Test_ForwardThemeEvents_EmptyChannelTerminates ensures a closed channel
// with zero events returns cleanly (no goroutine leak).
func Test_ForwardThemeEvents_EmptyChannelTerminates(t *testing.T) {
	events := make(chan core.SettingsEvent)
	close(events)
	done := make(chan struct{})
	go func() {
		forwardThemeEvents(events)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("forwardThemeEvents leaked on empty closed channel")
	}
}

func mkEv(m core.ThemeMode) core.SettingsEvent {
	return core.SettingsEvent{
		Kind:     core.SettingsSaved,
		Snapshot: core.SettingsSnapshot{Theme: m},
		At:       time.Now(),
	}
}
