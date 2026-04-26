// m0007_naming_convention_lock.go is a documentation-only migration
// that records the Phase 2.1 schema-naming verdict in the migration
// sequence. It performs ZERO schema changes.
//
// **Why this slot exists at all.** The original Phase 1 plan
// (mem://workflow/phase1-plan.md, P1.12) reserved m0007 for
// "rename WatchEvents to singular". That slice was BLOCKED on the
// Phase 2.1 schema convention decision. The decision (Slice #68,
// 2026-04-26) was: **plural is correct for entity audit logs**, so
// the rename is a no-op — see mem://design/schema-naming-convention.md
// for the full reasoning matrix.
//
// **Why we don't just skip the number.** Two reasons:
//
//   1. **Migration numbering is a contract.** Every previously-
//      observed `_SchemaVersion.Version` row stays valid. A skipped
//      number forever leaves future readers asking "what was m0007?".
//      Landing a doc-only entry answers the question in source.
//   2. **The cross-pkg idempotence test (P1.17) counts
//      `len(migrate.All())`.** If we left the slot empty, the count
//      would silently misalign with the visible m000N filenames.
//      Registering this no-op keeps that invariant honest and gives
//      the test a reason to assert the ledger row for v=7 exists.
//
// **Up SQL is intentionally a single harmless no-op statement.**
// SQLite's `SELECT 1` is the smallest valid statement that passes
// `db.ExecContext` without side effects. Crucially, the migration
// harness records the ledger row only AFTER Up runs successfully —
// using `SELECT 1` (rather than the empty string) keeps that contract
// intact while satisfying `Register`'s "exactly one of Up or UpFunc
// must be set" rule.
//
// **Future renames** that DO need to mutate schema will land as their
// own m000N slices and follow the convention locked in this file's
// doc-comment. This entry is the canonical reference for "why we
// chose plural for entity tables".
package migrate

func init() {
	Register(Migration{
		Version: 7,
		Name:    "naming_convention_lock",
		// SELECT 1 is a no-op: SQLite parses, plans, and discards
		// the result row. Zero DDL, zero side effects, ledger row
		// recorded as proof the verdict was applied to this DB.
		Up: `SELECT 1`,
	})
}
