// m0011_add_account_health_view.go creates `v_AccountHealth`, a
// read-only SQL view that gives the UI / CLI a one-row-per-alias
// rollup of the most recent watch poll outcome.
//
// **Why a view, not a table?** The data is pure derivation from the
// authoritative `WatchEvents` audit table (added in m0008). Writing
// it as a view keeps the schema honest — every health refresh reads
// "live" data; there is no second source of truth to keep in sync,
// no triggers to maintain, no backfill job to schedule. SQLite views
// are inlined into the planning stage (essentially a saved sub-query)
// so cost is the same as embedding the SELECT at every call-site,
// minus the copy-paste drift risk.
//
// **Schema (one row per alias that has ever emitted a WatchEvent):**
//
//   - `Alias`         — the watcher alias (PK of the rollup).
//   - `LastEventId`   — `WatchEvents.Id` of the most recent event for
//                       this alias. Stable identifier the UI can use
//                       to "jump to log line".
//   - `LastKind`      — most recent `WatchEvents.Kind` value
//                       (1=Start, 2=Stop, 3=Error, 4=Heartbeat).
//   - `LastOccurredAt`— most recent `WatchEvents.OccurredAt`. RFC3339
//                       UTC, matches the rest of the schema.
//   - `Status`        — derived TEXT bucket the UI renders directly:
//                       'ok'   when LastKind in (1,4)  — Start/Heartbeat
//                       'warn' when LastKind = 2       — Stop (clean shutdown)
//                       'err'  when LastKind = 3       — Error
//                       (Anything outside the closed enum collapses to
//                       'ok' — defensive default; future kinds will get
//                       an explicit case before they ship.)
//
// **The "most recent" trick.** SQLite has no native DISTINCT ON.
// We use the standard correlated-subquery pattern: pick rows whose
// `(Alias, OccurredAt, Id)` triple equals the per-alias MAX. The
// `IxWatchEventsAliasOccurredAt` index from m0008 makes the
// per-alias MAX(OccurredAt) lookup an index seek; the tie-breaker on
// `Id DESC` handles the (extremely rare) case of two events recorded
// in the same `strftime('%Y-%m-%dT%H:%M:%fZ', 'now')` millisecond.
//
// **Idempotent — `CREATE VIEW IF NOT EXISTS`.**
//
// Spec hook: future Phase 2 Dashboard service will SELECT * FROM
// v_AccountHealth to drive the per-account status dot.
package migrate

func init() {
	Register(Migration{
		Version: 11,
		Name:    "add_account_health_view",
		Up: `CREATE VIEW IF NOT EXISTS v_AccountHealth AS
             SELECT
               we.Alias            AS Alias,
               we.Id               AS LastEventId,
               we.Kind             AS LastKind,
               we.OccurredAt       AS LastOccurredAt,
               CASE we.Kind
                 WHEN 1 THEN 'ok'
                 WHEN 2 THEN 'warn'
                 WHEN 3 THEN 'err'
                 WHEN 4 THEN 'ok'
                 ELSE 'ok'
               END                 AS Status
             FROM WatchEvents AS we
             WHERE we.Id = (
               SELECT inner_we.Id
               FROM WatchEvents AS inner_we
               WHERE inner_we.Alias = we.Alias
               ORDER BY inner_we.OccurredAt DESC, inner_we.Id DESC
               LIMIT 1
             )`,
	})
}
