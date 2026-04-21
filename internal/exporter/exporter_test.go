package exporter

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/store"
)

func TestExportCSV(t *testing.T) {
	tmp := t.TempDir()
	// Run from tmp so ./data/ lands under tmp.
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	st, err := store.OpenAt(filepath.Join(tmp, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	_, _, err = st.UpsertEmail(ctx, &store.Email{
		Alias: "a", MessageId: "<m1@x>", Uid: 1,
		FromAddr: "x@y", Subject: "hi", BodyText: "body",
		ReceivedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := ExportCSV(ctx, st)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(tmp, "data")) {
		t.Fatalf("path not under cwd/data: %s", path)
	}

	f, _ := os.Open(path)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "Id" || rows[1][2] != "<m1@x>" {
		t.Fatalf("unexpected csv content: %v", rows)
	}
}
