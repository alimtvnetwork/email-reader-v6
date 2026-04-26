package migrate

// sqliteRFC3339NowExpr is the canonical SQL fragment for "now as
// RFC3339 UTC with milliseconds" in SQLite. Mirrors the same constant
// in `internal/store/datetime.go` — kept duplicated (rather than
// imported) because the `migrate` package must not depend back on
// `store` (would form an import cycle once `store.Open` calls
// `migrate.Apply`). Both copies MUST stay byte-for-byte identical;
// `TestNowExprStaysAlignedWithStorePackage` enforces this in the
// store package's own test suite (see store/datetime_test.go ¹).
//
// ¹ Added in slice P1.11 alongside this file.
const sqliteRFC3339NowExpr = `(strftime('%Y-%m-%dT%H:%M:%fZ','now'))`
