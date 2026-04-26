// Package exporter writes the Emails table to a timestamped CSV file
// under ./data/ in the current working directory (NOT next to the EXE).
package exporter

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// Columns is the PascalCase header row written to every export.
var Columns = []string{
	"Id", "Alias", "MessageId", "Uid",
	"FromAddr", "ToAddr", "CcAddr", "Subject",
	"BodyText", "BodyHtml", "ReceivedAt", "FilePath", "CreatedAt",
}

// ExportCSV writes ./data/export-<ts>.csv relative to cwd and returns the path.
func ExportCSV(ctx context.Context, st *store.Store) (string, error) {
	path, err := prepareExportPath()
	if err != nil {
		return "", err
	}
	f, w, err := openCSVWriter(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	defer w.Flush()

	if err := w.Write(Columns); err != nil {
		return "", errtrace.Wrap(err, "write csv header")
	}
	rows, err := st.DB.QueryContext(ctx, exportQuery)
	if err != nil {
		return "", errtrace.Wrap(err, "query emails")
	}
	defer rows.Close()
	if err := writeEmailRows(rows, w); err != nil {
		return "", err
	}
	return path, nil
}

const exportQuery = `
		SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
		       Subject, BodyText, BodyHtml, ReceivedAt, FilePath, CreatedAt
		FROM Emails ORDER BY Id ASC`

// prepareExportPath ensures ./data exists under cwd and returns the timestamped
// CSV file path to write.
func prepareExportPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errtrace.Wrap(err, "getwd")
	}
	dir := filepath.Join(cwd, "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errtrace.Wrapf(err, "mkdir %s", dir)
	}
	name := fmt.Sprintf("export-%s.csv", time.Now().Format("20060102-150405"))
	return filepath.Join(dir, name), nil
}

// openCSVWriter creates the output file and wraps it in a csv.Writer.
func openCSVWriter(path string) (*os.File, *csv.Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, errtrace.Wrapf(err, "create %s", path)
	}
	return f, csv.NewWriter(f), nil
}

// writeEmailRows iterates the email result set, writing each row to the CSV.
func writeEmailRows(rows rowsScanner, w *csv.Writer) error {
	for rows.Next() {
		var (
			id                                              int64
			uid                                             int64
			alias, msgId, fromA, toA, ccA, subj, bt, bh, fp string
			received, created                               any
		)
		if err := rows.Scan(&id, &alias, &msgId, &uid, &fromA, &toA, &ccA,
			&subj, &bt, &bh, &received, &fp, &created); err != nil {
			return errtrace.Wrap(err, "scan row")
		}
		if err := w.Write([]string{
			strconv.FormatInt(id, 10), alias, msgId, strconv.FormatInt(uid, 10),
			fromA, toA, ccA, subj, bt, bh,
			fmtAny(received), fp, fmtAny(created),
		}); err != nil {
			return errtrace.Wrap(err, "write csv row")
		}
	}
	if err := rows.Err(); err != nil {
		return errtrace.Wrap(err, "rows iterate")
	}
	return nil
}

// rowsScanner is the subset of *sql.Rows used by writeEmailRows; defined here
// to keep the helper testable without dragging in database/sql.
type rowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func fmtAny(v any) string {
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
