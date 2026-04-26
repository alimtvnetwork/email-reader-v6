// m0009_opened_urls_opened_at_index.go adds a single-column
// `(OpenedAt)` index to the OpenedUrls table.
//
// **Why not the originally-planned `(EmailId, Url)` composite?**
// The Phase-1 plan row for P1.14 calls for an `(EmailId, Url)` index
// "for dedupe perf" — but `IxOpenedUrlsUnique(EmailId, Url)` already
// ships in m0004 (it's the natural-key constraint backing
// `RecordOpenedUrlExt`'s `ON CONFLICT(EmailId, Url) DO NOTHING`).
// Re-creating it would be a no-op. Slice repurposed at land time to
// fill the next real index gap.
//
// **What this index covers:**
//
//   1. `queries.OpenedUrlsList` **without** an alias filter —
//      `WHERE OpenedAt < ? [AND Origin = ?] ORDER BY OpenedAt DESC,
//      Id DESC LIMIT ?`. The existing m0006 composite is
//      `(Alias, OpenedAt)`; SQLite can't use it efficiently when
//      `Alias` is unbound, so the planner falls back to a full scan.
//   2. `queries.PruneOpenedUrlsBatched` — the inner sub-select
//      `SELECT rowid FROM OpenedUrls WHERE OpenedAt < ? LIMIT ?`
//      drives the bounded vacuum loop. With the composite index
//      alone, this is a full scan even when 99% of rows are kept.
//
// Composite alternative considered: `(OpenedAt, Alias)`. Rejected —
// readers that DO filter by alias already hit the m0006 composite
// (which has `Alias` first); a flipped composite would just duplicate
// storage cost without serving any query better than the simpler
// single-column index does.
//
// Idempotent — `CREATE INDEX IF NOT EXISTS`.
package migrate

func init() {
	Register(Migration{
		Version: 9,
		Name:    "opened_urls_opened_at_index",
		Up:      `CREATE INDEX IF NOT EXISTS IxOpenedUrlsOpenedAt ON OpenedUrls(OpenedAt)`,
	})
}
