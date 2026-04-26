// emails_deps_test.go — Phase 4.8 coverage for the deps-struct
// constructor and the backwards-compat shim path.
//
// Coverage matrix (8 cases):
//   - HappyPath_AllDepsWired                 → struct fields populated
//   - NilStore_ReturnsInvalidArgument        → required-field guard
//   - NilClock_DefaultsToTimeNow             → optional-field default
//   - CustomClock_FlowsThrough               → injection works end-to-end
//   - OptionalRefresherNil_RefreshFailsCleanly → P4.4 contract preserved
//   - OptionalRefresherSet_RefreshDelegates  → wire path proven
//   - LegacyConstructor_RoutesViaDeps        → shim equivalence
//   - WithRefresher_StillWorksAfterDeps      → fluent path co-exists

package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

func TestNewEmailsServiceFromDeps_HappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	r := &fakeRefresher{}
	stubClock := func() time.Time {
		return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	}

	res := NewEmailsServiceFromDeps(EmailsServiceDeps{
		Store: opener, Refresher: r, Clock: stubClock,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	s := res.Value()
	if s.openStore == nil {
		t.Error("openStore not wired")
	}
	if s.refresher != r {
		t.Error("refresher not wired")
	}
	if s.clock == nil {
		t.Fatal("clock should never be nil after construction")
	}
	if got := s.clock(); !got.Equal(stubClock()) {
		t.Errorf("clock = %v, want stub value %v", got, stubClock())
	}
}

func TestNewEmailsServiceFromDeps_NilStore_Rejected(t *testing.T) {
	t.Parallel()
	res := NewEmailsServiceFromDeps(EmailsServiceDeps{Store: nil})
	if !res.HasError() {
		t.Fatal("nil Store should be rejected")
	}
	var coded *errtrace.Coded
	if !errAs(res.Error(), &coded) || coded.Code != errtrace.ErrCoreInvalidArgument {
		t.Errorf("code = %v, want ErrCoreInvalidArgument", coded)
	}
}

func TestNewEmailsServiceFromDeps_NilClock_DefaultsToTimeNow(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)

	before := time.Now()
	res := NewEmailsServiceFromDeps(EmailsServiceDeps{Store: opener}) // Clock omitted
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	s := res.Value()
	if s.clock == nil {
		t.Fatal("nil Clock should be defaulted, not propagated")
	}
	got := s.clock()
	after := time.Now()
	// The default must produce a "now"-ish value bracketed by our
	// before/after measurements (with generous tolerance for slow CI).
	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Errorf("default clock = %v, want between %v and %v", got, before, after)
	}
}

func TestNewEmailsServiceFromDeps_CustomClock_FlowsThrough(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	frozen := time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC)
	res := NewEmailsServiceFromDeps(EmailsServiceDeps{
		Store: opener, Clock: func() time.Time { return frozen },
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	if got := res.Value().clock(); !got.Equal(frozen) {
		t.Errorf("clock = %v, want frozen %v", got, frozen)
	}
}

func TestNewEmailsServiceFromDeps_RefresherSet_RefreshDelegates(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	r := &fakeRefresher{}
	res := NewEmailsServiceFromDeps(EmailsServiceDeps{
		Store: opener, Refresher: r,
	})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	if rres := res.Value().Refresh(context.Background(), "primary"); rres.HasError() {
		t.Fatalf("Refresh: %v", rres.Error())
	}
	if atomic.LoadInt32(&r.calls) != 1 {
		t.Errorf("PollOnce calls = %d, want 1", r.calls)
	}
}

func TestNewEmailsServiceFromDeps_NoRefresher_RefreshFailsCleanly(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	res := NewEmailsServiceFromDeps(EmailsServiceDeps{Store: opener})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	rres := res.Value().Refresh(context.Background(), "primary")
	if !rres.HasError() {
		t.Fatal("Refresh without a Refresher should error (P4.4 contract)")
	}
	var coded *errtrace.Coded
	if !errAs(rres.Error(), &coded) || coded.Code != errtrace.ErrCoreInvalidArgument {
		t.Errorf("code = %v, want ErrCoreInvalidArgument", coded)
	}
}

// Backwards-compat shim: NewEmailsService(openStore) must produce
// the same observable shape as NewEmailsServiceFromDeps with only
// Store set.
func TestNewEmailsService_LegacyShim_RoutesViaDeps(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)

	a := NewEmailsService(opener)
	if a.HasError() {
		t.Fatalf("legacy ctor: %v", a.Error())
	}
	if a.Value().clock == nil {
		t.Error("legacy ctor must still default the Clock")
	}
	if a.Value().openStore == nil {
		t.Error("legacy ctor must still wire the store")
	}
	if a.Value().refresher != nil {
		t.Error("legacy ctor must leave Refresher nil")
	}
}

// Fluent WithRefresher continues to work after NewEmailsServiceFromDeps.
func TestNewEmailsServiceFromDeps_WithRefresher_StillChainable(t *testing.T) {
	t.Parallel()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	res := NewEmailsServiceFromDeps(EmailsServiceDeps{Store: opener})
	if res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}

	r := &fakeRefresher{}
	s := res.Value().WithRefresher(r)
	if rres := s.Refresh(context.Background(), "primary"); rres.HasError() {
		t.Fatalf("Refresh after WithRefresher: %v", rres.Error())
	}
	if atomic.LoadInt32(&r.calls) != 1 {
		t.Errorf("PollOnce calls = %d, want 1", r.calls)
	}
}

// errAs is a tiny helper to keep test bodies focused. errors.As with
// less ceremony at the call site.
func errAs(err error, target any) bool {
	return errors.As(err, target)
}
