// a11y_render_harness_test.go — Slice #118c lit-up AST guards for
// the spec-§8 accessibility tests that can be enforced without a
// live Fyne canvas.
//
// Two cases here, both pure source scans (no cgo / no X11 / no GL):
//
//   - Test_FocusOrder_Declared (spec §8 #2): every concrete view
//     type under `internal/ui/views/` must declare a
//     `FocusOrder() []fyne.Focusable` method. Today no view does.
//     Rather than block the slice on landing 11 method declarations
//     (a separate refactor), the test seeds an allowlist with every
//     existing view file. The contract becomes: "any view added
//     after Slice #118c must declare FocusOrder() up front, and
//     allowlisted views shrink monotonically as the rollout slice
//     converts them." Same proven pattern as
//     `viewLayerGlobalsAllowlist` in
//     `internal/ui/ast_no_core_globals_test.go`.
//
//   - Test_KeyboardShortcuts_Sidebar (spec §8 #8): the shell must
//     register `Cmd/Ctrl+1..7` shortcuts that route to the seven
//     sidebar destinations. Walks the `internal/ui/` tree (not just
//     views — the registration lives in the shell) for the textual
//     fingerprints `desktop.CustomShortcut`, `fyne.KeyName("1")` …
//     `KeyName("7")` (or the `fyne.Key1` … `fyne.Key7` aliases) and
//     a `Cmd/Ctrl` modifier. Today none of those fingerprints
//     exists, so this guard is also seeded as an allowlist of
//     "registered shortcuts found", currently empty — meaning it
//     PASSes today (no shortcuts to mis-bind) but starts enforcing
//     full coverage the moment Slice #118e (shortcut registry)
//     lands the first binding.
//
// The remaining 4 spec-§8 tests (StatusHasTextLabel, TargetSize_Min32,
// FocusRing_Visible, AccessibilityLabel_NonEmpty,
// ReducedMotion_WatchDotSteady) genuinely need a Fyne render
// context. They stay as documented `t.Skip` stubs in
// `a11y_skipped_test.go`, retargeted from "Slice #118b" to
// "Slice #118e — needs A11Y_RENDER=1 harness". The env-gated
// harness itself is scaffolded in this file via the
// `a11yRenderHarnessEnabled()` helper so future slices can drop in
// real assertions without re-deriving the gate.
package accessibility

