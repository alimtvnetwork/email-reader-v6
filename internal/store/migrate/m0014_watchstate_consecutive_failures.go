// m0014_watchstate_consecutive_failures.go adds a single counter
// column to the `WatchState` table:
//
//	ConsecutiveFailures INTEGER NOT NULL DEFAULT 0
//
// **Why on `WatchState` (not a derived window over `WatchEvents`)** —
// `core.ComputeHealth` needs the count cheaply on every dashboard
// refresh; computing it from `WatchEvents` would require a per-alias
// CTE walking back to the most recent non-error event (see the
// rationale in `queries.AccountHealthSelectAll`). The natural home
// is a counter the watcher bumps on each poll outcome:
//
//   - Poll error  → `ConsecutiveFailures = ConsecutiveFailures + 1`
//   - Poll OK     → `ConsecutiveFailures = 0`
//
// Both are single-row UPDATEs hitting the alias-keyed primary index;
// O(1) per poll vs. O(events-per-alias) for the window approach.
//
// **Why a counter (not a boolean / last-error timestamp)** — the spec
// `Health` rule is `ConsecutiveFailures >= 3 → Error`. A boolean would
// flip on the first error and erase ordering history; a timestamp
// would force the dashboard to pre-compute the count anyway. The
// integer counter is the smallest representation that survives the
// `>= N` test for any future N without schema churn.
//
// **Why `UpFunc` + PRAGMA gating?** SQLite's `ALTER TABLE ADD COLUMN`
// has no `IF NOT EXISTS` form; re-running raw `ALTER` against a DB
// that already has the column errors with "duplicate column name".
// The harness's `_SchemaVersion` ledger normally prevents that, but
// we follow the same defensive pattern as m0005 / m0010 so test
// fixtures bypassing the ledger still work and the migration is
// self-documenting as the canonical recipe.
//
// Spec: `mem://workflow/roadmap-phases.md` Phase 3 deferred work +
// `core.ComputeHealth` "≥3 failures → Error" branch.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	Register(Migration{
		Version: 14,
		Name:    "watchstate_consecutive_failures",
		UpFunc:  applyAddConsecutiveFailures,
	})
}

// applyAddConsecutiveFailures introspects `PRAGMA table_info(WatchState)`
// and only emits the ADD when the column is missing — matches m0010's
// pattern.
func applyAddConsecutiveFailures(ctx context.Context, db *sql.DB) error {
	have, err := watchStateColumns(ctx, db)
	if err != nil {
		return fmt.Errorf("introspect WatchState: %w", err)
	}
	if have["ConsecutiveFailures"] {
		return nil
	}
	const ddl = `ALTER TABLE WatchState ADD COLUMN ConsecutiveFailures INTEGER NOT NULL DEFAULT 0`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("add column ConsecutiveFailures: %w", err)
	}
	return nil
}

// watchStateColumns mirrors `emailsColumns` (m0010) for the WatchState
// table. Kept as a dedicated helper rather than a generic
// `tableColumns(table)` because injecting a table name into
// `PRAGMA table_info(?)` requires string interpolation, which the
// codebase intentionally avoids in the SQL layer.
func watchStateColumns(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(WatchState)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}
