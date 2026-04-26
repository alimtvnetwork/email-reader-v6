// emails.go — typed *EmailsService + thin package-level wrappers.
//
// **Phase 2.4 refactor.** The old shape exposed `ListEmails`,
// `GetEmail`, `CountEmails` as package-level funcs that each called
// `store.Open()` (process-global) and `defer st.Close()`. That made
// emails logic untestable without spinning up a real SQLite DB and
// also made it impossible to share a single store handle across
// dashboard + emails calls in the same UI tick.
//
// The new shape mirrors `core.DashboardService` (see `dashboard.go`):
//
//   - `EmailsService` struct holds one injected dep — `openStore` —
//     a function that returns an open `*store.Store` plus a `close`
//     callback. Production passes `defaultStoreOpener` (which wraps
//     `store.Open`); tests inject a fake that hands out an in-memory
//     store with no Close side-effects.
//   - `NewEmailsService` is the explicit constructor; nil dep →
//     ErrCoreInvalidArgument.
//   - `List` / `Get` / `Count` are the typed methods that replace
//     the old package funcs. Same error envelope, same scopes — only
//     the dependency source changed.
//   - The package-level `ListEmails` / `GetEmail` / `CountEmails`
//     stay as deprecated thin wrappers that build a default-injected
//     service per call. Wrappers go away in P2.8 once the UI is
//     fully wired through bootstrap.
package core

import (
	"context"
	"strings"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// EmailSummary is a UI-friendly projection of a stored email row. The body
// fields are kept separate so the UI can render a list cheaply (subject +
// from + date) and only render the body when the user opens an email.
type EmailSummary struct {
	Id         int64
	Alias      string
	Uid        uint32
	From       string
	Subject    string
	ReceivedAt string // formatted RFC3339 in UTC; empty when not stored
	Snippet    string // first ~140 chars of BodyText (or BodyHtml fallback)
	FilePath   string
}

// EmailDetail extends EmailSummary with the full body fields needed by the
// detail pane.
type EmailDetail struct {
	EmailSummary
	To       string
	Cc       string
	BodyText string
	BodyHtml string
}

// ListEmailsOptions mirrors store.EmailQuery but is exported for UI use
// without leaking the store package into views.
type ListEmailsOptions struct {
	Alias  string
	Search string
	Limit  int
	Offset int
}

// emailsStore is the narrow read-surface EmailsService needs. It exists
// so tests can fake the store without spinning up SQLite. Production
// implementation is `*store.Store`.
type emailsStore interface {
	ListEmails(ctx context.Context, q store.EmailQuery) ([]store.Email, error)
	GetEmailByUid(ctx context.Context, alias string, uid uint32) (*store.Email, error)
	CountEmails(ctx context.Context, alias string) (int, error)
	// SetEmailRead is the Phase 4 (P4.2) extension. Returns total
	// RowsAffected across all internal batches; (0, nil) for empty
	// `uids` (the spec's "no SQL" branch).
	SetEmailRead(ctx context.Context, alias string, uids []uint32, read bool) (int64, error)
	// CountUnreadEmails is the Phase 4 (P4.5) extension. Counts rows
	// with `IsRead = 0` matching the alias (empty alias = all).
	CountUnreadEmails(ctx context.Context, alias string) (int, error)
}

// storeOpener returns an open emailsStore plus a `close` callback the
// caller MUST invoke when done. Functional dependency (matching
// `urlLauncher` / `configLoader` style in tools.go / dashboard.go) so
// tests inject a one-line closure.
//
// `close` is always non-nil — even on failure — to keep call-site
// shape uniform: callers can always defer it. On error, close is a
// no-op.
type storeOpener func() (s emailsStore, close func() error, err error)

// EmailsService renders email summaries / details / counts from an
// injected store opener. Stateless — concurrent method calls open
// independent store handles.
type EmailsService struct {
	openStore storeOpener
}

// NewEmailsService constructs an EmailsService. The opener dep is
// required: passing nil returns ErrCoreInvalidArgument (no defensive
// default-injection — bootstrap is the right place for that choice).
func NewEmailsService(openStore storeOpener) errtrace.Result[*EmailsService] {
	if openStore == nil {
		return errtrace.Err[*EmailsService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "NewEmailsService: openStore is nil"))
	}
	return errtrace.Ok(&EmailsService{openStore: openStore})
}

