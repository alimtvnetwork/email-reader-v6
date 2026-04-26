// ast_helpers_test.go contains the small helpers used by ast_test.go.
// Kept in a separate file so ast_test.go reads as a flat list of guard
// tests, and so each helper stays well under the 15-stmt fn-length lint.
package theme

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanGoFilesFull is the body-aware variant of scanGoFiles — it parses
// full ASTs (not just imports) so callers can walk composite literals,
// statements, etc. AST-T1 needs this. AST-T2/T4 don't, so they use the
// imports-only fast path in ast_test.go for speed.
func scanGoFilesFull(t *testing.T, root string, includeTests bool, fn func(path string, file *ast.File, fset *token.FileSet)) {
	t.Helper()
	for _, top := range []string{"internal", "cmd"} {
		walkAndParseFull(t, filepath.Join(root, top), includeTests, fn)
	}
}

func walkAndParseFull(t *testing.T, dir string, includeTests bool, fn func(string, *ast.File, *token.FileSet)) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, fileFilter(includeTests), parser.ParseComments)
	collectAndRecurseFull(t, dir, includeTests, fn, fset, pkgs, err)
}

func collectAndRecurseFull(
	t *testing.T, dir string, includeTests bool,
	fn func(string, *ast.File, *token.FileSet),
	fset *token.FileSet, pkgs map[string]*ast.Package, err error,
) {
	t.Helper()
	if err == nil {
		for _, pkg := range pkgs {
			for path, file := range pkg.Files {
				fn(path, file, fset)
			}
		}
	}
	subs, _ := readSubdirs(dir)
	for _, name := range subs {
		if shouldSkipDir(name) {
			continue
		}
		walkAndParseFull(t, filepath.Join(dir, name), includeTests, fn)
	}
}

// fileFilter returns the file predicate used by parser.ParseDir.
func fileFilter(includeTests bool) func(os.FileInfo) bool {
	return func(fi os.FileInfo) bool {
		n := fi.Name()
		if !includeTests && strings.HasSuffix(n, "_test.go") {
			return false
		}
		return strings.HasSuffix(n, ".go")
	}
}

// shouldSkipDir filters out vendor, hidden dirs, testdata, and node_modules.
func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	switch name {
	case "vendor", "node_modules", "testdata":
		return true
	}
	return false
}

// readSubdirs returns names of immediate subdirectories of dir.
func readSubdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// isBannedColorLiteral matches `color.NRGBA{...}` and `color.RGBA{...}`
// composite literals — the two forms AST-T1 forbids outside the theme
// package.
func isBannedColorLiteral(cl *ast.CompositeLit) bool {
	sel, ok := cl.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if pkgIdent.Name != "color" {
		return false
	}
	return sel.Sel.Name == "NRGBA" || sel.Sel.Name == "RGBA"
}
