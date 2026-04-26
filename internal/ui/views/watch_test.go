// watch_test.go covers the CF-W3 live cadence consumer plumbed by
// internal/ui/views/watch.go. Build-tagged with the rest of the package.
//go:build !nofyne

package views

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// Test_ForwardCadenceEvents_UpdatesLabel pushes two events and asserts
// the label reflects the latest value, plus the goroutine exits when the
// channel closes.
func Test_ForwardCadenceEvents_UpdatesLabel(t *testing.T) {
	lbl := widget.NewLabel("init")
	events := make(chan core.SettingsEvent, 4)
	events <- mkCadenceEv(5)
	events <- mkCadenceEv(12)
	close(events)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { forwardCadenceEvents(events, lbl, cancel); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("forwardCadenceEvents did not return after channel close")
	}
	if got, want := lbl.Text, "cadence: every 12 s"; got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
	// cancel is invoked via defer inside the goroutine — confirm ctx is done.
	select {
	case <-ctx.Done():
	default:
		t.Fatal("ctx not cancelled after goroutine exit")
	}
}

// Test_FormatCadence locks the spec text format so a typo doesn't quietly
// drift the live indicator.
func Test_FormatCadence(t *testing.T) {
	cases := map[uint16]string{
		1:  "cadence: every 1 s",
		3:  "cadence: every 3 s",
		60: "cadence: every 60 s",
	}
	for in, want := range cases {
		if got := formatCadence(in); got != want {
			t.Errorf("formatCadence(%d) = %q, want %q", in, got, want)
		}
	}
}

// Test_AliasLabel_EmptyFallback locks the header empty-state.
func Test_AliasLabel_EmptyFallback(t *testing.T) {
	if got := aliasLabel(""); got != "(no account)" {
		t.Errorf("aliasLabel(\"\") = %q", got)
	}
	if got := aliasLabel("atto"); got != "atto" {
		t.Errorf("aliasLabel(\"atto\") = %q", got)
	}
}

func mkCadenceEv(secs uint16) core.SettingsEvent {
	return core.SettingsEvent{
		Kind:     core.SettingsSaved,
		Snapshot: core.SettingsSnapshot{PollSeconds: secs},
		At:       time.Now(),
	}
}
