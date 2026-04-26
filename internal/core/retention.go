// retention.go contains pure helpers for scheduling the OpenedUrls retention
// sweep. The actual sweep is performed by `store.PruneOpenedUrlsBefore`; this
// file decides *when* to call it (once per ~24h) and *what cutoff* to pass.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §1–§2.
package core

import "time"

// RetentionCutoff returns the OpenedAt threshold below which rows are
// eligible for deletion. `retentionDays == 0` means "retention disabled" and
// is signalled by returning the zero `time.Time` (which `store.PruneOpenedUrlsBefore`
// also treats as a no-op, so the helpers compose cleanly).
func RetentionCutoff(now time.Time, retentionDays uint16) time.Time {
	if retentionDays == 0 {
		return time.Time{}
	}
	return now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
}

// ShouldRunRetentionTick reports whether the retention sweeper should run
// right now. The sweeper fires at most once per `intervalHours` (default 24).
// `lastRun` is the timestamp of the previous successful sweep — pass the
// zero time on the first call.
//
// `retentionDays == 0` short-circuits to false: no point scheduling a sweep
// when retention is disabled.
func ShouldRunRetentionTick(lastRun, now time.Time, intervalHours int, retentionDays uint16) bool {
	if retentionDays == 0 {
		return false
	}
	if intervalHours <= 0 {
		intervalHours = 24
	}
	if lastRun.IsZero() {
		return true
	}
	return now.Sub(lastRun) >= time.Duration(intervalHours)*time.Hour
}
