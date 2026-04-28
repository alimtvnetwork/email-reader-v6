// errlog_notifier.go decides *when* a newly-appended error should
// raise a desktop toast vs. only bump the sidebar badge.
//
// Phase 3.5 of the error-trace logging upgrade (.lovable/plan.md):
//
//	Notification level: Badge + toast on **first** error,
//	badge-only after (until the user reads → resets).
//
// "First" here means the 0→1 transition of the unread counter — i.e.
// the user is currently caught up (no unread errors) and a brand-new
// one arrives. Subsequent errors that pile on top of an already-
// unread count only bump the badge, keeping the UI calm during an
// error storm.
//
// Lives in package ui (fyne-free) so the decision logic is unit-
// testable headlessly. The Fyne `app.SendNotification` adapter sits
// in app.go and feeds this notifier via NewErrLogNotifier.
package ui

import (
	"sync"

	"github.com/lovable/email-read/internal/ui/errlog"
)

// ToastFn is the platform side of the notifier — typically
// `fyne.CurrentApp().SendNotification`. Kept as a func type so tests
// can substitute a recorder without importing fyne.
type ToastFn func(title, body string)

// ErrLogNotifier subscribes to the errlog ring and invokes Toast on
// the 0→1 unread transition. After a toast fires, further appends
// only bump the badge (no toast) until ResetQuietPeriod is called —
// done by the Error Log view's MarkRead path so the next batch of
// errors after the user catches up gets a fresh toast.
type ErrLogNotifier struct {
	toast ToastFn

	mu      sync.Mutex
	inStorm bool // true once we've toasted; false again after Reset
}

// NewErrLogNotifier wires a notifier to the given toast adapter and
// starts a goroutine that drains the errlog Subscribe channel. Pass
// nil toast to disable (useful for tests / headless runs); the
// notifier is still constructed so ResetQuietPeriod is safe to call.
//
// Returns the notifier so the caller can hold a reference (and call
// ResetQuietPeriod when the user opens the Error Log view).
func NewErrLogNotifier(toast ToastFn) *ErrLogNotifier {
	n := &ErrLogNotifier{toast: toast}
	go n.consume(errlog.Subscribe())
	return n
}

// consume is the goroutine body — exposed for tests so they can feed
// a synthetic channel.
func (n *ErrLogNotifier) consume(ch <-chan errlog.Entry) {
	for e := range ch {
		n.handle(e)
	}
}

// handle implements the 0→1 transition rule. The decision uses the
// notifier's own `inStorm` flag (not errlog.Unread) because two
// appends can race the unread atomic — using a private mutex makes
// the rule deterministic for tests.
func (n *ErrLogNotifier) handle(e errlog.Entry) {
	n.mu.Lock()
	first := !n.inStorm
	n.inStorm = true
	n.mu.Unlock()

	if first && n.toast != nil {
		n.toast(toastTitle(e), toastBody(e))
	}
}

// ResetQuietPeriod clears the storm flag so the next append fires a
// toast again. Called by the sidebar OnSelect handler when the user
// opens NavErrorLog (right next to the existing list.Refresh that
// clears the badge — see sidebar.go).
func (n *ErrLogNotifier) ResetQuietPeriod() {
	n.mu.Lock()
	n.inStorm = false
	n.mu.Unlock()
}

// toastTitle / toastBody are the user-facing strings. Title is short
// ("Error in <component>"); body shows the one-line summary so the
// user gets context without clicking through. Both pure helpers so
// the strings are unit-testable.
func toastTitle(e errlog.Entry) string {
	if e.Component == "" {
		return "email-read · Error"
	}
	return "email-read · Error in " + e.Component
}

func toastBody(e errlog.Entry) string {
	const maxBody = 140
	s := e.Summary
	if len(s) > maxBody {
		s = s[:maxBody-1] + "…"
	}
	return s + "\n\nOpen Diagnostics → Error Log for the full trace."
}
