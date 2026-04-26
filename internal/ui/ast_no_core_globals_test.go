// ast_no_core_globals_test.go — Phase 2.9 contract guard.
//
// Locks the outcome of Phase 2 (slices P2.2 → P2.8b). The migration
// lifted Dashboard/Emails/Rules out of process-global package-level
// functions in `internal/core` into typed `*Service` structs that
// must be constructed at app boot (`BuildServices`) and threaded
// through `viewFor` into every view. Once that contract held across
// 17 packages we *deleted* 9 deprecated wrappers + 2 panic helpers
// in P2.8b.
//
// Without an automated guard, a future "quick fix" would be free to:
//
//   - re-add `core.AddRule(...)` as a thin convenience wrapper, or
//   - sneak `config.Load()` into a new view file, or
//   - paper over a missing service injection by calling the old
//     package-level entry point directly.
//
// Each of those would silently un-do months of refactor work and
// reintroduce the exact globals-in-views shape Phase 2 fixed.
//
// This test parses every `.go` file under `internal/ui/` (regardless
// of build tag — we use `go/parser` directly so `-tags nofyne` does
// not hide `//go:build !nofyne` files) and enforces two contracts:
//
//	Contract A — Dead-symbol blocklist
//	  No call to any of the 11 deleted core symbols may appear
//	  anywhere under internal/ui/. If someone re-adds (and re-imports)
//	  e.g. `core.ListEmails`, this test fails immediately.
//
//	Contract B — View-layer purity
//	  Files under internal/ui/views/ may not call `config.Load()`
//	  or `store.Default()`. Views must receive their dependencies
//	  via injected services or via the shared `BuildServices()`
//	  bundle. A small allowlist captures the pre-existing legacy
//	  call sites; these are tracked TODOs that will be migrated in
//	  a follow-up slice. The allowlist makes them visible and
//	  prevents new ones from creeping in.
//
// Why an allowlist instead of failing-loud: the alternative is to
// either (a) hold up shipping the guard until those files are
// migrated — coupling unrelated work into a single slice, against
// our atomic-slice contract — or (b) ship no guard at all and let
// regression risk accumulate. The allowlist is the lesser evil and
// is itself the official to-do list for the follow-up.
package ui

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

// uiRoot is the directory the guard scans. Resolved at runtime
// because `go test` sets CWD to the package directory.
const uiRoot = "."

// deletedCoreSymbols is the closed set of core package-level
// functions deleted in slice P2.8b. Re-introducing any of them
// (and calling them from internal/ui/) must fail this test.
//
// Keep alphabetised. If you delete *more* core wrappers in a future
// slice, add their names here; that's the entire upkeep.
var deletedCoreSymbols = map[string]struct{}{
	"AddRule":                  {},
	"CountEmails":              {},
	"GetEmail":                 {},
	"GetRule":                  {},
	"ListEmails":               {},
	"ListRules":                {},
	"LoadDashboardStats":       {},
	"RemoveRule":               {},
	"SetRuleEnabled":           {},
	"mustDefaultEmailsService": {},
	"mustDefaultRulesService":  {},
}

// viewLayerGlobalsAllowlist captures known pre-Phase-2 calls to
// `config.Load()` from `internal/ui/views/`. Each entry is a
// repo-relative file path (using forward slashes for portability).
// Anything *not* in this set must be empty.
//
// To remove an entry: migrate the file to receive its config via an
// injected dependency (typically a `*core.Tools` or new service),
// then delete the entry. The test will start enforcing purity for
// that file from then on.
var viewLayerGlobalsAllowlist = map[string]struct{}{
	"views/launch.go":        {}, // browser launcher — needs cfg.Browser
	"views/tools_openurl.go": {}, // tools tab open-url helper
	"views/tools_read.go":    {}, // tools tab read helper
}

// TestAST_NoDeletedCoreSymbolsFromUI enforces Contract A.
func TestAST_NoDeletedCoreSymbolsFromUI(t *testing.T) {
	violations := walkUITree(t, func(rel string, file *ast.File, fset *token.FileSet) []string {
		var hits []string
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "core" {
				return true
			}
			if _, dead := deletedCoreSymbols[sel.Sel.Name]; dead {
				pos := fset.Position(sel.Pos())
				hits = append(hits, rel+":"+itoa(pos.Line)+": core."+sel.Sel.Name)
			}
			return true
		})
		return hits
	})
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("internal/ui/ references %d deleted core symbol(s) — these were removed in P2.8b and must not return:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// TestAST_NoConfigGlobalsInViews enforces Contract B.
func TestAST_NoConfigGlobalsInViews(t *testing.T) {
	violations := walkUITree(t, func(rel string, file *ast.File, fset *token.FileSet) []string {
		relSlash := filepath.ToSlash(rel)
		// Only enforce inside views/.
		if !strings.HasPrefix(relSlash, "views/") {
			return nil
		}
		// Skip allowlisted legacy call sites.
		if _, ok := viewLayerGlobalsAllowlist[relSlash]; ok {
			return nil
		}
		// Skip test files — fixtures may legitimately stub config.
		if strings.HasSuffix(rel, "_test.go") {
			return nil
		}
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
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos())
			switch {
			case pkg.Name == "config" && sel.Sel.Name == "Load":
				hits = append(hits, relSlash+":"+itoa(pos.Line)+": config.Load()")
			case pkg.Name == "store" && sel.Sel.Name == "Default":
				hits = append(hits, relSlash+":"+itoa(pos.Line)+": store.Default()")
			}
			return true
		})
		return hits
	})
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("view-layer purity violation — internal/ui/views/ files must receive deps via injected services, not call package globals directly:\n  %s\n\nFix: either inject the dependency through *Services / *core.Tools, OR (if this is a known legacy site that needs follow-up migration) add the file path to viewLayerGlobalsAllowlist with a comment explaining the migration plan.",
			strings.Join(violations, "\n  "))
	}
}

// walkUITree parses every non-vendor .go file under uiRoot and runs
// `check` against each. Returns aggregated violation strings.
//
// We use go/parser directly (not packages.Load) because the latter
// requires a working build for the active tag set; this guard must
// run under `-tags nofyne` and still see `//go:build !nofyne` files.
func walkUITree(t *testing.T, check func(rel string, file *ast.File, fset *token.FileSet) []string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var out []string
	err := filepath.Walk(uiRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			// Skip vendored / generated tree just in case.
			if info.Name() == "vendor" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// ParseFile honours build tags only via the BuildContext in
		// go/build; go/parser itself ignores them, which is exactly
		// what we want — the guard sees every file regardless of
		// `//go:build` line so it can't be bypassed by tag.
		f, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		rel, relErr := filepath.Rel(uiRoot, path)
		if relErr != nil {
			rel = path
		}
		out = append(out, check(rel, f, fset)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", uiRoot, err)
	}
	return out
}

// itoa is a tiny strconv.Itoa stand-in to avoid importing strconv
// just for line numbers in error messages.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
