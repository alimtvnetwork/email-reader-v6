// services_watch_test.go — Slice #116b (Phase 6.2): AttachWatch
// wire-up. Mirror of services_activity_test.go. Pins the nil-safety,
// nil-input, real-input, and idempotency contracts of AttachWatch so
// a future refactor that breaks any of them trips a test rather than
// crashing the NavWatch render path.
//
//go:build !nofyne

package ui

import (
	"testing"
	"time"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/eventbus"
)

// makeTestWatch builds a minimal *core.Watch with a stub LoopFactory
// and a fresh event bus. We only need a non-nil pointer for the
// attach contract — Start/Stop are exercised in watch_test.go.
func makeTestWatch(t *testing.T) *core.Watch {
	t.Helper()
	bus := eventbus.New[core.WatchEvent](4)
	r := core.NewWatch(stubLoopFactory{}, bus, time.Now)
	if r.HasError() {
		t.Fatalf("NewWatch: %v", r.Error())
	}
	return r.Value()
}

// stubLoopFactory is the smallest LoopFactory that satisfies the
// interface — never invoked by the attach tests but required by
// NewWatch's nil-dep guard.
type stubLoopFactory struct{}

func (stubLoopFactory) New(core.WatchOptions) core.Loop { return nil }

func TestServices_AttachWatch_NilReceiver_NoPanic(t *testing.T) {
	var s *Services
	s.AttachWatch(makeTestWatch(t)) // must not panic
}

func TestServices_AttachWatch_NilWatch_LeavesFieldNil(t *testing.T) {
	s := &Services{}
	s.AttachWatch(nil)
	if s.Watch != nil {
		t.Fatalf("Watch: expected nil, got non-nil")
	}
}

func TestServices_AttachWatch_RealWatch_PopulatesField(t *testing.T) {
	s := &Services{}
	w := makeTestWatch(t)
	s.AttachWatch(w)
	if s.Watch == nil {
		t.Fatalf("Watch: expected non-nil after attach with real watch")
	}
	if s.Watch != w {
		t.Fatalf("Watch: stored pointer differs from input — attach must not wrap")
	}
}

func TestServices_AttachWatch_Idempotent(t *testing.T) {
	s := &Services{}
	w := makeTestWatch(t)
	s.AttachWatch(w)
	if s.Watch == nil {
		t.Fatal("Watch: nil after first attach")
	}
	s.AttachWatch(w)
	if s.Watch != w {
		t.Fatal("Watch: pointer changed after second attach with same input")
	}
}
