// tools_export.go is the ExportCsv slice. Slice 1 wrapped the existing
// `internal/exporter` whole-table writer. Slice 2 (this file) adds
// per-alias and date-range filtering with a streaming SELECT and a
// progress tick every `progressTickRows` written rows. The unfiltered
// path still delegates to `exporter.ExportCSV` so the existing CLI
// behaviour and exporter_test.go stay green.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.2.
package core

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/exporter"
	"github.com/lovable/email-read/internal/store"
)

// ExportPhase is the progress lifecycle for ExportCsv.
type ExportPhase string

const (
	PhaseCounting ExportPhase = "counting"
	PhaseWriting  ExportPhase = "writing"
	PhaseFlushing ExportPhase = "flushing"
	PhaseDone     ExportPhase = "done"
)

// progressTickRows is the row stride between PhaseWriting ticks. 256 is
// small enough to keep UIs responsive on cheap laptops yet large enough
// to keep channel send overhead negligible vs. csv.Writer.Write.
const progressTickRows = 256

// ExportProgress is one progress tick on the caller-supplied channel.
type ExportProgress struct {
	Phase       ExportPhase
	RowsWritten int
	TotalRows   int
}

// ExportSpec controls ExportCsv. Slice 2 honours Alias / Since / Until
// in addition to OutPath / Overwrite. Empty Alias means "all aliases";
// zero Since / Until means "no lower / upper bound".
type ExportSpec struct {
	OutPath   string // empty → exporter chooses ./data/export-<ts>.csv
	Overwrite bool
	Alias     string
	Since     time.Time
	Until     time.Time
}

// hasFilter reports whether any slice-2 filter is set. When false we
// fall back to the slice-1 whole-table exporter for byte-for-byte parity
// with the CLI.
func (s ExportSpec) hasFilter() bool {
	return s.Alias != "" || !s.Since.IsZero() || !s.Until.IsZero()
}

// ExportReport is the terminal result of ExportCsv.
type ExportReport struct {
	OutPath  string
	RowCount int
}

// ExportCsv runs the export, publishing phase events on `progress`. The
// channel is closed exactly once on return.
func (t *Tools) ExportCsv(ctx context.Context, spec ExportSpec, progress chan<- ExportProgress) errtrace.Result[ExportReport] {
	defer closeExportProgress(progress)
	if err := preflightExport(spec); err != nil {
		return errtrace.Err[ExportReport](err)
	}
	st, err := openExportStore()
	if err != nil {
		return errtrace.Err[ExportReport](err)
	}
	defer st.Close()
	if spec.hasFilter() {
		return runExportCsvFiltered(ctx, st, spec, progress)
	}
	return runExportCsv(ctx, st, progress)
}

func runExportCsv(ctx context.Context, st *store.Store, progress chan<- ExportProgress) errtrace.Result[ExportReport] {
	total, err := st.CountEmails(ctx, store.EmailExportFilter{})
	if err != nil {
		return errtrace.Err[ExportReport](errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "count emails"))
	}
	sendExport(progress, ExportProgress{Phase: PhaseCounting, TotalRows: total})
	sendExport(progress, ExportProgress{Phase: PhaseWriting, TotalRows: total})
	path, err := exporter.ExportCSV(ctx, st)
	if err != nil {
		return errtrace.Err[ExportReport](errtrace.WrapCode(err, errtrace.ErrExportWriteRow, "ExportCSV"))
	}
	sendExport(progress, ExportProgress{Phase: PhaseFlushing, TotalRows: total, RowsWritten: total})
	sendExport(progress, ExportProgress{Phase: PhaseDone, TotalRows: total, RowsWritten: total})
	return errtrace.Ok(ExportReport{OutPath: path, RowCount: total})
}

// runExportCsvFiltered is the slice-2 streaming path. It owns its own
// COUNT query and its own SELECT so we can emit accurate TotalRows in
// the Counting tick before any I/O happens on the writer.
func runExportCsvFiltered(ctx context.Context, st *store.Store, spec ExportSpec, progress chan<- ExportProgress) errtrace.Result[ExportReport] {
	total, err := st.CountEmails(ctx, emailExportFilterFromSpec(spec))
	if err != nil {
		return errtrace.Err[ExportReport](errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "count emails"))
	}
	sendExport(progress, ExportProgress{Phase: PhaseCounting, TotalRows: total})

	path, err := resolveExportPath(spec)
	if err != nil {
		return errtrace.Err[ExportReport](err)
	}
	written, err := streamFilteredExport(ctx, st, spec, path, total, progress)
	if err != nil {
		return errtrace.Err[ExportReport](errtrace.WrapCode(err, errtrace.ErrExportWriteRow, "stream export"))
	}
	sendExport(progress, ExportProgress{Phase: PhaseFlushing, TotalRows: total, RowsWritten: written})
	sendExport(progress, ExportProgress{Phase: PhaseDone, TotalRows: total, RowsWritten: written})
	return errtrace.Ok(ExportReport{OutPath: path, RowCount: written})
}

