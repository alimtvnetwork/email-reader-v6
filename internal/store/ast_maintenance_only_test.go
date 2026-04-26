// ast_maintenance_only_test.go enforces AC-DB-47:
//
//	"AST scan: only `internal/store/maintenance` issues
//	 `VACUUM` / `ANALYZE` / `PRAGMA wal_checkpoint`."
//
// Our maintenance file is `internal/store/vacuum.go` (the spec's
// "store/maintenance" responsibility lives in that single file). The
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
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func Test_AST_MaintenanceOnly(t *testing.T) {
	root := repoRootForMaintenanceGuard(t)

	// Production files allowed to issue maintenance SQL. Paths are
	// relative to repo root, forward-slashed.
	allow := map[string]bool{
		"internal/store/vacuum.go": true,
	}

	// Match VACUUM / ANALYZE as whole upper-case words and the
	// wal_checkpoint pragma in any case. SQLite statements are
	// conventionally upper-case across this codebase.
	keywordRe := regexp.MustCompile(`\b(VACUUM|ANALYZE)\b|(?i)PRAGMA\s+wal_checkpoint`)

	fset := token.NewFileSet()
	var violations []string

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == ".lovable" || base == "dist" || base == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if allow[rel] {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		file, parseErr := parser.ParseFile(fset, path, src, 0)
		if parseErr != nil {
			// Files that fail to parse aren't our concern here.
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if keywordRe.MatchString(lit.Value) {
				violations = append(violations, rel+": "+lit.Value)
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk repo: %v", walkErr)
	}

	if len(violations) > 0 {
		t.Fatalf("AC-DB-47 violation: maintenance SQL found outside internal/store/vacuum.go:\n  %s",
			strings.Join(violations, "\n  "))
	}
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
