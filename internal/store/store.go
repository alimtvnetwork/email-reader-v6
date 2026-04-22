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
			CreatedAt   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS IxEmailsAliasUid ON Emails(Alias, Uid)`,
		`CREATE TABLE IF NOT EXISTS WatchState (
			Alias          TEXT PRIMARY KEY,
			LastUid        INTEGER NOT NULL DEFAULT 0,
			LastSubject    TEXT    NOT NULL DEFAULT '',
			LastReceivedAt DATETIME,
			UpdatedAt      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS OpenedUrls (
			Id        INTEGER PRIMARY KEY AUTOINCREMENT,
			EmailId   INTEGER NOT NULL,
			RuleName  TEXT    NOT NULL DEFAULT '',
			Url       TEXT    NOT NULL,
			OpenedAt  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(EmailId) REFERENCES Emails(Id) ON DELETE CASCADE
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS IxOpenedUrlsUnique ON OpenedUrls(EmailId, Url)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return errtrace.Wrapf(err, "migrate stmt: %s", q)
		}
	}
	return nil
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
		e.Subject, e.BodyText, e.BodyHtml, e.ReceivedAt, e.FilePath,
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
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO WatchState (Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(Alias) DO UPDATE SET
			LastUid        = excluded.LastUid,
			LastSubject    = excluded.LastSubject,
			LastReceivedAt = excluded.LastReceivedAt,
			UpdatedAt      = CURRENT_TIMESTAMP`,
		ws.Alias, ws.LastUid, ws.LastSubject, ws.LastReceivedAt,
	)
	if err != nil {
		return errtrace.Wrap(err, "upsert watch state")
	}
	return nil
}

// RecordOpenedUrl inserts a row into OpenedUrls. Returns true if newly inserted,
// false if (EmailId, Url) already exists (dedup hit).
func (s *Store) RecordOpenedUrl(ctx context.Context, emailId int64, ruleName, url string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO OpenedUrls (EmailId, RuleName, Url)
		VALUES (?, ?, ?)
		ON CONFLICT(EmailId, Url) DO NOTHING`,
		emailId, ruleName, url,
	)
	if err != nil {
		return false, errtrace.Wrap(err, "record opened url")
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
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
	err := s.DB.QueryRowContext(ctx, `
		SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr, Subject,
		       BodyText, BodyHtml, ReceivedAt, FilePath
		FROM Emails
		WHERE Alias = ? AND Uid = ?`,
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
	sqlStr := `SELECT Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr, Subject,
	                  BodyText, BodyHtml, ReceivedAt, FilePath
	           FROM Emails`
	var args []any
	var where []string
	if q.Alias != "" {
		where = append(where, "Alias = ?")
		args = append(args, q.Alias)
	}
	if q.Search != "" {
		where = append(where, "(LOWER(Subject) LIKE ? OR LOWER(FromAddr) LIKE ?)")
		needle := "%" + strings.ToLower(q.Search) + "%"
		args = append(args, needle, needle)
	}
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY Uid DESC, Id DESC"
	if q.Limit > 0 {
		sqlStr += " LIMIT ?"
		args = append(args, q.Limit)
		if q.Offset > 0 {
			sqlStr += " OFFSET ?"
			args = append(args, q.Offset)
		}
	}
	rows, err := s.DB.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, errtrace.Wrap(err, "list emails")
	}
	defer rows.Close()
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
		err = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM Emails`).Scan(&n)
	} else {
		err = s.DB.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM Emails WHERE Alias = ?`, alias).Scan(&n)
	}
	if err != nil {
		return 0, errtrace.Wrap(err, "count emails")
	}
	return n, nil
}
