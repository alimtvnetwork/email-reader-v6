// ast_test.go enforces the design-system isolation guards from
// spec/24-app-design-system-and-ui/02-theme-implementation.md §6.
//
// Why a test (not a CI script): the guard is intrinsic to the package
// contract, runs anywhere `go test` runs, and fails fast on any drift.
//
// Implemented guards (T3 deferred until internal/ui/anim/ ships):
//   AST-T1  Only internal/ui/theme/ constructs color.NRGBA / color.RGBA
//           composite literals.
//   AST-T2  Only internal/ui/theme/ imports "fyne.io/fyne/v2/theme".
//   AST-T4  No file under internal/ui/views/ imports "image/color".
//   AST-T5  Every ColorName in tokens.go has an entry in BOTH palettes.
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

// scanGoFiles invokes fn for every .go file under root, skipping vendor,
// hidden dirs, testdata, and node_modules. Skips _test.go files unless
// includeTests is true.
func scanGoFiles(t *testing.T, root string, includeTests bool, fn func(path string, file *ast.File, fset *token.FileSet)) {
	t.Helper()
	internalDir := filepath.Join(root, "internal")
	cmdDir := filepath.Join(root, "cmd")
	for _, dir := range []string{internalDir, cmdDir} {
		walkAndParse(t, dir, includeTests, fn)
	}
}

// walkAndParse is the recursive workhorse for scanGoFiles. Split out so
// scanGoFiles stays under the 15-stmt fn-length lint limit.
func walkAndParse(t *testing.T, dir string, includeTests bool, fn func(string, *ast.File, *token.FileSet)) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi interface{ Name() string }) bool {
		n := fi.Name()
		if !includeTests && strings.HasSuffix(n, "_test.go") {
			return false
		}
		return strings.HasSuffix(n, ".go")
	}, parser.ImportsOnly|parser.ParseComments)
	collectAndRecurse(t, dir, includeTests, fn, fset, pkgs, err)
}

// collectAndRecurse runs fn over every parsed file at this dir level then
// descends into subdirectories. Extracted so walkAndParse stays small.
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
	entries, _ := readDirNames(dir)
	for _, name := range entries {
		if shouldSkipDir(name) {
			continue
		}
		walkAndParse(t, filepath.Join(dir, name), includeTests, fn)
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

// readDirNames returns subdirectory names (only directories). Wraps the
// stdlib call so the parse helpers above don't need the os import twice.
func readDirNames(dir string) ([]string, error) {
	return readSubdirs(dir)
}

// AST-T1: only files under internal/ui/theme/ may write
// `color.NRGBA{...}` or `color.RGBA{...}` composite literals.
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
