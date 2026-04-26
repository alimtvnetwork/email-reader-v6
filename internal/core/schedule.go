// schedule.go owns the pure clock-aware predicates that drive the
// weekly VACUUM and the 6-hourly WAL-checkpoint jobs in
// core.Maintenance.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §2 rows 4-5.
//
// Both helpers are deterministic functions of (lastRun, now, …) so the
// goroutine in maintenance.go can drive them with a fake clock in
// tests. None of them touch SQL: the actual VACUUM / wal_checkpoint
// calls live in internal/store/vacuum.go.
package core

import "time"

// ShouldRunWalCheckpoint returns true when at least intervalHours have
// passed since lastRun, OR when lastRun is the zero value (first tick).
// A zero/negative intervalHours is treated as 6h per spec default.
func ShouldRunWalCheckpoint(lastRun, now time.Time, intervalHours int) bool {
	if intervalHours <= 0 {
		intervalHours = 6
	}
	if lastRun.IsZero() {
		return true
	}
	return now.Sub(lastRun) >= time.Duration(intervalHours)*time.Hour
}

// ShouldRunWeeklyVacuum returns true when:
//   - lastRun is the zero value (never ran) AND now is at/after the
//     configured weekday+hour, OR
//   - now is at/after the next scheduled slot following lastRun.
//
// "Slot" = (weekday at hourLocal:00 in now.Location()).
//
// The check is hour-granular: any tick within the target hour on the
// target weekday qualifies, so a 1-minute TickInterval will fire once
// per slot (the Maintenance loop bumps lastRun on success, so the
// remaining ticks within the hour are no-ops).
//
// `weekday` and `hourLocal` are validated by the caller (Settings); this
// helper clamps to safe ranges so an out-of-range value never panics.
func ShouldRunWeeklyVacuum(lastRun, now time.Time, weekday time.Weekday, hourLocal int) bool {
	if hourLocal < 0 {
		hourLocal = 0
	}
	if hourLocal > 23 {
		hourLocal = 23
	}
	// Wrong weekday or wrong hour ⇒ never the slot.
	if now.Weekday() != weekday || now.Hour() != hourLocal {
		return false
	}
	if lastRun.IsZero() {
		return true
	}
	// Same slot already serviced? (within last 23h)
	return now.Sub(lastRun) >= 23*time.Hour
}
