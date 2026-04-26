// ast_no_sql_type_leak_test.go enforces AC-DB-51:
//
//	"AST scan: no `*sql.DB`, `*sql.Tx`, or `*sql.Rows` value escapes
//	 `internal/store/...` (return types of public methods do not
//	 include them)."
//
// Rationale: `internal/store/...` is the SQL chokepoint (per AC-DB-50).
// If a public store method returns a raw `*sql.DB` / `*sql.Tx` /
// `*sql.Rows`, callers in `internal/core/*` and `internal/ui/*` can
// drive the database directly — bypassing typed query helpers,
// migration guarantees, and AST guards #34/#35.
//
// The guard parses every production .go file under `internal/store/...`
// and walks every top-level `*ast.FuncDecl`. For each one whose name
// is exported, it inspects the return-type list (`Type.Results`) and
// fails when it finds any `*sql.DB|Tx|Rows` reference at any depth
// (bare pointer, slice element, map value, channel element, generic
// type-arg, named-type embed, etc.).
//
// Test files (`*_test.go`) are exempt — fixtures legitimately surface
// `*sql.DB` for in-memory test setup.
//
// Spec: spec/23-app-database/97-acceptance-criteria.md AC-DB-51.
package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// leakedSqlTypes lists the unqualified type names from package
// `database/sql` that must never appear in a public store method's
// return list.
var leakedSqlTypes = map[string]bool{
	"DB":   true,
	"Tx":   true,
	"Rows": true,
}

// storeRootRel is the repo-relative directory whose public methods we
// audit. Forward-slashed.
const storeRootRel = "internal/store"

func Test_AST_NoSqlTypeLeak(t *testing.T) {
	root := repoRootForMaintenanceGuard(t) // shared with #34/#35
	storeDir := filepath.Join(root, filepath.FromSlash(storeRootRel))
	violations := scanStoreForSqlReturnLeaks(t, storeDir, root)
	if len(violations) > 0 {
		t.Fatalf("AC-DB-51 violation: public store methods leak sql.{DB,Tx,Rows} in their return types:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// scanStoreForSqlReturnLeaks walks storeDir and returns
// "<rel>:<line> <Receiver.Method>: returns *sql.X" entries for every
// public method/function whose return list mentions a forbidden type.
func scanStoreForSqlReturnLeaks(t *testing.T, storeDir, repoRoot string) []string {
	t.Helper()
	var violations []string
	walk := func(path string, d fs.DirEntry, err error) error {
		return inspectStoreGoFile(repoRoot, path, d, err, &violations)
	}
	if err := filepath.WalkDir(storeDir, walk); err != nil {
		t.Fatalf("walk store dir: %v", err)
	}
	return violations
}

// inspectStoreGoFile filters non-Go / test files, parses the source,
// and appends per-function violations.
func inspectStoreGoFile(repoRoot, path string, d fs.DirEntry, walkErr error, out *[]string) error {
	if walkErr != nil {
		return walkErr
	}
	if d.IsDir() {
		return nil
	}
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	*out = append(*out, scanFileForSqlReturnLeaks(path, rel)...)
	return nil
}

// scanFileForSqlReturnLeaks parses path and returns one violation
// string per public function whose return list mentions sql.{DB,Tx,Rows}.
func scanFileForSqlReturnLeaks(path, rel string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil
	}
	var hits []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !isExportedFuncDecl(fn) {
			continue
		}
		hits = append(hits, leaksInReturnList(fn, fset, rel)...)
	}
	return hits
}

// isExportedFuncDecl reports whether fn declares a public symbol
// reachable from outside the package.
func isExportedFuncDecl(fn *ast.FuncDecl) bool {
	if fn.Name == nil || !fn.Name.IsExported() {
		return false
	}
	// Exported method on an unexported receiver is still unreachable
	// from outside the package; skip it to avoid false positives.
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvType := unwrapPointerType(fn.Recv.List[0].Type)
		if id, ok := recvType.(*ast.Ident); ok && !id.IsExported() {
			return false
		}
	}
	return true
}

// leaksInReturnList walks the FuncType.Results and returns one entry
// for each forbidden `sql.{DB,Tx,Rows}` reference found at any depth.
func leaksInReturnList(fn *ast.FuncDecl, fset *token.FileSet, rel string) []string {
	if fn.Type == nil || fn.Type.Results == nil {
		return nil
	}
	var hits []string
	for _, field := range fn.Type.Results.List {
		ast.Inspect(field.Type, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			if !isIdent || pkg.Name != "sql" || sel.Sel == nil {
				return true
			}
			if !leakedSqlTypes[sel.Sel.Name] {
				return true
			}
			pos := fset.Position(field.Pos())
			hits = append(hits, formatLeakHit(rel, pos.Line, fn, sel.Sel.Name))
			return true
		})
	}
	return hits
}

// formatLeakHit produces "rel:line Receiver.Method: returns sql.Type".
func formatLeakHit(rel string, line int, fn *ast.FuncDecl, typeName string) string {
	name := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		if recv := receiverTypeName(fn.Recv.List[0].Type); recv != "" {
			name = recv + "." + name
		}
	}
	return rel + ":" + itoa(line) + " " + name + ": returns sql." + typeName
}

// receiverTypeName extracts the bare type name from a receiver type
// expression — handling `T`, `*T`, and (for completeness) generic
// `T[...]` / `*T[...]`.
func receiverTypeName(expr ast.Expr) string {
	expr = unwrapPointerType(expr)
	if idx, ok := expr.(*ast.IndexExpr); ok {
		expr = idx.X
	}
	if idx, ok := expr.(*ast.IndexListExpr); ok {
		expr = idx.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func unwrapPointerType(expr ast.Expr) ast.Expr {
	if star, ok := expr.(*ast.StarExpr); ok {
		return star.X
	}
	return expr
}

// itoa is a tiny stdlib-free int→string formatter so we don't pull in
// strconv for one call site (keeps the import block lean).
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

// (repoRootForMaintenanceGuard defined in ast_maintenance_only_test.go.)
