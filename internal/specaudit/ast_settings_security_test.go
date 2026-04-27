// ast_settings_security_test.go enforces the AC-SX (Settings cross-cutting
// security) acceptance rows that are pure AST/log scans:
//
//   - AC-SX-01  AST scan: only `internal/config` writes `config.json`.
//   - AC-SX-02  AST scan: `Settings.Save` body never references identifiers
//     `Accounts` or `Rules`.
//   - AC-SX-03  AST scan: `internal/ui/views/settings.go` contains no
//     `color.RGBA{` / `color.NRGBA{` literals.
//   - AC-SX-04  Log scan: `ChromePath` value never appears at level ≥ INFO
//     across all Settings tests (headless half).
//   - AC-SX-05  Log scan: `IncognitoArg` value never appears at any level
//     across all Settings tests (headless half).
//
// All five rows are headless and require no Fyne canvas. They are repo-root
// scans modelled on `internal/store/ast_maintenance_only_test.go`.
//
// Spec: spec/21-app/02-features/07-settings/97-acceptance-criteria.md §AC-SX.
package specaudit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Repo-root resolution (shared by every AC-SX scanner below).
// ---------------------------------------------------------------------------

func repoRootForSXGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot locate test source file")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate go.mod walking up from test file")
	return ""
}

func skipUninterestingDirSX(name string) error {
	switch name {
	case ".git", "node_modules", ".lovable", "dist", "build":
		return filepath.SkipDir
	}
	return nil
}

// candidateProductionGo returns (rel, true) when path is a production .go
// file (excludes _test.go). rel is forward-slashed and repo-relative.
func candidateProductionGo(root, path string) (string, bool) {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// candidateTestGo returns (rel, true) when path is a Go test file.
func candidateTestGo(root, path string) (string, bool) {
	if !strings.HasSuffix(path, "_test.go") {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// ---------------------------------------------------------------------------
// AC-SX-01 — Only `internal/config` writes `config.json`.
//
// Strategy: walk every production .go file, parse it, and look for any
// string literal whose trimmed value equals "config.json". The single
// permitted occurrence is in internal/config/config.go (the canonical
// path constant). We additionally allow internal/core/settings_extension.go
// to *reference* the path *only* via the config-package API, which it
// does — the extension writer composes its tmp path from `config.Path()`,
// not a literal. The allowlist locks the only string-literal site.
// ---------------------------------------------------------------------------

var configJSONLiteralAllowlist = map[string]bool{
	"internal/config/config.go": true,
}

func Test_AST_OnlyConfigWritesFile(t *testing.T) {
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
		if configJSONLiteralAllowlist[rel] {
			return nil
		}
		violations = append(violations, scanFileForConfigJSONLiteral(path, rel)...)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("AC-SX-01 violation: %q literal found outside internal/config:\n  %s",
			"config.json", strings.Join(violations, "\n  "))
	}
}

func scanFileForConfigJSONLiteral(path, rel string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return nil
	}
	var hits []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		body := strings.Trim(lit.Value, "`\"")
		if body == "config.json" {
			hits = append(hits, rel+": "+lit.Value)
		}
		return true
	})
	return hits
}

// ---------------------------------------------------------------------------
// AC-SX-02 — `Settings.Save` body never references identifiers `Accounts`
// or `Rules`. Save is a thin §5 entrypoint that delegates to `persist`;
// it MUST NOT touch the Accounts or Rules sub-trees of config (those are
// owned by their own services and persisted through CF-A2 / CF-R1).
// ---------------------------------------------------------------------------

func Test_AST_Save_NoAccountsRulesRefs(t *testing.T) {
	root := repoRootForSXGuard(t)
	target := filepath.Join(root, "internal", "core", "settings.go")
	src, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read settings.go: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), target, src, 0)
	if err != nil {
		t.Fatalf("parse settings.go: %v", err)
	}
	saveBody := findMethodBody(file, "Save", "Settings")
	if saveBody == nil {
		t.Fatal("AC-SX-02: could not locate (*Settings).Save body in settings.go")
	}
	var hits []string
	forbidden := map[string]bool{"Accounts": true, "Rules": true}
	ast.Inspect(saveBody, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if forbidden[ident.Name] {
			hits = append(hits, ident.Name)
		}
		return true
	})
	if len(hits) > 0 {
		t.Fatalf("AC-SX-02 violation: Settings.Save body references forbidden identifiers: %v", hits)
	}
}

// findMethodBody returns the body of method `name` on receiver type
// `recvType` (matching either `*recvType` or `recvType`). Returns nil
// if not found.
func findMethodBody(file *ast.File, name, recvType string) *ast.BlockStmt {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		if recvIdent(fn.Recv.List[0].Type) == recvType {
			return fn.Body
		}
	}
	return nil
}

