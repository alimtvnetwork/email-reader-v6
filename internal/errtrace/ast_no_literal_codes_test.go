// ast_no_literal_codes_test.go enforces P1.5:
//
//	"No string literal matching the error-code shape `ER-XXX-2NNNN`
//	 may appear in production .go files outside `internal/errtrace/`."
//
// **Rationale.** The errtrace package owns the canonical mapping
// from Go identifier (`ErrConfigOpen`) to wire-format code string
// (`"ER-CFG-21001"`). The pair is generated from `codes.yaml` and
// guarded by the P1.4 bidirectional registry test. If a caller side-
// steps the typed const and embeds the raw string literal in their
// own code:
//
//   - The P1.4 drift guard cannot see it (it only reads codes.go).
//   - Renaming/renumbering a code in `codes.yaml` will silently leave
//     the literal stale.
//   - UI/error-registry surfaces that enumerate `RegisteredCodes`
//     will under-report what the binary actually emits.
//
// The fix is mechanical: replace `"ER-CFG-21001"` with
// `errtrace.ErrConfigOpen`. This guard ensures nobody re-introduces
// the smell.
//
// **Scope.**
//
//   - Walks `internal/` recursively from the module root (resolved
//     by walking up from this file's package dir until we find go.mod).
//   - Parses every non-test `.go` file. Test files are EXCLUDED on
//     purpose: tests legitimately assert "the wire format value of
//     ErrConfigOpen is `ER-CFG-21001`" — that's a feature, not a leak.
//   - Skips the entire `internal/errtrace/` subtree (incl. codegen
//     and the registry test): those are the source of truth.
//   - Skips files marked `// Code generated …` per Go convention.
//
// **Detection.** Every `*ast.BasicLit{Kind:STRING}` whose unquoted
// body matches `^ER-[A-Z]+-2\d{4}$` is a violation. Substring matches
// are intentionally NOT flagged — the regex is anchored — because
// docstrings/comments use `//` and `/* */` (not string literals)
// and a future doc-string that *embeds* a code in prose ("see
// ER-CFG-21001 for details") would live in a Go comment, not a
// `BasicLit`.
//
// **Allowlist.** Empty today. If a future slice legitimately needs
// the literal (e.g. a JSON fixture file embedded as a string const),
// add the file path to `allowedFiles` with a one-line justification.
// The list MUST stay short — every entry is a hole in the guard.
//
// Spec: mem://workflow/phase1-plan.md (slice P1.5).
package errtrace_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// codeLiteralPattern matches the canonical error-code wire format:
// `ER-` then one-or-more uppercase letters (the block prefix), then
// `-`, then a 5-digit number starting with `2` (the block range:
// 21000–21999 today). Anchored to avoid false positives on URLs,
// IDs, comment fragments, etc.
var codeLiteralPattern = regexp.MustCompile(`^ER-[A-Z]+-2\d{4}$`)

// allowedFiles is the explicit per-file escape hatch. Empty today —
// add only with a justification comment. Path is relative to the
// module root (e.g. "internal/foo/fixture.go").
var allowedFiles = map[string]string{
	// "internal/foo/embedded_fixture.go": "JSON fixture asserting the wire-format payload — needs the literal string for round-trip test parity. Tracked: <slice-id>.",
}

// Satisfies AC-PROJ-12 — AST scan ensures no raw error literals
// leak to the UI; every error must carry an `ER-XXX-NNNNN` code.
func TestAST_NoLiteralErrorCodes(t *testing.T) {
	root := findModuleRoot(t)
	internalDir := filepath.Join(root, "internal")
	errtraceDir := filepath.Join(root, "internal", "errtrace")

	type violation struct {
		Path string
		Line int
		Lit  string
	}
	var violations []violation

	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the whole errtrace subtree — it's the source of truth.
		if d.IsDir() {
			if path == errtraceDir || strings.HasPrefix(path, errtraceDir+string(os.PathSeparator)) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Test files legitimately assert wire-format values.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		// Normalise path separators so allowlist matches on every OS.
		relSlash := filepath.ToSlash(rel)
		if _, allow := allowedFiles[relSlash]; allow {
			return nil
		}

		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			t.Fatalf("parse %s: %v", rel, perr)
		}
		// Skip generated files. The Go convention (`// Code generated
		// … DO NOT EDIT.`) lives in a top-of-file comment.
		if isGeneratedFile(f) {
			return nil
		}

		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				// Backtick-quoted multi-line strings can't hold our
				// pattern (newlines fail the anchored regex), and
				// strconv.Unquote handles both `"..."` and `\`...\``
				// shapes — so an unquote failure here is a malformed
				// AST, not a violation.
				return true
			}
			if codeLiteralPattern.MatchString(value) {
				violations = append(violations, violation{
					Path: relSlash,
					Line: fset.Position(lit.Pos()).Line,
					Lit:  value,
				})
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", internalDir, err)
	}

	if len(violations) == 0 {
		return
	}
	// Deterministic ordering for stable CI output.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Path != violations[j].Path {
			return violations[i].Path < violations[j].Path
		}
		return violations[i].Line < violations[j].Line
	})
	var b strings.Builder
	b.WriteString("Found bare error-code string literals outside internal/errtrace/.\n")
	b.WriteString("Replace each with the typed errtrace.Err* const (see codes.go):\n\n")
	for _, v := range violations {
		b.WriteString("  ")
		b.WriteString(v.Path)
		b.WriteString(":")
		b.WriteString(strconv.Itoa(v.Line))
		b.WriteString(": literal ")
		b.WriteString(strconv.Quote(v.Lit))
		b.WriteString("\n")
	}
	t.Fatal(b.String())
}

// findModuleRoot walks up from the current working directory looking
// for go.mod. Mirrors the helper used by the inline-SQL AST guard.
func findModuleRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find go.mod walking up from %s", cwd)
	return ""
}

// isGeneratedFile honours the Go convention that any file containing
// a line matching `^// Code generated .* DO NOT EDIT\.$` (anywhere in
// the comments) is machine-emitted and should be skipped by linters.
// Reference: https://golang.org/s/generatedcode
var generatedHeader = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

func isGeneratedFile(f *ast.File) bool {
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if generatedHeader.MatchString(c.Text) {
				return true
			}
		}
	}
	return false
}
