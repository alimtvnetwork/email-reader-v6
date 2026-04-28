# 06 — Error Registry

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **complete, exhaustive registry** of every error code emitted by `email-read`. An AI implementing any function MUST pick the error code from this file — never invent a new one inline. Adding a new error = first PR adds a row here, second PR uses the constant.

Every row defines:

| Column | Meaning |
|---|---|
| `Code` | `ER-<LAYER>-NNNNN` — stable, never reused |
| `Const` | Go identifier in `internal/errtrace/codes.go` |
| `Layer` | Owning package |
| `Severity` | `WARN`, `ERROR`, or `FATAL` (drives log routing) |
| `Trigger` | Exact condition that emits this error |
| `Wrap site` | File:func that creates the wrap |
| `Log line` | Exact `Logger`, `Op`, required fields, `Msg` template |
| `User-facing message` | What CLI/UI shows the user (no stack) |
| `Recovery` | What the user must do |
| `Test ref` | Test function name guaranteeing this code is emitted |

---

## Citation Map

| Topic | Source |
|---|---|
| Code range allocation | [04-coding-standards.md §5.4](./04-coding-standards.md) |
| Wrap mechanics | [04-coding-standards.md §5.1–§5.3](./04-coding-standards.md), `internal/errtrace/errtrace.go` |
| Log line shape | [05-logging-strategy.md §6](./05-logging-strategy.md) |
| Error envelope (cross-project standard) | `spec/12-consolidated-guidelines/03-error-management.md` |
| Solved-issue context | `.lovable/solved-issues/01..08` (cited per row where relevant) |

---

## 1. Reserved Code Ranges

| Range | Prefix | Layer | Owner package |
|---|---|---|---|
| `21000–21099` | `ER-CFG` | Configuration | `internal/config` |
| `21100–21199` | `ER-STO` | Storage / SQLite | `internal/store` |
| `21200–21299` | `ER-MAIL` | IMAP / mailclient | `internal/mailclient` |
| `21300–21399` | `ER-RUL` | Rules engine | `internal/rules` |
| `21400–21499` | `ER-WCH` | Watcher loop | `internal/watcher` |
| `21500–21599` | `ER-BRW` | Browser launcher | `internal/browser` |
| `21600–21699` | `ER-EXP` | Exporter (CSV) | `internal/exporter` |
| `21700–21799` | `ER-COR` | Core (cross-cutting) | `internal/core` |
| `21800–21899` | `ER-CLI` | CLI / Cobra | `internal/cli` |
| `21900–21999` | `ER-UI` | Fyne UI | `internal/ui` |
| `21999` | `ER-UNKNOWN` | Fallback (logger uses when no code attached) | (synthetic) |

> **Hard rule:** never assign a code outside its layer's range. Violation = lint failure (`linter-scripts/validate-guidelines.go`).

---

## 2. Code Constants File (Required)

```go
// internal/errtrace/codes.go
package errtrace

type Code string

const (
    // ER-CFG (Configuration)
    ErrConfigLoadFailed       Code = "ER-CFG-21001"
    ErrConfigSaveFailed       Code = "ER-CFG-21002"
    ErrConfigHiddenUnicodePwd Code = "ER-CFG-21003"
    ErrConfigAliasNotFound    Code = "ER-CFG-21004"
    ErrConfigAliasDuplicate   Code = "ER-CFG-21005"
    ErrConfigInvalidJSON      Code = "ER-CFG-21006"
    ErrConfigPasswordDecode   Code = "ER-CFG-21007"
    ErrConfigSchemaMigration  Code = "ER-CFG-21008"

    // ER-DB (Storage / SQLite) — aligned to internal/errtrace/codes.yaml
    // (Slice #196 A1-grow). The impl registry is the operational source of
    // truth: prefix is `ER-DB-*`, consts are `ErrDb*`, block starts at
    // 21101 (not 21100). The historical `ErrStoreSelect` slot is split in
    // the impl into `ErrDbQueryEmail` (21104) + `ErrDbQueryUrl` (21106)
    // so callers can attribute SELECT failures to the right table.
    ErrDbOpen              Code = "ER-DB-21101"
    ErrDbMigrate           Code = "ER-DB-21102"
    ErrDbInsertEmail       Code = "ER-DB-21103"
    ErrDbQueryEmail        Code = "ER-DB-21104"
    ErrDbInsertUrl         Code = "ER-DB-21105"
    ErrDbQueryUrl          Code = "ER-DB-21106"
    ErrDbWatchState        Code = "ER-DB-21107"
    ErrDbUniqueViolation   Code = "ER-DB-21108"
    ErrDbFkViolation       Code = "ER-DB-21109"
    ErrDbBusy              Code = "ER-DB-21110"
    ErrDbReadOnly          Code = "ER-DB-21111"
    ErrDbCorrupt           Code = "ER-DB-21112"

    // ER-MAIL (IMAP) — aligned to internal/errtrace/codes.yaml (Slice #161).
    // The impl registry is the operational source of truth for error codes;
    // log lines and wrap sites use these exact strings.
    ErrMailDial               Code = "ER-MAIL-21201"
    ErrMailLogin              Code = "ER-MAIL-21202"
    ErrMailFetchUid           Code = "ER-MAIL-21203"
    ErrMailParseEnvelope      Code = "ER-MAIL-21204"
    ErrMailWriteEml           Code = "ER-MAIL-21205"
    ErrMailLogout             Code = "ER-MAIL-21206"
    ErrMailTLSHandshake       Code = "ER-MAIL-21207"
    ErrMailTimeout            Code = "ER-MAIL-21208"
    ErrMailIdleUnsupported    Code = "ER-MAIL-21209"
    ErrMailSelectMailbox      Code = "ER-MAIL-21210"

    // ER-RUL (Rules)
    ErrRulePatternInvalid     Code = "ER-RUL-21301"
    ErrRuleNotFound           Code = "ER-RUL-21302"
    ErrRuleDuplicate          Code = "ER-RUL-21303"
    ErrRuleEvaluate           Code = "ER-RUL-21304"
    ErrRuleSeedDefault        Code = "ER-RUL-21305"

    // ER-WCH (Watcher)
    ErrWatcherStart           Code = "ER-WCH-21401"
    ErrWatcherPollCycle       Code = "ER-WCH-21402"
    ErrWatcherProcessEmail    Code = "ER-WCH-21403"
    ErrWatcherEventPublish    Code = "ER-WCH-21404"
    ErrWatcherShutdown        Code = "ER-WCH-21405"

    // ER-BRW (Browser)
    ErrBrowserLaunch          Code = "ER-BRW-21501"
    ErrBrowserNotFound        Code = "ER-BRW-21502"
    ErrBrowserDedupHit        Code = "ER-BRW-21503"
    ErrBrowserUrlInvalid      Code = "ER-BRW-21504"
    ErrBrowserIncognitoFlag   Code = "ER-BRW-21505"

    // ER-EXP (Exporter)
    ErrExportOpenFile         Code = "ER-EXP-21601"
    ErrExportWriteRow         Code = "ER-EXP-21602"
    ErrExportFlush            Code = "ER-EXP-21603"
    ErrExportNoData           Code = "ER-EXP-21604"

    // ER-COR (Core / cross-cutting)
    ErrCoreInvalidArgument    Code = "ER-COR-21701"
    ErrCoreNotImplemented     Code = "ER-COR-21702"
    ErrCoreContextCancelled   Code = "ER-COR-21703"
    ErrCorePathOutsideData    Code = "ER-COR-21704"
    ErrCoreClockSkew          Code = "ER-COR-21705"

    // ER-CLI (Cobra)
    ErrCliUsage               Code = "ER-CLI-21801"
    ErrCliFlagConflict        Code = "ER-CLI-21802"
    ErrCliMissingRequiredArg  Code = "ER-CLI-21803"
    ErrCliInteractiveAborted  Code = "ER-CLI-21804"

    // ER-UI (Fyne)
    ErrUiStateLoad            Code = "ER-UI-21901"
    ErrUiStateSave            Code = "ER-UI-21902"
    ErrUiFormValidation       Code = "ER-UI-21903"
    ErrUiViewRender           Code = "ER-UI-21904"
    ErrUiClipboard            Code = "ER-UI-21905"

    // Fallback
    ErrUnknown                Code = "ER-UNKNOWN-21999"
)
```

