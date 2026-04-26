// m0008_add_watch_event.go creates the persistent `WatchEvents` audit
// table that mirrors the in-memory `core.WatchEvent` stream. Today
// `core.WatchEvent` lives only on `eventbus.Bus[WatchEvent]` (lost on
// restart); this table gives the Watch view a durable activity feed
// and unblocks future per-alias forensics ("why did poll #847 error?").
//
// **Naming — plural `WatchEvents`:** matches the pluralization of
// `Emails` / `OpenedUrls` (the existing house style). The
// blocked-on-Phase-2.1 slice P1.12 decides whether the codebase
// standardises on singular or plural; if singular wins, P1.12 will
// rename this table. Until then, plural is the local minimum-surprise
// choice.
//
// **Column shape (per `mem://workflow/phase1-plan.md` row P1.13):**
//
//   - `Id` — surrogate PK, autoincrement.
//   - `Alias` — required; matches `Emails.Alias` / `WatchState.Alias`.
//   - `Kind` — small integer mirroring `core.WatchEventKind` enum
//     (1=Start, 2=Stop, 3=Error, 4=Heartbeat). Stored as INTEGER
//     (not TEXT) because the enum is closed and indexed lookups stay
//     cheap. A TEXT shadow column may be added later if non-Go
//     readers need it.
//   - `Payload` — TEXT JSON blob carrying the variable parts of
//     `core.WatchEvent` (Message, Err). Empty JSON `{}` is the
//     default — never NULL.
//   - `OccurredAt` — RFC3339 UTC; uses the canonical `strftime`
//     expression so the column matches the rest of the schema's
//     timestamp convention (AC-DB-53).
//
// **Indexes:** one composite `(Alias, OccurredAt DESC)`-shaped index
// drives the typical reader query ("last 50 events for alias X").
// SQLite ignores `DESC` in CREATE INDEX but honours the column order;
// a `WHERE Alias = ? ORDER BY OccurredAt DESC` query uses this index
// for both the seek and the sort.
//
// Idempotent — `CREATE TABLE/INDEX IF NOT EXISTS`.
package migrate

func init() {
	Register(Migration{
		Version: 8,
		Name:    "add_watch_events",
		Up: `CREATE TABLE IF NOT EXISTS WatchEvents (
                Id          INTEGER PRIMARY KEY AUTOINCREMENT,
                Alias       TEXT    NOT NULL,
                Kind        INTEGER NOT NULL,
                Payload     TEXT    NOT NULL DEFAULT '{}',
                OccurredAt  DATETIME NOT NULL DEFAULT ` + sqliteRFC3339NowExpr + `
              );
              CREATE INDEX IF NOT EXISTS IxWatchEventsAliasOccurredAt
                ON WatchEvents(Alias, OccurredAt)`,
	})
}
