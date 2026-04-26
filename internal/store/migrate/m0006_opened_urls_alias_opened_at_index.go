// m0006_opened_urls_alias_opened_at_index.go adds the (Alias,
// OpenedAt) composite index that powers `OpenedUrlsList`'s alias
// filter + `OpenedAt < ?` cursor. Non-unique — multiple URLs can be
// opened for the same alias at the same instant.
//
// Must come after m0005 because the `Alias` column is added there.
package migrate

func init() {
	Register(Migration{
		Version: 6,
		Name:    "opened_urls_alias_opened_at_index",
		Up:      `CREATE INDEX IF NOT EXISTS IxOpenedUrlsAliasOpenedAt ON OpenedUrls(Alias, OpenedAt)`,
	})
}
