// maintenance_log.go owns the structured log lines emitted by the
// maintenance loop (retention sweep, ANALYZE, VACUUM, WAL checkpoint).
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §6.
// Canonical INFO-level shape (key=value, single line):
//
//	component=maintenance event=prune          deleted=N ok|error=…
//	component=maintenance event=analyze        triggered_at=N ok|error=…
//	component=maintenance event=vacuum         reclaimed_bytes=N ok|error=…
//	component=maintenance event=wal_checkpoint pages=N ok|error=…
//
// We use the stdlib `log/slog` text handler (lazy-initialised, package-
// private, written to stdout) so ops dashboards can grep on the
// `component=maintenance event=…` prefix. The Format* helpers stay
// pure and string-returning so unit tests can pin the key=value tail
// without dragging the slog timestamp/level prefix into assertions.
package ui

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// maintenanceLoggerOnce + maintenanceLogger lazy-init the slog text
// handler so importing this package costs nothing until the first log
// call. The handler is package-private; callers go through the
// log*Run helpers below.
var (
	maintenanceLoggerOnce sync.Once
	maintenanceLogger     *slog.Logger
)

// componentMaintenance is the spec-mandated component tag for every
// line emitted from this file. Defining it as a const keeps the value
// in one place and makes grep audits cheap.
const componentMaintenance = "maintenance"

// maintenanceSlog returns the lazily-built structured logger. Sink is
// os.Stdout so it interleaves with the existing stdlib `log` output
// the rest of the runtime still uses (one stream, easy to capture).
func maintenanceSlog() *slog.Logger {
	maintenanceLoggerOnce.Do(func() {
		maintenanceLogger = slog.New(slog.NewTextHandler(os.Stdout, nil)).
			With(slog.String("component", componentMaintenance))
	})
	return maintenanceLogger
}

// formatTail renders the spec key=value tail used by both the slog
// message body and the Format* helpers (so tests pin the same string
// the operator sees in logs, sans slog's `time=… level=INFO` prefix).
// Pure: no allocation beyond the returned string.
func formatTail(event string, primaryKey string, primaryVal int64, err error) string {
	if err != nil {
		return fmt.Sprintf("event=%s %s=%d error=%v", event, primaryKey, primaryVal, err)
	}
	return fmt.Sprintf("event=%s %s=%d ok", event, primaryKey, primaryVal)
}

// FormatRetentionSweep returns the spec key=value tail for one prune
// outcome. `deleted=0 ok` is preserved on idle ticks so liveness
// monitoring keeps working.
func FormatRetentionSweep(deleted int64, err error) string {
	return formatTail("prune", "deleted", deleted, err)
}

// FormatAnalyzeRun returns the spec tail for one ANALYZE invocation.
// `triggeredAt` is the cumulative-delete count that crossed
// AnalyzeThreshold and fired this run.
func FormatAnalyzeRun(triggeredAt int64, err error) string {
	return formatTail("analyze", "triggered_at", triggeredAt, err)
}

// FormatVacuumRun returns the spec tail for one VACUUM. A negative
// `reclaimedBytes` (rare: the file grew) is surfaced verbatim rather
// than hidden so post-mortems can spot it.
func FormatVacuumRun(reclaimedBytes int64, err error) string {
	return formatTail("vacuum", "reclaimed_bytes", reclaimedBytes, err)
}

// FormatWalCheckpoint returns the spec tail for one
// `wal_checkpoint(TRUNCATE)`. `pages` is the WAL frame count present
// before truncation.
func FormatWalCheckpoint(pages int64, err error) string {
	return formatTail("wal_checkpoint", "pages", pages, err)
}

// emitMaintenance routes a single event through the structured logger.
// Errors land at WARN (so dashboards can split healthy vs degraded
// without parsing the message body); successes land at INFO per spec.
func emitMaintenance(event, primaryKey string, primaryVal int64, err error) {
	lg := maintenanceSlog().With(
		slog.String("event", event),
		slog.Int64(primaryKey, primaryVal),
	)
	if err != nil {
		lg.Warn("maintenance error", slog.Any("error", err))
		return
	}
	lg.Info("maintenance ok")
}

// logRetentionSweep is the production OnSweep callback.
func logRetentionSweep(deleted int64, err error) {
	emitMaintenance("prune", "deleted", deleted, err)
}

// logAnalyzeRun is the production OnAnalyze callback.
func logAnalyzeRun(triggeredAt int64, err error) {
	emitMaintenance("analyze", "triggered_at", triggeredAt, err)
}

// logVacuumRun is the production OnVacuum callback.
func logVacuumRun(reclaimedBytes int64, err error) {
	emitMaintenance("vacuum", "reclaimed_bytes", reclaimedBytes, err)
}

// logWalCheckpoint is the production OnWalCheckpoint callback.
func logWalCheckpoint(pages int64, err error) {
	emitMaintenance("wal_checkpoint", "pages", pages, err)
}
