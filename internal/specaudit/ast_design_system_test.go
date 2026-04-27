// ast_design_system_test.go closes one AC-DS row that is a pure AST
// scan and needs no Fyne canvas harness:
//
//   - AC-DS-19  AST: only `internal/ui/anim/` may use
//               `canvas.NewColorRGBAAnimation`. Currently vacuously
//               true — there is no `internal/ui/anim/` package yet,
//               and the call is unused anywhere in the tree. This
//               test pins that invariant so the day someone adds an
//               animation, they're forced to put it in the right
//               package or this scanner FAILs.
//
// AC-DS-05 (no duplicate RGB triples per variant) was scoped out of
// this slice — the current palette intentionally aliases foreground
// roles (white-on-primary == white-on-active-sidebar), so closing it
// requires either a spec carve-out for named aliases or a palette
// refactor. Both are behaviour work outside an AC-coverage slice.
//
// AC-DS-15 (Apply(ThemeSystem) → ThemeVariant routing) was scoped
// out because the routing function `resolvedMode` lives in the
// `!nofyne`-tagged `fyne_theme.go` and would require either a public
// helper export or a canvas-bound test. Same scope reasoning.
//
// Same template as `ast_project_linters_test.go` (Slice #131) and
// `ast_settings_security_test.go` (Slice #130). Reuses the shared
// `repoRootForSXGuard`, `skipUninterestingDirSX`, and
// `candidateProductionGo` helpers — do not duplicate.
//
// Spec:
//   - spec/24-app-design-system-and-ui/97-acceptance-criteria.md (AC-DS-19)
//   - mem://decisions/06-ac-coverage-rollout-pattern.md (slice template)
package specaudit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// AC-DS-19 — Only `internal/ui/anim/` may use
// `canvas.NewColorRGBAAnimation`. Vacuously true today (no anim
// package, zero usages). Pinning the invariant means the day someone
// adds an animation, they're forced to put it in the right package or
// this scanner FAILs.
// ---------------------------------------------------------------------------

func Test_AST_AnimImportLimit(t *testing.T) {
	root := repoRootForSXGuard(t)
	var violations []string
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipUninterestingDirSX(d.Name())
		}
		rel, ok := candidateProductionGo(root, path)
		if !ok {
			return nil
		}
		// Allowed package: `internal/ui/anim/`. Doesn't exist
		// yet — that's fine, the scanner just walks past it.
		if strings.HasPrefix(rel, filepath.Join("internal", "ui", "anim")+string(filepath.Separator)) {
			return nil
		}
		if usesNewColorRGBAAnimation(t, path) {
			violations = append(violations, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("AC-DS-19 violation: canvas.NewColorRGBAAnimation used outside internal/ui/anim/:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// usesNewColorRGBAAnimation parses one .go file and returns true iff
// it references `canvas.NewColorRGBAAnimation` as a SelectorExpr (which
// is how the qualified function call appears in the AST regardless of
// the import alias). On parse error we log + return false — a syntax
// error is somebody else's test's problem.
func usesNewColorRGBAAnimation(t *testing.T, path string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Logf("AC-DS-19: parse %s: %v (skipping)", path, err)
		return false
	}
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel != nil && sel.Sel.Name == "NewColorRGBAAnimation" {
			// We don't bother checking the X identifier — the
			// function name is unique enough in the Go ecosystem
			// to flag a real violation regardless of import alias.
			found = true
			return false
		}
		return true
	})
	return found
}