// emailExportFilterFromSpec translates the user-facing ExportSpec into
// the primitive store-side filter. Keeps the import direction one-way:
// core → store.
func emailExportFilterFromSpec(spec ExportSpec) store.EmailExportFilter {
	return store.EmailExportFilter{
		Alias: spec.Alias,
		Since: spec.Since,
		Until: spec.Until,
	}
}

// resolveExportPath honours an explicit OutPath (already preflighted) or
// falls back to ./data/export-<ts>.csv when caller didn't pick one.
func resolveExportPath(spec ExportSpec) (string, error) {
	if spec.OutPath != "" {
		if dir := filepath.Dir(spec.OutPath); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "mkdir out dir")
			}
		}
		return spec.OutPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "getwd")
	}
	dir := filepath.Join(cwd, "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "mkdir data")
	}
	return filepath.Join(dir, fmt.Sprintf("export-%s.csv", time.Now().Format("20060102-150405"))), nil
}

// streamFilteredExport opens the file, writes the header + filtered
// rows, and emits a PhaseWriting tick every progressTickRows.
func streamFilteredExport(ctx context.Context, st *store.Store, spec ExportSpec, path string, total int, progress chan<- ExportProgress) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, errtrace.Wrapf(err, "create %s", path)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(exporter.Columns); err != nil {
		return 0, errtrace.Wrap(err, "write csv header")
	}
	rows, err := st.QueryEmailExportRows(ctx, emailExportFilterFromSpec(spec))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return writeFilteredRows(rows, w, total, progress)
}

// writeFilteredRows iterates the result set, formatting each row with
// the same column order as exporter.Columns and emitting a tick every
// progressTickRows. Takes the typed `store.RowsScanner` so this file
// stays free of `database/sql`.
func writeFilteredRows(rows store.RowsScanner, w *csv.Writer, total int, progress chan<- ExportProgress) (int, error) {
	written := 0
	for rows.Next() {
		var (
			id, uid                                         int64
			alias, msgId, fromA, toA, ccA, subj, bt, bh, fp string
			received, created                               any
		)
		if err := rows.Scan(&id, &alias, &msgId, &uid, &fromA, &toA, &ccA,
			&subj, &bt, &bh, &received, &fp, &created); err != nil {
			return written, errtrace.Wrap(err, "scan row")
		}
		if err := w.Write([]string{
			strconv.FormatInt(id, 10), alias, msgId, strconv.FormatInt(uid, 10),
			fromA, toA, ccA, subj, bt, bh,
			fmtExportTime(received), fp, fmtExportTime(created),
		}); err != nil {
			return written, errtrace.Wrap(err, "write csv row")
		}
		written++
		if written%progressTickRows == 0 {
			sendExport(progress, ExportProgress{Phase: PhaseWriting, TotalRows: total, RowsWritten: written})
		}
	}
	if err := rows.Err(); err != nil {
		return written, errtrace.Wrap(err, "rows iterate")
	}
	// Final Writing tick so subscribers always see RowsWritten == total
	// (avoids a stale UI on row counts that aren't multiples of 256).
	sendExport(progress, ExportProgress{Phase: PhaseWriting, TotalRows: total, RowsWritten: written})
	return written, nil
}


func preflightExport(spec ExportSpec) error {
	if !spec.Since.IsZero() && !spec.Until.IsZero() && !spec.Until.After(spec.Since) {
		return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "Until must be after Since")
	}
	if spec.OutPath == "" || spec.Overwrite {
		return nil
	}
	if _, err := os.Stat(spec.OutPath); err == nil {
		return errtrace.NewCoded(errtrace.ErrToolsExportPathExists, "OutPath exists; pass Overwrite=true")
	}
	return nil
}

func openExportStore() (*store.Store, error) {
	st, err := store.Open()
	if err != nil {
		return nil, errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "store.Open")
	}
	return st, nil
}

// fmtExportTime mirrors exporter.fmtAny so filtered exports format
// timestamps identically to the whole-table writer.
func fmtExportTime(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case time.Time:
		return x.UTC().Format(time.RFC3339)
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func sendExport(ch chan<- ExportProgress, p ExportProgress) {
	if ch == nil {
		return
	}
	select {
	case ch <- p:
	default:
	}
}

func closeExportProgress(ch chan<- ExportProgress) {
	defer func() { _ = recover() }()
	if ch != nil {
		close(ch)
	}
}
