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

// defaultStoreOpener wraps `store.Open` so the package-level wrappers
// can construct a service per call. Removed in P2.8.
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

// ============================================================
// Deprecated package-level wrappers — preserved for back-compat
// during the Phase 2 view migration. Removed in P2.8 once every
// caller goes through an injected *EmailsService.
// ============================================================

// Deprecated: use (*EmailsService).List. See NewEmailsService.
func ListEmails(ctx context.Context, opts ListEmailsOptions) errtrace.Result[[]EmailSummary] {
	svc := mustDefaultEmailsService()
	return svc.List(ctx, opts)
}

// Deprecated: use (*EmailsService).Get. See NewEmailsService.
func GetEmail(ctx context.Context, alias string, uid uint32) errtrace.Result[*EmailDetail] {
	svc := mustDefaultEmailsService()
	return svc.Get(ctx, alias, uid)
}

// Deprecated: use (*EmailsService).Count. See NewEmailsService.
func CountEmails(ctx context.Context, alias string) errtrace.Result[int] {
	svc := mustDefaultEmailsService()
	return svc.Count(ctx, alias)
}

// mustDefaultEmailsService builds a default-injected service.
// Constructor cannot fail here — defaultStoreOpener is non-nil — so
// we panic on the impossible branch to keep the wrapper signatures
// clean. Removed with the wrappers in P2.8.
func mustDefaultEmailsService() *EmailsService {
	res := NewEmailsService(defaultStoreOpener)
	if res.HasError() {
		panic("core: default EmailsService construction failed: " + res.Error().Error())
	}
	return res.Value()
}

func toSummary(e store.Email) EmailSummary {
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
