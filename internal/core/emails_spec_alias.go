// emails_spec_alias.go — spec-name alias for EmailsService.
//
// **Phase 4.1 audit verdict.** spec/21-app/02-features/02-emails/01-backend.md
// §1 names the service `Emails` with constructor signature
// `NewEmails(store.Store, *Watch, *Rules, Clock) *Emails`. The Phase 2.4
// refactor landed the same surface as `EmailsService` with a functional
// `storeOpener` injection (test-friendly) instead of the spec's direct
// `store.Store` + `*Watch` + `*Rules` + `Clock` injection.
//
// **Decision (P4.1):** keep the existing struct name to avoid a churn-only
// rename across the UI (`internal/ui/services.go`, `internal/ui/views/emails.go`,
// CLI exports, AST guard allowlist). Add a doc-only spec-name alias so:
//
//  1. Future spec-driven slices (P4.2 `MarkRead`, P4.3 `Delete`/`Undelete`,
//     P4.4 `Refresh`, P4.5 `Counts`, P4.6 `EmailQuery`/`EmailPage` shape,
//     P4.7 perf gate, P4.8 watch+rules injection) can reference the
//     spec name `*core.Emails` directly without callers needing a type
//     translation step.
//  2. Symbol Map row 13.1 (status `⏳ Emails`) flips to `🟡 partial`
//     once the alias is in — the *type* exists; the broader 7-method
//     surface still ships incrementally.
//  3. The constructor name `NewEmails` resolves to the same default
//     wiring used by `NewDefaultEmailsService`, so spec-aligned bootstrap
//     code can call `core.NewEmails(...)` even though the underlying
//     impl still uses the functional opener seam.
//
// **What this file is not:** it is **not** a behavior change. The
// type and the constructor below resolve to the existing
// `EmailsService` / `NewDefaultEmailsService`. The spec's wider surface
// (MarkRead/Delete/Refresh/Counts) is delivered by P4.2–P4.5; this slice
// only ships the name bridge so those slices can be authored against
// the spec identifier from line one.
package core

import "github.com/lovable/email-read/internal/errtrace"

// Emails is the spec-canonical name for the email-read core service.
// See spec/21-app/02-features/02-emails/01-backend.md §1.
//
// Currently a type alias for EmailsService. The two names refer to
// the exact same struct — methods defined on `*EmailsService`
// (`List`, `Get`, `Count`) are reachable via `*Emails` without any
// adapter layer.
//
// Methods landing in P4.2+ (`MarkRead`, `Delete`, `Undelete`,
// `Refresh`, `Counts`) will be defined directly on `*EmailsService`
// and will likewise be reachable via `*Emails`.
type Emails = EmailsService

// NewEmails is the spec-canonical constructor name. It returns a
// production-wired `*Emails` (== `*EmailsService`) using the same
// default `store.Open`-backed opener that `NewDefaultEmailsService`
// uses.
//
// Spec signature is `NewEmails(store.Store, *Watch, *Rules, Clock) *Emails`.
// That broader injection lands in P4.8 (once `*Watch` and `*Rules`
// are themselves typed services with stable constructors). Until
// then, the no-arg form is the supported entry point and matches the
// Phase 2.5 bootstrap (`internal/ui/services.go::BuildServices`).
//
// Returns the same `errtrace.Result[*EmailsService]` envelope as
// `NewDefaultEmailsService` to keep the constructor shape uniform
// with the rest of the core package.
func NewEmails() errtrace.Result[*Emails] {
	return NewDefaultEmailsService()
}
