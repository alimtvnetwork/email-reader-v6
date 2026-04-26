// emails_refresh_watcher_test.go — compile-time + runtime assertion
// that the production `*watcher.Watcher` adapter satisfies the
// `core.Refresher` interface introduced by P4.4.
//
// This is the smallest possible test that proves the
// `EmailsService.Refresh` seam has a real production wire-up. If
// someone changes the Refresher signature without updating the
// watcher adapter (or vice versa), this fails at compile time, which
// is exactly the guarantee we want.

package core

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/watcher"
)

// Compile-time assertion: *watcher.Watcher implements core.Refresher.
var _ Refresher = (*watcher.Watcher)(nil)

// Runtime smoke test: the production adapter wires cleanly into
// EmailsService via WithRefresher and the chain accepts the typed dep.
func TestEmailsService_AcceptsProductionWatcherAsRefresher(t *testing.T) {
	t.Parallel()
	w := watcher.NewWatcher()
	if err := w.Register(watcher.Options{
		Account: config.Account{Alias: "primary"},
	}); err != nil {
		t.Fatalf("watcher.Register: %v", err)
	}

	fake := &fakeEmailsStore{}
	opener, _ := makeOpener(fake, nil)
	svc, _ := NewEmailsService(opener).Value(), NewEmailsService(opener).Error()
	if svc == nil {
		t.Fatal("NewEmailsService returned nil")
	}
	got := svc.WithRefresher(w)
	if got == nil {
		t.Fatal("WithRefresher returned nil")
	}
	// Don't actually invoke Refresh — pollOnce would dial real IMAP.
	// The compile-time _ assertion above is the contract; this test
	// just proves the bootstrap shape compiles & links end-to-end.
}
