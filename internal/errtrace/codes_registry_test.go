// codes_registry_test.go is the P1.4 bidirectional drift guard.
//
// Every error code lives in TWO places that must stay in lock-step:
//
//   1. `codes.go` — hand-edited Go `const Err* Code = "ER-XXX-NNNNN"`
//      declarations. Source of truth for the *Go identifier*.
//   2. `codes_gen.go` (generated from `codes.yaml`) — populates
//      `RegisteredCodes map[Code]CodeMeta`. Source of truth for the
//      *runtime registry* and for downstream consumers (UI surfacing,
//      error-registry doc, P1.5 AST guard).
//
// If a developer adds a `const` to `codes.go` but forgets to update
// `codes.yaml` + re-run codegen — or vice versa — the two files
// silently disagree. This test catches that drift at `go test` time
// by parsing `codes.go` with `go/parser` and asserting set-equality
// with the generated registry, in both directions:
//
//   - **Every const in codes.go is a key in RegisteredCodes.**
//     Catches: "added const, forgot codegen run".
//   - **Every key in RegisteredCodes corresponds to a const in codes.go.**
//     Catches: "added yaml row, forgot to add the Go const".
//   - **Per-entry: code string in codes.go matches the Code key.**
//     Catches: "typo in yaml — yaml says ER-CFG-21001 but codes.go
//     says ER-CFG-21010 for the same const name".
//   - **Per-entry: const identifier matches `CodeMeta.Const`.**
//     Catches: "yaml `const:` field drifted from Go identifier".
//
// We use `go/parser` (not `reflect` or build-tags) because:
//
//   - It works on the source text — no compile cycle needed when the
//     test itself drives the assertion.
//   - It surfaces the source-line number on failure, so the error
//     message points right at the offending const declaration.
//
// IMPORTANT: this file is `package errtrace` (white-box) so it can
// reference `RegisteredCodes` and the `Err*` consts directly without
// fighting an import cycle.
package errtrace

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// parsedCode mirrors what we extract per-const from the AST. Kept
// minimal — we only need (identifier, code string, source position)
// to drive every assertion in this file.
type parsedCode struct {
	Const string
	Code  string
	Pos   token.Position
}

// parseCodesGo reads codes.go from disk and returns one parsedCode
// per `Err* Code = "..."` declaration. Helper kept tiny so the test
// bodies focus on assertions, not AST plumbing.
func parseCodesGo(t *testing.T) []parsedCode {
	t.Helper()
	fset := token.NewFileSet()
	// Path is relative to the test binary's working directory, which
	// for `go test ./internal/errtrace/...` is the package dir.
	f, err := parser.ParseFile(fset, "codes.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse codes.go: %v", err)
	}
	var out []parsedCode
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			// We only care about declarations of form
			//   ErrXxx Code = "ER-..."
			// — i.e. a single name + a single string-literal value
			// + a Type ident equal to "Code". Skip anything else
			// (no type ident, no value, untyped iota etc.).
			if len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			typIdent, ok := vs.Type.(*ast.Ident)
			if !ok || typIdent.Name != "Code" {
				continue
			}
			lit, ok := vs.Values[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("unquote %s value %q: %v",
					vs.Names[0].Name, lit.Value, err)
			}
			out = append(out, parsedCode{
				Const: vs.Names[0].Name,
				Code:  value,
				Pos:   fset.Position(vs.Names[0].Pos()),
			})
		}
	}
	if len(out) == 0 {
		t.Fatal("parser found zero `Err* Code = \"...\"` declarations in codes.go — AST extractor is broken")
	}
	return out
}

func TestRegistry_NonEmpty(t *testing.T) {
	// Sanity: codegen actually produced a populated map. If this
	// fires, every other assertion below would be vacuously true.
	if len(RegisteredCodes) == 0 {
		t.Fatal("RegisteredCodes is empty — did codegen run?")
	}
}

func TestRegistry_EveryConstIsRegistered(t *testing.T) {
	// Direction 1: codes.go → RegisteredCodes. Every hand-written
	// const must have a generated registry entry.
	parsed := parseCodesGo(t)
	for _, p := range parsed {
		meta, ok := RegisteredCodes[Code(p.Code)]
		if !ok {
			t.Errorf("%s: const %s = %q is not in RegisteredCodes\n"+
				"  → add the matching row to codes.yaml and re-run "+
				"`go generate ./internal/errtrace/...`",
				p.Pos, p.Const, p.Code)
			continue
		}
		// Per-entry consistency: the Const field on CodeMeta must
		// match the actual Go identifier we just parsed.
		if meta.Const != p.Const {
			t.Errorf("%s: const %s has Code %q whose CodeMeta.Const = %q (want %q)\n"+
				"  → codes.yaml row for %q likely has a typo'd `const:` field",
				p.Pos, p.Const, p.Code, meta.Const, p.Const, p.Code)
		}
	}
}

func TestRegistry_EveryRegisteredHasConst(t *testing.T) {
	// Direction 2: RegisteredCodes → codes.go. Every yaml/registry
	// entry must have a corresponding Go const. Catches the
	// "added yaml row, forgot the const" failure mode.
	parsed := parseCodesGo(t)
	known := make(map[Code]string, len(parsed))
	for _, p := range parsed {
		known[Code(p.Code)] = p.Const
	}
	// Sort keys so failures are deterministic across test runs.
	missing := make([]string, 0)
	for code, meta := range RegisteredCodes {
		if _, ok := known[code]; !ok {
			missing = append(missing, string(code)+" (CodeMeta.Const="+meta.Const+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("RegisteredCodes contains %d entries with no matching const in codes.go:\n  %s\n"+
			"  → add the matching `const Err* Code = \"...\"` line to codes.go",
			len(missing), strings.Join(missing, "\n  "))
	}
}

func TestRegistry_CountsMatch(t *testing.T) {
	// Belt-and-braces: a stricter cardinality check that surfaces a
	// summary even when the per-direction tests above already
	// caught the offending entries. Useful in CI logs to see
	// "89 == 89" vs "89 != 90" at a glance.
	parsed := parseCodesGo(t)
	if len(parsed) != len(RegisteredCodes) {
		t.Fatalf("count mismatch: codes.go has %d const declarations, RegisteredCodes has %d",
			len(parsed), len(RegisteredCodes))
	}
}

func TestRegistry_PrefixDerivableFromCode(t *testing.T) {
	// Cross-check that `CodeMeta.Prefix` was filled correctly by
	// codegen — every entry's Prefix must be the substring of its
	// Code value up to the second `-` (e.g. "ER-CFG-21001" → "ER-CFG").
	// Catches yaml `prefix:` field drift from the codes underneath.
	for code, meta := range RegisteredCodes {
		s := string(code)
		idx := strings.LastIndex(s, "-")
		if idx <= 0 {
			t.Errorf("Code %q malformed (no `-`)", s)
			continue
		}
		wantPrefix := s[:idx]
		if meta.Prefix != wantPrefix {
			t.Errorf("Code %q: CodeMeta.Prefix = %q, want %q (derived from code string)",
				s, meta.Prefix, wantPrefix)
		}
	}
}