// NewDefaultEmailsService is the production-bootstrap convenience
// constructor: builds an `*EmailsService` wired to the real
// `store.Open`. Used by `internal/ui` (Phase 2.5 wiring) so the UI
// package never needs to know the unexported `storeOpener` shape.
//
// Cannot fail in practice (defaultStoreOpener is non-nil), but
// returns a Result envelope to keep the constructor signature
// parallel with `NewEmailsService` and to leave the door open for
// future validation.
func NewDefaultEmailsService() errtrace.Result[*EmailsService] {
	return NewEmailsService(defaultStoreOpener)
}

// defaultStoreOpener wraps `store.Open` so the package-level wrappers
// (and `NewDefaultEmailsService`) can construct a service per call.
// Removed in P2.8 once bootstrap fully owns the opener wiring.
func defaultStoreOpener() (emailsStore, func() error, error) {
	st, err := store.Open()
	if err != nil {
		return nil, func() error { return nil }, err
	}
	return st, st.Close, nil
}

// List returns email summaries matching opts. Same error envelope as
// the pre-refactor `ListEmails` package func: ErrDbOpen for open
// failures, ErrDbQueryEmail (with alias context) for query failures.
func (s *EmailsService) List(ctx context.Context, opts ListEmailsOptions) errtrace.Result[[]EmailSummary] {
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[[]EmailSummary](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.List"),
		)
	}
	defer closeFn()
	rows, err := st.ListEmails(ctx, store.EmailQuery{
		Alias:  opts.Alias,
		Search: opts.Search,
		Limit:  opts.Limit,
		Offset: opts.Offset,
	})
	if err != nil {
		return errtrace.Err[[]EmailSummary](
			errtrace.WrapCode(err, errtrace.ErrDbQueryEmail, "core.EmailsService.List").
				WithContext("alias", opts.Alias),
		)
	}
	out := make([]EmailSummary, 0, len(rows))
	for _, e := range rows {
		out = append(out, toSummary(e))
	}
	return errtrace.Ok(out)
}

// Get returns the full detail for one stored email identified by
// (alias, uid). Returns an error if no such row exists.
func (s *EmailsService) Get(ctx context.Context, alias string, uid uint32) errtrace.Result[*EmailDetail] {
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[*EmailDetail](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.Get"),
		)
	}
	defer closeFn()
	e, err := st.GetEmailByUid(ctx, alias, uid)
	if err != nil {
		return errtrace.Err[*EmailDetail](
			errtrace.WrapCode(err, errtrace.ErrDbQueryEmail, "core.EmailsService.Get").
				WithContext("alias", alias).
				WithContext("uid", uid),
		)
	}
	d := EmailDetail{
		EmailSummary: toSummary(*e),
		To:           e.ToAddr,
		Cc:           e.CcAddr,
		BodyText:     e.BodyText,
		BodyHtml:     e.BodyHtml,
	}
	return errtrace.Ok(&d)
}

// Count returns how many emails are stored for the given alias
// (empty alias = total across all accounts).
func (s *EmailsService) Count(ctx context.Context, alias string) errtrace.Result[int] {
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[int](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.Count"),
		)
	}
	defer closeFn()
	n, err := st.CountEmails(ctx, alias)
	if err != nil {
		return errtrace.Err[int](
			errtrace.WrapCode(err, errtrace.ErrDbQueryEmail, "core.EmailsService.Count").
				WithContext("alias", alias),
		)
	}
	return errtrace.Ok(n)
}

// MarkReadMaxUids caps how many UIDs the caller may pass to MarkRead
// in a single call. The store layer further chunks the slice into
// batches of 999 (`SQLITE_MAX_VARIABLE_NUMBER`); this ceiling exists
// at the service boundary so a buggy caller surfaces an error code
// (`ErrCoreInvalidArgument`) rather than silently issuing thousands
// of UPDATE statements. Spec
// `spec/21-app/02-features/02-emails/01-backend.md` §2.3 sets the
// limit at 1000.
const MarkReadMaxUids = 1000

// Unit is the spec-canonical empty-success value for MarkRead/Delete/
// Undelete (per spec §2.3/2.4). Defined locally so callers don't need
// to invent a sentinel; future package-wide use is fine.
type Unit struct{}

