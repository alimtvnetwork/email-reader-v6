// Package core: export.go provides a UI/CLI-agnostic CSV export entry point.
// It opens the store, calls the existing exporter, and returns the absolute
// output path. Both the CLI's `export-csv` and the Fyne UI use this.
//
// Per spec/21-app/04-coding-standards.md §4.2 every exported core API
// returns errtrace.Result[T] (not (T, error)). Lower-level adapters
// (internal/store, internal/exporter) still return raw error and are
// wrapped with a stable error code at this boundary.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/exporter"
	"github.com/lovable/email-read/internal/store"
)

// ExportCSV writes ./data/export-<timestamp>.csv (relative to cwd) and
// returns the absolute path of the produced file inside a Result envelope.
func ExportCSV(ctx context.Context) errtrace.Result[string] {
	if ctx == nil {
		ctx = context.Background()
	}
	st, err := store.Open()
	if err != nil {
		return errtrace.Err[string](
			errtrace.WrapCode(err, errtrace.ErrDbOpen, "open store"),
		)
	}
	defer st.Close()

	path, err := exporter.ExportCSV(ctx, st)
	if err != nil {
		return errtrace.Err[string](
			errtrace.WrapCode(err, errtrace.ErrExportWriteRow, "export csv").
				WithContext("OutPath", path),
		)
	}
	return errtrace.Ok(path)
}
