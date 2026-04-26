// Package store wraps the SQLite database used by email-read.
// All column names are intentionally PascalCase as requested.
package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO required

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store/migrate"
	"github.com/lovable/email-read/internal/store/queries"
)

// Store is a thin wrapper around *sql.DB providing typed helpers.
type Store struct {
	DB   *sql.DB
	Path string
}

// Email mirrors a row in the Emails table.
type Email struct {
	Id         int64
	Alias      string
	MessageId  string
	Uid        uint32
	FromAddr   string
	ToAddr     string
	CcAddr     string
	Subject    string
	BodyText   string
	BodyHtml   string
	ReceivedAt time.Time
	FilePath   string
	// IsRead reflects the M0010 `Emails.IsRead` column (1 once the
	// user has opened/viewed the email; 0 otherwise). Surfaced on
	// the Go struct so service-layer filters (e.g. EmailQuery.OnlyUnread
	// in core/emails_query.go) can do `if e.IsRead { continue }` without
	// a second round-trip. Default false matches the DB DEFAULT 0.
	IsRead bool
}

// WatchState mirrors a row in the WatchState table.
type WatchState struct {
	Alias          string
	LastUid        uint32
	LastSubject    string
	LastReceivedAt time.Time
	UpdatedAt      time.Time
}

// Open opens (and migrates) the SQLite DB at data/emails.db next to the EXE.
func Open() (*Store, error) {
	d, err := config.DataDir()
	if err != nil {
		return nil, errtrace.Wrap(err, "data dir")
	}
	return OpenAt(filepath.Join(d, "emails.db"))
}

// OpenAt opens a SQLite DB at an explicit path. Used by tests.
func OpenAt(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, errtrace.Wrap(err, "open sqlite")
	}
	if err := db.Ping(); err != nil {
		return nil, errtrace.Wrap(err, "ping sqlite")
	}
	s := &Store{DB: db, Path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, errtrace.Wrap(err, "migrate")
	}
	return s, nil
}

// Close shuts the DB down.
func (s *Store) Close() error { return s.DB.Close() }

// migrate applies the schema by delegating to the typed migrate
// package (slices P1.10–P1.11). Idempotent — safe to call on every
// startup. Each `m000N_*.go` file under `internal/store/migrate/`
// registers exactly one schema step; `migrate.Apply` records each
// successful step in the `_SchemaVersion` ledger and skips rows that
// are already there.
//
// On first boot after the P1.11 upgrade, existing user DBs have all
// six steps present but no `_SchemaVersion` ledger. The harness
// re-runs every `Up` (which uses `CREATE … IF NOT EXISTS` for DDL or
// PRAGMA-gated introspection for `ALTER TABLE ADD COLUMN` in m0005)
// and back-fills the ledger.
func (s *Store) migrate() error {
	return migrate.Apply(context.Background(), s.DB)
}

