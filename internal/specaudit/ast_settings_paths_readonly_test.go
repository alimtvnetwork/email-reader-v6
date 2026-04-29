// ast_settings_paths_readonly_test.go closes AC-SF-03 with a pure-AST
// guard over the Settings frontend's "Paths" card.
//
// **Spec contract (AC-SF-03 in
// `spec/21-app/02-features/07-settings/97-acceptance-criteria.md` line 64).**
// "Read-only path rows are not `*widget.Entry`." The user must not be
// able to type into Config path / Data dir / Email archive — those
// values are derived from the running process and editing them in the
// Settings card would silently desync the on-disk reality from the
// in-memory snapshot. Spec sketches a runtime test
// (`Test_Render_PathsReadOnly`), but the structural half — "no
// `widget.NewEntry` call lives inside the paths-card constructor" —
// is checkable from the AST alone, today, without the canvas-bound
// Settings view harness deferred to Slice #118e.
//
// **What this test pins.** The function `newSettingsPaths` in
// `internal/ui/views/settings.go` must not contain any call to
// `widget.NewEntry` (or its multiline cousin `widget.NewMultiLineEntry`).
// The current implementation builds each row from
// `widget.NewLabelWithStyle` + `widget.NewLabel` — a label is read-only
// by construction in Fyne. Vacuously safe today; the day someone
// pastes a `widget.NewEntry(...)` into the paths panel (e.g. while
// "fixing" a width issue), this scanner FAILs and forces them to
// keep the read-only contract or document the spec drift in the same
// diff.
//
// **Scope deliberately narrow.** We only check `newSettingsPaths` —
// the *other* settings cards (Watcher cadence, Browser path,
// Appearance) intentionally use `widget.NewEntry` because those
// fields ARE editable. A broader "no Entry anywhere in settings.go"
// rule would be wrong.
//
// Same template as `ast_export_stream_test.go` (Slice #147). Reuses
// `repoRootForSXGuard` from `ast_settings_security_test.go`.
//
// Spec:
//   - spec/21-app/02-features/07-settings/97-acceptance-criteria.md (AC-SF-03)
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

// Test_AST_SettingsPaths_NoEntryWidget enforces AC-SF-03's structural
// contract: the `newSettingsPaths` function in
// `internal/ui/views/settings.go` may not invoke any
// `widget.New*Entry`-shaped constructor. Each violation is reported
// with file:line so a reviewer can jump straight to the bad row.
//
// We don't reach for go/types (cheap, deterministic, no module-graph
// dependency in CI) — string-matching the selector identifier is
// sufficient because:
//
//   - Fyne's editable-text widgets all share the `*Entry` suffix
//     (`NewEntry`, `NewMultiLineEntry`, `NewPasswordEntry`,
//     `NewEntryWithData`).
//   - A non-Fyne `widget.NewEntry` call from a different package is
//     vanishingly unlikely in this codebase, and even if one
//     appeared, flagging it from the read-only paths card is the
//     right behaviour (the spec is about user-editable affordances,
//     not about which import path produced the widget).
func Test_AST_SettingsPaths_NoEntryWidget(t *testing.T) {
	root := repoRootForSXGuard(t)
	rel := "internal/ui/views/settings.go"
	path := filepath.Join(root, filepath.FromSlash(rel))

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	// Locate the paths-card constructor. Re-pinned in Slice #212
	// (Settings redesign Phase 1) when the helper was renamed from
	// `newSettingsPaths` → `newSettingsPathsCard` to reflect that
	// the new implementation wraps the rows in a `widget.Card` and
	// adds Copy / Open / Reveal buttons. Spec contract is unchanged:
	// no `widget.NewEntry` may live inside this function (the path
	// values stay read-only — the new buttons are the only way to
	// interact with them). Future renames must keep this constant
	// in sync in the same diff.
	const fnName = "newSettingsPathsCard"
	var target *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name != nil && fn.Name.Name == fnName {
			target = fn
			break
		}
	}
	if target == nil {
		t.Fatalf("AC-SF-03 (%s): function %s not found — Settings frontend refactor changed the paths-card entry point. Re-pin the audit's target function in this file's `fnName` constant in the same diff (or document the new contract).",
			rel, fnName)
	}

	// Walk the body and flag any `widget.New*Entry`-shaped call.
	var bad []string
	ast.Inspect(target.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "widget" {
			return true
		}
		if !isEntryConstructorName(sel.Sel.Name) {
			return true
		}
		bad = append(bad, fset.Position(call.Pos()).String()+": widget."+sel.Sel.Name+" is editable; paths card must use widget.NewLabel*")
		return true
	})

	if len(bad) > 0 {
		sort.Strings(bad)
		t.Fatalf("AC-SF-03 (%s): %d editable-Entry constructor call(s) found inside %s — read-only path rows must not be `*widget.Entry`:\n  %s\n\nFix: replace each `widget.NewEntry(...)` with a `widget.NewLabelWithStyle(label, ...) + widget.NewLabel(value)` pair, or — if the field really should become editable — update both the spec (AC-SF-03 + 04-frontend-design.md §3.1 Paths card) and this audit's contract in the same diff.",
			rel, len(bad), fnName, strings.Join(bad, "\n  "))
	}
}

// entryConstructorNames lists the Fyne widget constructors that
// produce a user-editable text field. Kept as a small explicit set
// so future Fyne releases adding new variants are an explicit
// audit-update decision rather than silent drift.
var entryConstructorNames = map[string]struct{}{
	"NewEntry":          {},
	"NewMultiLineEntry": {},
	"NewPasswordEntry":  {},
	"NewEntryWithData":  {},
	"NewSelectEntry":    {},
}

func isEntryConstructorName(name string) bool {
	_, ok := entryConstructorNames[name]
	return ok
}
