package core

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExportCSV runs the core wrapper end-to-end. It uses an isolated cwd so
// the produced ./data/ tree is sandboxed and confirms the file is created
// with the expected header row.
func TestExportCSV(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	path, err := ExportCSV(context.Background())
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(tmp, "data")) {
		t.Fatalf("expected path under %s/data, got %s", tmp, path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) < 1 {
		t.Fatalf("expected at least header row, got 0")
	}
	if rows[0][0] != "Id" {
		t.Fatalf("expected Id header, got %q", rows[0][0])
	}
}