func recvIdent(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// ---------------------------------------------------------------------------
// AC-SX-03 — `internal/ui/views/settings.go` contains no `color.RGBA{` /
// `color.NRGBA{` composite literals. All colors must flow through the
// theme tokens (cf. AC-DS hard-coded-color guard). The scan parses the
// file and walks every CompositeLit, checking the type expression.
// ---------------------------------------------------------------------------

func Test_AST_Settings_NoDirectColor(t *testing.T) {
	root := repoRootForSXGuard(t)
	target := filepath.Join(root, "internal", "ui", "views", "settings.go")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		// File hasn't been broken out yet — vacuously pass; the
		// AST guard locks it the moment the file appears.
		t.Skip("internal/ui/views/settings.go not present yet — guard vacuously passes")
	}
	src, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read settings.go: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), target, src, 0)
	if err != nil {
		t.Fatalf("parse settings.go: %v", err)
	}
	var hits []string
	ast.Inspect(file, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if name := compositeTypeName(cl.Type); name == "color.RGBA" || name == "color.NRGBA" {
			hits = append(hits, name)
		}
		return true
	})
	if len(hits) > 0 {
		t.Fatalf("AC-SX-03 violation: settings.go contains forbidden color literals: %v", hits)
	}
}

// compositeTypeName returns "color.RGBA"-style names for SelectorExpr
// types and the bare identifier name for Ident types. Empty otherwise.
func compositeTypeName(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + sel.Sel.Name
}

// ---------------------------------------------------------------------------
// AC-SX-04 — Log scan: `ChromePath` value never appears at level ≥ INFO.
// Headless interpretation: no Settings test source contains a logger call
// that interpolates a `ChromePath` field at INFO/WARN/ERROR. We approximate
// by scanning every Settings test file (`internal/core/settings*_test.go`
// and `internal/browser/*_test.go`) for direct format-string interpolation
// of `.ChromePath` at INFO+ logging APIs. A literal log line MUST redact
// to `<redacted>` per spec §SX. The current implementation never logs the
// raw path, so the scan should find zero hits.
// ---------------------------------------------------------------------------

func Test_LogScan_NoChromePathLeak(t *testing.T) {
	root := repoRootForSXGuard(t)
	violations := scanLogLeaks(t, root, "ChromePath", logLevelsInfoPlus())
	if len(violations) > 0 {
		t.Fatalf("AC-SX-04 violation: ChromePath value leaked at INFO+ in:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// ---------------------------------------------------------------------------
// AC-SX-05 — Log scan: `IncognitoArg` value never appears at any level.
// Same scanner as SX-04 but covers every logging level (Debug included).
// ---------------------------------------------------------------------------

func Test_LogScan_NoIncognitoArgLeak(t *testing.T) {
	root := repoRootForSXGuard(t)
	violations := scanLogLeaks(t, root, "IncognitoArg", logLevelsAll())
	if len(violations) > 0 {
		t.Fatalf("AC-SX-05 violation: IncognitoArg value leaked at any level in:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// logLevelsInfoPlus enumerates the substrings that identify a logging
// call at level INFO or higher. We deliberately include both the
// stdlib `log.Printf` family (which is treated as INFO+) and slog's
// Info/Warn/Error levels.
func logLevelsInfoPlus() []string {
	return []string{
		"log.Printf", "log.Println", "log.Print",
		"log.Fatalf", "log.Fatalln", "log.Fatal",
		"logger.Printf", "logger.Println", "logger.Print",
		"slog.Info", "slog.Warn", "slog.Error",
		".Info(", ".Infof(", ".Warn(", ".Warnf(", ".Error(", ".Errorf(",
	}
}

func logLevelsAll() []string {
	return append(logLevelsInfoPlus(),
		"slog.Debug", ".Debug(", ".Debugf(",
		"fmt.Printf", "fmt.Println", "fmt.Print",
	)
}

// scanLogLeaks walks every production AND test .go file under root and
// flags lines that (a) match one of the logging-call substrings AND
// (b) reference the field name (e.g. `.ChromePath`). A match means the
// raw value flows into a logger argument list — a potential leak.
//
// Production code is scanned because spec §SX forbids the leak there
// regardless of whether tests trigger it; test code is scanned because
// the AC explicitly says "across all Settings tests". To keep the scan
// fast and dependency-free, we use line-based matching over the file
// bytes rather than full AST resolution.
func scanLogLeaks(t *testing.T, root, fieldName string, levels []string) []string {
	t.Helper()
	needle := "." + fieldName
	var violations []string
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipUninterestingDirSX(d.Name())
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, perr := filepath.Rel(root, path)
		if perr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		// Skip this guard file itself — it legitimately mentions
		// the field names as string literals (the scanner needles).
		if strings.HasSuffix(rel, "internal/specaudit/ast_settings_security_test.go") {
			return nil
		}
		violations = append(violations, scanFileForLogLeak(path, rel, needle, levels)...)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return violations
}

func scanFileForLogLeak(path, rel, needle string, levels []string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var hits []string
	for i, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		for _, lvl := range levels {
			if strings.Contains(line, lvl) {
				hits = append(hits, formatLogHit(rel, i+1, line))
				break
			}
		}
	}
	return hits
}

func formatLogHit(rel string, lineNo int, line string) string {
	trim := strings.TrimSpace(line)
	if len(trim) > 120 {
		trim = trim[:117] + "..."
	}
	return rel + ":" + itoa(lineNo) + ": " + trim
}

// itoa is a tiny dependency-free int→string helper (avoids pulling
// strconv into a guard file that already has a wide blast radius).
func itoa(n int) string {
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
