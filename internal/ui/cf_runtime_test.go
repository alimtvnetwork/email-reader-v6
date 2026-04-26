// cf_runtime_test.go locks two CF acceptance contracts that hinge on
// `forwardSettingsEvents` — the single Settings → consumers fan-out
// running in the WatchRuntime:
//
//	CF-T1 — `Settings.Save` updates the launcher's `ChromePath` so the
//	        next `OpenUrl` honors the new binary without restart. We
//	        prove the launcher.Reload() seam is invoked and Path()
//	        reflects it within the same fan-out turn.
//
//	CF-W3 — A new `PollSeconds` lands in the cadence accessor used by
//	        new Start calls within ≤1s of the publish. Drives the
//	        forwarder with a stub event channel and polls the
//	        accessor.
//
// Build-tag-free on purpose: the forwarder lives in watch_runtime.go
// (also tag-free) so this test compiles under `-tags nofyne`.
//
// Spec: spec/21-app/02-features/06-tools/99-consistency-report.md CF-T1,
//       spec/21-app/02-features/05-watch/99-consistency-report.md CF-W3.
package ui

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
)

// TestCF_T1_Tools_OpenUrl_RespectsNewChromePath proves a Settings event
// carrying a fresh BrowserOverride.ChromePath flows through
// forwardSettingsEvents into launcher.Reload, and the next Path() call
// returns the new binary. This is the contract Tools.OpenUrl depends on
// (it calls launcher.Path() right before launching).
func TestCF_T1_Tools_OpenUrl_RespectsNewChromePath(t *testing.T) {
	dir := t.TempDir()
	pathA := writeFakeBin(t, dir, "alpha-browser")
	pathB := writeFakeBin(t, dir, "beta-browser")

	launcher := browser.New(config.Browser{ChromePath: pathA})
	if got, err := launcher.Path(); err != nil || got != pathA {
		t.Fatalf("baseline launcher.Path = %q,%v; want %q,nil", got, err, pathA)
	}

	rt := &WatchRuntime{
		PollChans: core.NewPollChanRegistry(),
	}
	events := make(chan core.SettingsEvent, 1)
	events <- core.SettingsEvent{Snapshot: core.SettingsSnapshot{
		PollSeconds:     7,
		BrowserOverride: core.BrowserOverride{ChromePath: pathB},
	}}
	close(events)

	var liveMu sync.RWMutex
	livePoll := 3
	done := make(chan struct{})
	go func() {
		forwardSettingsEvents(events, launcher, rt, &liveMu, &livePoll)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("forwardSettingsEvents did not return after channel close")
	}

	got, err := launcher.Path()
	if err != nil || got != pathB {
		t.Fatalf("post-Settings launcher.Path = %q,%v; want %q,nil (CF-T1)", got, err, pathB)
	}
}

// TestCF_W3_Watch_LiveCadenceUpdate proves a new PollSeconds value is
// visible via the accessor closure within the same fan-out turn (well
// under the spec's ≤1s budget). Also asserts the PollChans registry
// receives the broadcast for live runners.
func TestCF_W3_Watch_LiveCadenceUpdate(t *testing.T) {
	rt := &WatchRuntime{PollChans: core.NewPollChanRegistry()}
	regCh := rt.PollChans.Acquire("alias-w3")

	var liveMu sync.RWMutex
	livePoll := 3
	read := func() int {
		liveMu.RLock()
		defer liveMu.RUnlock()
		return livePoll
	}

	events := make(chan core.SettingsEvent, 1)
	go forwardSettingsEvents(events, nil, rt, &liveMu, &livePoll)

	deadline := time.Now().Add(1 * time.Second)
	events <- core.SettingsEvent{Snapshot: core.SettingsSnapshot{PollSeconds: 21}}

	for read() != 21 {
		if time.Now().After(deadline) {
			t.Fatalf("livePoll never reached 21 within 1s (got %d) — CF-W3", read())
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case got := <-regCh:
		if got != 21 {
			t.Fatalf("PollChans registry got %d, want 21", got)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("registry chan never received the broadcast")
	}
	close(events)
}

// writeFakeBin drops a file the browser launcher's fileExists() check
// will accept (regular file, mode 0755). Returns the absolute path.
func writeFakeBin(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}
	return p
}
