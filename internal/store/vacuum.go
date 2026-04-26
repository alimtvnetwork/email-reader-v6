// vacuum.go houses the OpenedUrls retention sweeper.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §1, §3 (Q-OPEN-PRUNE-LAUNCHED
// and Q-OPEN-PRUNE-BLOCKED). The OpenedUrls v1 schema does not yet carry a
// `Decision` column, so we collapse both prune queries into a single
// time-cutoff DELETE on `OpenedAt`. When the Decision column lands the prune
// will fan out into the per-decision queries documented in the spec.
//
// The function is intentionally minimal — no batching loop, no ANALYZE/VACUUM
// — those land alongside the maintenance loop in a future slice. This call
// only needs to be safe under the watcher's daily tick.
package store

import (
	"context"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// PruneOpenedUrlsBefore deletes OpenedUrls rows whose OpenedAt is strictly
// older than `cutoff`. Returns the number of rows deleted. A zero `cutoff`
// is treated as a no-op (returns 0, nil) so callers can pass time.Time{}
// to mean "retention disabled".
func (s *Store) PruneOpenedUrlsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if cutoff.IsZero() {
		return 0, nil
	}
	res, err := s.DB.ExecContext(ctx,
		`DELETE FROM OpenedUrls WHERE OpenedAt < ?`,
		cutoff.UTC(),
	)
	if err != nil {
		return 0, errtrace.Wrap(err, "prune opened urls")
	}
	n, _ := res.RowsAffected()
	return n, nil
}