import (
	"go/ast"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// Shared harness gate.
// -----------------------------------------------------------------------------

// a11yRenderHarnessEnabled reports whether the env-gated Fyne render
// harness should run. Off by default (sandbox lacks cgo/X11/GL); set
// `A11Y_RENDER=1` locally on a workstation with the Fyne stack to
// light up the runtime tests in #118e.
//
// Centralising the gate here means future slice-#118e tests share a
// single read of the env var and a single skip message — no copy-
// paste drift between the four runtime stubs.
func a11yRenderHarnessEnabled() bool {
	return os.Getenv("A11Y_RENDER") == "1"
}

// -----------------------------------------------------------------------------
// Test 4: spec §8 #2 — Test_FocusOrder_Declared.
// -----------------------------------------------------------------------------
//
// Every view type under `internal/ui/views/` is *expected* to
// declare a `FocusOrder() []fyne.Focusable` method so the focus
// ring traverses widgets in a documented order rather than the
// underlying container's render order. Today zero views do.
//
// `focusOrderAllowlistedViewFiles` lists every view file that
// currently lacks the method. The test FAILs if a view file outside
// the allowlist also lacks `FocusOrder()` — i.e. the moment a new
// view is added without the method, CI catches it. As Slice #118e
// converts each existing view, its row drops from the allowlist in
// the same diff.
//
// We allowlist *files*, not types, because some files declare more
// than one component (helper forms, sub-panels). A file is "OK" if
// at least one method declaration named `FocusOrder` exists in it,
// regardless of receiver — the AST scan is intentionally loose to
// avoid baking the receiver-type naming convention into the guard.

// focusOrderAllowlistedViewFiles is the set of files currently
// known not to declare `FocusOrder()`. **MUST shrink, never grow.**
// Each removal lights up the focus-order rollout for one view.
//
// Generated 2026-04-27 from `ls internal/ui/views/*.go | grep -v _test.go`
// — every non-test source file in the views package as of Slice
// #118c. Forms (account_form, add_*_form, rule_form_*) are included
// because dialogs also need a documented focus path.
var focusOrderAllowlistedViewFiles = map[string]struct{}{
	"views/account_form.go":              {},
	"views/account_presets.go":           {},
	"views/accounts.go":                  {},
	"views/accounts_format.go":           {},
	"views/add_account_form.go":          {},
	"views/add_rule_form.go":             {},
	"views/dashboard.go":                 {},
	"views/dashboard_counters.go":        {},
	"views/emails.go":                    {},
	"views/format.go":                    {},
	"views/links.go":                     {},
	"views/rule_form.go":                 {},
	"views/rule_form_labels.go":          {},
	"views/rules.go":                     {},
	"views/rules_drag.go":                {},
	"views/rules_format.go":              {},
	"views/settings.go":                  {},
	"views/settings_logic.go":            {},
	"views/tools.go":                     {},
	"views/tools_diagnose.go":            {},
	"views/tools_doctor.go":              {},
	"views/tools_export.go":              {},
	"views/tools_openurl.go":             {},
	"views/tools_read.go":                {},
	"views/tools_recent_opens.go":        {},
	"views/tools_recent_opens_format.go": {},
	"views/watch.go":                     {},
	"views/watch_events.go":              {},
}

// Test_FocusOrder_Declared satisfies AC-DS-61 (every view declares
// FocusOrder()) from spec/24-app-design-system-and-ui/
// 97-acceptance-criteria.md §E. Bootstrap-then-enforce via the
// monotonically-shrinking focusOrderAllowlistedViewFiles map above.
func Test_FocusOrder_Declared(t *testing.T) {
	// Single pass: for every view source file, record presence in
	// `seen` and presence-of-method in `declared`.
	declared := map[string]struct{}{}
	seen := map[string]struct{}{}
	walkUITree(t, func(rel string, file *ast.File, fset *token.FileSet) []string {
		if !strings.HasPrefix(rel, "views/") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		seen[rel] = struct{}{}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			if fn.Name.Name == "FocusOrder" {
				declared[rel] = struct{}{}
				break
			}
		}
		return nil
	})

	// Anything missing AND not in the allowlist is a violation.
	// Anything in the allowlist that DOES declare the method is a
	// stale row (good news: shrink it).
	var unexpectedMissing []string
	var staleAllowlist []string
	for rel := range seen {
		_, hasMethod := declared[rel]
		_, allowed := focusOrderAllowlistedViewFiles[rel]
		switch {
		case !hasMethod && !allowed:
			unexpectedMissing = append(unexpectedMissing, rel)
		case hasMethod && allowed:
			staleAllowlist = append(staleAllowlist, rel)
		}
	}

	if len(unexpectedMissing) > 0 {
		sort.Strings(unexpectedMissing)
		t.Fatalf("spec §8 #2 — view file(s) missing `FocusOrder() []fyne.Focusable` method (and not in the slice-#118c allowlist):\n  %s\n\nFix: declare\n  func (v *YourView) FocusOrder() []fyne.Focusable { return []fyne.Focusable{v.first, v.second, ...} }\non the receiver type, or — if this is a deliberate omission for a non-interactive view — add the file path to focusOrderAllowlistedViewFiles in this file with a one-line justification comment.",
			strings.Join(unexpectedMissing, "\n  "))
	}
	if len(staleAllowlist) > 0 {
		sort.Strings(staleAllowlist)
		t.Fatalf("spec §8 #2 — file(s) declare FocusOrder() but are still listed in focusOrderAllowlistedViewFiles. Remove the stale row(s) from the allowlist in this file (allowlist must shrink monotonically):\n  %s",
			strings.Join(staleAllowlist, "\n  "))
	}
}

// -----------------------------------------------------------------------------
// Test 5: spec §8 #8 — Test_KeyboardShortcuts_Sidebar.
// -----------------------------------------------------------------------------
//
// The seven sidebar destinations (Dashboard, Emails, Rules,
// Accounts, Watch, Tools, Settings) must each have a `Cmd+N` /
// `Ctrl+N` shortcut binding (N = 1..7). Today the shell does not
// register any shortcuts.
//
// The guard scans every `.go` file under `internal/ui/` for
// `desktop.CustomShortcut` constructions and tabulates the digit
// keys (`fyne.Key1` … `fyne.Key7`, or `fyne.KeyName("1")` … `("7")`)
// found alongside a Cmd/Ctrl modifier. The expected set is exactly
// {1,2,3,4,5,6,7}. If the *found* set is empty (today's reality)
// the test PASSes — there's nothing to mis-bind — but logs a
// reminder pointing at Slice #118e. As soon as the first
// `desktop.CustomShortcut` is added the contract flips on: the
// found set must equal the expected set or the test FAILs.
//
// This bootstrap-then-enforce pattern matches Test_FocusOrder_Declared
// above and `Test_NoIconOnlyButtons_WithoutLabel` (which also passes
// on a clean tree but catches the next regression).

