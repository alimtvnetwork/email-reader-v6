package store

import (
	"context"
	"strings"
	"testing"
)

// Test_BooleanPositive — AC-DB-54.
//
// Iterates `PRAGMA table_info(<table>)` over every user table in the
// schema and rejects any column whose name begins with `Is` or `Has`
// and whose default value is `1`.
//
// Rationale (spec/04-database-conventions §2): boolean columns must be
// positive-named — i.e. `1` always encodes the positive condition the
// column name implies (`IsDeduped=1` means "yes, deduped"). A default
// of `1` on an `Is*`/`Has*` column flips that: `0` would then mean
// "yes, the positive condition holds", which is exactly the
// anti-pattern this AC outlaws.
//
// The test is intentionally schema-driven (no hard-coded column list)
// so future ADD COLUMN migrations are checked automatically.
func Test_BooleanPositive(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	ctx := context.Background()

	tables, err := userTables(ctx, st)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("no user tables found — schema not migrated?")
	}

	type violation struct {
		Table, Column, Default string
	}
	var bad []violation

	for _, table := range tables {
		cols, err := tableInfo(ctx, st, table)
		if err != nil {
			t.Fatalf("PRAGMA table_info(%s): %v", table, err)
		}
		for _, c := range cols {
			if !isBooleanPrefixed(c.Name) {
				continue
			}
			if strings.TrimSpace(c.DfltValue) == "1" {
				bad = append(bad, violation{Table: table, Column: c.Name, Default: c.DfltValue})
			}
		}
	}

	if len(bad) > 0 {
		var b strings.Builder
		b.WriteString("AC-DB-54: positive-boolean convention violated:\n")
		for _, v := range bad {
			b.WriteString("  - ")
			b.WriteString(v.Table)
			b.WriteString(".")
			b.WriteString(v.Column)
			b.WriteString(" defaults to ")
			b.WriteString(v.Default)
			b.WriteString(" (Is*/Has* columns must default to 0 so 1 always means the positive condition)\n")
		}
		t.Fatal(b.String())
	}
}

// isBooleanPrefixed reports whether the column name uses the boolean
// `Is`/`Has` prefix that the convention applies to. Matches PascalCase
// names like `IsDeduped`, `HasAttachment`. Does NOT match unrelated
// names like `Issue`, `Hash`, or `Island` — the next char after the
// prefix must be uppercase.
func isBooleanPrefixed(name string) bool {
	for _, prefix := range []string{"Is", "Has"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			continue
		}
		c := rest[0]
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}

// userTables returns all non-internal table names in the open store.
func userTables(ctx context.Context, st *Store) ([]string, error) {
	rows, err := st.DB.QueryContext(ctx,
		`SELECT name FROM sqlite_master
		   WHERE type='table' AND name NOT LIKE 'sqlite_%'
		   ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type columnInfo struct {
	Name      string
	Type      string
	NotNull   int
	DfltValue string
	PK        int
}

// tableInfo wraps `PRAGMA table_info(<name>)`. The PRAGMA returns
// `dflt_value` as TEXT (or NULL); we coerce NULL to "" so callers
// can compare with a plain string.
func tableInfo(ctx context.Context, st *Store, table string) ([]columnInfo, error) {
	// PRAGMA does not accept bind parameters; the table name comes from
	// sqlite_master so it is not user-controlled.
	rows, err := st.DB.QueryContext(ctx, `PRAGMA table_info(`+quoteIdent(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []columnInfo
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    *string
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		var d string
		if dflt != nil {
			d = *dflt
		}
		out = append(out, columnInfo{
			Name:      name,
			Type:      ctype,
			NotNull:   notnull,
			DfltValue: d,
			PK:        pk,
		})
	}
	return out, rows.Err()
}

// quoteIdent wraps a SQLite identifier in double quotes, escaping any
// embedded quotes. Defense-in-depth even though our table names come
// from sqlite_master.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