// openedUrlsColumns returns the set of column names currently present
// on the OpenedUrls table. Retained as a small introspection helper
// after the schema migration moved into `internal/store/migrate/` —
// `TestOpenedUrlsDelta1_Migration_Idempotent` uses it to lock the
// post-migration column shape without depending on the migrate
// package's internals.
func (s *Store) openedUrlsColumns() (map[string]bool, error) {
	rows, err := s.DB.Query(`PRAGMA table_info(OpenedUrls)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

// UpsertEmail inserts a new email row or returns the existing Id when the
// MessageId is already known. Returns (id, inserted).
func (s *Store) UpsertEmail(ctx context.Context, e *Email) (int64, bool, error) {
	res, err := s.DB.ExecContext(ctx, queries.EmailUpsert,
		e.Alias, e.MessageId, e.Uid, e.FromAddr, e.ToAddr, e.CcAddr,
		e.Subject, e.BodyText, e.BodyHtml, formatRFC3339UTC(e.ReceivedAt), e.FilePath,
	)
	if err != nil {
		return 0, false, errtrace.Wrap(err, "insert email")
	}
	if n, _ := res.RowsAffected(); n > 0 {
		id, _ := res.LastInsertId()
		return id, true, nil
	}
	// Already existed — fetch the existing Id.
	var id int64
	if err := s.DB.QueryRowContext(ctx, queries.EmailIdByMessageId, e.MessageId,
	).Scan(&id); err != nil {
		return 0, false, errtrace.Wrap(err, "select existing email")
	}
	return id, false, nil
}

// GetWatchState returns the last-seen state for the alias (zero value if none).
func (s *Store) GetWatchState(ctx context.Context, alias string) (WatchState, error) {
	var ws WatchState
	var received sql.NullTime
	err := s.DB.QueryRowContext(ctx, queries.WatchStateGet, alias,
	).Scan(&ws.Alias, &ws.LastUid, &ws.LastSubject, &received, &ws.UpdatedAt)
	if err == sql.ErrNoRows {
		return WatchState{Alias: alias}, nil
	}
	if err != nil {
		return WatchState{}, errtrace.Wrap(err, "get watch state")
	}
	if received.Valid {
		ws.LastReceivedAt = received.Time
	}
	return ws, nil
}

// UpsertWatchState writes/updates the alias' last-seen position.
func (s *Store) UpsertWatchState(ctx context.Context, ws WatchState) error {
	_, err := s.DB.ExecContext(ctx, queries.WatchStateUpsert(sqliteRFC3339NowExpr),
		ws.Alias, ws.LastUid, ws.LastSubject, formatRFC3339UTC(ws.LastReceivedAt),
	)
	if err != nil {
		return errtrace.Wrap(err, "upsert watch state")
	}
	return nil
}

// OpenedUrlInsert is the rich payload for `RecordOpenedUrlExt` introduced
// by Delta #1. The legacy `RecordOpenedUrl(emailId, ruleName, url)` call
// path still works (Tools.OpenUrl uses the Ext form; watcher and CLI
// readers stay on the slim form). Empty fields are persisted as defaults.
type OpenedUrlInsert struct {
	EmailId     int64
	RuleName    string
	Url         string // canonical / post-redaction
	Alias       string
	Origin      string // OpenUrlOrigin string value (manual|rule|cli)
	OriginalUrl string // pre-redaction; "" when same as Url
	IsDeduped   bool
	IsIncognito bool
	TraceId     string
}

// RecordOpenedUrl inserts a row into OpenedUrls. Returns true if newly inserted,
// false if (EmailId, Url) already exists (dedup hit). Slim form: leaves the
// Delta-#1 columns at their default values.
func (s *Store) RecordOpenedUrl(ctx context.Context, emailId int64, ruleName, url string) (bool, error) {
	return s.RecordOpenedUrlExt(ctx, OpenedUrlInsert{
		EmailId: emailId, RuleName: ruleName, Url: url,
	})
}

// RecordOpenedUrlExt is the Delta-#1 rich-insert variant. Persists Alias,
// Origin, OriginalUrl, IsDeduped, IsIncognito, and TraceId alongside the
// legacy columns. Returns true on insert, false on (EmailId, Url) conflict.
func (s *Store) RecordOpenedUrlExt(ctx context.Context, in OpenedUrlInsert) (bool, error) {
	res, err := s.DB.ExecContext(ctx, queries.OpenedUrlInsert,
		in.EmailId, in.RuleName, in.Url, in.Alias, in.Origin,
		in.OriginalUrl, boolToInt(in.IsDeduped), boolToInt(in.IsIncognito), in.TraceId,
	)
	if err != nil {
		return false, errtrace.Wrap(err, "record opened url ext")
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// HasOpenedUrl reports whether the (emailId, url) pair has already been opened.
func (s *Store) HasOpenedUrl(ctx context.Context, emailId int64, url string) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, queries.HasOpenedUrl, emailId, url).Scan(&n)
	if err != nil {
		return false, errtrace.Wrap(err, "has opened url")
	}
	return n > 0, nil
}

// GetEmailByUid returns a previously saved email row identified by
// (alias, uid). Returns sql.ErrNoRows-wrapped error if no such row exists.
func (s *Store) GetEmailByUid(ctx context.Context, alias string, uid uint32) (*Email, error) {
	var e Email
	var received sql.NullTime
	var isRead int
	err := s.DB.QueryRowContext(ctx, queries.EmailByUid,
		alias, uid,
	).Scan(&e.Id, &e.Alias, &e.MessageId, &e.Uid, &e.FromAddr, &e.ToAddr,
		&e.CcAddr, &e.Subject, &e.BodyText, &e.BodyHtml, &received, &e.FilePath, &isRead)
	if err != nil {
		return nil, errtrace.Wrapf(err, "select email alias=%s uid=%d", alias, uid)
	}
	if received.Valid {
		e.ReceivedAt = received.Time
	}
	e.IsRead = isRead != 0
	return &e, nil
}

// EmailQuery filters and pages a list of stored emails. All fields are optional.
//   - Alias: when non-empty, restrict to one account.
//   - Search: substring match (LIKE %s%) against Subject + FromAddr (case-insensitive).
//   - Limit: max rows to return; 0 means "no limit". Negative is treated as 0.
//   - Offset: rows to skip; useful for paging.
// Results are ordered by Uid DESC (newest first), tie-broken by Id DESC.
type EmailQuery struct {
	Alias  string
	Search string
	Limit  int
	Offset int
}

// ListEmails returns email rows matching the query. Body fields are populated
// so the UI can render snippets without a second round-trip.
func (s *Store) ListEmails(ctx context.Context, q EmailQuery) ([]Email, error) {
	sqlStr, args := queries.EmailsList(queries.EmailsListInput{
		Alias:  q.Alias,
		Search: q.Search,
		Limit:  q.Limit,
		Offset: q.Offset,
	})
	rows, err := s.DB.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, errtrace.Wrap(err, "list emails")
	}
	defer rows.Close()
	return scanEmailRows(rows)
}

// scanEmailRows materializes the result set from ListEmails into []Email.
func scanEmailRows(rows *sql.Rows) ([]Email, error) {
	var out []Email
	for rows.Next() {
		var e Email
		var received sql.NullTime
		if err := rows.Scan(&e.Id, &e.Alias, &e.MessageId, &e.Uid, &e.FromAddr,
			&e.ToAddr, &e.CcAddr, &e.Subject, &e.BodyText, &e.BodyHtml,
			&received, &e.FilePath); err != nil {
			return nil, errtrace.Wrap(err, "scan email row")
		}
		if received.Valid {
			e.ReceivedAt = received.Time
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, errtrace.Wrap(err, "iterate email rows")
	}
	return out, nil
}

// CountEmails returns the row count matching the alias (or all when empty).
func (s *Store) CountEmails(ctx context.Context, alias string) (int, error) {
	var n int
	var err error
	if alias == "" {
		err = s.DB.QueryRowContext(ctx, queries.EmailsCountAll).Scan(&n)
	} else {
		err = s.DB.QueryRowContext(ctx, queries.EmailsCountByAlias, alias).Scan(&n)
	}
	if err != nil {
		return 0, errtrace.Wrap(err, "count emails")
	}
	return n, nil
}