---

## 3. ER-CFG — Configuration

### ER-CFG-21001 — `ErrConfigLoadFailed`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | `os.ReadFile(configPath)` returns error OR JSON unmarshal fails before logger init |
| Wrap site | `internal/config/config.go:Load()` |
| Log line | `FATAL config.Load Path=<path> ErrCode=ER-CFG-21001 ErrFrames=[…] config load failed` |
| User msg | `Cannot read config at <path>. Run 'email-read init' to create one.` |
| Recovery | Run `email-read init`, or fix file permissions |
| Test ref | `TestConfig_Load_MissingFileFailsFatal` |

### ER-CFG-21002 — `ErrConfigSaveFailed`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `os.WriteFile` or atomic-rename fails on save |
| Wrap site | `internal/config/config.go:Save()` |
| Log line | `ERROR config.Save Path=<path> ErrCode=ER-CFG-21002 ErrFrames=[…] config save failed` |
| User msg | `Could not save config: <reason>` |
| Recovery | Check disk space + permissions on `data/` directory |
| Test ref | `TestConfig_Save_ReadOnlyDirReturnsError` |

### ER-CFG-21003 — `ErrConfigHiddenUnicodePwd`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Stored Base64 password decodes to a string containing U+200B/U+200C/U+200D/U+2060 (zero-width or word-joiner code points) |
| Wrap site | `internal/config/config.go:DecodePassword()` |
| Log line | `WARN config.DecodePassword Alias=<a> CodePoint=U+2060 ErrCode=ER-CFG-21003 hidden unicode in password — see solved-issues/03` |
| User msg | `Password for '<alias>' contains an invisible character. Re-enter via 'email-read add <alias> --update-password'.` |
| Recovery | Re-enter password without copy-paste from chat tools |
| Test ref | `TestConfig_DecodePassword_DetectsWordJoiner` |
| Origin | `.lovable/solved-issues/03-imap-auth-failed-hidden-unicode.md` |

### ER-CFG-21004 — `ErrConfigAliasNotFound`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Lookup of `Accounts[alias]` returns nothing |
| Wrap site | `internal/config/config.go:GetAccount()` |
| Log line | `ERROR config.GetAccount Alias=<a> ErrCode=ER-CFG-21004 alias not found` |
| User msg | `No account named '<alias>'. Run 'email-read list' to see configured aliases.` |
| Recovery | Add the account: `email-read add <alias>` |
| Test ref | `TestConfig_GetAccount_UnknownAliasErrors` |

### ER-CFG-21005 — `ErrConfigAliasDuplicate`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `add` command receives an alias already present |
| Wrap site | `internal/config/config.go:AddAccount()` |
| Log line | `ERROR config.AddAccount Alias=<a> ErrCode=ER-CFG-21005 alias already exists` |
| User msg | `Alias '<alias>' already exists. Choose a different alias or run 'email-read remove <alias>' first.` |
| Recovery | Choose new alias or remove existing |
| Test ref | `TestConfig_AddAccount_DuplicateAliasErrors` |

### ER-CFG-21006 — `ErrConfigInvalidJSON`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | `json.Unmarshal` of `config.json` returns syntax error |
| Wrap site | `internal/config/config.go:Load()` |
| Log line | `FATAL config.Load Path=<path> ErrCode=ER-CFG-21006 invalid json at line <n>` |
| User msg | `Config file is corrupt JSON at line <n>. Restore from backup or delete to regenerate.` |
| Recovery | Fix JSON or delete file (loses accounts/rules) |
| Test ref | `TestConfig_Load_InvalidJSONFatal` |

### ER-CFG-21007 — `ErrConfigPasswordDecode`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `base64.StdEncoding.DecodeString` of stored password fails |
| Wrap site | `internal/config/config.go:DecodePassword()` |
| Log line | `ERROR config.DecodePassword Alias=<a> ErrCode=ER-CFG-21007 password is not valid base64` |
| User msg | `Stored password for '<alias>' is corrupt. Re-add the account.` |
| Recovery | `email-read remove <alias>` then `email-read add <alias>` |
| Test ref | `TestConfig_DecodePassword_InvalidBase64` |

### ER-CFG-21008 — `ErrConfigSchemaMigration`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | Config file uses an older schema and migration step fails |
| Wrap site | `internal/config/migrate.go:Run()` |
| Log line | `FATAL config.Migrate FromVersion=<n> ToVersion=<m> ErrCode=ER-CFG-21008 schema migration failed` |
| User msg | `Could not upgrade config from v<n> to v<m>. Backup at <path>.bak.` |
| Recovery | Restore backup or delete config |
| Test ref | `TestConfig_Migrate_FailureWritesBackup` |

---

## 4. ER-DB — Storage / SQLite

