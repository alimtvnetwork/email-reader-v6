// emails_query.go — Phase 4.6: spec-canonical EmailQuery / EmailPage /
// EmailSortKey types + the `(*EmailsService).ListPage` method.
//
// Why a new file (vs. expanding emails.go)?
//
//   - Keeps the existing `List(ListEmailsOptions) []EmailSummary` path
//     binary-compatible for the P2.5 UI wiring (`internal/ui/views/
//     emails.go` still calls it). That path will be retired in a
//     later slice once the UI is migrated to ListPage.
//   - Isolates the spec-shape surface so future additions
//     (sort-by-from, sort-by-subject, server-side facet counts) land
//     in one focused file rather than scattered across emails.go.
//
// Spec source: spec/21-app/02-features/02-emails/01-backend.md §2.1
// (List query shape) + §2.2 (paginated result shape).
//
// SQL pushdown vs. service-layer filter:
//   The store's `EmailsList` query (queries.go) only knows about
//   {Alias, Search, Limit, Offset}. Rather than break the store API
//   in this slice, the new fields (`OnlyUnread`, `IncludeDeleted`,
//   `SinceAt`, `UntilAt`, `SortBy`) are applied as a post-fetch
//   filter+sort on the rows the store returned. That is correct, just
//   not optimal — the perf gate (P4.7) already proved the underlying
//   query is 11x under budget at 100k rows, so we have headroom for a
//   service-layer filter pass. A follow-up slice can push these into
//   SQL without touching the public ListPage shape.
//
//   `IncludeDeleted` is a no-op until P4.3 lands the `DeletedAt`
//   column (see `EmailCounts.Deleted` for the matching tripwire).
//
// `Limit==0` semantics: matches `List` — zero means "no LIMIT clause"
// at the store layer, i.e. return everything matching the filter.
// `EmailPage.NextOffset` is then `Offset+len(Items)`; the caller
// detects "end of results" by comparing against `Total`.

package core

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// EmailSortKey enumerates the orderings the UI can request. Spec
// §2.1 lists three; default (zero value) is `EmailSortReceivedDesc`,
// which matches the store's existing `ORDER BY Uid DESC, Id DESC`
// (Uid being a monotonic IMAP-assigned proxy for arrival time).
type EmailSortKey int

const (
	// EmailSortReceivedDesc is the default — newest first.
	EmailSortReceivedDesc EmailSortKey = iota
	// EmailSortReceivedAsc — oldest first. Useful for chronological
	// thread reconstruction.
	EmailSortReceivedAsc
	// EmailSortSubjectAsc — alphabetical by subject. Case-folded.
	EmailSortSubjectAsc
)

// EmailQuery is the spec-canonical filter+pagination input for
// `(*EmailsService).ListPage`. All fields are optional; the zero
// value is "all aliases, all rows, default sort, no pagination".
//
// Field maturity (P4.6 + follow-up):
//   - Alias, Search, Limit, Offset, SortBy, SinceAt, UntilAt, OnlyUnread —
//     fully wired (sort + window + unread post-filter in core).
//   - IncludeDeleted — no-op until P4.3 lands the `DeletedAt` column
//     (matches the `EmailCounts.Deleted == 0` tripwire).
type EmailQuery struct {
	Alias          string       // empty = all accounts
	Search         string       // case-insensitive substring on Subject + From
	OnlyUnread     bool         // when true, drop rows with IsRead = true
	IncludeDeleted bool         // no-op until P4.3 (DeletedAt column)
	SinceAt        time.Time    // zero = no lower bound
	UntilAt        time.Time    // zero = no upper bound
	SortBy         EmailSortKey // default = EmailSortReceivedDesc
	Limit          int          // 0 = no LIMIT
	Offset         int          // 0 = from start
}

// EmailPage is the canonical paginated result shape (spec §2.2). The
// `Total` field reflects the post-filter row count *ignoring*
// Limit/Offset — i.e. the number of rows the caller would get if
// they paged through everything. `NextOffset` is `Offset+len(Items)`,
// suitable for direct re-use as the next call's `EmailQuery.Offset`;
// callers detect end-of-results when `NextOffset >= Total`.
type EmailPage struct {
	Total      int
	Items      []EmailSummary
	NextOffset int
}

// ListPage is the spec-shape replacement for `List`. Returns an
// `EmailPage` so the UI can render pagination controls without a
// second Count round-trip.
//
// Implementation note: fetches with the legacy {Alias, Search}
// filters via the existing store query (which gives us index hits),
// then applies the new filters in-process. See file-level note for
// the rationale.
//
// Error envelope mirrors `List`: `ErrDbOpen` for open failures,
// `ErrDbQueryEmail` (with alias context) for query failures.
func (s *EmailsService) ListPage(ctx context.Context, q EmailQuery) errtrace.Result[EmailPage] {
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[EmailPage](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.ListPage"),
		)
	}
	defer closeFn()

	// Pull the full filtered set (alias+search) WITHOUT pushing the
	// LIMIT/OFFSET — we need every matching row to compute Total
	// after the post-filter pass. This is fine within the perf
	// budget proven by P4.7 (5.29ms p95 at 100k); pushdown is a
	// follow-up.
	rows, err := st.ListEmails(ctx, storeQueryFromCore(q))
	if err != nil {
		return errtrace.Err[EmailPage](
			errtrace.WrapCode(err, errtrace.ErrDbQueryEmail, "core.EmailsService.ListPage").
				WithContext("alias", q.Alias),
		)
	}

	// Project + post-filter in one pass.
	filtered := make([]EmailSummary, 0, len(rows))
	for _, e := range rows {
		// OnlyUnread: drop rows the user has already read.
		if q.OnlyUnread && e.IsRead {
			continue
		}
		// SinceAt / UntilAt window.
		if !q.SinceAt.IsZero() && e.ReceivedAt.Before(q.SinceAt) {
			continue
		}
		if !q.UntilAt.IsZero() && e.ReceivedAt.After(q.UntilAt) {
			continue
		}
		// IncludeDeleted: pinned no-op until P4.3 lands the DeletedAt
		// column (see EmailQuery field comments + EmailCounts.Deleted
		// tripwire).
		_ = q.IncludeDeleted
		filtered = append(filtered, toSummary(e))
	}

	// Sort. The store already returned EmailSortReceivedDesc-ordered
	// rows, so the default branch is a no-op.
	switch q.SortBy {
	case EmailSortReceivedAsc:
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].ReceivedAt < filtered[j].ReceivedAt
		})
	case EmailSortSubjectAsc:
		sort.SliceStable(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Subject) <
				strings.ToLower(filtered[j].Subject)
		})
	}

	total := len(filtered)
	// Apply Offset/Limit window.
	lo := q.Offset
	if lo > total {
		lo = total
	}
	hi := total
	if q.Limit > 0 && lo+q.Limit < hi {
		hi = lo + q.Limit
	}
	page := filtered[lo:hi]

	return errtrace.Ok(EmailPage{
		Total:      total,
		Items:      page,
		NextOffset: lo + len(page),
	})
}

// storeQueryFromCore is the trivial conversion: the only fields the
// store understands today are Alias + Search. The new filters happen
// in-process inside ListPage.
func storeQueryFromCore(q EmailQuery) store.EmailQuery {
	return store.EmailQuery{Alias: q.Alias, Search: q.Search}
}
