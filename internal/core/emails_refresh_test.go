// emails_refresh_test.go — Phase 4.4 coverage for
// `(*EmailsService).Refresh` + `Refresher` interface + `WithRefresher`.
//
// Coverage matrix (8 cases):
//   - HappyPath_DelegatesToRefresherWithAlias
//   - PropagatesAliasInPollOnceCall
//   - EmptyAlias_ReturnsAliasRequired         (errtrace code)
//   - WhitespaceAlias_AlsoRejected            (TrimSpace contract)
//   - NoRefresher_ReturnsConfigBugError       (ErrEmailsRefresherUnwired)
//   - CtxCancelledBeforePoll_ShortCircuits    (refresher MUST NOT be invoked)
//   - RefresherError_WrappedWithPollCycleCode (+ alias context)
//   - WithRefresher_IsChainable_ReplacesPriorDep

package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
)

// fakeRefresher records every PollOnce call and lets the test
// program a synthetic error.
type fakeRefresher struct {
	calls       int32
	lastAlias   string
	pollErr     error
	observedCtx context.Context
}

func (f *fakeRefresher) PollOnce(ctx context.Context, alias string) error {
	atomic.AddInt32(&f.calls, 1)
	f.lastAlias = alias
	f.observedCtx = ctx
	return f.pollErr
}

// newRefreshSvc builds an EmailsService with a no-op store opener so
// the Refresh path can be exercised in isolation.
func newRefreshSvc(t *testing.T, r Refresher) *EmailsService {
	t.Helper()
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	res := NewEmailsService(opener)
	if res.HasError() {
		t.Fatalf("NewEmailsService: %v", res.Error())
	}
	return res.Value().WithRefresher(r)
}

func TestEmailsService_Refresh_HappyPath(t *testing.T) {
	r := &fakeRefresher{}
	s := newRefreshSvc(t, r)

	res := s.Refresh(context.Background(), "primary")
	if res.HasError() {
		t.Fatalf("unexpected err: %v", res.Error())
	}
	if got := atomic.LoadInt32(&r.calls); got != 1 {
		t.Errorf("PollOnce called %d times, want 1", got)
	}
	if r.lastAlias != "primary" {
		t.Errorf("alias = %q, want primary", r.lastAlias)
	}
}

func TestEmailsService_Refresh_EmptyAlias_ReturnsAliasRequired(t *testing.T) {
	r := &fakeRefresher{}
	s := newRefreshSvc(t, r)

	res := s.Refresh(context.Background(), "")
	if !res.HasError() {
		t.Fatal("want error, got nil")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrWatchAliasRequired {
		t.Errorf("code = %v, want ErrWatchAliasRequired", coded)
	}
	if atomic.LoadInt32(&r.calls) != 0 {
		t.Errorf("PollOnce should NOT be invoked when alias is empty")
	}
}

func TestEmailsService_Refresh_WhitespaceAlias_AlsoRejected(t *testing.T) {
	r := &fakeRefresher{}
	s := newRefreshSvc(t, r)

	res := s.Refresh(context.Background(), "   \t ")
	if !res.HasError() {
		t.Fatal("whitespace-only alias should be rejected")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrWatchAliasRequired {
		t.Errorf("code = %v, want ErrWatchAliasRequired", coded)
	}
}

func TestEmailsService_Refresh_NoRefresher_ReturnsConfigError(t *testing.T) {
	// Build the service WITHOUT calling WithRefresher.
	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	svcRes := NewEmailsService(opener)
	if svcRes.HasError() {
		t.Fatalf("NewEmailsService: %v", svcRes.Error())
	}

	res := svcRes.Value().Refresh(context.Background(), "primary")
	if !res.HasError() {
		t.Fatal("want config error, got nil")
	}
	var coded *errtrace.Coded
	// Errtrace registry restructure: the no-refresher branch now
	// returns the dedicated ER-EML-22003 (ErrEmailsRefresherUnwired)
	// instead of the generic ER-COR-21701. This lets bootstrap
	// dashboards/log alerts distinguish "the user clicked Refresh
	// before the watcher runtime came up" from generic argument-
	// validation failures.
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrEmailsRefresherUnwired {
		t.Errorf("code = %v, want ErrEmailsRefresherUnwired", coded)
	}
	// alias context still attached for diagnostics.
	if !findCtxKey(coded, "alias", "primary") {
		t.Errorf("alias context missing: %+v", coded.Context)
	}
}

func TestEmailsService_Refresh_CtxCancelled_ShortCircuits(t *testing.T) {
	r := &fakeRefresher{}
	s := newRefreshSvc(t, r)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done before Refresh runs

	res := s.Refresh(ctx, "primary")
	if !res.HasError() {
		t.Fatal("want ctx error, got nil")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrCoreInvalidArgument {
		t.Errorf("code = %v, want ErrCoreInvalidArgument", coded)
	}
	if !errors.Is(res.Error(), context.Canceled) {
		t.Errorf("err chain should wrap context.Canceled: %v", res.Error())
	}
	if atomic.LoadInt32(&r.calls) != 0 {
		t.Errorf("PollOnce called despite cancelled ctx")
	}
}

func TestEmailsService_Refresh_RefresherError_WrappedWithPollCycle(t *testing.T) {
	pollErr := errors.New("imap unreachable")
	r := &fakeRefresher{pollErr: pollErr}
	s := newRefreshSvc(t, r)

	res := s.Refresh(context.Background(), "primary")
	if !res.HasError() {
		t.Fatal("want wrapped error, got nil")
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != errtrace.ErrWatcherPollCycle {
		t.Fatalf("code = %v, want ErrWatcherPollCycle", coded)
	}
	if !errors.Is(res.Error(), pollErr) {
		t.Errorf("err chain must wrap original pollErr: %v", res.Error())
	}
	if !findCtxKey(coded, "alias", "primary") {
		t.Errorf("alias context missing: %+v", coded.Context)
	}
}

func TestEmailsService_WithRefresher_ReplacesPriorDep(t *testing.T) {
	first := &fakeRefresher{}
	second := &fakeRefresher{}
	s := newRefreshSvc(t, first).WithRefresher(second)

	if res := s.Refresh(context.Background(), "primary"); res.HasError() {
		t.Fatalf("err: %v", res.Error())
	}
	if atomic.LoadInt32(&first.calls) != 0 {
		t.Errorf("first refresher should be detached, got calls=%d", first.calls)
	}
	if atomic.LoadInt32(&second.calls) != 1 {
		t.Errorf("second refresher should receive the call, got calls=%d", second.calls)
	}
}

// findCtxKey scans a Coded error's context slice for a key/value pair.
func findCtxKey(c *errtrace.Coded, key string, want any) bool {
	for _, f := range c.Context {
		if f.Key == key && f.Value == want {
			return true
		}
	}
	return false
}
