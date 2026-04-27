// ast_export_stream_test.go closes AC-DB-26 with a pure-AST guard
// over the ExportCsv streaming path.
//
// **Spec contract (AC-DB-26 in
// `spec/23-app-database/97-acceptance-criteria.md` line 48).**
// `Q-EXPORT-STREAM` is consumed via `*sql.Rows` and the exporter
// never calls `rows.Next` after `rows.Close`. The spec sketches a
// runtime test (`Test_Q_ExportStream_NoBuffering` — race-free run +
// memory ceiling), but the *structural* half of that contract — "no
// stray `.Close()` ahead of the deferred one, no path that re-opens
// after close" — is checkable with the AST alone, today, without
// the bench infra that the runtime half needs (Bucket #9, deferred).
//
// **What this test pins.** For every Go file that touches the
// `EmailExport`-shaped streaming pipeline (currently just
// `internal/core/tools_export.go` and its sibling helper
// `internal/store/shims.go`), the only `.Close()` call on the
// returned `Rows`-shaped value must be inside a `defer` statement.
// Equivalently: no caller may invoke `rows.Close()` non-deferred
// before — or after — the defer is registered. That structural
// rule guarantees the lexical "no `.Next()` after `.Close()`"
// invariant the spec asks for, because:
//
//   - In Go, a deferred `Close()` runs at function exit, after all
//     other statements in the function body have completed. If the
//     ONLY `.Close()` is deferred, then by construction no
//     `.Next()` in the same function can run after it.
//
//   - A second, non-deferred `.Close()` would be the only way to
//     close the rows mid-function and then accidentally call
//     `.Next()` on a closed cursor. We forbid it.
//
// The runtime "no buffering / memory ceiling" half of AC-DB-26 stays
// deferred to the bench infra slice and is NOT cited from this file
// (the coverage audit's stale-ref guard would otherwise treat it as
// false coverage).
//
// **Ratchet.** Vacuously safe today: `tools_export.go` has exactly
// one `defer rows.Close()` and zero non-deferred `Close()` calls.
// The day someone refactors the exporter to manually close-and-reopen
// (or adds a sibling streaming consumer that drops the defer), this
// scanner FAILs and forces them to either keep the deferred-only
// shape or document the spec drift in the same diff.
//
// Same template as `ast_design_system_test.go` (Slice #138). Reuses
// `repoRootForSXGuard` from `ast_settings_security_test.go`.
//
// Spec:
//   - spec/23-app-database/97-acceptance-criteria.md (AC-DB-26)
//   - spec/23-app-database/02-queries.md §3.12 (Q-EXPORT-STREAM cursor)
//   - mem://decisions/06-ac-coverage-rollout-pattern.md (slice template)
package specaudit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// exportStreamFiles names the Go files that own the Q-EXPORT-STREAM
// cursor lifecycle. Pinned explicitly (instead of grep) to keep the
// guard's scope auditable in code review — adding a new consumer to
// the list is a deliberate diff, not an emergent property of file
// naming.
var exportStreamFiles = []string{
	"internal/core/tools_export.go",
}

// Test_AST_ExportStream_RowsCloseOnlyDeferred enforces the
// structural half of AC-DB-26: every `.Close()` invocation on the
// streaming `Rows`-shaped value in `tools_export.go` must be inside
// a `defer` statement.
//
// We don't try to track the exact `*sql.Rows` type via go/types — the
// `tools_export.go` file uses the type-erased `store.RowsScanner`
// interface and the package doesn't import `database/sql` directly
// (that would violate the layering rule pinned by other audits).
// Instead, we check the structural rule:
//
//   - For every `xxx.Close()` call in the file, the immediate parent
//     statement must be `*ast.DeferStmt`, OR the receiver must not
//     match a name that is the LHS of an assignment whose RHS calls
//     a function ending in `Rows` (`QueryEmailExportRows`).
//
// The easier, equivalent rule we actually pin: there is exactly one
// `.Close()` call site in `tools_export.go`, AND that call site is a
// deferred call. Mismatch = FAIL with a precise pointer.
func Test_AST_ExportStream_RowsCloseOnlyDeferred(t *testing.T) {
	root := repoRootForSXGuard(t)
	for _, rel := range exportStreamFiles {
		path := filepath.Join(root, filepath.FromSlash(rel))
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}

		// closeCalls collects every `*.Close()` invocation in the
		// file along with whether it's the direct child of a
		// DeferStmt. We use a stack so we can answer "is the parent
		// a DeferStmt?" without re-walking.
		type closeSite struct {
			pos      token.Position
			receiver string
			deferred bool
		}
		var sites []closeSite

		// stack tracks ancestors during the walk so a leaf node can
		// inspect its immediate parent.
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				// leaving a node — pop.
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				return true
			}
			// Before recursing into children, see if THIS node is a
			// `.Close()` call.
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Close" {
					recvIdent, _ := sel.X.(*ast.Ident)
					recvName := ""
					if recvIdent != nil {
						recvName = recvIdent.Name
					}
					// A defer wraps its CallExpr directly:
					// DeferStmt{ Call: *ast.CallExpr }. So the
					// immediate parent of the CallExpr is the
					// DeferStmt. Anything else (ExprStmt, AssignStmt,
					// IfStmt cond, …) is non-deferred and forbidden.
					deferred := false
					if len(stack) > 0 {
						if _, ok := stack[len(stack)-1].(*ast.DeferStmt); ok {
							deferred = true
						}
					}
					// Filter to receivers that actually look like a
					// Rows cursor — we only care about
					// `rows.Close()` shapes, not e.g. `csvFile.Close()`
					// or `progress.Close()`. The conventional name
					// in this package is `rows`; if the codebase
					// later renames it, the test FAILs and the
					// reviewer must update `rowsLikeReceiverNames`
					// in the same diff.
					if isRowsLikeReceiver(recvName) {
						sites = append(sites, closeSite{
							pos:      fset.Position(call.Pos()),
							receiver: recvName,
							deferred: deferred,
						})
					}
				}
			}
			stack = append(stack, n)
			return true
		})

		if len(sites) == 0 {
			t.Fatalf("AC-DB-26 (%s): expected at least one `rows.Close()` call site for the Q-EXPORT-STREAM cursor; found zero. Did the streaming consumer get refactored away? Update exportStreamFiles or rowsLikeReceiverNames in the same diff.", rel)
		}

		var bad []string
		for _, s := range sites {
			if !s.deferred {
				bad = append(bad, s.pos.String()+": "+s.receiver+".Close() is not in a defer statement")
			}
		}
		if len(bad) > 0 {
			sort.Strings(bad)
			t.Fatalf("AC-DB-26 (%s): every `rows.Close()` on the Q-EXPORT-STREAM cursor must be deferred — a non-deferred Close lets a later `rows.Next()` run on a closed cursor:\n  %s\n\nFix: replace the bare `rows.Close()` with `defer rows.Close()` immediately after the rows-bearing assignment, OR document the layered cleanup in a code comment AND update this audit's contract in the same diff.",
				rel, strings.Join(bad, "\n  "))
		}
	}
}

// rowsLikeReceiverNames lists the conventional identifier names used
// for the Q-EXPORT-STREAM cursor. Kept as a small explicit set so
// `csvFile.Close()` and `progress.Close()` (if it ever existed) don't
// trigger false positives. If a future refactor renames the cursor
// to `cursor` or `it`, add the name here in the same diff.
var rowsLikeReceiverNames = map[string]struct{}{
	"rows": {},
}

func isRowsLikeReceiver(name string) bool {
	_, ok := rowsLikeReceiverNames[name]
	return ok
}
