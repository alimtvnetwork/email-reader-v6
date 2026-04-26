// tools_export_test.go covers the small synchronous parts of ExportCsv
// — preflight (Overwrite gate) and progress channel close. The full
// integration path (real DB + real exporter) is exercised by the
// existing exporter_test.go in `internal/exporter`.
package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
)

func TestPreflightExport_OverwriteGate(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := preflightExport(ExportSpec{OutPath: existing, Overwrite: false}); err == nil {
		t.Fatal("expected ErrToolsExportPathExists")
	} else {
		var coded *errtrace.Coded
		if !errors.As(err, &coded) || coded.Code != errtrace.ErrToolsExportPathExists {
			t.Fatalf("expected ErrToolsExportPathExists, got %v", err)
		}
	}

	if err := preflightExport(ExportSpec{OutPath: existing, Overwrite: true}); err != nil {
		t.Fatalf("Overwrite=true must pass: %v", err)
	}
	if err := preflightExport(ExportSpec{OutPath: filepath.Join(dir, "fresh.csv")}); err != nil {
		t.Fatalf("non-existing path must pass: %v", err)
	}
	if err := preflightExport(ExportSpec{OutPath: ""}); err != nil {
		t.Fatalf("empty OutPath must pass (exporter chooses): %v", err)
	}
}

func TestSendExport_NonBlockingOnFullChannel(t *testing.T) {
	ch := make(chan ExportProgress, 1)
	ch <- ExportProgress{Phase: PhaseCounting}
	// must not block / panic
	sendExport(ch, ExportProgress{Phase: PhaseWriting})
	sendExport(nil, ExportProgress{Phase: PhaseDone})
}

func TestCloseExportProgress_ToleratesNilAndDoubleClose(t *testing.T) {
	closeExportProgress(nil) // nil tolerated
	ch := make(chan ExportProgress)
	close(ch)
	closeExportProgress(ch) // double-close tolerated via recover
}
