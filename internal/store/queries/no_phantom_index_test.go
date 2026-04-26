// no_phantom_index_test.go — Slice #109 guard.
//
// History: prior to Slice #109, the doc-comment on
// `EmailsCountUnreadAll` claimed the Unread COUNT was served by an
// index named `IxEmailAliasIsRead` allegedly created by M0010. That
// claim was false on two counts: (a) M0010 only ALTERs columns, it
// creates no indexes (verified by `m0010_add_email_flags.go`); and
// (b) no migration anywhere creates an index with that name. The
// comment was a copy-paste from an earlier draft of the migration
// plan that was never landed.
//
// This test makes the lie impossible to re-introduce: it scans every
// `.go` source file under `internal/store/` for the literal token
// `IxEmailAliasIsRead` and fails if any hit is found. If a future
// slice legitimately adds that index in a new migration, this test
// is the obvious place to update — flip the assertion to "must
// appear in exactly one CREATE INDEX statement under migrate/" and
// land it together with the migration.
package queries_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoPhantomIndex_IxEmailAliasIsRead(t *testing.T) {
	root := findRepoRoot(t)
	storeDir := filepath.Join(root, "internal", "store")

	const phantom = "IxEmailAliasIsRead"
	var hits []string

	err := filepath.WalkDir(storeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip this guard file itself — it legitimately mentions
		// the token in prose.
		if strings.HasSuffix(path, "no_phantom_index_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), phantom) {
			rel, _ := filepath.Rel(root, path)
			hits = append(hits, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", storeDir, err)
	}

	if len(hits) > 0 {
		t.Fatalf("phantom index name %q referenced in: %v\n"+
			"No migration creates this index. If you intend to add it, "+
			"land the CREATE INDEX migration first and update this guard.",
			phantom, hits)
	}
}

// findRepoRoot walks up from CWD looking for go.mod. Mirrors the
// helper in errtrace/ast_no_literal_codes_test.go.
func findRepoRoot(t *testing.T) string {
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
	t.Fatalf("no go.mod found above %s", cwd)
	return ""
}