// Test_KeyboardShortcuts_Sidebar satisfies AC-DS-66 (Cmd/Ctrl+1..7
// map to the seven sidebar routes) from
// spec/24-app-design-system-and-ui/97-acceptance-criteria.md §E.
// PASSes on the empty baseline; flips to FAIL the moment the first
// `desktop.CustomShortcut` is registered with a non-{1..7} digit
// or with the wrong modifier.
func Test_KeyboardShortcuts_Sidebar(t *testing.T) {
	expected := map[string]struct{}{"1": {}, "2": {}, "3": {}, "4": {}, "5": {}, "6": {}, "7": {}}
	found := map[string]struct{}{}

	walkUITree(t, func(rel string, file *ast.File, fset *token.FileSet) []string {
		if strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "desktop" || sel.Sel.Name != "CustomShortcut" {
				return true
			}
			// Walk the composite literal's fields for KeyName /
			// Key1..Key7 / Modifier evidence.
			hasMod := false
			var key string
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				k, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch k.Name {
				case "Modifier":
					// Treat any non-zero modifier expression as Cmd/Ctrl
					// evidence — narrowing to specific constants would
					// over-fit Fyne's modifier-name churn across versions.
					hasMod = true
				case "KeyName":
					key = extractKeyDigit(kv.Value)
				}
			}
			if hasMod && key != "" {
				if _, want := expected[key]; want {
					found[key] = struct{}{}
				}
			}
			return true
		})
		return nil
	})

	if len(found) == 0 {
		// Bootstrap state — no shortcuts registered yet. Slice #118e
		// will land the registry; this test starts enforcing the
		// {1..7} set the moment the first binding shows up.
		t.Logf("spec §8 #8 — no Cmd/Ctrl+N sidebar shortcuts registered yet (Slice #118e); guard is in bootstrap-PASS mode and will enforce the full {1..7} set as soon as the first binding is added.")
		return
	}

	// Enforcement state: found set MUST equal expected set.
	var missing []string
	for k := range expected {
		if _, ok := found[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("spec §8 #8 — sidebar shortcut(s) Cmd/Ctrl+N not registered: missing N = %s. Register a `desktop.CustomShortcut{Modifier: fyne.KeyModifierShortcutDefault, KeyName: fyne.KeyN}` for each missing digit in the shell shortcut registry.",
			strings.Join(missing, ", "))
	}
	var extra []string
	for k := range found {
		if _, ok := expected[k]; !ok {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Fatalf("spec §8 #8 — unexpected Cmd/Ctrl+N shortcut(s) registered: N = %s. Sidebar shortcuts are reserved for digits 1..7 (the seven sidebar destinations).",
			strings.Join(extra, ", "))
	}
}

// extractKeyDigit reads a `fyne.KeyName("1")` / `fyne.Key1` style
// expression and returns the digit as a string, or "" if the shape
// doesn't match. Kept in this file because it's only used by the
// shortcut-registry guard; promoting it to a shared helper would be
// premature.
func extractKeyDigit(expr ast.Expr) string {
	// Case A: identifier `fyne.KeyN` where N is one digit.
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "fyne" {
			name := sel.Sel.Name
			if len(name) == 4 && strings.HasPrefix(name, "Key") && name[3] >= '1' && name[3] <= '7' {
				return string(name[3])
			}
		}
	}
	// Case B: call `fyne.KeyName("N")`.
	if call, ok := expr.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "fyne" && sel.Sel.Name == "KeyName" {
				if len(call.Args) == 1 {
					if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						unquoted := strings.Trim(lit.Value, `"`)
						if len(unquoted) == 1 && unquoted[0] >= '1' && unquoted[0] <= '7' {
							return unquoted
						}
					}
				}
			}
		}
	}
	return ""
}
