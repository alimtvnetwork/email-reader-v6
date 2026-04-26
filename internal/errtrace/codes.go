// Package errtrace — codes.go is the single source of truth for error code
// constants used by internal/core and friends. Every constant here MUST
// appear in spec/21-app/06-error-registry.md. Adding a code requires:
//
//  1. A new entry in 06-error-registry.md (block + table row + section).
//  2. A new constant here, in numeric order.
//  3. A note in spec/21-app/99-consistency-report.md if the surrounding
//     block is being expanded.
//
// Codes are intentionally string-typed (not iota) so log output and the
// JSON envelope are stable across reorderings.
package errtrace

const (
	// ER-CFG (Config) — block 21000–21099
	ErrConfigOpen          Code = "ER-CFG-21001"
	ErrConfigDecode        Code = "ER-CFG-21002"
	ErrConfigEncode        Code = "ER-CFG-21003"
	ErrConfigValidate      Code = "ER-CFG-21004"
	ErrConfigPasswordCrypt Code = "ER-CFG-21005"
	ErrConfigAccountDup    Code = "ER-CFG-21006"
	ErrConfigAccountMissing Code = "ER-CFG-21007"

	// ER-DB (Store / SQLite) — block 21100–21199
	ErrDbOpen        Code = "ER-DB-21101"
	ErrDbMigrate     Code = "ER-DB-21102"
	ErrDbInsertEmail Code = "ER-DB-21103"
	ErrDbQueryEmail  Code = "ER-DB-21104"
	ErrDbInsertUrl   Code = "ER-DB-21105"
	ErrDbQueryUrl    Code = "ER-DB-21106"
	ErrDbWatchState  Code = "ER-DB-21107"

	// ER-MAIL (IMAP) — block 21200–21299
	ErrMailDial            Code = "ER-MAIL-21201"
	ErrMailLogin           Code = "ER-MAIL-21202"
	ErrMailFetchUid        Code = "ER-MAIL-21203"
	ErrMailParseEnvelope   Code = "ER-MAIL-21204"
	ErrMailWriteEml        Code = "ER-MAIL-21205"
	ErrMailLogout          Code = "ER-MAIL-21206"
	ErrMailTLSHandshake    Code = "ER-MAIL-21207"
	ErrMailTimeout         Code = "ER-MAIL-21208"
	ErrMailIdleUnsupported Code = "ER-MAIL-21209"

	// ER-RUL (Rules) — block 21300–21399
	ErrRulePatternInvalid Code = "ER-RUL-21301"
	ErrRuleNotFound       Code = "ER-RUL-21302"
	ErrRuleDuplicate      Code = "ER-RUL-21303"
	ErrRuleEvaluate       Code = "ER-RUL-21304"
	ErrRuleSeedDefault    Code = "ER-RUL-21305"

	// ER-WCH (Watcher) — block 21400–21499
	ErrWatcherStart        Code = "ER-WCH-21401"
	ErrWatcherPollCycle    Code = "ER-WCH-21402"
	ErrWatcherProcessEmail Code = "ER-WCH-21403"
	ErrWatcherEventPublish Code = "ER-WCH-21404"
	ErrWatcherShutdown     Code = "ER-WCH-21405"

	// ER-BRW (Browser) — block 21500–21599
	ErrBrowserLaunch        Code = "ER-BRW-21501"
	ErrBrowserNotFound      Code = "ER-BRW-21502"
	ErrBrowserDedupHit      Code = "ER-BRW-21503"
	ErrBrowserUrlInvalid    Code = "ER-BRW-21504"
	ErrBrowserIncognitoFlag Code = "ER-BRW-21505"

	// ER-EXP (Exporter) — block 21600–21699
	ErrExportOpenFile Code = "ER-EXP-21601"
	ErrExportWriteRow Code = "ER-EXP-21602"
	ErrExportFlush    Code = "ER-EXP-21603"
	ErrExportNoData   Code = "ER-EXP-21604"

	// ER-COR (Core / cross-cutting) — block 21700–21799
	ErrCoreInvalidArgument  Code = "ER-COR-21701"
	ErrCoreNotImplemented   Code = "ER-COR-21702"
	ErrCoreContextCancelled Code = "ER-COR-21703"
	ErrCorePathOutsideData  Code = "ER-COR-21704"
	ErrCoreClockSkew        Code = "ER-COR-21705"

	// ER-CLI (Cobra) — block 21800–21899
	ErrCliUsage              Code = "ER-CLI-21801"
	ErrCliFlagConflict       Code = "ER-CLI-21802"
	ErrCliMissingRequiredArg Code = "ER-CLI-21803"
	ErrCliInteractiveAborted Code = "ER-CLI-21804"

	// ER-UI (Fyne) — block 21900–21999 (reserved low end)
	ErrUiThemeUnknownToken Code = "ER-UI-21900"
	ErrUiStateLoad         Code = "ER-UI-21901"
	ErrUiStateSave         Code = "ER-UI-21902"
	ErrUiFormValidation    Code = "ER-UI-21903"
	ErrUiViewRender        Code = "ER-UI-21904"
	ErrUiClipboard         Code = "ER-UI-21905"

	// ER-SET (Settings) — block 21770–21789
	ErrSettingsConstruct        Code = "ER-SET-21770"
	ErrSettingsPollSeconds      Code = "ER-SET-21771"
	ErrSettingsTheme            Code = "ER-SET-21772"
	ErrSettingsUrlScheme        Code = "ER-SET-21773"
	ErrSettingsChromePath       Code = "ER-SET-21774"
	ErrSettingsIncognitoArg     Code = "ER-SET-21775"
	ErrSettingsLocalhostUrls    Code = "ER-SET-21776"
	ErrSettingsCompositeRule    Code = "ER-SET-21777"
	ErrSettingsPersist          Code = "ER-SET-21778"
	ErrSettingsConcurrentEdit   Code = "ER-SET-21779"
	ErrSettingsDetectChromeStat Code = "ER-SET-21780"
	ErrSettingsEventDropped     Code = "ER-SET-21781"

	// Fallback — used by Err[T](nil) defensive path and as last resort.
	ErrUnknown Code = "ER-UNKNOWN-21999"
)
