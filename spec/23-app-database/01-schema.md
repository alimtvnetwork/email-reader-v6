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

| # | Table | Kind | Purpose | Owning feature backend |
|---|---|---|---|---|
| 1 | `Emails` | entity collection (plural) | Persisted IMAP messages and their on-disk `.eml` references. | Emails (`02-features/02-emails`) |
| 2 | `WatchState` | singleton-per-key state (singular) | Per-alias high-water mark for incremental polling. One row per alias = the live state, never history. | Watch (`02-features/05-watch`) |
| 3 | `OpenedUrls` | entity collection (plural) | Audit + dedup ledger for every browser launch and every blocked-url decision. | Tools (`02-features/06-tools`) + Rules (`02-features/03-rules`) |
| 4 | `WatchEvents` | entity collection (plural) | Audit log of watcher events (one row per event). | Watch (`02-features/05-watch`) |
| 5 | `_SchemaVersion` | bookkeeping (singular, `_` prefix) | Migration runner ledger. Owned by `internal/store/migrate`. Leading underscore hides it from business queries. | infra |

Total: 5 tables. No others. Adding a table requires bumping this spec's MAJOR version.

### Naming convention (LOCKED — Phase 2.1, Slice #68, 2026-04-26)

| Table kind | Form | Reasoning |
|---|---|---|
| **Entity collection** (one row = one entity) | **plural** | `SELECT … FROM Emails` reads as "select from the emails". `Emails`, `OpenedUrls`, `WatchEvents`. |
| **Singleton-per-key state** (one row = current state of one logical key) | **singular** | `WatchState` holds the *current* watch state for an alias. Pluralising would falsely imply history. |
| **Aggregate / view** (derived rollup) | **singular** with `v_` prefix | `v_AccountHealth` — one rollup record per concept. |
| **Bookkeeping / housekeeping** | **singular** with `_` prefix | `_SchemaVersion` — internal infrastructure, leading underscore hides it from business queries. |

**Go side**: a struct that maps to one row stays singular (`Email`, `OpenedUrl`, `WatchEvent`); a slice is plural (`[]Email`). This is independent of the table-name rule.

**No renames are pending.** The ledger slot `m0007_naming_convention_lock` (`internal/store/migrate/m0007_naming_convention_lock.go`) is a doc-only migration that pins this verdict next to the schema it governs — see `mem://design/schema-naming-convention.md` for the full Phase 2.1 audit. The earlier `legacy/spec.md` §7 and prior drafts of this file that referred to a singular rename (`M-002`) are **superseded**: the rename was the wrong direction and the slot is intentionally a no-op `SELECT 1`.

> **Cleanup note (non-blocking)**: §2–§5 below still show DDL with the historical singular forms. The actual database uses the plural / `_`-prefixed forms shown in the inventory above. A follow-up slice will sweep §2–§5 to pluralise the DDL examples and column references; this slice locks the convention at the canonical anchor point (§1) so no future schema change can drift further.



## 2. Table `Emails`

Persisted copy of every IMAP message that the watcher has fetched. The on-disk `.eml` file is the source of truth for body bytes; this table indexes metadata for search and filtering.

> **Drift notice (Slice #137).** The DDL block below shows the **logical / specified** shape (post-Phase-2.1: `HasAttachment`, `ReceivedAt NOT NULL`, four named indexes). The **canonical, in-database** shape is whatever `internal/store/migrate/m0001_emails_table.go` (+ additive `m0010_add_email_flags.go`, `m0012_add_email_deletedat.go`, `m0013_email_deletedat_index.go`) emits. Differences (e.g. `ReceivedAt` is nullable in m0001; `MessageId UNIQUE` is inline rather than via the named `UX_Emails_Alias_MessageId`; flags & soft-delete columns are not reflected here yet) are tracked in the deferred "schema-evolution work, ~12 AC-DB rows" backlog item — they require new numbered migrations to reconcile, not edits to this prose. The table **name** (plural `Emails`) and the convention rule are now authoritative; column-by-column reconciliation lands later.

```sql
CREATE TABLE Emails (
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

CREATE UNIQUE INDEX UX_Emails_Alias_MessageId ON Emails (Alias, MessageId);
CREATE        INDEX IX_Emails_Alias_Uid       ON Emails (Alias, Uid DESC);
CREATE        INDEX IX_Emails_ReceivedAt      ON Emails (ReceivedAt DESC);
CREATE        INDEX IX_Emails_Alias_Received  ON Emails (Alias, ReceivedAt DESC);
```

| Column | Type | Notes |
|---|---|---|
| `Id` | `INTEGER` PK | rowid alias; FK target for `OpenedUrls.EmailId`. |
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

> **Drift notice (Slice #137).** The DDL block below shows the **logical** shape (with `LastMessageId`, `LastPolledAt`, `LastErrorCode`). The **canonical, in-database** shape is whatever `internal/store/migrate/m0003_watch_state_table.go` (+ additive `m0014_watchstate_consecutive_failures.go`) emits. m0003 currently lacks `LastMessageId`/`LastPolledAt`/`LastErrorCode`; m0014 added `ConsecutiveFailures` (not reflected here). Bringing the two into alignment is part of the deferred "schema-evolution work" backlog. The table **name** (singular — singleton-per-key state, per the §1 verdict matrix) is authoritative.

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
| `Alias` | `TEXT` PK | Matches `Emails.Alias`. No FK (Emails rows can be pruned independently). |
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

## 4. Table `OpenedUrls`

Forensic ledger for every URL the app considers opening — including blocked decisions. Used both for deduplication (don't double-launch the same URL within `OpenUrlDedupWindow`) and for the Tools "Recent opened URLs" view.

```sql
CREATE TABLE OpenedUrls (
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
    FOREIGN KEY (EmailId) REFERENCES Emails(Id) ON DELETE SET NULL
);

CREATE        INDEX IX_OpenedUrls_Alias_Created ON OpenedUrls (Alias, CreatedAt DESC);
CREATE        INDEX IX_OpenedUrls_EmailId       ON OpenedUrls (EmailId);
CREATE UNIQUE INDEX UX_OpenedUrls_Dedup
       ON OpenedUrls (Alias, OriginalUrl, LaunchedAt)
       WHERE Decision = 'Launched';
```

| Column | Type | Notes |
|---|---|---|
| `Id` | `INTEGER` PK | |
| `EmailId` | `INTEGER NULL` | FK → `Emails.Id`. NULL for `Origin='Manual'` (Tools UI launches). `ON DELETE SET NULL` so pruning Emails rows preserves the audit trail. |
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
- `UX_OpenedUrls_Dedup` is a **partial unique index** — only over `Decision='Launched'` rows. Blocked attempts are never deduped (every block is independently audited).
- `OriginalUrl` is encrypted at rest? **No** — explicit non-feature; documented in Tools backend §11.
- The Tools backend MUST insert the `OpenedUrl` row **after** the browser launch returns (forensic completeness — Tools backend §3.4 step 7).

---

## 5. Table `_SchemaVersion`

Owned by `internal/store/migrate`. Feature code MUST NOT read or write this table.

```sql
CREATE TABLE _SchemaVersion (
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
| X-1 | `OpenedUrls.EmailId` MUST point at an existing `Emails.Id` or be NULL. | FK `ON DELETE SET NULL`. |
| X-2 | `WatchState.Alias` and `Emails.Alias` SHOULD agree, but no FK — Emails rows survive alias deletion (audit value). | Application-level. |
| X-3 | `OpenedUrls.Alias` SHOULD match `Emails.Alias` when `EmailId` is set. | Application-level (Tools backend §3.4 step 5). |
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
