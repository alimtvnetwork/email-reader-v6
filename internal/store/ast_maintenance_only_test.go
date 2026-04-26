// ast_maintenance_only_test.go enforces AC-DB-47:
//
//	"AST scan: only `internal/store/maintenance` issues
//	 `VACUUM` / `ANALYZE` / `PRAGMA wal_checkpoint`."
//
// Our maintenance file is `internal/store/vacuum.go` — the spec's
// "store/maintenance" responsibility lives in that single file. The
// guard walks every .go file under the repository root, parses it, and
// inspects every basic-string-literal token. If any production file
// outside the allowlist contains one of the maintenance SQL keywords as
// a string literal, the test fails — locking the chokepoint.
//
// Test files (`*_test.go`) are exempt because they legitimately assert
// against these keywords (format pinning, behaviour smoke tests).
//
// Spec: spec/23-app-database/04 §2-§5; spec/23-app-database/97-acceptance-criteria.md AC-DB-47.
package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	
	"runtime"
	"strings"
	"testing"
)

// maintenanceAllowlist names the production files permitted to issue
// VACUUM / ANALYZE / wal_checkpoint statements. Paths are repo-relative
// and forward-slashed.
var maintenanceAllowlist = map[string]bool{
	"internal/store/vacuum.go": true,
}

// looksLikeMaintenanceSQL reports whether a Go string literal value
// (still wrapped in its surrounding quotes / backticks) is an actual
// SQLite maintenance statement, as opposed to incidental prose like a
// UI label "Weekly VACUUM weekday". A statement must be the *whole*
// payload of the literal after stripping quotes, optional trailing
// semicolons, and surrounding whitespace.
func looksLikeMaintenanceSQL(literal string) bool {
	body := strings.Trim(literal, "`\"")
	body = strings.TrimSpace(body)
	body = strings.TrimRight(body, ";")
	body = strings.TrimSpace(body)
	if body == "VACUUM" || body == "ANALYZE" {
		return true
	}
	lower := strings.ToLower(body)
	return strings.HasPrefix(lower, "pragma wal_checkpoint")
}

func Test_AST_MaintenanceOnly(t *testing.T) {
	root := repoRootForMaintenanceGuard(t)
	violations := scanRepoForMaintenanceSQL(t, root)
	if len(violations) > 0 {
		t.Fatalf("AC-DB-47 violation: maintenance SQL found outside internal/store/vacuum.go:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// scanRepoForMaintenanceSQL walks every .go production file under root
// and returns a list of "<rel>: <literal>" violations.
func scanRepoForMaintenanceSQL(t *testing.T, root string) []string {
	t.Helper()
	var violations []string
	walk := func(path string, d fs.DirEntry, err error) error {
		return inspectGoPath(root, path, d, err, &violations)
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return violations
}

// inspectGoPath is the per-entry callback: it filters dirs/files,
// resolves the allowlist, parses the file, and appends violations.
func inspectGoPath(root, path string, d fs.DirEntry, walkErr error, out *[]string) error {
	if walkErr != nil {
		return walkErr
	}
	if d.IsDir() {
		return skipUninterestingDir(d.Name())
	}
	rel, ok := candidateGoFile(root, path)
	if !ok {
		return nil
	}
	*out = append(*out, scanFileForMaintenanceSQL(path, rel)...)
	return nil
}

func skipUninterestingDir(name string) error {
	switch name {
	case ".git", "node_modules", ".lovable", "dist", "build":
		return filepath.SkipDir
	}
	return nil
}

// candidateGoFile returns (rel, true) when path is a production .go
// file outside the allowlist; otherwise (_, false).
func candidateGoFile(root, path string) (string, bool) {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if maintenanceAllowlist[rel] {
		return "", false
	}
	return rel, true
}

// scanFileForMaintenanceSQL parses path and returns violation strings
// for every string literal containing a forbidden keyword.
func scanFileForMaintenanceSQL(path, rel string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return nil
	}
	var hits []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if looksLikeMaintenanceSQL(lit.Value) {
			hits = append(hits, rel+": "+lit.Value)
		}
		return true
	})
	return hits
}

// repoRootForMaintenanceGuard walks up from this test file until it
// finds a go.mod, which marks the repository root.
func repoRootForMaintenanceGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot locate test source file")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate go.mod walking up from test file")
	return ""
}
