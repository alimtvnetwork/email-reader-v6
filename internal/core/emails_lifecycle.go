// emails_lifecycle.go — Phase 4.3 soft-delete lifecycle.
//
// Adds `(*EmailsService).Delete` and `(*EmailsService).Undelete` —
// the symmetric pair that mutates the M0012 `Emails.DeletedAt`
// column added in `internal/store/migrate/m0012_add_email_deletedat.go`.
//
// **Why one column, two methods (vs. one "SetDeleted(bool)" toggle)**
//
//   - Delete needs the wall-clock (`s.clock().Unix()`) to stamp
//     `DeletedAt`. Undelete writes SQL NULL and intentionally has no
//     timestamp. Hiding both behind a single `SetDeleted(bool)`
//     forces every caller to invent a "what timestamp?" answer when
//     bool=false, which is meaningless. Two methods keep each call
//     site's intent obvious from the symbol alone.
//   - Mirrors the spec shape (§2.4 of
//     `spec/21-app/02-features/02-emails/01-backend.md`) which lists
//     `Delete` and `Undelete` as separate verbs.
//
// **Why idempotent (vs. strict pre-condition checks)**
//
//   Strict mode would query `DeletedAt` first to fail-fast on
//   already-deleted / not-deleted UIDs (the reserved
//   `ErrEmailsDeleteAlreadyDeleted` / `ErrEmailsUndeleteNotDeleted`
//   codes). That doubles the per-call DB round-trips and races with
//   any concurrent UI Refresh. The idempotent shape — re-issuing the
//   same op leaves the store in the same state, RowsAffected may be
//   nonzero on repeat — matches `MarkRead`'s contract and lets the
//   UI fire-and-forget without read-modify-write ceremony. The codes
//   are reserved so a future strict-mode service variant can adopt
//   them without a registry rewire.
//
// **Why ErrEmailsLifecycleNotFound for zero RowsAffected**
//
//   Distinct from the idempotent semantics above: when the caller
//   passes a non-empty UID set and the UPDATE matches zero rows, that
//   means *none* of the UIDs exist for this alias — a real caller
//   bug (stale UI snapshot, alias typo, race against an account
//   delete). Surfacing a coded error here lets the UI render
//   "selected emails no longer exist" rather than silently no-oping.
//   Note: this is one error for the whole batch; we don't currently
//   distinguish "5 of 10 UIDs missing" — that needs a per-UID
//   pre-check that we deliberately skipped (see idempotency rationale).

package core

import (
	"context"

	"github.com/lovable/email-read/internal/errtrace"
)

// LifecycleMaxUids caps how many UIDs a single Delete/Undelete call
// may target. Same ceiling as `MarkReadMaxUids` so all batch-mutation
// methods share one budget — keeps the UI's "select-all + bulk-op"
// failure mode uniform across actions.
const LifecycleMaxUids = MarkReadMaxUids

// Delete soft-deletes every (alias, uid) pair in `uids` by stamping
// `Emails.DeletedAt = s.clock().Unix()`. Idempotent: re-issuing the
// same call updates the timestamp to the new clock reading (UI may
// rely on the latest stamp for "deleted N seconds ago" labels).
//
// Empty `uids` is a fast-path no-op — no SQL is issued and
// `(Unit{}, nil)` is returned without opening the store.
//
// Validation: `len(uids) > LifecycleMaxUids` → `ErrCoreInvalidArgument`
// with `uid_count` + `max` context. Store-open failures surface as
// `ErrDbOpen`; UPDATE failures as `ErrDbInsertEmail` (write-path
// bucket, matches `MarkRead`). Zero RowsAffected on non-empty input
// → `ErrEmailsLifecycleNotFound` (alias + uid_count context).
func (s *EmailsService) Delete(ctx context.Context, alias string, uids []uint32) errtrace.Result[Unit] {
	return s.setLifecycle(ctx, alias, uids, true)
}

// Undelete clears `Emails.DeletedAt` (writes SQL NULL) for every
// (alias, uid) pair in `uids`. Idempotent like `Delete`. Empty `uids`
// returns `(Unit{}, nil)` without opening the store. Same error
// envelope as `Delete`.
func (s *EmailsService) Undelete(ctx context.Context, alias string, uids []uint32) errtrace.Result[Unit] {
	return s.setLifecycle(ctx, alias, uids, false)
}

// setLifecycle is the shared implementation of Delete / Undelete.
// Splitting at this seam — rather than duplicating the validation /
// open / exec / close boilerplate twice — keeps the two public
// methods one-line each and ensures any future change (e.g.
// telemetry, rate-limit guard) lands in exactly one place.
//
// `delete=true` stamps `clock().Unix()`; `delete=false` writes nil
// (NULL in the store). The `*int64` polarity convention is documented
// on `Store.SetEmailDeletedAt`.
func (s *EmailsService) setLifecycle(ctx context.Context, alias string, uids []uint32, delete bool) errtrace.Result[Unit] {
	if len(uids) > LifecycleMaxUids {
		op := "Undelete"
		if delete {
			op = "Delete"
		}
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"core.EmailsService."+op+": too many uids").
			WithContext("uid_count", len(uids)).
			WithContext("max", LifecycleMaxUids))
	}
	if len(uids) == 0 {
		return errtrace.Ok(Unit{})
	}
	st, closeFn, err := s.openStore()
	if err != nil {
		return errtrace.Err[Unit](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "core.EmailsService.Delete/Undelete"),
		)
	}
	defer closeFn()

	var deletedAt *int64
	if delete {
		ts := s.clock().Unix()
		deletedAt = &ts
	}
	rows, err := st.SetEmailDeletedAt(ctx, alias, uids, deletedAt)
	if err != nil {
		return errtrace.Err[Unit](
			errtrace.WrapCode(err, errtrace.ErrDbInsertEmail, "core.EmailsService.Delete/Undelete").
				WithContext("alias", alias).
				WithContext("uid_count", len(uids)).
				WithContext("delete", delete),
		)
	}
	// Zero RowsAffected on non-empty input means none of the UIDs
	// matched — see file-level rationale.
	if rows == 0 {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrEmailsLifecycleNotFound,
			"core.EmailsService.Delete/Undelete: no matching emails").
			WithContext("alias", alias).
			WithContext("uid_count", len(uids)).
			WithContext("delete", delete))
	}
	return errtrace.Ok(Unit{})
}
