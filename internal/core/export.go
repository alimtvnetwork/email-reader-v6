// Package core: export.go provides a UI/CLI-agnostic CSV export entry point.
// It opens the store, calls the existing exporter, and returns the absolute
// output path. Both the CLI's `export-csv` and the upcoming Fyne UI use this.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/exporter"
	"github.com/lovable/email-read/internal/store"
)

// ExportCSV writes ./data/export-<timestamp>.csv (relative to cwd) and
// returns the absolute path of the produced file.
func ExportCSV(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	st, err := store.Open()
	if err != nil {
		return "", errtrace.Wrap(err, "open store")
	}
	defer st.Close()

	path, err := exporter.ExportCSV(ctx, st)
	if err != nil {
		return "", errtrace.Wrap(err, "export csv")
	}
	return path, nil
}
