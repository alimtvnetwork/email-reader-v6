// tools_export.go is the slim ExportCsv slice: wraps the existing
// `internal/exporter` to produce a phased progress stream (Counting →
// Writing → Flushing → Done) on the caller's channel. Filtering by
// alias / date range is deferred to the next slice — v1 exports the
// whole Emails table, matching the existing CLI behaviour.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.2.
package core

import (
	"context"
	"os"

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

// ExportProgress is one progress tick on the caller-supplied channel.
type ExportProgress struct {
	Phase       ExportPhase
	RowsWritten int
	TotalRows   int
}

// ExportSpec controls ExportCsv. v1 only honours Overwrite — alias /
// date filtering lands with the §2.2 streaming variant.
type ExportSpec struct {
	OutPath   string // empty → exporter chooses ./data/export-<ts>.csv
	Overwrite bool
}

// ExportReport is the terminal result of ExportCsv.
type ExportReport struct {
	OutPath  string
	RowCount int
}

// ExportCsv runs the export, publishing phase events on `progress`. The
// channel is closed exactly once on return. v1 wraps the existing
// `exporter.ExportCSV` whole-table writer; rich filtering lands later.
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
	return runExportCsv(ctx, st, progress)
}

func runExportCsv(ctx context.Context, st *store.Store, progress chan<- ExportProgress) errtrace.Result[ExportReport] {
	total, err := countEmails(ctx, st)
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

func preflightExport(spec ExportSpec) error {
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

func countEmails(ctx context.Context, st *store.Store) (int, error) {
	row := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM Emails`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
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