> **Block ownership note (Slice #196 — A1-grow):** This section's code
> numbers and const names are aligned with `internal/errtrace/codes.yaml`
> (the operational source of truth). The historical `ER-STO-*` /
> `ErrStore*` naming used in earlier drafts has been retired in favour
> of the impl prefix `ER-DB-*` / `ErrDb*` to avoid spec ↔ impl drift.
> Block range remains `21100–21199`; numbering starts at `21101` (not
> `21100`) to match the registry's first slot.
>
> **Slot split note:** the historical `ErrStoreSelect` (single SELECT
> code) has been split in the impl into `ErrDbQueryEmail` (21104) and
> `ErrDbQueryUrl` (21106) so callers can attribute SELECT failures to
> the right table. The two entries below replace the single old entry.

### ER-DB-21101 — `ErrDbOpen`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | `sql.Open("sqlite", path)` or first ping fails |
| Wrap site | `internal/store/store.go:Open()` |
| Log line | `FATAL store.Open Path=<p> ErrCode=ER-DB-21101 ErrFrames=[…] db open failed` |
| User msg | `Cannot open database at <p>: <reason>` |
| Recovery | Check file/dir permissions; ensure `modernc.org/sqlite` build (no CGO) |
| Test ref | `TestStore_Open_NonexistentDirFails` |

### ER-DB-21102 — `ErrDbMigrate`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | A `CREATE TABLE` / `CREATE INDEX` statement during migration returns an error |
| Wrap site | `internal/store/migrate.go:Migrate()` |
| Log line | `FATAL store.Migrate Step=<n> ErrCode=ER-DB-21102 migration step <n> failed` |
| User msg | `Database migration failed at step <n>: <reason>. Backup at <path>.bak.` |
| Recovery | Restore backup; report bug |
| Test ref | `TestStore_Migrate_BadDDLFails` |

### ER-DB-21103 — `ErrDbInsertEmail`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `INSERT INTO Email` returns error other than UNIQUE-violation |
| Wrap site | `internal/store/emails.go:InsertEmail()` |
| Log line | `ERROR store.InsertEmail Alias=<a> Uid=<u> ErrCode=ER-DB-21103 ErrFrames=[…] insert email failed` |
| User msg | `Could not save email uid=<u> for '<alias>'.` |
| Recovery | Inspect error frames; may indicate disk/perm issue |
| Test ref | `TestStore_InsertEmail_PropagatesDBError` |

### ER-DB-21104 — `ErrDbQueryEmail`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | A `SELECT` against `Emails` returns a `Scan` or `Query` error |
| Wrap site | `internal/store/emails.go:Find*()` |
| Log line | `ERROR store.FindEmails ErrCode=ER-DB-21104 ErrFrames=[…] select failed` |
| User msg | `Could not read emails from database.` |
| Recovery | Check db file integrity |
| Test ref | `TestStore_FindEmails_PropagatesScanError` |

### ER-DB-21105 — `ErrDbInsertUrl`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | `INSERT INTO OpenedUrl` fails (other than unique-violation, which is normal dedup) |
| Wrap site | `internal/store/openedurl.go:Record()` |
| Log line | `WARN store.RecordOpenedUrl Alias=<a> Url=<u> ErrCode=ER-DB-21105 record opened-url failed` |
| User msg | (none — internal) |
| Recovery | Will retry on next match |
| Test ref | `TestStore_RecordOpenedUrl_DBErrorWarns` |

### ER-DB-21106 — `ErrDbQueryUrl`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | A `SELECT` against `OpenedUrls` returns a `Scan` or `Query` error |
| Wrap site | `internal/store/openedurl.go:Find*()` |
| Log line | `ERROR store.FindOpenedUrls ErrCode=ER-DB-21106 ErrFrames=[…] select failed` |
| User msg | `Could not read opened-urls from database.` |
| Recovery | Check db file integrity |
| Test ref | `TestStore_FindOpenedUrls_PropagatesScanError` |

### ER-DB-21107 — `ErrDbWatchState`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `UPSERT INTO WatchState` returns error |
| Wrap site | `internal/store/watchstate.go:Update()` |
| Log line | `ERROR store.UpdateWatchState Alias=<a> LastUid=<u> ErrCode=ER-DB-21107 update watch state failed` |
| User msg | `Could not save watch state for '<alias>'. Next run may re-fetch <n> messages.` |
| Recovery | Watcher will resume from previous LastUid |
| Test ref | `TestStore_UpdateWatchState_DBErrorReturns` |

### ER-DB-21108 — `ErrDbUniqueViolation`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | SQLite returns `SQLITE_CONSTRAINT_UNIQUE` |
| Wrap site | `store.checkSqliteErr()` helper *(reserved — Slice #196 added the const; not yet wired at any call site)* |
| Log line | `WARN store.<Op> Table=<t> Constraint=UNIQUE ErrCode=ER-DB-21108 unique constraint hit` |
| User msg | (none — usually benign dedup) |
| Recovery | Skip insert (caller decides) |
| Test ref | *(future)* `TestStore_DuplicateInsertReportsUnique` |

### ER-DB-21109 — `ErrDbFkViolation`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `SQLITE_CONSTRAINT_FOREIGNKEY` returned |
| Wrap site | `store.checkSqliteErr()` *(reserved — Slice #196)* |
| Log line | `ERROR store.<Op> Table=<t> Constraint=FK ErrCode=ER-DB-21109 fk constraint hit` |
| User msg | `Internal data inconsistency in <table>.` |
| Recovery | Report bug; usually a missing parent row |
| Test ref | *(future)* `TestStore_OrphanInsertReportsFK` |

### ER-DB-21110 — `ErrDbBusy`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | `SQLITE_BUSY` after `busy_timeout` exhausted |
| Wrap site | `store.checkSqliteErr()` *(reserved — Slice #196)* |
| Log line | `WARN store.<Op> ErrCode=ER-DB-21110 db busy after <ms>ms` |
| User msg | `Database is busy; retrying.` |
| Recovery | Caller retries with backoff |
| Test ref | *(future)* `TestStore_BusyAfterTimeoutWraps` |

### ER-DB-21111 — `ErrDbReadOnly`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | `SQLITE_READONLY` (filesystem read-only or WAL not writable) |
| Wrap site | `store.checkSqliteErr()` *(reserved — Slice #196)* |
| Log line | `FATAL store.<Op> Path=<p> ErrCode=ER-DB-21111 db is read-only` |
| User msg | `Database file is read-only at <p>. Check permissions.` |
| Recovery | Fix permissions |
| Test ref | *(future)* `TestStore_ReadOnlyDBFailsFatal` |

### ER-DB-21112 — `ErrDbCorrupt`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | `SQLITE_CORRUPT` returned |
| Wrap site | `store.checkSqliteErr()` *(reserved — Slice #196)* |
| Log line | `FATAL store.<Op> Path=<p> ErrCode=ER-DB-21112 db file corrupt` |
| User msg | `Database is corrupt at <p>. Restore from backup.` |
| Recovery | Restore backup or delete + re-fetch |
| Test ref | *(future)* `TestStore_CorruptFileFailsFatal` |

---

## 5. ER-MAIL — IMAP / Mailclient

> **Block ownership note (Slice #161):** This section's code numbers and
> const names are aligned with `internal/errtrace/codes.yaml` (the
> operational source of truth). Earlier revisions of this spec used an
> off-by-one numbering (`21200..21209`) and the names `ErrMailLoginFailed`
> + `ErrMailSelectMailbox` at slot 21202. Both have been corrected:
> the impl never emitted those strings, so log archives, wrap sites,
> and tests are unaffected.

### ER-MAIL-21201 — `ErrMailDial`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `tls.Dial(host:port)` fails (network, DNS, refused) |
| Wrap site | `internal/mailclient/mailclient.go:Dial()` |
| Log line | `ERROR mailclient.Dial Alias=<a> Host=<h> Port=<p> ErrCode=ER-MAIL-21201 ErrFrames=[…] imap dial failed` |
| User msg | `Cannot reach <host>:<port>. Check network or IMAP host.` |
| Recovery | Run `email-read doctor <alias>` to diagnose |
| Test ref | `TestMailclient_Dial_RefusedReturnsErr` |

### ER-MAIL-21202 — `ErrMailLogin`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | IMAP server returns `AUTHENTICATIONFAILED` (or `BAD`) on `LOGIN`/`AUTHENTICATE` |
| Wrap site | `internal/mailclient/mailclient.go:Login()` (impl: `internal/mailclient/dial_plain.go`) |
| Log line | `ERROR mailclient.Login Alias=<a> User=<u> ErrCode=ER-MAIL-21202 ErrFrames=[…] imap login failed` |
| User msg | `Login failed for '<alias>'. Run 'email-read doctor <alias>' to check for hidden unicode in password.` |
| Recovery | Doctor command; verify app password; see `solved-issues/01` + `03` |
| Test ref | `TestMailclient_Login_AuthFailedReturnsErr` |
| Origin | `.lovable/solved-issues/01-imap-auth-failed-wrong-password.md`, `03-imap-auth-failed-hidden-unicode.md` |

### ER-MAIL-21203 — `ErrMailFetchUid`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `UID FETCH` returns error mid-stream |
| Wrap site | `mailclient.go:FetchSince()` |
| Log line | `ERROR mailclient.FetchSince Alias=<a> UidFrom=<u> ErrCode=ER-MAIL-21203 fetch uids failed` |
| User msg | `Could not fetch new messages for '<alias>'.` |
| Recovery | Watcher backs off and retries (`ER-WCH-21402` follow-up) |
| Test ref | `TestMailclient_FetchSince_StreamErrorWraps` |

### ER-MAIL-21204 — `ErrMailParseEnvelope`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | RFC822 envelope parse fails for one message |
| Wrap site | `mailclient.go:parseEnvelope()` |
| Log line | `WARN mailclient.parseEnvelope Alias=<a> Uid=<u> ErrCode=ER-MAIL-21204 envelope parse failed — skipping` |
| User msg | (none — single message skipped) |
| Recovery | Message skipped; .eml still saved raw |
| Test ref | `TestMailclient_ParseEnvelope_MalformedSkips` |

### ER-MAIL-21205 — `ErrMailWriteEml`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `os.WriteFile(<eml-path>, body, 0644)` fails |
| Wrap site | `mailclient.go:saveEml()` |
| Log line | `ERROR mailclient.saveEml Alias=<a> Uid=<u> Path=<p> ErrCode=ER-MAIL-21205 write .eml failed` |
| User msg | `Could not save raw email file: <reason>` |
| Recovery | Check disk space + permissions |
| Test ref | `TestMailclient_SaveEml_PermissionErrorWraps` |

### ER-MAIL-21206 — `ErrMailLogout`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | `LOGOUT` returns error during shutdown |
| Wrap site | `mailclient.go:Close()` |
| Log line | `WARN mailclient.Close Alias=<a> ErrCode=ER-MAIL-21206 imap logout failed` |
| User msg | (none) |
| Recovery | Connection dropped anyway |
| Test ref | `TestMailclient_Close_LogoutFailureWarns` |

### ER-MAIL-21207 — `ErrMailTLSHandshake`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | TLS handshake fails (cert invalid, protocol mismatch) |
| Wrap site | `mailclient.go:Dial()` |
| Log line | `ERROR mailclient.Dial Alias=<a> Host=<h> ErrCode=ER-MAIL-21207 tls handshake failed` |
| User msg | `TLS handshake failed for <host>. Server may use STARTTLS or self-signed cert.` |
| Recovery | Verify port (993 vs 143 + STARTTLS); doctor command |
| Test ref | `TestMailclient_Dial_TLSMismatchWraps` |

### ER-MAIL-21208 — `ErrMailTimeout`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Operation exceeds `WatchTimeoutSeconds` |
| Wrap site | `mailclient.go:withTimeout()` |
| Log line | `ERROR mailclient.<Op> Alias=<a> TimeoutMs=<n> ErrCode=ER-MAIL-21208 imap operation timed out` |
| User msg | `IMAP timed out for '<alias>'.` |
| Recovery | Increase `WatchTimeoutSeconds` in config |
| Test ref | `TestMailclient_Timeout_ContextDeadlineWraps` |

### ER-MAIL-21209 — `ErrMailIdleUnsupported`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Server `CAPABILITY` lacks `IDLE` and IDLE was requested |
| Wrap site | `mailclient.go:StartIdle()` |
| Log line | `WARN mailclient.StartIdle Alias=<a> ErrCode=ER-MAIL-21209 server does not support IDLE — falling back to polling` |
| User msg | (info-level toast in UI) |
| Recovery | Polling continues |
| Test ref | `TestMailclient_StartIdle_NoCapabilityWarns` |

### ER-MAIL-21210 — `ErrMailSelectMailbox`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `SELECT INBOX` (or other mailbox) returns IMAP `NO`/`BAD` after a successful login |
| Wrap site | `internal/watcher/watcher.go:connectAndSelect` (current) / `mailclient.SelectMailbox()` |
| Log line | `ERROR mailclient.SelectMailbox Alias=<a> Mailbox=<m> ErrCode=ER-MAIL-21210 select mailbox failed` |
| User msg | `Cannot open mailbox '<m>' for '<alias>'.` |
| Recovery | Verify mailbox name (case-sensitive on some servers) |
| Test ref | `TestMailclient_SelectMailbox_BadResponseErrors` |
| Note | Slot moved from 21202 to 21210 in Slice #161 to match impl. |

---

## 6. ER-RUL — Rules Engine

### ER-RUL-21301 — `ErrRulePatternInvalid`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `regexp.Compile(rule.Pattern)` returns error |
| Wrap site | `internal/rules/rules.go:Compile()` |
| Log line | `ERROR rules.Compile RuleId=<r> Pattern=<p> ErrCode=ER-RUL-21301 ErrFrames=[…] rule regex invalid` |
| User msg | `Rule '<name>' has invalid regex: <reason>` |
| Recovery | Edit rule pattern in UI Rules view |
| Test ref | `TestRules_Compile_InvalidRegexErrors` |

### ER-RUL-21302 — `ErrRuleNotFound`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Lookup of `RuleId` returns nothing |
| Wrap site | `rules.go:GetById()` |
| Log line | `ERROR rules.GetById RuleId=<r> ErrCode=ER-RUL-21302 rule not found` |
| User msg | `Rule #<id> does not exist.` |
| Recovery | Refresh rules list |
| Test ref | `TestRules_GetById_UnknownErrors` |

### ER-RUL-21303 — `ErrRuleDuplicate`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `Add` called with name already present |
| Wrap site | `rules.go:Add()` |
| Log line | `ERROR rules.Add Name=<n> ErrCode=ER-RUL-21303 rule name already exists` |
| User msg | `A rule named '<n>' already exists.` |
| Recovery | Choose different name |
| Test ref | `TestRules_Add_DuplicateNameErrors` |

### ER-RUL-21304 — `ErrRuleEvaluate`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Rule evaluation panics or compiled-regex match returns unexpected error |
| Wrap site | `rules.go:Evaluate()` |
| Log line | `WARN rules.Evaluate RuleId=<r> EmailId=<e> ErrCode=ER-RUL-21304 rule evaluation failed — skipping` |
| User msg | (none) |
| Recovery | Rule skipped for this email |
| Test ref | `TestRules_Evaluate_PanicRecoveredWarns` |

### ER-RUL-21305 — `ErrRuleSeedDefault`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | First-run seed of default rules fails to insert |
| Wrap site | `rules.go:SeedDefaults()` |
| Log line | `WARN rules.SeedDefaults Count=<n> ErrCode=ER-RUL-21305 default rule seed failed` |
| User msg | (none — user can add rules manually) |
| Recovery | Add rules via UI |
| Test ref | `TestRules_SeedDefaults_DBErrorWarns` |
| Origin | `.lovable/solved-issues/07-zero-rules-default-seed.md` |

---

## 7. ER-WCH — Watcher

### ER-WCH-21401 — `ErrWatcherStart`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `watcher.Start(alias)` cannot create initial connection (first dial+login fails) |
| Wrap site | `internal/watcher/watcher.go:Start()` |
| Log line | `ERROR watcher.Start Alias=<a> ErrCode=ER-WCH-21401 ErrFrames=[…] watcher start failed` |
| User msg | `Could not start watching '<alias>': <reason>` |
| Recovery | Diagnose with `email-read doctor <alias>` |
| Test ref | `TestWatcher_Start_DialFailureReturns` |

### ER-WCH-21402 — `ErrWatcherPollCycle`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | A poll cycle returns any unrecoverable error (after retries) |
| Wrap site | `watcher.go:pollOnce()` |
| Log line | `ERROR watcher.pollOnce Alias=<a> Cycle=<n> ErrCode=ER-WCH-21402 ErrFrames=[…] poll cycle failed` |
| User msg | `Watch cycle failed for '<alias>'; backing off <s>s.` |
| Recovery | Watcher applies exponential backoff and retries |
| Test ref | `TestWatcher_PollOnce_CycleErrorTriggersBackoff` |

### ER-WCH-21403 — `ErrWatcherProcessEmail`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Per-email processing (rules eval + browser open + persist) returns error |
| Wrap site | `watcher.go:processEmail()` |
| Log line | `ERROR watcher.processEmail Alias=<a> Uid=<u> ErrCode=ER-WCH-21403 ErrFrames=[…] process email failed` |
| User msg | `Failed to process uid=<u>; will retry next cycle.` |
| Recovery | Skip; `LastUid` not advanced past this UID |
| Test ref | `TestWatcher_ProcessEmail_StoreFailureWraps` |

### ER-WCH-21404 — `ErrWatcherEventPublish`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Event-bus channel send blocks past `EventPublishTimeoutMs` |
| Wrap site | `internal/watcher/events.go:Publish()` |
| Log line | `WARN watcher.Publish EventType=<t> Alias=<a> ErrCode=ER-WCH-21404 event publish dropped — slow subscriber` |
| User msg | (none) |
| Recovery | Subscriber missed event; not fatal |
| Test ref | `TestWatcher_Events_SlowSubscriberDropsAfterTimeout` |

### ER-WCH-21405 — `ErrWatcherShutdown`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Shutdown cleanup (close mailclient, flush events) fails |
| Wrap site | `watcher.go:Stop()` |
| Log line | `WARN watcher.Stop Alias=<a> ErrCode=ER-WCH-21405 shutdown cleanup failed` |
| User msg | (none) |
| Recovery | Force-exit acceptable |
| Test ref | `TestWatcher_Stop_PartialCleanupWarns` |

### ER-WCH-21412 — `ErrWatchAccountNotFound`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `watch <alias>` invoked with an alias not present in `config.json` |
| Wrap site | `internal/watcher/watcher.go` (`Start` lookup) |
| Log line | `ERROR watcher.Start Alias=<a> ErrCode=ER-WCH-21412 account not found` |
| User msg | `No account found with alias '<alias>'. Run 'email-read accounts' to list configured aliases.` |
| Recovery | User adds the account or corrects the alias |
| Test ref | `TestWatcher_Start_UnknownAliasReturnsCode` |

---

## 8. ER-BRW — Browser Launcher

### ER-BRW-21501 — `ErrBrowserLaunch`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `exec.Command(chromePath, "--incognito", url).Start()` fails |
| Wrap site | `internal/browser/browser.go:Open()` |
| Log line | `ERROR browser.Open Alias=<a> Url=<u> ChromePath=<p> ErrCode=ER-BRW-21501 ErrFrames=[…] browser launch failed` |
| User msg | `Could not launch browser: <reason>` |
| Recovery | Verify Chrome path in Settings |
| Test ref | `TestBrowser_Open_BadPathErrors` |
| Origin | `.lovable/solved-issues/05-rule-match-silent-no-browser-open.md` |

### ER-BRW-21502 — `ErrBrowserNotFound`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Auto-detect of Chrome path returns no match across all known locations |
| Wrap site | `browser.go:DetectChrome()` |
| Log line | `ERROR browser.DetectChrome SearchedPaths=[…] ErrCode=ER-BRW-21502 chrome not found` |
| User msg | `Chrome not found. Set 'ChromePath' in Settings or install Google Chrome.` |
| Recovery | Set explicit path in Settings view |
| Test ref | `TestBrowser_DetectChrome_NoneFoundErrors` |

### ER-BRW-21503 — `ErrBrowserDedupHit`

| Field | Value |
|---|---|
| Severity | `WARN` (logged, not surfaced) |
| Trigger | `OpenedUrl` table already has this URL for this alias within dedup window |
| Wrap site | `browser.go:Open()` (precheck) |
| Log line | `WARN browser.Open Alias=<a> Url=<u> OpenedAt=<ts> ErrCode=ER-BRW-21503 url already opened — skipping` |
| User msg | (none) |
| Recovery | None — intentional skip |
| Test ref | `TestBrowser_Open_DedupHitSkips` |

### ER-BRW-21504 — `ErrBrowserUrlInvalid`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | URL fails `url.Parse` or scheme not in `{http, https}` |
| Wrap site | `browser.go:Open()` |
| Log line | `WARN browser.Open Alias=<a> Url=<u> ErrCode=ER-BRW-21504 url invalid — skipping` |
| User msg | (none) |
| Recovery | Fix rule pattern to capture only valid URLs |
| Test ref | `TestBrowser_Open_NonHttpUrlSkips` |

### ER-BRW-21505 — `ErrBrowserIncognitoFlag`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Incognito flag override invalid (e.g., contains shell metacharacters) |
| Wrap site | `browser.go:buildArgs()` |
| Log line | `WARN browser.buildArgs Flag=<f> ErrCode=ER-BRW-21505 incognito flag rejected — using default` |
| User msg | (none) |
| Recovery | Edit Settings to fix |
| Test ref | `TestBrowser_BuildArgs_RejectsShellMetachars` |

---

## 9. ER-EXP — Exporter

### ER-EXP-21601 — `ErrExportOpenFile`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `os.Create(outPath)` fails |
| Wrap site | `internal/exporter/exporter.go:WriteCSV()` |
| Log line | `ERROR exporter.WriteCSV OutPath=<p> ErrCode=ER-EXP-21601 ErrFrames=[…] export open file failed` |
| User msg | `Cannot create export file at <p>.` |
| Recovery | Check destination dir exists + writable |
| Test ref | `TestExporter_WriteCSV_BadPathErrors` |

### ER-EXP-21602 — `ErrExportWriteRow`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | `csv.Writer.Write` returns error mid-stream |
| Wrap site | `exporter.go:WriteCSV()` |
| Log line | `ERROR exporter.WriteCSV OutPath=<p> RowsWritten=<n> ErrCode=ER-EXP-21602 csv write failed` |
| User msg | `Export aborted at row <n>.` |
| Recovery | Disk full or permission change mid-write |
| Test ref | `TestExporter_WriteCSV_DiskFullPropagates` |

### ER-EXP-21603 — `ErrExportFlush`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Final `csv.Writer.Flush` returns error |
| Wrap site | `exporter.go:WriteCSV()` |
| Log line | `ERROR exporter.WriteCSV OutPath=<p> ErrCode=ER-EXP-21603 csv flush failed` |
| User msg | `Export not finalized: <reason>.` |
| Recovery | Re-run export |
| Test ref | `TestExporter_WriteCSV_FlushFailureWraps` |

### ER-EXP-21604 — `ErrExportNoData`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Query returned zero rows |
| Wrap site | `exporter.go:WriteCSV()` |
| Log line | `WARN exporter.WriteCSV Alias=<a> ErrCode=ER-EXP-21604 no rows to export` |
| User msg | `No emails to export for '<alias>'.` |
| Recovery | None |
| Test ref | `TestExporter_WriteCSV_EmptyDatasetWarns` |

---

## 10. ER-COR — Core (Cross-Cutting)

### ER-COR-21701 — `ErrCoreInvalidArgument`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Public `core.*` function called with empty/invalid required arg |
| Wrap site | `internal/core/<file>.go:<Func>()` |
| Log line | `ERROR core.<Func> Arg=<name> Value=<v> ErrCode=ER-COR-21701 invalid argument` |
| User msg | `Invalid value for '<arg>': <reason>` |
| Recovery | Caller fixes arg |
| Test ref | `TestCore_<Func>_EmptyAliasErrors` |

### ER-COR-21702 — `ErrCoreNotImplemented`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Stub function called before feature shipped |
| Wrap site | per stub |
| Log line | `ERROR core.<Func> ErrCode=ER-COR-21702 not implemented` |
| User msg | `Feature not yet available.` |
| Recovery | Wait for release |
| Test ref | (n/a — temp) |

### ER-COR-21703 — `ErrCoreContextCancelled`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | `ctx.Err() == context.Canceled` during long op |
| Wrap site | core long-running ops |
| Log line | `WARN core.<Func> Alias=<a> ErrCode=ER-COR-21703 cancelled by caller` |
| User msg | (none — usually Ctrl+C or UI navigation) |
| Recovery | None |
| Test ref | `TestCore_<Func>_RespectsContextCancel` |

### ER-COR-21704 — `ErrCorePathOutsideData`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | A path arg resolves outside `data/` directory (path traversal guard) |
| Wrap site | `internal/core/paths.go:Validate()` |
| Log line | `ERROR core.paths.Validate Path=<p> ErrCode=ER-COR-21704 path escapes data dir` |
| User msg | `Path is outside the data directory.` |
| Recovery | Use a relative path under `data/` |
| Test ref | `TestCorePaths_Validate_RejectsTraversal` |

### ER-COR-21705 — `ErrCoreClockSkew`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | System clock differs from IMAP server's `Date` header by > 5 minutes |
| Wrap site | `internal/core/emails.go:detectClockSkew()` |
| Log line | `WARN core.detectClockSkew DeltaSeconds=<n> ErrCode=ER-COR-21705 system clock skew detected` |
| User msg | `Your system clock is off by <n> minutes — this may affect timestamps.` |
| Recovery | Sync NTP |
| Test ref | `TestCoreEmails_DetectClockSkew_LargeDeltaWarns` |

---

## 11. ER-CLI — Cobra

### ER-CLI-21801 — `ErrCliUsage`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Command invoked with unknown subcommand or missing required positional |
| Wrap site | `internal/cli/cli.go:NewRoot()` (`SilenceUsage=false`) |
| Log line | `WARN cli.<Cmd> ErrCode=ER-CLI-21801 usage error: <detail>` |
| User msg | Cobra-rendered usage text |
| Recovery | Run `email-read help` |
| Exit code | `2` |
| Test ref | `TestCLI_UnknownSubcommandExits2` |

### ER-CLI-21802 — `ErrCliFlagConflict`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Two mutually exclusive flags set (e.g. `--quiet --verbose`) |
| Wrap site | `cli/<cmd>.go:preRun()` |
| Log line | `ERROR cli.<Cmd> Flags=[--quiet,--verbose] ErrCode=ER-CLI-21802 mutually exclusive flags` |
| User msg | `--quiet and --verbose are mutually exclusive.` |
| Exit code | `2` |
| Test ref | `TestCLI_QuietAndVerboseConflict` |

### ER-CLI-21803 — `ErrCliMissingRequiredArg`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Required positional missing |
| Wrap site | `cli/<cmd>.go:Args()` |
| Log line | `WARN cli.<Cmd> MissingArg=<name> ErrCode=ER-CLI-21803 missing required argument: <name>` |
| User msg | `Missing required argument: <name>` |
| Exit code | `2` |
| Test ref | `TestCLI_MissingAliasArgErrors` |

### ER-CLI-21804 — `ErrCliInteractiveAborted`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | User Ctrl+C during a Survey prompt |
| Wrap site | `cli/add.go:promptForFields()` |
| Log line | `WARN cli.add ErrCode=ER-CLI-21804 prompt aborted by user` |
| User msg | `Aborted.` |
| Exit code | `130` (SIGINT convention) |
| Test ref | `TestCLI_AddPromptAbortExits130` |

---

## 12. ER-UI — Fyne

### ER-UI-21900 — `ErrUiThemeUnknownToken`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | A theme code requested a semantic token name that doesn't exist in the active palette |
| Wrap site | `internal/ui/theme/theme.go` (token resolver) |
| Log line | `WARN ui.theme.Resolve Token=<t> Palette=<p> ErrCode=ER-UI-21900 unknown token` |
| User msg | (none — falls back to default token; logged for diagnostics) |
| Recovery | Theme falls back to default; user/dev fixes the token name |
| Test ref | `TestTheme_UnknownTokenWarns` |

### ER-UI-21901 — `ErrUiStateLoad`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | `data/ui-state.json` exists but unmarshal fails |
| Wrap site | `internal/ui/state.go:Load()` |
| Log line | `WARN ui.state.Load Path=<p> ErrCode=ER-UI-21901 ui state load failed — using defaults` |
| User msg | (none — silent fallback) |
| Recovery | Defaults applied |
| Test ref | `TestUiState_Load_CorruptFileFallsBackToDefaults` |

### ER-UI-21902 — `ErrUiStateSave`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Save of `ui-state.json` fails |
| Wrap site | `state.go:Save()` |
| Log line | `WARN ui.state.Save Path=<p> ErrCode=ER-UI-21902 ui state save failed` |
| User msg | (none) |
| Recovery | Window state may not persist |
| Test ref | `TestUiState_Save_ReadOnlyDirWarns` |

### ER-UI-21903 — `ErrUiFormValidation`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Form submit blocked by failing validator |
| Wrap site | `internal/ui/views/<form>.go:onSubmit()` |
| Log line | `WARN ui.<Form>.onSubmit Field=<f> Reason=<r> ErrCode=ER-UI-21903 validation failed` |
| User msg | Inline field-level red message (`text-destructive`) per `16-app-design-system §Form Validation States` |
| Recovery | User corrects field |
| Test ref | `TestUi<Form>_Submit_InvalidFieldShowsError` |

### ER-UI-21904 — `ErrUiViewRender`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | View constructor returns error (data fetch fails) |
| Wrap site | `internal/ui/views/<view>.go:New<View>()` |
| Log line | `ERROR ui.<View>.New Alias=<a> ErrCode=ER-UI-21904 ErrFrames=[…] view render failed` |
| User msg | Inline error card with retry button |
| Recovery | Retry button re-runs constructor |
| Test ref | `TestUi<View>_New_DataFailureShowsErrorCard` |

### ER-UI-21905 — `ErrUiClipboard`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Clipboard write returns error (no display server, etc.) |
| Wrap site | `internal/ui/views/util.go:CopyToClipboard()` |
| Log line | `WARN ui.util.CopyToClipboard ErrCode=ER-UI-21905 clipboard write failed` |
| User msg | Toast: `Could not copy to clipboard.` |
| Recovery | None |
| Test ref | `TestUiUtil_Copy_NoClipboardFailsGracefully` |

---

## 13. ER-SET — Settings

> **Block ownership note (Slice #197):** Codes 21770–21783 are owned by
> `internal/core/settings.go` and friends. The 10 entries below cover
> the pre-Density set referenced across spec/21-app/02-features/07-settings/.
> `ER-SET-21780..21783` (DetectChromeStat, EventDropped, RetentionDays,
> Density) exist in the impl registry; their per-code tables are deferred
> to a future doc-completeness slice (no spec references currently surface
> them via `Test_AllErrorRefsResolveInRegistry`).

### ER-SET-21770 — `ErrSettingsConstruct`

| Field | Value |
|---|---|
| Severity | `FATAL` |
| Trigger | `NewSettingsService` construction failed (missing dependency / nil store) |
| Wrap site | `internal/core/settings.go:NewSettingsService()` |
| Log line | `FATAL settings.Construct ErrCode=ER-SET-21770 settings service construction failed` |
| User msg | `Cannot start settings service: <reason>.` |
| Recovery | App startup aborts; report bug |
| Test ref | `TestSettings_Construct_NilStoreFatals` |

### ER-SET-21771 — `ErrSettingsPollSeconds`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | `PollSeconds` value outside the allowed range |
| Wrap site | `internal/core/settings.go` (validate path) |
| Log line | `WARN settings.Validate Field=PollSeconds Value=<v> ErrCode=ER-SET-21771 out of range` |
| User msg | Toast: `Poll interval must be between <min> and <max> seconds.` |
| Recovery | UI rejects the change; previous value retained |
| Test ref | `TestSettings_PollSeconds_OutOfRangeRejected` |

### ER-SET-21772 — `ErrSettingsTheme`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Theme value not one of the allowed palette identifiers |
| Wrap site | `internal/core/settings.go` (validate path) |
| Log line | `WARN settings.Validate Field=Theme Value=<v> ErrCode=ER-SET-21772 unknown palette` |
| User msg | Toast: `Unknown theme '<v>'.` |
| Recovery | UI rejects the change |
| Test ref | `TestSettings_Theme_UnknownRejected` |

### ER-SET-21773 — `ErrSettingsUrlScheme`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | URL scheme rule rejects the candidate (e.g. `javascript:` or `file:`) |
| Wrap site | `internal/core/settings.go` (URL gate) |
| Log line | `WARN settings.UrlGate Scheme=<s> ErrCode=ER-SET-21773 scheme rejected` |
| User msg | Toast: `URL scheme '<s>' is not allowed.` |
| Recovery | UI rejects the change |
| Test ref | `TestSettings_UrlScheme_DisallowedRejected` |

### ER-SET-21774 — `ErrSettingsChromePath`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Configured Chrome/Chromium path is not executable |
| Wrap site | `internal/core/settings.go` (Chrome detection) |
| Log line | `WARN settings.ChromePath Path=<p> ErrCode=ER-SET-21774 not executable` |
| User msg | Toast: `Chrome at '<p>' is not executable.` |
| Recovery | User picks a different binary |
| Test ref | `TestSettings_ChromePath_NotExecutableRejected` |

### ER-SET-21775 — `ErrSettingsIncognitoArg`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Incognito CLI argument template is malformed |
| Wrap site | `internal/core/settings.go` (incognito-arg parse) |
| Log line | `WARN settings.IncognitoArg Tmpl=<t> ErrCode=ER-SET-21775 malformed template` |
| User msg | Toast: `Incognito argument is not valid.` |
| Recovery | UI rejects the change |
| Test ref | `TestSettings_IncognitoArg_MalformedRejected` |

### ER-SET-21776 — `ErrSettingsLocalhostUrls`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | localhost URL setting violates the gate rule |
| Wrap site | `internal/core/settings.go` (localhost gate) |
| Log line | `WARN settings.LocalhostGate Url=<u> ErrCode=ER-SET-21776 gate rule violation` |
| User msg | Toast: `Localhost URLs are not allowed by current settings.` |
| Recovery | User adjusts setting or URL |
| Test ref | `TestSettings_LocalhostUrls_GateRejected` |

### ER-SET-21777 — `ErrSettingsCompositeRule`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | A composite settings rule (multiple fields) failed validation |
| Wrap site | `internal/core/settings.go` (composite validate) |
| Log line | `WARN settings.Validate Rule=<r> ErrCode=ER-SET-21777 composite rule violation` |
| User msg | Toast: `Settings combination is invalid: <reason>.` |
| Recovery | UI rejects the change |
| Test ref | `TestSettings_CompositeRule_Rejected` |

### ER-SET-21778 — `ErrSettingsPersist`

| Field | Value |
|---|---|
| Severity | `ERROR` |
| Trigger | Persisting settings to disk failed (write error / permission denied) |
| Wrap site | `internal/core/settings.go:Save()` |
| Log line | `ERROR settings.Save ErrCode=ER-SET-21778 ErrFrames=[…] persist failed` |
| User msg | Toast: `Could not save settings: <reason>.` |
| Recovery | User retries; previous value still in memory |
| Test ref | `TestSettings_Save_DiskErrorPropagates` |

### ER-SET-21779 — `ErrSettingsConcurrentEdit`

| Field | Value |
|---|---|
| Severity | `WARN` |
| Trigger | Detected a stale revision token on save (concurrent edit from another window/process) |
| Wrap site | `internal/core/settings.go:Save()` (revision check) |
| Log line | `WARN settings.Save Revision=<r> Stored=<s> ErrCode=ER-SET-21779 stale revision` |
| User msg | Toast: `Settings were changed elsewhere. Reload and try again.` |
| Recovery | UI reloads from disk; user re-applies edit |
| Test ref | `TestSettings_Save_StaleRevisionRejected` |

---

## 14. Mapping: ErrCode → CLI Exit Code

| ErrCode prefix | Exit code (CLI only) |
|---|---|
| `ER-CFG-*` | `3` |
| `ER-DB-*` | `4` |
| `ER-MAIL-*` | `5` |
| `ER-CLI-21801`, `ER-CLI-21802`, `ER-CLI-21803` | `2` |
| `ER-CLI-21804` | `130` |
| All other `ERROR`/`FATAL` | `1` |
| Success path | `0` |

Implemented in `internal/cli/cli.go:exitCodeFor(err error) int`.

---

## 15. Adding a New Error Code (Procedure)

1. Pick the lowest unused integer in the appropriate `21XYY` range.
2. Add a row to this file with all 10 columns filled.
3. Add the constant to `internal/errtrace/codes.go`.
4. Add or extend the `Test ref` test.
5. Bump `Version` in both `cmd/*/main.go` (per `04-coding-standards.md §10`).
6. Run `linter-scripts/validate-guidelines.go` — must pass.
7. Update `98-changelog.md`.

---

## 16. Forbidden Anti-Patterns

| Pattern | Why banned |
|---|---|
| `errtrace.New("…")` without an `ErrCode` carried alongside | Can't be looked up in this registry |
| Reusing a retired code | Breaks log analytics |
| One code for two distinct triggers | Loses diagnostic value |
| Code outside layer's range | Lint failure |
| Code emitted but missing from this file | Lint failure (validator parses both sides) |

---

## 16. Cross-References

| Reference | Location |
|---|---|
| Coding standards (wrap rules) | [04-coding-standards.md §5](./04-coding-standards.md) |
| Logging strategy (log line shape) | [05-logging-strategy.md §6](./05-logging-strategy.md) |
| Architecture (which package owns what) | [07-architecture.md](./07-architecture.md) |
| Watcher feature (must restate ER-WCH + ER-MAIL lines) | [02-features/05-watch/00-overview.md](./02-features/05-watch/00-overview.md) |
| `internal/errtrace` source | `internal/errtrace/errtrace.go` |
| Validator | `linter-scripts/validate-guidelines.go` |
| Solved issues | `.lovable/solved-issues/01..08` |
