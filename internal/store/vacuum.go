// vacuum.go houses the OpenedUrls retention sweeper plus the ANALYZE
// trigger that follows large prunes.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §1, §2, §3.
//
//	§1/§3: PruneOpenedUrlsBefore — single time-cutoff DELETE on OpenedAt.
//	       (When the Decision column lands the prune fans out into the
//	       per-decision queries Q-OPEN-PRUNE-LAUNCHED / Q-OPEN-PRUNE-BLOCKED.)
//	§2:    ANALYZE runs after any prune that deletes ≥ 1 000 rows. Tracking
//	       is cumulative across ticks so a long tail of small prunes still
//	       eventually rebuilds statistics. Weekly VACUUM and the
//	       wal_checkpoint cadence land in a follow-up.
package store

import (
	"context"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// AnalyzeThreshold is the row-count at which cumulative deletes trigger
// `ANALYZE`. Spec/23-app-database/04 §2 row 3.
const AnalyzeThreshold int64 = 1000

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

// Analyze runs SQLite `ANALYZE` to refresh the stat tables. Cheap on a
// small DB but unbounded on a huge one, so callers gate it on
// ShouldAnalyze (cumulative-deletes ≥ AnalyzeThreshold).
func (s *Store) Analyze(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `ANALYZE`); err != nil {
		return errtrace.Wrap(err, "analyze")
	}
	return nil
}

// ShouldAnalyze decides whether the running cumulative-delete tally has
// reached the ANALYZE threshold. Pure: callers do `cum += deleted; if
// ShouldAnalyze(cum) { Analyze(); cum = 0 }`. Returning true on exactly
// equal lets a single bulk prune of 1 000 rows trigger immediately.
func ShouldAnalyze(cumulativeDeletes int64) bool {
	return cumulativeDeletes >= AnalyzeThreshold
}
