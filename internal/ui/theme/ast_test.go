// ast_test.go enforces the design-system isolation guards from
// spec/24-app-design-system-and-ui/02-theme-implementation.md §6.
//
// Why a test (not a CI script): the guard is intrinsic to the package
// contract, runs anywhere `go test` runs, and fails fast on any drift.
//
// Implemented guards (T3 deferred until internal/ui/anim/ ships):
//
//	AST-T1  Only internal/ui/theme/ constructs color.NRGBA / color.RGBA
//	        composite literals.
//	AST-T2  Only internal/ui/theme/ imports "fyne.io/fyne/v2/theme".
//	AST-T4  No file under internal/ui/views/ imports "image/color".
//	AST-T5  Every ColorName in tokens.go has an entry in BOTH palettes.
//
// Test runs under `-tags nofyne` (uses go/parser, no Fyne deps).
package theme

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test's working directory (this file lives
// under internal/ui/theme/) to the project root so the AST scan can
// crawl every package.
func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	// internal/ui/theme → ../../.. is repo root.
	return filepath.Clean(filepath.Join(abs, "..", "..", ".."))
}

// scanGoFiles invokes fn for every .go file under the project's
// internal/ + cmd/ trees. Imports-only parse for speed; tests that need
// statement bodies use scanGoFilesFull (see ast_helpers_test.go).
func scanGoFiles(t *testing.T, root string, includeTests bool, fn func(path string, file *ast.File, fset *token.FileSet)) {
	t.Helper()
	for _, top := range []string{"internal", "cmd"} {
		walkAndParse(t, filepath.Join(root, top), includeTests, fn)
	}
}

func walkAndParse(t *testing.T, dir string, includeTests bool, fn func(string, *ast.File, *token.FileSet)) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, fileFilter(includeTests),
		parser.ImportsOnly|parser.ParseComments)
	collectAndRecurse(t, dir, includeTests, fn, fset, pkgs, err)
}

func collectAndRecurse(
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
		walkAndParse(t, filepath.Join(dir, name), includeTests, fn)
	}
}

// AST-T1: only files under internal/ui/theme/ may write
// `color.NRGBA{...}` or `color.RGBA{...}` composite literals.
//
// Satisfies AC-DS-17 (AST: only internal/ui/theme/ constructs
// color.NRGBA{...} / color.RGBA{...} literals).
func Test_AST_T1_ColorLiteralsConfinedToTheme(t *testing.T) {
	root := repoRoot(t)
	allowedPrefix := filepath.Join(root, "internal", "ui", "theme") + string(filepath.Separator)
	// Re-parse with full bodies so we can walk composite literals.
	scanGoFilesFull(t, root, false, func(path string, file *ast.File, _ *token.FileSet) {
		if strings.HasPrefix(path, allowedPrefix) {
			return
		}
		ast.Inspect(file, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if isBannedColorLiteral(cl) {
				t.Errorf("AST-T1 violation: %s constructs color literal "+
					"outside internal/ui/theme/ — use theme.Color(...) instead", path)
			}
			return true
		})
	})
}

// AST-T2: only internal/ui/theme/ imports "fyne.io/fyne/v2/theme".
func Test_AST_T2_FyneThemeImportConfinedToTheme(t *testing.T) {
	root := repoRoot(t)
	allowedPrefix := filepath.Join(root, "internal", "ui", "theme") + string(filepath.Separator)
	scanGoFiles(t, root, false, func(path string, file *ast.File, _ *token.FileSet) {
		if strings.HasPrefix(path, allowedPrefix) {
			return
		}
		for _, imp := range file.Imports {
			if imp.Path.Value == `"fyne.io/fyne/v2/theme"` {
				t.Errorf("AST-T2 violation: %s imports fyne.io/fyne/v2/theme "+
					"outside internal/ui/theme/", path)
			}
		}
	})
}

// AST-T4: no file under internal/ui/views/ imports "image/color".
func Test_AST_T4_ViewsDoNotImportImageColor(t *testing.T) {
	root := repoRoot(t)
	viewsPrefix := filepath.Join(root, "internal", "ui", "views") + string(filepath.Separator)
	scanGoFiles(t, root, false, func(path string, file *ast.File, _ *token.FileSet) {
		if !strings.HasPrefix(path, viewsPrefix) {
			return
		}
		for _, imp := range file.Imports {
			if imp.Path.Value == `"image/color"` {
				t.Errorf("AST-T4 violation: %s imports image/color — views must "+
					"go through theme.Color(ColorName) only", path)
			}
		}
	})
}

// AST-T5: every ColorName constant has an entry in BOTH palette maps.
// This is the runtime mirror of the Test_Palettes_Parity check, but
// phrased as an AST-style invariant for the spec's checklist.
func Test_AST_T5_TokenPalettesParity(t *testing.T) {
	for _, name := range AllColorNames() {
		if _, ok := paletteDark[name]; !ok {
			t.Errorf("AST-T5 violation: paletteDark missing %q", name)
		}
		if _, ok := paletteLight[name]; !ok {
			t.Errorf("AST-T5 violation: paletteLight missing %q", name)
		}
	}
}
