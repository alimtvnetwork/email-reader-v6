// cf_r2_ast_guard_test.go locks CF-R2: the Rules UI must not expose
// any widget that lets the user override the OpenUrl scheme allowlist
// or the localhost / private-IP guards. All of those validations live
// in core.Tools.OpenUrl (single chokepoint per spec/21-app/02-features/
// 06-tools/00-overview.md §5) and the Rules UI MUST NOT carry a
// per-rule bypass.
//
// We enforce this with a token-level scan of rules*.go and the rule
// form file: any occurrence of the forbidden identifiers fails the
// build. If a future feature needs one of these knobs, it has to land
// in Settings (where the guards apply globally) — not in Rules.
//
// Spec: spec/21-app/02-features/03-rules/99-consistency-report.md CF-R2.
package views

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCF_R2_RulesUI_NoSchemeBypass(t *testing.T) {
	files, err := filepath.Glob("rule*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no rules UI files matched — guard would silently pass")
	}

	// Forbidden identifier substrings — case-insensitive. Any of these in
	// an identifier or string literal in the Rules UI is a CF-R2 violation.
	forbidden := []string{
		"AllowLocalhostUrls",
		"AllowPrivateIpUrls",
		"OpenUrlAllowedSchemes",
		"javascript:",
		"file:",
		"data:",
	}
	identRe := regexp.MustCompile(`(?i)(allowedSchemes|allowLocalhost|allowPrivateIp|schemeBypass)`)

	fset := token.NewFileSet()
	for _, f := range files {
		// Skip generated test files and this very guard.
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, tok := range forbidden {
			if strings.Contains(string(src), tok) {
				t.Errorf("CF-R2 violation: %s contains forbidden token %q", f, tok)
			}
		}
		if loc := identRe.FindString(string(src)); loc != "" {
			t.Errorf("CF-R2 violation: %s contains scheme-bypass-shaped identifier %q", f, loc)
		}

		// Parse + walk to catch identifier composition (e.g.,
		// `cfg.AllowLocalhostUrls` access). The substring scan above
		// already covers this, but the AST walk hardens against
		// reformatting.
		file, err := parser.ParseFile(fset, f, src, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, tok := range []string{"AllowLocalhostUrls", "AllowPrivateIpUrls", "OpenUrlAllowedSchemes"} {
				if id.Name == tok {
					t.Errorf("CF-R2 violation: %s references identifier %q", f, tok)
				}
			}
			return true
		})
	}
}
