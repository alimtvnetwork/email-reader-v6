// emails_refresh.go — Phase 4.4: `(*EmailsService).Refresh(ctx, alias)`.
//
// What it does
//   Triggers a one-shot poll of the IMAP server for one alias, so the
//   UI's "🔄 Refresh" button can ask the watcher to drain new mail
//   without waiting for the next scheduled tick. Spec source:
//   `spec/21-app/02-features/02-emails/01-backend.md` §2.5.
//
// Why a narrow `Refresher` interface (vs. importing watcher directly)
//   `internal/watcher` does not yet expose a one-shot `PollOnce` —
//   today it only has the long-running `Run` loop. Wiring that
//   refactor + the spec method into one slice would balloon scope
//   (watcher's batching, cursor advancement, and stats reset all
//   need careful sequencing). Instead this slice:
//
//     1. Defines `Refresher` — a one-method interface in core that
//        the watcher will satisfy with a future `PollOnce` shim.
//     2. Adds an optional `refresher` dep on `EmailsService` set via
//        `WithRefresher` so existing `NewEmailsService` call sites
//        (UI bootstrap, all tests in this package) stay
//        binary-compatible.
//     3. Implements `Refresh` with a complete error envelope:
//          - empty alias               → ErrCoreInvalidArgument
//          - no refresher injected     → ErrCoreInvalidArgument
//                                        (config-time bug, not runtime)
//          - context already cancelled → ctx.Err() wrapped as
//                                        ErrCoreInvalidArgument so
//                                        the caller can distinguish
//                                        from a watcher-internal failure
//          - watcher returns error     → wrapped with
//                                        ErrWatcherPollFailed + alias ctx
//                                        (registry code reused from the
//                                        watcher domain; new dedicated
//                                        emails-refresh code is a
//                                        registry-restructure concern,
//                                        tracked separately).
//
//   When the watcher gains `PollOnce`, the production wiring is one
//   line in bootstrap: `svc = svc.WithRefresher(watcherAdapter{w})`.
//   No core changes required.

package core

import (
	"context"
	"strings"

	"github.com/lovable/email-read/internal/errtrace"
)

// Refresher is the narrow one-shot poll surface EmailsService.Refresh
// delegates to. Implementations live outside core (production: a
// thin watcher adapter; tests: a hand-rolled fake).
//
// Contract:
//   - alias is non-empty (caller-validated by Refresh before invoking).
//   - On success returns nil; the IMAP fetch + persist round-trip is
//     complete and any new rows are visible to subsequent `List` calls.
//   - On failure returns an error describing the IMAP/DB cause; the
//     caller (Refresh) decides which registry code to wrap it with.
//   - MUST honor ctx cancellation by returning promptly.
type Refresher interface {
	PollOnce(ctx context.Context, alias string) error
}

// WithRefresher returns a copy of s with the given Refresher
// attached. Returns the same `*EmailsService` pointer for fluent
// chaining at bootstrap time:
//
//	svc, _ := core.NewDefaultEmailsService().Unwrap()
//	svc = svc.WithRefresher(watcherAdapter{w})
//
// Passing nil clears any previously-set refresher (useful in tests).
// Mutating-in-place (vs. returning a fresh struct) keeps the
// accessor surface small — EmailsService is documented as "stateless
// w.r.t. method calls" but its dep set is set-once at bootstrap, so
// this is consistent with how `openStore` works.
func (s *EmailsService) WithRefresher(r Refresher) *EmailsService {
	s.refresher = r
	return s
}

// Refresh triggers a one-shot watcher poll for `alias`. Returns
// `Unit{}` on success — the new rows are observable via subsequent
// `List` / `ListPage` calls.
//
// Spec: §2.5. Error envelope detailed in the file-level comment.
func (s *EmailsService) Refresh(ctx context.Context, alias string) errtrace.Result[Unit] {
	if strings.TrimSpace(alias) == "" {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"core.EmailsService.Refresh: alias is required"))
	}
	if s.refresher == nil {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"core.EmailsService.Refresh: no Refresher injected; "+
				"call (*EmailsService).WithRefresher at bootstrap").
			WithContext("alias", alias))
	}
	// Honor ctx eagerly — keeps the error cause local to core rather
	// than asking each Refresher impl to re-implement the pre-check.
	if err := ctx.Err(); err != nil {
		return errtrace.Err[Unit](
			errtrace.WrapCode(err, errtrace.ErrCoreInvalidArgument,
				"core.EmailsService.Refresh: ctx cancelled before poll").
				WithContext("alias", alias),
		)
	}
	if err := s.refresher.PollOnce(ctx, alias); err != nil {
		return errtrace.Err[Unit](
			errtrace.WrapCode(err, errtrace.ErrWatcherPollFailed,
				"core.EmailsService.Refresh").
				WithContext("alias", alias),
		)
	}
	return errtrace.Ok(Unit{})
}
