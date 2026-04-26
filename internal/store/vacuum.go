// vacuum.go houses the OpenedUrls retention sweeper plus the ANALYZE,
// VACUUM, and wal_checkpoint(TRUNCATE) primitives that round out the
// maintenance toolkit.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §1, §2, §3, §4.
//
//	§1/§3: PruneOpenedUrlsBefore — single time-cutoff DELETE on OpenedAt.
//	       (When the Decision column lands the prune fans out into the
//	       per-decision queries Q-OPEN-PRUNE-LAUNCHED / Q-OPEN-PRUNE-BLOCKED.)
//	§2:    ANALYZE runs after any prune that deletes ≥ 1 000 rows. Tracking
//	       is cumulative across ticks so a long tail of small prunes still
//	       eventually rebuilds statistics.
//	§4:    VACUUM is skipped when the SQLite free-list is < 5% of total
//	       page count; otherwise reclaims file space. Caller serialises it
//	       inside the maintenance loop so no other write is in flight.
//	wal_checkpoint(TRUNCATE) runs every 6h to keep the WAL file small.
package store

import (
	"context"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store/queries"
)

// AnalyzeThreshold is the row-count at which cumulative deletes trigger
// `ANALYZE`. Spec/23-app-database/04 §2 row 3.
const AnalyzeThreshold int64 = 1000

// VacuumFreelistMinPercent is the spec-mandated free-list / page-count
// ratio below which VACUUM is skipped (no meaningful space to reclaim).
// Spec/23-app-database/04 §4.
const VacuumFreelistMinPercent = 5

// DefaultPruneBatchSize mirrors the spec/23-app-database/04 §5 default.
// Used when callers pass batchSize ≤ 0 to PruneOpenedUrlsBeforeBatched.
const DefaultPruneBatchSize = 5000

// PruneOpenedUrlsBefore deletes OpenedUrls rows whose OpenedAt is strictly
// older than `cutoff`. Returns the number of rows deleted. A zero `cutoff`
// is treated as a no-op (returns 0, nil) so callers can pass time.Time{}
// to mean "retention disabled".
//
// Backwards-compatible single-shot wrapper around
// PruneOpenedUrlsBeforeBatched(ctx, cutoff, DefaultPruneBatchSize). The
// batched method is the one wired into the maintenance loop so the user's
// PruneBatchSize knob takes effect; this entry point stays so existing
// callers / tests don't break.
func (s *Store) PruneOpenedUrlsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	n, _, err := s.PruneOpenedUrlsBeforeBatched(ctx, cutoff, DefaultPruneBatchSize)
	return n, err
}

// PruneOpenedUrlsBeforeBatched deletes in chunks of `batchSize` rows so a
// huge backlog cannot hold the writer lock for an unbounded duration. It
// loops until a batch deletes fewer than `batchSize` rows (i.e. the tail
// is reached) or ctx is cancelled. Returns total rows deleted + the
// number of batches issued.
//
// Spec/23-app-database/04 §5 + AC-DB-43.
//
// `batchSize <= 0` falls back to DefaultPruneBatchSize so callers don't
// have to special-case "knob never set".
func (s *Store) PruneOpenedUrlsBeforeBatched(ctx context.Context, cutoff time.Time, batchSize int) (int64, int, error) {
	if cutoff.IsZero() {
		return 0, 0, nil
	}
	if batchSize <= 0 {
		batchSize = DefaultPruneBatchSize
	}
	var total int64
	var batches int
	cutoffUTC := cutoff.UTC()
	for {
		if err := ctx.Err(); err != nil {
			return total, batches, err
		}
		res, err := s.DB.ExecContext(ctx,
			queries.PruneOpenedUrlsBatched,
			cutoffUTC, batchSize,
		)
		if err != nil {
			return total, batches, errtrace.Wrap(err, "prune opened urls (batched)")
		}
		n, _ := res.RowsAffected()
		batches++
		total += n
		if n < int64(batchSize) {
			return total, batches, nil
		}
	}
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

// FreelistRatio returns (freelist_count, page_count) via cheap PRAGMAs.
// Used by ShouldVacuum + observability. Both readings are point-in-time;
// SQLite serialises PRAGMA reads, so they are internally consistent
// even under concurrent writes.
func (s *Store) FreelistRatio(ctx context.Context) (freelist int64, pages int64, err error) {
	if err = s.DB.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&freelist); err != nil {
		return 0, 0, errtrace.Wrap(err, "pragma freelist_count")
	}
	if err = s.DB.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pages); err != nil {
		return 0, 0, errtrace.Wrap(err, "pragma page_count")
	}
	return freelist, pages, nil
}

// ShouldVacuum is the pure free-list-ratio gate. Spec §4: skip when
// freelist < 5% of total pages. A zero or negative `pages` is treated as
// "not enough info, skip" so a fresh DB never triggers a no-op VACUUM.
func ShouldVacuum(freelist, pages int64) bool {
	if pages <= 0 {
		return false
	}
	// Use *100 to stay in integer math: ratio*100 >= threshold.
	return freelist*100 >= pages*int64(VacuumFreelistMinPercent)
}

// Vacuum runs SQLite `VACUUM`. The caller must guarantee no other writers
// are in flight (the maintenance loop holds an idle-window guard).
// Returns the bytes reclaimed (page_count_before - page_count_after) *
// page_size; a negative value means the file grew (very rare; surfaced
// for observability rather than hidden).
func (s *Store) Vacuum(ctx context.Context) (reclaimedBytes int64, err error) {
	pageSize, before, err := s.pageStats(ctx)
	if err != nil {
		return 0, errtrace.Wrap(err, "vacuum pre-stats")
	}
	if _, err := s.DB.ExecContext(ctx, `VACUUM`); err != nil {
		return 0, errtrace.Wrap(err, "vacuum")
	}
	_, after, err := s.pageStats(ctx)
	if err != nil {
		return 0, errtrace.Wrap(err, "vacuum post-stats")
	}
	return (before - after) * pageSize, nil
}

// pageStats reads `PRAGMA page_size` + `PRAGMA page_count`. Split out
// so Vacuum stays under the 15-statement linter cap.
func (s *Store) pageStats(ctx context.Context) (pageSize, pageCount int64, err error) {
	if err = s.DB.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&pageSize); err != nil {
		return 0, 0, err
	}
	if err = s.DB.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pageCount); err != nil {
		return 0, 0, err
	}
	return pageSize, pageCount, nil
}

// WalCheckpointTruncate runs `PRAGMA wal_checkpoint(TRUNCATE)` and returns
// the number of WAL pages checkpointed (column `pages_log` per SQLite
// docs). On a journal_mode != WAL DB the PRAGMA is a no-op and returns 0.
func (s *Store) WalCheckpointTruncate(ctx context.Context) (pages int64, err error) {
	// PRAGMA returns 3 columns: busy, log, checkpointed. We want `log`
	// (total frames in WAL before truncation) for observability.
	var busy, log, checkpointed int64
	row := s.DB.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	if err := row.Scan(&busy, &log, &checkpointed); err != nil {
		return 0, errtrace.Wrap(err, "pragma wal_checkpoint(TRUNCATE)")
	}
	return log, nil
}

