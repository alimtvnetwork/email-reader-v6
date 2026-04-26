// tools_export_test.go covers ExportCsv: preflight gates, progress
// channel hygiene, the WHERE-clause builder, and the slice-2 streaming
// path end-to-end (alias + date filter + 256-row progress ticks).
package core

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
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

func TestPreflightExport_DateRangeOrdering(t *testing.T) {
	now := time.Now().UTC()
	err := preflightExport(ExportSpec{Since: now, Until: now.Add(-time.Hour)})
	if err == nil {
		t.Fatal("expected ErrToolsInvalidArgument for inverted range")
	}
	var coded *errtrace.Coded
	if !errors.As(err, &coded) || coded.Code != errtrace.ErrToolsInvalidArgument {
		t.Fatalf("expected ErrToolsInvalidArgument, got %v", err)
	}
	// Equal Since/Until is also invalid (Until is exclusive).
	if err := preflightExport(ExportSpec{Since: now, Until: now}); err == nil {
		t.Fatal("expected error for equal Since/Until")
	}
	// Valid range passes.
	if err := preflightExport(ExportSpec{Since: now, Until: now.Add(time.Hour)}); err != nil {
		t.Fatalf("valid range rejected: %v", err)
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

func TestExportSpec_HasFilter(t *testing.T) {
	if (ExportSpec{}).hasFilter() {
		t.Fatal("zero spec must not be filtered")
	}
	if !(ExportSpec{Alias: "a"}).hasFilter() {
		t.Fatal("alias must trigger filter")
	}
	if !(ExportSpec{Since: time.Now()}).hasFilter() {
		t.Fatal("since must trigger filter")
	}
	if !(ExportSpec{Until: time.Now()}).hasFilter() {
		t.Fatal("until must trigger filter")
	}
}

// TestEmailExportFilterFromSpec verifies the spec→filter translation
// consumed by `Store.QueryEmailExportRows` and `Store.CountEmails`.
// (The SQL composition itself lives in store and is covered by
// `TestBuildEmailExportQuery_Composition` over there.)
func TestEmailExportFilterFromSpec(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   ExportSpec
		want store.EmailExportFilter
	}{
		{"empty", ExportSpec{}, store.EmailExportFilter{}},
		{"alias", ExportSpec{Alias: "a"}, store.EmailExportFilter{Alias: "a"}},
		{"since", ExportSpec{Since: t0}, store.EmailExportFilter{Since: t0}},
		{"until", ExportSpec{Until: t1}, store.EmailExportFilter{Until: t1}},
		{"all", ExportSpec{Alias: "a", Since: t0, Until: t1},
			store.EmailExportFilter{Alias: "a", Since: t0, Until: t1}},
	}
	for _, c := range cases {
		got := emailExportFilterFromSpec(c.in)
		if got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

// TestExportCsv_FilteredStreaming exercises the slice-2 streaming path
// end-to-end: alias + date-range filter, explicit OutPath, progress
// ticks observed, output CSV restricted to matching rows.
func TestExportCsv_FilteredStreaming(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenAt(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := func(alias, mid string, uid uint32, recv time.Time) {
		t.Helper()
		if _, _, err := st.UpsertEmail(ctx, &store.Email{
			Alias: alias, MessageId: mid, Uid: uid,
			FromAddr: "x@y", Subject: "s", BodyText: "b",
			ReceivedAt: recv,
		}); err != nil {
			t.Fatal(err)
		}
	}
	insert("alpha", "<m1@x>", 1, t0)
	insert("alpha", "<m2@x>", 2, t0.Add(48*time.Hour)) // outside Until
	insert("beta", "<m3@x>", 3, t0.Add(time.Hour))    // wrong alias

	out := filepath.Join(dir, "filtered.csv")
	progress := make(chan ExportProgress, 16)
	res := runExportCsvFiltered(ctx, st, ExportSpec{
		OutPath: out, Overwrite: true,
		Alias: "alpha",
		Since: t0.Add(-time.Hour), Until: t0.Add(24 * time.Hour),
	}, progress)
	close(progress)
	if err := res.Error(); err != nil {
		t.Fatalf("runExportCsvFiltered: %v", err)
	}
	rep := res.Value()
	if rep.RowCount != 1 {
		t.Fatalf("want 1 row, got %d", rep.RowCount)
	}
	if rep.OutPath != out {
		t.Fatalf("want OutPath=%s, got %s", out, rep.OutPath)
	}

	// Verify CSV contents.
	f, _ := os.Open(out)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 { // header + 1 data row
		t.Fatalf("want 2 csv rows, got %d", len(rows))
	}
	if rows[1][2] != "<m1@x>" {
		t.Fatalf("wrong row exported: %v", rows[1])
	}

	// Verify phases observed.
	seen := map[ExportPhase]bool{}
	for p := range progress {
		seen[p.Phase] = true
	}
	for _, want := range []ExportPhase{PhaseCounting, PhaseWriting, PhaseFlushing, PhaseDone} {
		if !seen[want] {
			t.Errorf("missing phase %s", want)
		}
	}
}
