// ast_driver_import_limit_test.go enforces AC-DB-50:
//
//	"AST scan: only `internal/store` (and its sub-packages) imports
//	 a SQL driver."
//
// Rationale: SQLite drivers (`modernc.org/sqlite`, `mattn/go-sqlite3`,
// or any other `database/sql/driver` registrant) must remain a private
// implementation detail of `internal/store/...`. If a feature-backend
// or UI package starts importing a driver directly, the typed-store
// chokepoint (AC-DB-50/51/52) collapses and migration / pragma / WAL
// guarantees can be bypassed.
//
// The guard walks every production .go file under the repository root,
// parses it, and inspects every import path. Any driver path that
// appears outside `internal/store/...` is a violation.
//
// Test files (`*_test.go`) are exempt: tests legitimately import a
// driver to spin up an in-memory DB (e.g., the migrate test suite).
//
// Spec: spec/23-app-database/97-acceptance-criteria.md AC-DB-50.
package store

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// driverImportPaths enumerates every Go SQL driver we know about. The
// list is deliberately broad: if a future contributor wires a different
// driver, adding it here keeps the guard honest.
var driverImportPaths = map[string]bool{
	"modernc.org/sqlite":             true,
	"github.com/mattn/go-sqlite3":    true,
	"github.com/lib/pq":              true,
	"github.com/jackc/pgx/v5":        true,
	"github.com/jackc/pgx/v5/stdlib": true,
	"github.com/go-sql-driver/mysql": true,
	"github.com/microsoft/go-mssqldb": true,
	"database/sql/driver":            true,
}

// driverAllowedPrefix is the only directory tree permitted to import
// a SQL driver. Forward-slashed, repo-relative.
const driverAllowedPrefix = "internal/store/"

func Test_AST_DriverImportLimit(t *testing.T) {
	root := repoRootForMaintenanceGuard(t) // reused from ast_maintenance_only_test.go
	violations := scanRepoForDriverImports(t, root)
	if len(violations) > 0 {
		t.Fatalf("AC-DB-50 violation: SQL driver imported outside internal/store/...:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// scanRepoForDriverImports walks every .go production file under root
// and returns "<rel>: <import path>" entries for each violation.
func scanRepoForDriverImports(t *testing.T, root string) []string {
	t.Helper()
	var violations []string
	walk := func(path string, d fs.DirEntry, err error) error {
		return inspectGoPathForImports(root, path, d, err, &violations)
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return violations
}

// inspectGoPathForImports filters dirs/files, resolves the allowlist,
// parses the file, and appends violations.
func inspectGoPathForImports(root, path string, d fs.DirEntry, walkErr error, out *[]string) error {
	if walkErr != nil {
		return walkErr
	}
	if d.IsDir() {
		return skipUninterestingDir(d.Name()) // shared with maintenance guard
	}
	rel, ok := candidateGoFileForDriver(root, path)
	if !ok {
		return nil
	}
	*out = append(*out, scanFileForDriverImports(path, rel)...)
	return nil
}

// candidateGoFileForDriver returns (rel, true) when path is a
// production .go file outside the store-package allowlist; otherwise
// (_, false). Test files are always skipped.
func candidateGoFileForDriver(root, path string) (string, bool) {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, driverAllowedPrefix) {
		return "", false
	}
	return rel, true
}

// scanFileForDriverImports parses path's import block (no need to walk
// the full AST) and returns violation strings.
func scanFileForDriverImports(path, rel string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var hits []string
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if driverImportPaths[p] {
			hits = append(hits, rel+": "+p)
		}
	}
	return hits
}

// (repoRootForMaintenanceGuard, skipUninterestingDir defined in
//  ast_maintenance_only_test.go — same package, same build.)

