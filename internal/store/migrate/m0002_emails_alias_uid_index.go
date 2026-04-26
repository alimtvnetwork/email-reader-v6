// m0002_emails_alias_uid_index.go adds the (Alias, Uid) lookup index
// used by `Store.GetEmailByUid` (the watcher's per-poll dedupe path).
//
// Split from m0001 so the index can be dropped or replaced
// independently in a future migration without touching the table DDL.
package migrate

func init() {
	Register(Migration{
		Version: 2,
		Name:    "emails_alias_uid_index",
		Up:      `CREATE INDEX IF NOT EXISTS IxEmailsAliasUid ON Emails(Alias, Uid)`,
	})
}
