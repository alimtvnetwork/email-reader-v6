# 01 — Schema

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Authoritative table definitions for `data/emails.db`. PascalCase column names per `spec/12-consolidated-guidelines/18-database-conventions.md`. Singular table names. Every constraint, index, and FK rule is explicit. No column may exist anywhere else in the codebase that is not listed here.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Queries: [`./02-queries.md`](./02-queries.md)
- Migrations: [`./03-migrations.md`](./03-migrations.md)
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`

---

## 1. Table Inventory

| # | Table | Purpose | Owning feature backend |
|---|---|---|---|
| 1 | `Email` | Persisted IMAP messages and their on-disk `.eml` references. | Emails (`02-features/02-emails`) |
| 2 | `WatchState` | Per-alias high-water mark for incremental polling. | Watch (`02-features/05-watch`) |
| 3 | `OpenedUrl` | Audit + dedup ledger for every browser launch and every blocked-url decision. | Tools (`02-features/06-tools`) + Rules (`02-features/03-rules`) |
| 4 | `SchemaMigration` | Migration runner ledger. Owned by `internal/store/migrate`. | infra |

Total: 4 tables. No others. Adding a table requires bumping this spec's MAJOR version.

Note on naming: the *table* is singular (`Email`); the *Go struct* is singular (`Email`); a *slice* is plural (`[]Email`). Legacy plural names (`Emails`, `OpenedUrls`) from `legacy/spec.md` §7 are **superseded by this spec**. The migration runner renames them in `M-002` (see `03-migrations.md`).

---

## 2. Table `Email`

Persisted copy of every IMAP message that the watcher has fetched. The on-disk `.eml` file is the source of truth for body bytes; this table indexes metadata for search and filtering.

```sql
CREATE TABLE Email (
    Id            INTEGER PRIMARY KEY AUTOINCREMENT,
    Alias         TEXT    NOT NULL,
    MessageId     TEXT    NOT NULL,
    Uid           INTEGER NOT NULL,
    FromAddr      TEXT    NOT NULL DEFAULT '',
    ToAddr        TEXT    NOT NULL DEFAULT '',
    CcAddr        TEXT    NOT NULL DEFAULT '',
    Subject       TEXT    NOT NULL DEFAULT '',
    BodyText      TEXT    NOT NULL DEFAULT '',
    BodyHtml      TEXT    NOT NULL DEFAULT '',
    ReceivedAt    DATETIME NOT NULL,
    FilePath      TEXT    NOT NULL,
    HasAttachment INTEGER NOT NULL DEFAULT 0 CHECK (HasAttachment IN (0,1)),
    CreatedAt     DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE UNIQUE INDEX UX_Email_Alias_MessageId ON Email (Alias, MessageId);
CREATE        INDEX IX_Email_Alias_Uid       ON Email (Alias, Uid DESC);
CREATE        INDEX IX_Email_ReceivedAt      ON Email (ReceivedAt DESC);
CREATE        INDEX IX_Email_Alias_Received  ON Email (Alias, ReceivedAt DESC);
```

| Column | Type | Notes |
|---|---|---|
| `Id` | `INTEGER` PK | rowid alias; FK target for `OpenedUrl.EmailId`. |
| `Alias` | `TEXT NOT NULL` | Account alias from `config.json` `Accounts[*].Alias`. |
| `MessageId` | `TEXT NOT NULL` | RFC 822 `Message-ID`. Empty messages get a synthesized `<sha1>@no-id.local` value (Watch backend §4). |
| `Uid` | `INTEGER NOT NULL` | IMAP UID. Per-`Alias` monotonic, but **never** assumed globally unique. |
| `FromAddr` | `TEXT NOT NULL` | Single RFC 5322 addr-spec string. |
| `ToAddr` | `TEXT NOT NULL` | Comma-joined recipient list. |
| `CcAddr` | `TEXT NOT NULL` | Comma-joined Cc list. |
| `Subject` | `TEXT NOT NULL` | Decoded subject (no MIME `=?...?=` tokens). |
| `BodyText` | `TEXT NOT NULL` | Plain-text body (or `html2text`-stripped HTML if no plain part). |
| `BodyHtml` | `TEXT NOT NULL` | Raw HTML if present. |
| `ReceivedAt` | `DATETIME NOT NULL` | UTC, ISO-8601 with millis. |
| `FilePath` | `TEXT NOT NULL` | Path to the `.eml` file under `email/<Alias>/<yyyy>/<mm>/<MessageId>.eml`. Validated by `paths.Validator` on every read. |
| `HasAttachment` | `INTEGER NOT NULL` | Positive boolean (per conventions §3). `1` = at least one MIME part with a `Content-Disposition: attachment`. |
| `CreatedAt` | `DATETIME NOT NULL` | UTC ISO-8601. Set by SQLite at insert time. |

**Invariants:**
- `(Alias, MessageId)` is unique. Re-fetching the same message is an idempotent UPSERT (see `Q-EMAIL-UPSERT`).
- `Uid` is **only** unique within an `Alias`; queries MUST scope by `Alias` before filtering by `Uid`.
- `FilePath` MUST live under the configured `EmailArchiveDir`; enforced at insert by `paths.Validator` (`ER-COR-21704` on escape).
- All `*Addr` columns store **already-decoded** UTF-8; no MIME word encoding.

---

## 3. Table `WatchState`

Per-alias high-water mark + last-seen metadata. Read at the start of every poll cycle, written at the end. Owned exclusively by the Watch backend.

```sql
CREATE TABLE WatchState (
    Alias          TEXT     PRIMARY KEY,
    LastUid        INTEGER  NOT NULL DEFAULT 0,
    LastMessageId  TEXT     NOT NULL DEFAULT '',
    LastSubject    TEXT     NOT NULL DEFAULT '',
    LastReceivedAt DATETIME,
    LastPolledAt   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    LastErrorCode  TEXT     NOT NULL DEFAULT '',
    UpdatedAt      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
```

| Column | Type | Notes |
|---|---|---|
| `Alias` | `TEXT` PK | Matches `Email.Alias`. No FK (Email rows can be pruned independently). |
| `LastUid` | `INTEGER NOT NULL` | Highest `Uid` successfully persisted for this alias. `0` = first run. |
| `LastMessageId` | `TEXT NOT NULL` | `Message-ID` of the most recent successfully persisted message. |
| `LastSubject` | `TEXT NOT NULL` | Subject of the most recent message (UI display). |
| `LastReceivedAt` | `DATETIME NULL` | UTC. NULL only when `LastUid = 0`. |
| `LastPolledAt` | `DATETIME NOT NULL` | UTC of the last completed poll cycle (success OR failure). |
| `LastErrorCode` | `TEXT NOT NULL` | Empty when last poll succeeded; otherwise `ER-MAIL-2120X` etc. |
| `UpdatedAt` | `DATETIME NOT NULL` | UTC of the last write. |

**Invariants:**
- One row per alias. UPSERT on `Alias` (see `Q-WATCH-UPSERT`).
- `LastUid` is monotonically non-decreasing. The runtime asserts the new value is `>= old` before writing; violations are `ER-WATCH-21503` (logged + skipped).
- `LastPolledAt` is updated on **every** poll completion, even on failure (heartbeat invariant in `05-logging-strategy.md`).

---

## 4. Table `OpenedUrl`

Forensic ledger for every URL the app considers opening — including blocked decisions. Used both for deduplication (don't double-launch the same URL within `OpenUrlDedupWindow`) and for the Tools "Recent opened URLs" view.

```sql
CREATE TABLE OpenedUrl (
    Id             INTEGER PRIMARY KEY AUTOINCREMENT,
    EmailId        INTEGER,
    Alias          TEXT    NOT NULL,
    RuleName       TEXT    NOT NULL DEFAULT '',
    Origin         TEXT    NOT NULL CHECK (Origin IN ('Watcher','Manual','Rule')),
    OriginalUrl    TEXT    NOT NULL,
    OpenedUrl      TEXT    NOT NULL DEFAULT '',
    Decision       TEXT    NOT NULL CHECK (Decision IN ('Launched','Blocked','Skipped','Failed')),
    BlockedReason  TEXT    NOT NULL DEFAULT '',
    LaunchedAt     DATETIME,
    CreatedAt      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY (EmailId) REFERENCES Email(Id) ON DELETE SET NULL
);

CREATE        INDEX IX_OpenedUrl_Alias_Created ON OpenedUrl (Alias, CreatedAt DESC);
CREATE        INDEX IX_OpenedUrl_EmailId       ON OpenedUrl (EmailId);
CREATE UNIQUE INDEX UX_OpenedUrl_Dedup
       ON OpenedUrl (Alias, OriginalUrl, LaunchedAt)
       WHERE Decision = 'Launched';
```

| Column | Type | Notes |
|---|---|---|
| `Id` | `INTEGER` PK | |
| `EmailId` | `INTEGER NULL` | FK → `Email.Id`. NULL for `Origin='Manual'` (Tools UI launches). `ON DELETE SET NULL` so pruning Email rows preserves the audit trail. |
| `Alias` | `TEXT NOT NULL` | Account alias context (always populated, even for Manual). |
| `RuleName` | `TEXT NOT NULL` | Empty unless `Origin='Rule'`. |
| `Origin` | `TEXT NOT NULL` | Enum: `Watcher` (auto-open by polling loop), `Manual` (Tools UI), `Rule` (explicit rule action). |
| `OriginalUrl` | `TEXT NOT NULL` | URL **as found** in the email body — including tracking params. **Never logged** above DEBUG (per Tools backend §11). |
| `OpenedUrl` | `TEXT NOT NULL` | URL after redaction stage 1 (tracking-param strip). Empty when blocked before stage 1. |
| `Decision` | `TEXT NOT NULL` | Enum: `Launched`, `Blocked`, `Skipped`, `Failed`. See Tools backend §3.4 for the decision matrix. |
| `BlockedReason` | `TEXT NOT NULL` | Error code (`ER-TOOL-2176X`) when `Decision != 'Launched'`. |
| `LaunchedAt` | `DATETIME NULL` | UTC. Populated only when `Decision = 'Launched'`. |
| `CreatedAt` | `DATETIME NOT NULL` | UTC. Always set, even for Blocked/Failed. |

**Invariants:**
- `UX_OpenedUrl_Dedup` is a **partial unique index** — only over `Decision='Launched'` rows. Blocked attempts are never deduped (every block is independently audited).
- `OriginalUrl` is encrypted at rest? **No** — explicit non-feature; documented in Tools backend §11.
- The Tools backend MUST insert the `OpenedUrl` row **after** the browser launch returns (forensic completeness — Tools backend §3.4 step 7).

---

## 5. Table `SchemaMigration`

Owned by `internal/store/migrate`. Feature code MUST NOT read or write this table.

```sql
CREATE TABLE SchemaMigration (
    Version    INTEGER PRIMARY KEY,
    Name       TEXT    NOT NULL,
    AppliedAt  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    Checksum   TEXT    NOT NULL
);
```

| Column | Type | Notes |
|---|---|---|
| `Version` | `INTEGER` PK | Monotonic. Gaps forbidden. |
| `Name` | `TEXT NOT NULL` | e.g. `M-003-add-hasattachment-to-email`. |
| `AppliedAt` | `DATETIME NOT NULL` | UTC. |
| `Checksum` | `TEXT NOT NULL` | SHA-256 of the migration's SQL bytes. Used to detect tampering on startup (see `03-migrations.md` §4). |

---

## 6. Cross-Table Constraints

| # | Constraint | Mechanism |
|---|---|---|
| X-1 | `OpenedUrl.EmailId` MUST point at an existing `Email.Id` or be NULL. | FK `ON DELETE SET NULL`. |
| X-2 | `WatchState.Alias` and `Email.Alias` SHOULD agree, but no FK — Email rows survive alias deletion (audit value). | Application-level. |
| X-3 | `OpenedUrl.Alias` SHOULD match `Email.Alias` when `EmailId` is set. | Application-level (Tools backend §3.4 step 5). |
| X-4 | All `*At` columns are stored as ISO-8601 UTC strings (SQLite `DATETIME` is text). | Insert defaults via `strftime('%Y-%m-%dT%H:%M:%fZ','now')`. |
| X-5 | All boolean columns are positive (`HasAttachment`, future booleans) per `18-database-conventions.md` §3. | `CHECK (col IN (0,1))`. |

---

## 7. PRAGMAs (set on every connection)

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous  = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA temp_store   = MEMORY;
```

`internal/store.Open` runs these on every new `*sql.Conn` via a connection initializer. Verified by `Test_Store_PragmaOnEveryConn`.

---

## 8. Forbidden Schema Changes (without spec bump)

The following are **breaking** and require a MAJOR version bump of this file:

- Adding/removing a table.
- Changing a PK or unique-index column set.
- Renaming any column.
- Changing a column's type or NULL-ability.
- Changing an enum's allowed values (`Origin`, `Decision`).

Adding a non-NULL column with a default, or adding a new index, is MINOR.
