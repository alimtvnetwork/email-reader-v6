// m0004_opened_urls_table.go creates the OpenedUrls audit table plus
// its (EmailId, Url) uniqueness constraint. The unique index is
// co-located with the table because it's the table's natural-key
// guarantee — `RecordOpenedUrlExt`'s `ON CONFLICT(EmailId, Url) DO
// NOTHING` depends on it for dedupe.
//
// The Delta-#1 audit columns (Alias / Origin / OriginalUrl /
// IsDeduped / IsIncognito / TraceId) are added separately in m0005,
// because they were retro-fitted onto existing user DBs and need
// PRAGMA-gated `ALTER TABLE ADD COLUMN` rather than a fresh CREATE.
package migrate

func init() {
	Register(Migration{
		Version: 4,
		Name:    "opened_urls_table",
		Up: `CREATE TABLE IF NOT EXISTS OpenedUrls (
                Id        INTEGER PRIMARY KEY AUTOINCREMENT,
                EmailId   INTEGER NOT NULL,
                RuleName  TEXT    NOT NULL DEFAULT '',
                Url       TEXT    NOT NULL,
                OpenedAt  DATETIME DEFAULT ` + sqliteRFC3339NowExpr + `,
                FOREIGN KEY(EmailId) REFERENCES Emails(Id) ON DELETE CASCADE
              );
              CREATE UNIQUE INDEX IF NOT EXISTS IxOpenedUrlsUnique ON OpenedUrls(EmailId, Url)`,
	})
}