// MarkRead flips the IsRead flag for every (alias, uid) pair in
// `uids`. Idempotent (re-issuing the same op leaves the store in the
// same state). Empty `uids` is a fast-path no-op — no SQL is issued
// and `(Unit{}, nil)` is returned without opening the store.
//
// Validates `len(uids) <= MarkReadMaxUids`; over-budget calls return
// `ErrCoreInvalidArgument` with `uid_count` context (spec §2.3
// "21221 EmailsMarkReadTooMany"; mapped onto the existing core
// invalid-argument code until the dedicated 21221 entry is minted in
// a later registry slice).
//
// On store error, wraps with `ErrDbInsertEmail` (UPDATE failures live
// in the same write-path bucket as inserts in the existing registry)
// + alias / uid_count context.
func (s *EmailsService) MarkRead(ctx context.Context, alias string, uids []uint32, read bool) errtrace.Result[Unit] {
	if len(uids) > MarkReadMaxUids {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"core.EmailsService.MarkRead: too many uids").
			WithContext("uid_count", len(uids)).
			WithContext("max", MarkReadMaxUids))
	}
	if len(uids) == 0 {
		return errtrace.Ok(Unit{})
	}
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[Unit](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.MarkRead"),
		)
	}
	defer closeFn()
	if _, err := st.SetEmailRead(ctx, alias, uids, read); err != nil {
		return errtrace.Err[Unit](
			errtrace.WrapCode(err, errtrace.ErrDbInsertEmail, "core.EmailsService.MarkRead").
				WithContext("alias", alias).
				WithContext("uid_count", len(uids)),
		)
	}
	return errtrace.Ok(Unit{})
}

// EmailCounts is the toolbar/dashboard projection populated by
// `(*EmailsService).Counts`. Spec
// `spec/21-app/02-features/02-emails/01-backend.md` §2.6 defines the
// shape as `{Total, Unread, Deleted}` — the `Deleted` field is wired
// to a constant `0` for now because the M0010 migration shipped
// `IsFlagged` instead of the spec's `DeletedAt` column. Soft-delete
// tracking lands in P4.3 (deferred pending the M0010 reconciliation
// decision); when it does, only this struct's `Deleted` populator
// changes and the field name is already in place so callers compile
// through.
type EmailCounts struct {
	Total   int
	Unread  int
	Deleted int // always 0 until P4.3; present so the spec shape is stable.
}

// Counts returns total + unread (and zero-pinned deleted, see above)
// for the given alias. Empty alias = all accounts. Issues two
// independent COUNT queries against the store inside one open
// handle; on either failure wraps with `ErrDbQueryEmail` + alias ctx.
//
// Spec: §2.6 / §3.5. Used by the toolbar badge and the dashboard
// `AccountHealthRow`.
func (s *EmailsService) Counts(ctx context.Context, alias string) errtrace.Result[EmailCounts] {
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[EmailCounts](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.Counts"),
		)
	}
	defer closeFn()
	total, err := st.CountEmails(ctx, alias)
	if err != nil {
		return errtrace.Err[EmailCounts](
			errtrace.WrapCode(err, errtrace.ErrDbQueryEmail, "core.EmailsService.Counts.Total").
				WithContext("alias", alias),
		)
	}
	unread, err := st.CountUnreadEmails(ctx, alias)
	if err != nil {
		return errtrace.Err[EmailCounts](
			errtrace.WrapCode(err, errtrace.ErrDbQueryEmail, "core.EmailsService.Counts.Unread").
				WithContext("alias", alias),
		)
	}
	return errtrace.Ok(EmailCounts{Total: total, Unread: unread, Deleted: 0})
}

	s := EmailSummary{
		Id:       e.Id,
		Alias:    e.Alias,
		Uid:      e.Uid,
		From:     e.FromAddr,
		Subject:  e.Subject,
		FilePath: e.FilePath,
		Snippet:  snippet(e.BodyText, e.BodyHtml),
	}
	if !e.ReceivedAt.IsZero() {
		s.ReceivedAt = e.ReceivedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return s
}

// snippet returns a short single-line preview of the body. Prefers plain
// text; falls back to HTML with tags stripped naively. Capped at ~140
// chars.
func snippet(text, html string) string {
	src := text
	if strings.TrimSpace(src) == "" {
		src = stripTags(html)
	}
	src = strings.Join(strings.Fields(src), " ") // collapse whitespace
	if len(src) > 140 {
		src = src[:139] + "…"
	}
	return src
}

func stripTags(s string) string {
	var b strings.Builder
	skip := false
	for _, r := range s {
		switch {
		case r == '<':
			skip = true
		case r == '>':
			skip = false
		case !skip:
			b.WriteRune(r)
		}
	}
	return b.String()
}
