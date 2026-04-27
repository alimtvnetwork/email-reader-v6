// a11y_test.go — Slice #118 enforceable AST guards for the spec
// §8 (`spec/24-app-design-system-and-ui/05-accessibility.md`)
// accessibility test contract.
//
// The contract enumerates 11 required cases. Three of them can be
// enforced *purely textually* (no Fyne runtime, no widget tree
// walk): icon-only-button rejection, stray ARIA-string rejection,
// and the package-presence smoke. Those three live here and pass
// today.
//
// The other 8 require a live Fyne build (contrast computations,
// focus-order traversal, target-size walk, focus-ring paint,
// reduced-motion token collapse, keyboard shortcut routing,
// status-text adjacency, AccessibilityLabel walk). They live in
// `a11y_skipped_test.go` as documented `t.Skip` stubs naming the
// follow-up slice (Slice #118b) so `go test -v` shows the work
// surface explicitly rather than hiding it.
//
// Why the split: an atomic slice ships exactly one shape change.
// Bootstrapping the package + locking the textually-enforceable
// rules is one shape; adding cgo / Fyne wiring for the runtime
// tests is another. Splitting now means today's three guards start
// catching regressions immediately (they pass on a clean tree;
// re-introducing a bare-icon button or pasting an HTML `aria-label`
// snippet into a Go file fails CI), while the runtime gates land
// in #118b without rebasing this slice's diff.
package accessibility

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// uiTreeRoot is the directory the AST guards scan. Resolved
// relative to the package CWD that `go test` sets — from
// `internal/ui/accessibility/` we walk up to `internal/ui/` so the
// guards see every view file (and the shell itself).
const uiTreeRoot = ".."

// -----------------------------------------------------------------------------
// Test 1: package-presence smoke (Slice #118 self-check).
// -----------------------------------------------------------------------------
//
// Confirms the package exports the foundational surface so a future
// "did anyone delete the a11y package?" regression fails loud
// instead of silently leaving the spec unanchored.

func TestAccessibility_PackageExportsFoundationalSurface(t *testing.T) {
	// Probe: run the documented happy paths. Each call must complete
	// without panic. We don't assert the boolean values — those depend
	// on the host OS — only that the surface exists and is callable.
	_ = PrefersReducedMotion()
	ResetReducedMotionCache()
	SetReducedMotionProbe(func() bool { return true })
	if !PrefersReducedMotion() {
		t.Fatal("override probe was ignored — SetReducedMotionProbe contract broken")
	}
	SetReducedMotionProbe(nil)

	// EnsureLabel: empty input rejected, non-empty trimmed.
	if got := EnsureLabel(nil, "  "); got != "" {
		t.Errorf("empty label should be rejected, got %q", got)
	}
	if got := EnsureLabel(nil, "  Refresh emails  "); got != "Refresh emails" {
		t.Errorf("label trim broken, got %q", got)
	}

	// Labeler dispatch: a stub implementing the interface receives
	// the trimmed label.
	st := &stubLabeler{}
	EnsureLabel(st, "  Open URL  ")
	if st.last != "Open URL" {
		t.Errorf("Labeler dispatch broken: last=%q", st.last)
	}
}

type stubLabeler struct{ last string }

func (s *stubLabeler) SetAccessibilityLabel(l string) { s.last = l }

// -----------------------------------------------------------------------------
// Test 2: spec §8 #11 — Test_NoIconOnlyButtons_WithoutLabel.
// -----------------------------------------------------------------------------
//
// AST scan: any call of the form `widget.NewButtonWithIcon("", ...)`
// (i.e. empty string literal as the label argument) is forbidden.
// Spec §6 requires every icon button to carry a non-empty label so
// screen readers can announce it.
//
// Today's tree is clean (`rg 'NewButtonWithIcon\("",'` returns
// nothing). This guard locks that and catches the next paste of
// `widget.NewButtonWithIcon("", theme.RefreshIcon(), ...)` from a
// design-system snippet.

func TestNoIconOnlyButtons_WithoutLabel(t *testing.T) {
	violations := walkUITree(t, func(rel string, file *ast.File, fset *token.FileSet) []string {
		var hits []string
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "widget" || sel.Sel.Name != "NewButtonWithIcon" {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			// Check for empty string literal: the literal value
			// includes the surrounding quotes, so "" is the four-byte
			// string `""` (two quotes around zero content).
			if lit.Value == `""` || lit.Value == "``" {
				pos := fset.Position(call.Pos())
				hits = append(hits, rel+":"+itoaA11y(pos.Line))
			}
			return true
		})
		return hits
	})
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("spec §6 / §8 #11 violation — icon-only buttons without a label are forbidden (screen readers cannot announce them):\n  %s\n\nFix: pass a non-empty label as the first argument:\n  widget.NewButtonWithIcon(\"Refresh\", theme.RefreshIcon(), onTap)",
			strings.Join(violations, "\n  "))
	}
}

// -----------------------------------------------------------------------------
// Test 3: stray HTML/React `aria-*` string rejection.
// -----------------------------------------------------------------------------
//
// Fyne is a native widget toolkit — it has no `aria-label` /
// `aria-describedby` attributes. The spec's design-system
// documents (`spec/24-app-design-system-and-ui/05-accessibility.md`)
// were drafted from a web-first context and occasionally suggest
// patterns that read as HTML. When devs paste those into Go code
// the result is a `"aria-label"` string literal that compiles fine
// but does nothing — silently breaking accessibility.
//
// This guard flags any source file under `internal/ui/` that
// contains the literal substring `aria-` (case-insensitive) inside
// a string literal or a comment that looks like a type/attribute
// hint. It deliberately scans raw bytes (not the AST) because the
// failure mode is "wrong text snuck in", not "wrong syntactic
// shape". The accessibility package itself is exempted (this file
// mentions `aria-` in its own doc comment).

func TestNoStrayARIAStringsInUITree(t *testing.T) {
	var violations []string
	err := filepath.Walk(uiTreeRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Self-exemption: this guard's own source legitimately
		// contains the forbidden substring in its doc strings.
		if strings.HasSuffix(path, "a11y_test.go") || strings.HasSuffix(path, "accessibility.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		// Case-insensitive substring scan. Cheap; runs in <1 ms
		// even across the full UI tree.
		lower := strings.ToLower(string(raw))
		if strings.Contains(lower, "aria-") {
			rel, _ := filepath.Rel(uiTreeRoot, path)
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", uiTreeRoot, err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("HTML/React `aria-*` patterns are forbidden in the Fyne UI tree (Fyne uses widget.AccessibilityLabel via the Labeler shim, not ARIA attributes):\n  %s\n\nFix: route accessible names through `accessibility.EnsureLabel(widget, label)` instead of pasting `aria-label=\"…\"`-style strings.",
			strings.Join(violations, "\n  "))
	}
}

// -----------------------------------------------------------------------------
// shared AST walker — mirrors `internal/ui/ast_no_core_globals_test.go`
// but scoped to the accessibility package.
// -----------------------------------------------------------------------------

func walkUITree(t *testing.T, check func(rel string, file *ast.File, fset *token.FileSet) []string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var out []string
	err := filepath.Walk(uiTreeRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		rel, relErr := filepath.Rel(uiTreeRoot, path)
		if relErr != nil {
			rel = path
		}
		out = append(out, check(rel, f, fset)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", uiTreeRoot, err)
	}
	return out
}

// itoaA11y is a tiny strconv.Itoa stand-in to avoid importing
// strconv just for line numbers.
func itoaA11y(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
