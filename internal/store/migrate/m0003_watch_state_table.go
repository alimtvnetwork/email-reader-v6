// m0003_watch_state_table.go creates the per-alias WatchState row used
// by the IMAP watcher to remember the last UID / Subject / ReceivedAt
// it has processed. `Alias` is the natural primary key (one row per
// configured account).
package migrate

func init() {
	Register(Migration{
		Version: 3,
		Name:    "watch_state_table",
		Up: `CREATE TABLE IF NOT EXISTS WatchState (
                Alias          TEXT PRIMARY KEY,
                LastUid        INTEGER NOT NULL DEFAULT 0,
                LastSubject    TEXT    NOT NULL DEFAULT '',
                LastReceivedAt DATETIME,
                UpdatedAt      DATETIME NOT NULL DEFAULT ` + sqliteRFC3339NowExpr + `
              )`,
	})
}
