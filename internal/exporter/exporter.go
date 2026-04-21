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
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	name := fmt.Sprintf("export-%s.csv", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write(Columns); err != nil {
		return "", err
	}

	rows, err := st.DB.QueryContext(ctx, `
		SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
		       Subject, BodyText, BodyHtml, ReceivedAt, FilePath, CreatedAt
		FROM Emails ORDER BY Id ASC`)
	if err != nil {
		return "", fmt.Errorf("query emails: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                                                 int64
			uid                                                int64
			alias, msgId, fromA, toA, ccA, subj, bt, bh, fp    string
			received, created                                  any
		)
		if err := rows.Scan(&id, &alias, &msgId, &uid, &fromA, &toA, &ccA,
			&subj, &bt, &bh, &received, &fp, &created); err != nil {
			return "", fmt.Errorf("scan: %w", err)
		}
		if err := w.Write([]string{
			strconv.FormatInt(id, 10), alias, msgId, strconv.FormatInt(uid, 10),
			fromA, toA, ccA, subj, bt, bh,
			fmtAny(received), fp, fmtAny(created),
		}); err != nil {
			return "", err
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return path, nil
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
