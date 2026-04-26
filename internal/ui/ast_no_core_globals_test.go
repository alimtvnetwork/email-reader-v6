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
//	  bundle. A small allowlist captures the three pre-existing
//	  legacy call sites (tools_read.go, tools_openurl.go,
//	  launch.go) — these are tracked TODOs that will be migrated in
//	  a follow-up slice; the allowlist makes them visible and
//	  prevents new ones from creeping in.
//
// Why an allowlist instead of failing-loud: the alternative is to
// either (a) hold up shipping the guard until those 3 files are
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
// repo-relative file path. Anything *not* in this set must be empty.
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

func TestAST_NoDeletedCoreSymbolsFromUI(t *testing.T) {
	violations := walkUITree(t, func(rel string, file *ast.File) []string {
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
				hits = append(hits, "core."+sel.Sel.Name)
			}
			return true
		})
		return hits
	})
	if len(violations) > 0 {
		t.Fatalf("internal/ui/ references %d deleted core symbol(s) — these were removed in P2.8b and must not return:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

func TestAST_NoConfigGlobalsInViews(t *testing.T) {
	violations := walkUITree(t, func(rel string, file *ast.File) []string {
		// Only enforce inside views/.
		if !strings.HasPrefix(filepath.ToSlash(rel), "views/") {
			return nil
		}
		// Skip allowlisted legacy call sites.
		if _, ok := viewLayerGlobalsAllowlist[filepath.ToSlash(rel)]; ok {
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
			switch {
			case pkg.Name == "config" && sel.Sel.Name == "Load":
				hits = append(hits, "config.Load()")
			case pkg.Name == "store" && sel.Sel.Name == "Default":
				hits = append(hits, "store.Default()")
			}
			return true
		})
		return hits
	})
	if len(violations) > 0 {
		t.Fatalf("view-layer purity violation — internal/ui/views/ files must receive deps via injected services, not call package globals directly:\n%s\n\nFix: either inject the dependency through *Services / *core.Tools, OR (if this is a known legacy site that needs follow-up migration) add the file path to viewLayerGlobalsAllowlist with a comment explaining the migration plan.",
			strings.Join(violations, "\n"))
	}
}

// walkUITree parses every non-vendor .go file under uiRoot and runs
// `check` against each. Returns aggregated "rel-path: hit" lines.
func walkUITree(t *testing.T, check func(rel string, file *ast.File) []string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var out []string

	err := filepath.Walk(uiRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip unparseable files
		}
		rel, _ := filepath.Rel(uiRoot, path)
		hits := check(rel, file)
		for _, h := range hits {
			out = append(out, rel+": "+h)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
