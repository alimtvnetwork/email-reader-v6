// ast_core_uses_store_only_test.go — AC-DB-52.
//
// Asserts feature backends (`internal/core/*`) and the higher-layer
// `internal/exporter`, `internal/ui`, and `internal/cli` packages do
// not import `database/sql`, `database/sql/driver`, or any concrete
// SQL driver. They must reach the database exclusively through typed
// `internal/store` methods.
//
// Spec: spec/23-app-database/97-acceptance-criteria.md §F (AC-DB-52).
//
// Implementation idiom mirrors `Test_AST_DriverImportLimit` (Slice
// #35) and `Test_AST_MaintenanceOnly` (Slice #34): walk the repo,
// `parser.ImportsOnly` parse each production `.go`, deny-list the
// known SQL imports for everything outside `internal/store/`.
package store

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// disallowedDbImportsForCallers builds the deny-list: `database/sql`,
// its driver subpackage, plus every concrete driver enumerated by
// `driverImportPaths` (defined in ast_driver_import_limit_test.go).
func disallowedDbImportsForCallers() map[string]bool {
	out := map[string]bool{
		"database/sql":        true,
		"database/sql/driver": true,
	}
	for p := range driverImportPaths {
		out[p] = true
	}
	return out
}

// callerDirsToScan are the package directory prefixes (relative to repo
// root) whose production code must reach the database only via typed
// `internal/store` methods. `internal/store` itself is exempt.
var callerDirsToScan = []string{
	"internal/core",
	"internal/exporter",
	"internal/ui",
	"internal/cli",
}

func Test_AST_CoreUsesStoreOnly(t *testing.T) {
	t.Parallel()

	root := repoRootForMaintenanceGuard(t)
	deny := disallowedDbImportsForCallers()
	fset := token.NewFileSet()

	type violation struct {
		File   string
		Import string
	}
	var bad []violation

	for _, prefix := range callerDirsToScan {
		dir := filepath.Join(root, prefix)
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("scan dir missing: %s (%v)", dir, err)
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return skipUninterestingDir(d.Name())
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				if deny[p] {
					rel, _ := filepath.Rel(root, path)
					bad = append(bad, violation{File: filepath.ToSlash(rel), Import: p})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}

	if len(bad) > 0 {
		var b strings.Builder
		b.WriteString("AC-DB-52: caller packages must use typed store.* methods, not database/sql:\n")
		for _, v := range bad {
			b.WriteString("  - ")
			b.WriteString(v.File)
			b.WriteString(" imports ")
			b.WriteString(v.Import)
			b.WriteByte('\n')
		}
		t.Fatal(b.String())
	}
}
