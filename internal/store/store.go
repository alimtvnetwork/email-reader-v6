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

// migrate applies the schema. Idempotent — safe to call on every startup.
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS Emails (
			Id          INTEGER PRIMARY KEY AUTOINCREMENT,
			Alias       TEXT    NOT NULL,
			MessageId   TEXT    NOT NULL UNIQUE,
			Uid         INTEGER NOT NULL,
			FromAddr    TEXT    NOT NULL DEFAULT '',
			ToAddr      TEXT    NOT NULL DEFAULT '',
			CcAddr      TEXT    NOT NULL DEFAULT '',
			Subject     TEXT    NOT NULL DEFAULT '',
			BodyText    TEXT    NOT NULL DEFAULT '',
			BodyHtml    TEXT    NOT NULL DEFAULT '',
			ReceivedAt  DATETIME,
			FilePath    TEXT    NOT NULL DEFAULT '',
			CreatedAt   DATETIME NOT NULL DEFAULT ` + sqliteRFC3339NowExpr + `
		)`,
		`CREATE INDEX IF NOT EXISTS IxEmailsAliasUid ON Emails(Alias, Uid)`,
		`CREATE TABLE IF NOT EXISTS WatchState (
			Alias          TEXT PRIMARY KEY,
			LastUid        INTEGER NOT NULL DEFAULT 0,
			LastSubject    TEXT    NOT NULL DEFAULT '',
			LastReceivedAt DATETIME,
			UpdatedAt      DATETIME NOT NULL DEFAULT ` + sqliteRFC3339NowExpr + `
		)`,
		`CREATE TABLE IF NOT EXISTS OpenedUrls (
			Id        INTEGER PRIMARY KEY AUTOINCREMENT,
			EmailId   INTEGER NOT NULL,
			RuleName  TEXT    NOT NULL DEFAULT '',
			Url       TEXT    NOT NULL,
			OpenedAt  DATETIME DEFAULT ` + sqliteRFC3339NowExpr + `,
			FOREIGN KEY(EmailId) REFERENCES Emails(Id) ON DELETE CASCADE
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS IxOpenedUrlsUnique ON OpenedUrls(EmailId, Url)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return errtrace.Wrapf(err, "migrate stmt: %s", q)
		}
	}
	return s.migrateOpenedUrlsDelta1()
}

// migrateOpenedUrlsDelta1 adds the six PascalCase audit columns specified
// by Delta #1 (Alias / Origin / OriginalUrl / IsDeduped / IsIncognito /
// TraceId). SQLite's `ALTER TABLE ADD COLUMN` is non-idempotent, so we
// introspect `PRAGMA table_info(OpenedUrls)` and emit each ADD only when
// missing. This keeps fresh DBs and re-migrated existing DBs both happy.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.5 + Delta #1.
func (s *Store) migrateOpenedUrlsDelta1() error {
	have, err := s.openedUrlsColumns()
	if err != nil {
		return errtrace.Wrap(err, "introspect OpenedUrls")
	}
	adds := []struct{ name, ddl string }{
		{"Alias", `ALTER TABLE OpenedUrls ADD COLUMN Alias TEXT NOT NULL DEFAULT ''`},
		{"Origin", `ALTER TABLE OpenedUrls ADD COLUMN Origin TEXT NOT NULL DEFAULT ''`},
		{"OriginalUrl", `ALTER TABLE OpenedUrls ADD COLUMN OriginalUrl TEXT NOT NULL DEFAULT ''`},
		{"IsDeduped", `ALTER TABLE OpenedUrls ADD COLUMN IsDeduped INTEGER NOT NULL DEFAULT 0`},
		{"IsIncognito", `ALTER TABLE OpenedUrls ADD COLUMN IsIncognito INTEGER NOT NULL DEFAULT 0`},
		{"TraceId", `ALTER TABLE OpenedUrls ADD COLUMN TraceId TEXT NOT NULL DEFAULT ''`},
	}
	for _, a := range adds {
		if have[a.name] {
			continue
		}
		if _, err := s.DB.Exec(a.ddl); err != nil {
			return errtrace.Wrapf(err, "add column %s", a.name)
		}
	}
	// Helpful (but non-unique) lookup index for the activated filters.
	if _, err := s.DB.Exec(
		`CREATE INDEX IF NOT EXISTS IxOpenedUrlsAliasOpenedAt ON OpenedUrls(Alias, OpenedAt)`,
	); err != nil {
		return errtrace.Wrap(err, "create IxOpenedUrlsAliasOpenedAt")
	}
	return nil
}

// openedUrlsColumns returns the set of column names currently present on
// the OpenedUrls table. Used by Delta #1 to skip already-applied ADDs.
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
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO Emails
			(Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr, Subject, BodyText, BodyHtml, ReceivedAt, FilePath)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(MessageId) DO NOTHING`,
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
	if err := s.DB.QueryRowContext(ctx,
		`SELECT Id FROM Emails WHERE MessageId = ?`, e.MessageId,
	).Scan(&id); err != nil {
		return 0, false, errtrace.Wrap(err, "select existing email")
	}
	return id, false, nil
}

// GetWatchState returns the last-seen state for the alias (zero value if none).
func (s *Store) GetWatchState(ctx context.Context, alias string) (WatchState, error) {
	var ws WatchState
	var received sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt
		FROM WatchState WHERE Alias = ?`, alias,
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
	const upsertWatchStateQuery = `
		INSERT INTO WatchState (Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt)
		VALUES (?, ?, ?, ?, ` + sqliteRFC3339NowExpr + `)
		ON CONFLICT(Alias) DO UPDATE SET
			LastUid        = excluded.LastUid,
			LastSubject    = excluded.LastSubject,
			LastReceivedAt = excluded.LastReceivedAt,
			UpdatedAt      = ` + sqliteRFC3339NowExpr + `
	`
	_, err := s.DB.ExecContext(ctx, upsertWatchStateQuery,
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
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO OpenedUrls (EmailId, RuleName, Url, Alias, Origin,
		                        OriginalUrl, IsDeduped, IsIncognito, TraceId)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(EmailId, Url) DO NOTHING`,
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
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM OpenedUrls WHERE EmailId = ? AND Url = ?`,
		emailId, url,
	).Scan(&n)
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
	err := s.DB.QueryRowContext(ctx, queries.EmailByUid,
		alias, uid,
	).Scan(&e.Id, &e.Alias, &e.MessageId, &e.Uid, &e.FromAddr, &e.ToAddr,
		&e.CcAddr, &e.Subject, &e.BodyText, &e.BodyHtml, &received, &e.FilePath)
	if err != nil {
		return nil, errtrace.Wrapf(err, "select email alias=%s uid=%d", alias, uid)
	}
	if received.Valid {
		e.ReceivedAt = received.Time
	}
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
