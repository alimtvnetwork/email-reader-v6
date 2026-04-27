// ast_no_inline_sql_test.go enforces P1.9b:
//
//	"No `string` or backtick-string literal beginning with SELECT,
//	 INSERT, UPDATE, or DELETE may appear in production .go files
//	 outside `internal/store/queries/`."
//
// Rationale: per the Phase-1 plan, `internal/store/queries/` is the
// single source of truth for SQL. Inline SQL anywhere else makes
// dialect changes O(repo) and silently routes around the queries
// package's lint coverage and unit tests.
//
// The guard walks the repo from the module root, parses every
// non-test .go file under `internal/{core,store,exporter,ui,cli}`
// (excluding `internal/store/queries/`), and inspects every
// `*ast.BasicLit` of kind STRING. A literal whose unquoted body's
// first non-whitespace token (case-insensitive) is one of
// SELECT|INSERT|UPDATE|DELETE counts as "inline SQL".
//
// Allowlist (`pendingMigration`): files that still hold inline SQL
// while their move-to-queries slice is pending. This list MUST shrink
// to empty by end of Phase 1; each entry references the slice ID that
// will remove it.
//
// Spec: mem://workflow/phase1-plan.md (P1.9b).
package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// pendingMigration enumerates files that legitimately still contain
// inline SQL pending later Phase-1 slices. Entries MUST be removed as
// the named slice lands. Empty ⇒ Phase-1 SQL centralisation complete.
var pendingMigration = map[string]string{}

// scannedRoots is the set of repo-relative directories whose
// non-test .go files are scanned by the guard. internal/store/queries
// is intentionally absent — that *is* the SQL home.
var scannedRoots = []string{
	filepath.Join("internal", "core"),
	filepath.Join("internal", "store"),
	filepath.Join("internal", "exporter"),
	filepath.Join("internal", "ui"),
	filepath.Join("internal", "cli"),
}

// sqlStarters is the case-insensitive set of leading tokens that
// classify a string literal as inline SQL.
var sqlStarters = []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE "}

// TestAST_NoInlineSQL satisfies AC-DB-21 (no SQL string outside
// internal/store/queries/ or internal/store/migrate/) from
// spec/23-app-database/97-acceptance-criteria.md §C. The
// `sqlStarters` table catches SELECT/INSERT/UPDATE/DELETE leading
// tokens; the `scannedRoots` allowlist scopes the scan to the
// callers (core, cli) we care about.
func TestAST_NoInlineSQL(t *testing.T) {
	t.Parallel()

	root := repoRootForMaintenanceGuard(t)

	type offence struct{ file string; line int; sample string }
	var offences []offence

	for _, sub := range scannedRoots {
		dir := filepath.Join(root, sub)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip the SQL home explicitly (handles future nested children too).
				if filepath.Clean(path) == filepath.Join(root, "internal", "store", "queries") {
					return fs.SkipDir
				}
				// Skip the migration harness: `internal/store/migrate/` legitimately
				// owns its bookkeeping `_SchemaVersion` DDL/DML, and individual
				// `m000N_*.go` files (P1.11+) are themselves SQL constants by design.
				// The queries-package boundary doesn't apply to schema migrations.
				if filepath.Clean(path) == filepath.Join(root, "internal", "store", "migrate") {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			rel, _ := filepath.Rel(root, path)
			if _, ok := pendingMigration[rel]; ok {
				return nil // covered by a pending slice; will be tightened later
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				body := unquoteLoose(lit.Value)
				if !looksLikeSQL(body) {
					return true
				}
				pos := fset.Position(lit.Pos())
				offences = append(offences, offence{
					file:   rel,
					line:   pos.Line,
					sample: firstLine(strings.TrimLeft(body, " \t\r\n")),
				})
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}

	if len(offences) == 0 {
		return
	}
	for _, o := range offences {
		t.Errorf("inline SQL in %s:%d — %q (move to internal/store/queries/ or add to pendingMigration with a slice ID)",
			o.file, o.line, o.sample)
	}
}

// TestAST_NoInlineSQL_PendingShrinks documents the allowlist's
// trajectory: any developer adding to `pendingMigration` should also
// open a follow-up slice. We assert the map is non-nil but allow it to
// be empty (the green end-state).
func TestAST_NoInlineSQL_PendingShrinks(t *testing.T) {
	t.Parallel()
	if pendingMigration == nil {
		t.Fatal("pendingMigration must be a (possibly empty) map, not nil")
	}
	for f, slice := range pendingMigration {
		if slice == "" {
			t.Errorf("pendingMigration[%q] missing slice ID", f)
		}
	}
}

// (repoRootForMaintenanceGuard provided by ast_maintenance_only_test.go)


// unquoteLoose strips the surrounding " or ` from an *ast.BasicLit
// value. We don't decode escape sequences — the only thing we care
// about is the leading SQL keyword.
func unquoteLoose(s string) string {
	if len(s) < 2 {
		return s
	}
	q := s[0]
	if (q == '"' || q == '`') && s[len(s)-1] == q {
		return s[1 : len(s)-1]
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + "…"
	}
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}

// looksLikeSQL classifies a string literal as SQL when it BOTH starts
// with a SELECT/INSERT/UPDATE/DELETE keyword AND contains a second
// SQL-structural keyword (FROM, INTO, SET, WHERE, VALUES, JOIN, ORDER,
// GROUP, ON CONFLICT, LIMIT). This filters UI labels like "Update rule",
// "Delete account", "Select an email…" while still catching real
// queries (which always reference a table).
func looksLikeSQL(raw string) bool {
	trimmed := strings.TrimLeft(raw, " \t\r\n")
	upper := strings.ToUpper(trimmed)
	startsWithSQL := false
	for _, s := range sqlStarters {
		if strings.HasPrefix(upper, s) {
			startsWithSQL = true
			break
		}
	}
	if !startsWithSQL {
		return false
	}
	hay := " " + upper + " "
	for _, kw := range []string{" FROM ", " INTO ", " SET ", " WHERE ",
		" VALUES ", " JOIN ", " ORDER ", " GROUP ", " ON CONFLICT", " LIMIT "} {
		if strings.Contains(hay, kw) {
			return true
		}
	}
	return false
}
