// ast_design_system_test.go closes pure-AST AC-DS rows that need no
// Fyne canvas harness:
//
//   - AC-DS-19  AST: only `internal/ui/anim/` may use
//               `canvas.NewColorRGBAAnimation`. Currently vacuously
//               true — there is no `internal/ui/anim/` package yet,
//               and the call is unused anywhere in the tree. This
//               test pins that invariant so the day someone adds an
//               animation, they're forced to put it in the right
//               package or this scanner FAILs.
//
//   - AC-DS-51  AST: only `WatchDot` (the status-footer indicator
//               widget) may call `anim.Pulse(...)`. Vacuously true
//               today — no `internal/ui/anim/` package, zero call
//               sites — but the same ratchet logic applies: the
//               first paste of `anim.Pulse` from outside a future
//               `WatchDot` widget file must trip this scanner.
//               Cluster-mate to AC-DS-19; same template, same
//               helpers.
//
// Other AC-DS rows in the same neighbourhood (palette duplicate
// detection, ThemeSystem variant routing) were scoped out of this
// slice — the first needs a spec carve-out for named aliases and
// the second needs a public helper export from the `!nofyne`-tagged
// `fyne_theme.go`. Both are behaviour work outside an AC-coverage
// slice, and per the honest-scope principle they are NOT cited from
// this file (the audit's stale-ref guard would otherwise treat the
// citation as false coverage).
//
// Same template as `ast_project_linters_test.go` (Slice #131) and
// `ast_settings_security_test.go` (Slice #130). Reuses the shared
// `repoRootForSXGuard`, `skipUninterestingDirSX`, and
// `candidateProductionGo` helpers — do not duplicate.
//
// Spec:
//   - spec/24-app-design-system-and-ui/97-acceptance-criteria.md (AC-DS-19, AC-DS-51)
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

// ---------------------------------------------------------------------------
// AC-DS-51 — Only the `WatchDot` widget may call `anim.Pulse(...)`.
// Vacuously true today (no `internal/ui/anim/` package, zero call
// sites). Pinning the invariant means the first paste of
// `anim.Pulse(...)` from outside a `watch_dot*.go` file FAILs this
// scanner, forcing the author to relocate the call or extend the
// allowlist with explicit justification.
// ---------------------------------------------------------------------------

func Test_AST_PulseOnlyInWatchDot(t *testing.T) {
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
		// Allowed call sites: any file whose basename starts with
		// `watch_dot` (e.g. `watch_dot.go`, `watch_dot_status.go`).
		// The widget doesn't exist yet — that's fine, the scanner
		// just walks past these files when they appear.
		base := filepath.Base(rel)
		if strings.HasPrefix(base, "watch_dot") {
			return nil
		}
		if usesAnimPulse(t, path) {
			violations = append(violations, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("AC-DS-51 violation: anim.Pulse(...) called outside watch_dot*.go:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// usesAnimPulse parses one .go file and returns true iff it
// references `anim.Pulse` as a SelectorExpr — the qualified call
// shape is stable across import aliases, so we match on the
// `(X.Name == "anim", Sel.Name == "Pulse")` pair specifically (the
// `Pulse` selector alone is too generic to use unqualified).
func usesAnimPulse(t *testing.T, path string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Logf("AC-DS-51: parse %s: %v (skipping)", path, err)
		return false
	}
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel == nil || sel.Sel.Name != "Pulse" {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || x.Name != "anim" {
			return true
		}
		found = true
		return false
	})
	return found
}
