# Consolidated: Database Conventions — Complete Reference

**Version:** 3.2.0  
**Updated:** 2026-04-16

---

## Purpose

This is the **standalone consolidated reference** for all database conventions. An AI reading only this file must be able to design, implement, test, and expose correct database schemas without consulting source specs.

**Source:** `spec/04-database-conventions/` (7 files) + `spec/02-coding-guidelines/01-cross-language/07-database-naming.md`

---

## 1. Naming Convention — PascalCase Everything

All database objects use **PascalCase**. No underscores, no snake_case, no camelCase.

### 1.1 CRITICAL: Singular Table Names

Table names MUST be **singular** — they represent the entity type, not the collection:

| ❌ Wrong (Plural) | ✅ Correct (Singular) |
|-------------------|----------------------|
| `Users` | `User` |
| `Projects` | `Project` |
| `Transactions` | `Transaction` |
| `AgentSites` | `AgentSite` |
| `StatusTypes` | `StatusType` |
| `UserRoles` | `UserRole` |

**Why singular?**
- The table defines the **entity schema**, not the collection
- PK becomes `{TableName}Id` naturally: `User` → `UserId`, `Transaction` → `TransactionId`
- No ambiguity about singular vs plural forms (`Status` vs `Statuses` vs `StatusTypes`)
- FK columns read naturally: `Transaction.AgentSiteId` → "this transaction's agent site"

### 1.2 Complete Naming Reference

| Object | Convention | Example |
|--------|-----------|---------|
| Table names | PascalCase, **singular** | `User`, `AgentSite`, `Transaction` |
| Column names | PascalCase | `PluginSlug`, `CreatedAt`, `DisplayName` |
| Primary key | `{TableName}Id` | `UserId`, `TransactionId`, `AgentSiteId` |
| Foreign key column | Exact same name as referenced PK | `AgentSiteId` in both `AgentSite` and `Transaction` |
| Boolean columns | `Is` or `Has` prefix, **positive only** | `IsActive`, `HasLicense` — never `IsDisabled` |
| Index names | `Idx{Table}_{Column}` | `IdxTransaction_CreatedAt` |
| View names | `Vw` prefix + PascalCase | `VwTransactionDetail`, `VwActiveUser` |
| Abbreviations | First letter only capitalized | `Id`, `Url`, `Api` — never `ID`, `URL`, `API` |

### 1.3 ❌ Wrong vs ✅ Correct

| ❌ Wrong | ✅ Correct | Why |
|----------|-----------|-----|
| `Users` | `User` | Singular table names |
| `user_id` | `UserId` | PascalCase required |
| `created_at` | `CreatedAt` | PascalCase required |
| `Id` (bare) | `UserId` | PK must include table name |
| `UsersId` (plural) | `UserId` | Table name is singular |
| `IsDisabled` | `IsEnabled` | Positive boolean only |
| `ID`, `URL` | `Id`, `Url` | First-letter-only caps for abbreviations |

### 1.4 WordPress Exception

WordPress core tables (`wp_posts`, `wp_options`) retain their native `snake_case` naming. Only **custom tables** follow PascalCase.

---

## 2. Primary Key Strategy

### 2.1 Naming: `{TableName}Id`

The primary key column MUST be named `{TableName}Id` — since table names are singular, the PK is naturally singular:

```sql
CREATE TABLE User (
    UserId INTEGER PRIMARY KEY AUTOINCREMENT
);

CREATE TABLE AgentSite (
    AgentSiteId INTEGER PRIMARY KEY AUTOINCREMENT
);

CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT
);

CREATE TABLE StatusType (
    StatusTypeId SMALLINT PRIMARY KEY
);
```

### 2.2 Type: Always Integer, Smallest Possible

Primary keys MUST be integer-based. Choose the smallest type that fits the 10-year projected volume:

| Expected Rows (10-year) | Key Type | Range | Storage |
|--------------------------|----------|-------|---------|
| < 32,000 | `SMALLINT` | ±32K | 2 bytes |
| < 2 billion | `INTEGER` (default) | ±2.1B | 4 bytes |
| > 2 billion | `BIGINT` | ±9.2 quintillion | 8 bytes |

```
How many rows in 10 years?
├── < 32,000 → SMALLINT (lookup tables: StatusType, Role, FileType)
├── < 2,000,000,000 → INTEGER (most entity tables — THIS IS THE DEFAULT)
└── > 2,000,000,000 → BIGINT (event streams, high-volume analytics)
```

### 2.3 Auto-Increment: Required for Entity Tables

```sql
-- ✅ Entity tables — AUTOINCREMENT
CREATE TABLE User (
    UserId INTEGER PRIMARY KEY AUTOINCREMENT
);

-- ✅ Lookup tables — manual seeding, no AUTOINCREMENT
CREATE TABLE StatusType (
    StatusTypeId SMALLINT PRIMARY KEY
);
```

### 2.4 UUID/GUID — Avoid

| Aspect | INTEGER | UUID |
|--------|---------|------|
| Storage | 4 bytes | 16 bytes (4x larger) |
| Index performance | Fast (sequential) | Slow (random distribution) |
| Readability | Easy to debug | Hard to read |
| Fragmentation | None | High (random inserts) |

> **Rule:** ❌ Do NOT use UUID as primary key unless **ALL THREE** are true:
> 1. Records created across multiple disconnected systems
> 2. No central ID authority
> 3. IDs must be publicly exposed and non-guessable
>
> If UUID is required, store as `BLOB(16)` — never `TEXT(36)`.

---

## 3. Column Right-Sizing

Apply the smallest-type principle to **ALL columns**, not just primary keys:

| Data | ❌ Oversized | ✅ Right-Sized |
|------|-------------|---------------|
| Status (5 values) | `TEXT` | `TINYINT` + lookup table |
| Age | `INTEGER` | `TINYINT` (0-255) |
| Year | `INTEGER` | `SMALLINT` (0-65535) |
| Boolean | `INTEGER` | `TINYINT(1)` or `BOOLEAN` |
| Country code | `TEXT` | `CHAR(2)` |
| Currency amount | `REAL` | `DECIMAL(10,2)` |

---

## 4. Foreign Key Relationships

### 4.1 FK Column = Exact PK Name

The FK column MUST use the **exact same name** as the PK it references:

```sql
-- Parent table
CREATE TABLE AgentSite (
    AgentSiteId INTEGER PRIMARY KEY AUTOINCREMENT,
    SiteName    TEXT NOT NULL
);

-- Child table — FK column name matches PK exactly
CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    AgentSiteId   INTEGER NOT NULL,           -- same name as AgentSite.AgentSiteId
    FOREIGN KEY (AgentSiteId) REFERENCES AgentSite(AgentSiteId)
);
```

### 4.2 Always Declare FOREIGN KEY Constraints

Every FK relationship MUST have an explicit `FOREIGN KEY` constraint:

```sql
-- ❌ WRONG — FK column exists but no constraint
CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    AgentSiteId   INTEGER NOT NULL
);

-- ✅ CORRECT — explicit constraint
CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    AgentSiteId   INTEGER NOT NULL,
    FOREIGN KEY (AgentSiteId) REFERENCES AgentSite(AgentSiteId)
);
```

### 4.3 Enable Foreign Key Enforcement (SQLite)

SQLite has FK enforcement **OFF by default**. Every connection MUST enable it:

```go
_, err := db.Exec("PRAGMA foreign_keys=ON")
```

### 4.4 One-to-Many Relationship Pattern

The "many" side holds the FK:

```sql
-- One AgentSite has many Transaction records
CREATE TABLE AgentSite (
    AgentSiteId INTEGER PRIMARY KEY AUTOINCREMENT,
    SiteName    TEXT NOT NULL
);

CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    AgentSiteId   INTEGER NOT NULL,
    Amount        REAL NOT NULL,
    FOREIGN KEY (AgentSiteId) REFERENCES AgentSite(AgentSiteId)
);
```

### 4.5 Many-to-Many (N-to-M) Pattern

Use a **junction table** with its own PK and FKs to both sides:

```sql
CREATE TABLE User (
    UserId INTEGER PRIMARY KEY AUTOINCREMENT,
    Name   TEXT NOT NULL
);

CREATE TABLE Role (
    RoleId SMALLINT PRIMARY KEY,
    Name   TEXT NOT NULL UNIQUE
);

-- Junction table (also singular)
CREATE TABLE UserRole (
    UserRoleId INTEGER PRIMARY KEY AUTOINCREMENT,
    UserId     INTEGER NOT NULL,
    RoleId     SMALLINT NOT NULL,
    UNIQUE (UserId, RoleId),
    FOREIGN KEY (UserId) REFERENCES User(UserId),
    FOREIGN KEY (RoleId) REFERENCES Role(RoleId)
);
```

**Junction table rules:**
- Name: singular compound name → `UserRole` (not `UserRoles`)
- PK: `{TableName}Id` → `UserRoleId`
- FK columns: exact same names as source PKs → `UserId`, `RoleId`
- `UNIQUE` constraint on the FK pair prevents duplicate assignments

### 4.6 Cross-Database FK (Split DB)

Foreign keys are enforced ONLY within the same database file. Cross-database references store the ID but rely on **application-layer validation**:

```sql
-- In transactions.db
CREATE TABLE Transaction (
    TransactionId   INTEGER PRIMARY KEY AUTOINCREMENT,
    AgentSiteId     INTEGER NOT NULL,  -- References agent-sites.db but NO FK constraint
    StatusTypeId    INTEGER NOT NULL,
    FOREIGN KEY (StatusTypeId) REFERENCES StatusType(StatusTypeId)  -- Same DB = FK OK
    -- NO FOREIGN KEY for AgentSiteId (different DB)
);
```

---

## 5. Normalization — Repeated Values Become Lookup Tables

### The Rule

> Any column that contains a **repeated set of values** (status types, file types, categories, roles, priorities) MUST be extracted into a separate lookup table with a FK relationship.

### ❌ Wrong — Repeated Strings

```sql
CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    Status        TEXT,     -- 'Pending', 'Complete', 'Failed' repeated thousands of times
    FileType      TEXT      -- 'Plugin', 'Theme', 'MuPlugin' repeated thousands of times
);
```

### ✅ Correct — Normalized with Lookup Tables

```sql
CREATE TABLE StatusType (
    StatusTypeId SMALLINT PRIMARY KEY,
    Name         TEXT NOT NULL UNIQUE
);

CREATE TABLE FileType (
    FileTypeId SMALLINT PRIMARY KEY,
    Name       TEXT NOT NULL UNIQUE
);

CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    StatusTypeId  SMALLINT NOT NULL,
    FileTypeId    SMALLINT NOT NULL,
    FOREIGN KEY (StatusTypeId) REFERENCES StatusType(StatusTypeId),
    FOREIGN KEY (FileTypeId)   REFERENCES FileType(FileTypeId)
);
```

---

## 6. Boolean Column Rules

### Rule 1: Always `Is` or `Has` Prefix

Every boolean column MUST start with `Is` or `Has`. No other prefixes (`Can`, `Was`, `Will`, `Did`).

### Rule 2: Always Positive — Never Negative Names

| ❌ Forbidden | ✅ Required |
|-------------|------------|
| `IsDisabled` | `IsEnabled` |
| `IsNotActive` | `IsActive` |
| `IsInvalid` | `IsValid` |
| `IsHidden` | `IsVisible` |
| `HasNoAccess` | `HasAccess` |
| `IsUnverified` | `IsVerified` |
| `IsLocked` | `IsEditable` |
| `IsIncomplete` | `IsComplete` |
| `IsUnavailable` | `IsAvailable` |
| `IsUnread` | `IsRead` |
| `IsUnpublished` | `IsPublished` |

### Rule 3: Always `NOT NULL DEFAULT`

Boolean columns MUST never be nullable:

```sql
-- ❌ WRONG
IsActive BOOLEAN

-- ✅ CORRECT
IsActive BOOLEAN NOT NULL DEFAULT 1
IsVerified BOOLEAN NOT NULL DEFAULT 0
```

### Rule 4: Prefer Timestamp Over Boolean When "When" Matters

| Boolean | Timestamp Alternative | Use Timestamp When |
|---------|----------------------|-------------------|
| `IsDeleted` | `DeletedAt TEXT NULL` | Soft deletes — need to know when |
| `IsExpired` | `ExpiresAt TEXT NULL` | Expiration tracking |
| `IsCompleted` | `CompletedAt TEXT NULL` | Duration tracking |
| `IsBanned` | `BannedAt TEXT NULL` | Audit trail needed |

### Rule 5: Query Readability Test

A well-named boolean reads naturally in both true and false checks:

| Column | True Check | False Check | Reads Naturally? |
|--------|-----------|-------------|-----------------|
| `IsActive` | `WHERE IsActive = 1` → "is active" | `WHERE IsActive = 0` → "is not active" | ✅ Yes |
| `IsDisabled` | `WHERE IsDisabled = 1` → "is disabled" | `WHERE IsDisabled = 0` → "is not disabled" (??) | ❌ Confusing |

---

## 7. Views — Flatten Joins for ORM

### Naming: `Vw` Prefix (singular)

```sql
CREATE VIEW VwTransactionDetail AS
SELECT
    t.TransactionId,
    t.PluginSlug,
    t.Amount,
    t.IsActive,
    t.CreatedAt,
    st.Name       AS StatusName,
    ft.Name       AS FileTypeName,
    a.SiteName    AS AgentSiteName
FROM Transaction t
INNER JOIN StatusType st ON t.StatusTypeId = st.StatusTypeId
INNER JOIN FileType ft   ON t.FileTypeId = ft.FileTypeId
LEFT JOIN AgentSite a    ON t.AgentSiteId = a.AgentSiteId;
```

**Rule:** Business layer queries views — no raw JOINs in application code.

### When to Create a View

- A query JOINs **2 or more tables**
- The same JOIN is needed in **more than one place**
- The business layer needs a **flattened result** from related tables
- A report or dashboard aggregates data across tables

---

## 8. Index Naming

Format: `Idx{TableName}_{ColumnName}`

```sql
CREATE INDEX IdxTransaction_CreatedAt    ON Transaction(CreatedAt);
CREATE INDEX IdxTransaction_PluginSlug   ON Transaction(PluginSlug);
CREATE INDEX IdxTransaction_StatusTypeId ON Transaction(StatusTypeId);
CREATE INDEX IdxUserRole_UserId          ON UserRole(UserId);
```

---

## 9. SQLite-Specific Rules

| Setting | PRAGMA | Purpose |
|---------|--------|---------|
| WAL mode | `PRAGMA journal_mode=WAL` | Concurrent reads during writes |
| Foreign keys | `PRAGMA foreign_keys=ON` | Enforce FK constraints (OFF by default) |
| Busy timeout | `PRAGMA busy_timeout=5000` | Prevent SQLITE_BUSY errors |

Every database connection MUST set all three PRAGMAs.

---

## 10. ORM-First Rule

### 10.1 The Rule

> **Never write raw SQL in the business/service layer.** Use the best ORM or query builder available for the language.

| Layer | Raw SQL Allowed? | What to Use |
|-------|-----------------|-------------|
| Business logic / services | ❌ No | ORM methods |
| Repository / data access | ❌ No | ORM / query builder |
| Migrations | ✅ Yes | Raw DDL statements |
| View definitions | ✅ Yes | `CREATE VIEW` statements |
| One-off scripts | ✅ Yes | With approval |

### 10.2 Recommended ORMs by Language

| Language | ORM / Query Builder | Why |
|----------|-------------------|-----|
| **Go** | `sqlc` or `GORM` | Type-safe generated code (sqlc) or full ORM (GORM) |
| **PHP** | Custom `Orm` class or Eloquent | Project uses custom PascalCase-aware Orm |
| **TypeScript** | Prisma or Drizzle | Type-safe, schema-first |
| **Rust** | Diesel or SeaORM | Compile-time query validation |
| **C#** | Entity Framework Core | Industry standard, LINQ queries |

### 10.3 Examples

```go
// ❌ WRONG — Raw SQL in service layer
func (s *TransactionService) GetPending() ([]Transaction, error) {
    rows, err := s.db.Query(
        "SELECT TransactionId, PluginSlug FROM Transaction WHERE StatusTypeId = 1",
    )
    // manual scanning...
}

// ✅ CORRECT — ORM via repository
func (s *TransactionService) GetPending() ([]Transaction, error) {
    return s.repo.FindAll(TransactionFilter{
        StatusTypeId: statustype.Pending,
        OrderBy:      "CreatedAt DESC",
    })
}
```

```php
// PHP — uses Orm class
$orm = new Orm(TableType::Transaction->value);
$pending = $orm->findAll(
    ['StatusTypeId' => StatusType::Pending->value],
    'CreatedAt DESC'
);
```

---

## 11. ORM & Struct Mapping (Go)

### DB Tags Required, JSON Tags Omitted

```go
// ✅ CORRECT — db tags match column names, no redundant json tags
type Transaction struct {
    TransactionId int64   `db:"TransactionId"`
    AgentSiteId   int64   `db:"AgentSiteId"`
    StatusTypeId  int     `db:"StatusTypeId"`
    PluginSlug    string  `db:"PluginSlug"`
    Amount        float64 `db:"Amount"`
    IsActive      bool    `db:"IsActive"`
    CreatedAt     string  `db:"CreatedAt"`
}
```

**Rules:**
- `db:"ColumnName"` tags are always required
- `json` tags are omitted — Go serializes PascalCase by default
- Add `json:",omitempty"` only when zero-value fields should be excluded
- Add `json:"-"` only when a field must be excluded from JSON

### Query via `dbutil` Package

```go
result := dbutil.QueryOne[Transaction](ctx, db, query, scanFn, id)
set := dbutil.QueryMany[Transaction](ctx, db, query, scanFn)
res := dbutil.Exec(ctx, db, query, args...)
```

---

## 12. REST API — PascalCase JSON + Universal Envelope

### 12.1 Golden Rule

> **Every JSON key in a REST API response MUST be PascalCase.** No camelCase, no snake_case.

### 12.2 End-to-End PascalCase Flow

```
Database Column     →  ORM Struct/Model   →  API Response JSON  →  Frontend Type
───────────────────────────────────────────────────────────────────────────────
PluginSlug TEXT      →  PluginSlug string  →  "PluginSlug": "x"  →  PluginSlug: string
IsActive BOOLEAN     →  IsActive bool      →  "IsActive": true   →  IsActive: boolean
StatusTypeId INT     →  StatusTypeId int   →  "StatusTypeId": 1  →  StatusTypeId: number
```

No transformation layer needed. PascalCase flows from DB to frontend without any key mapping.

### 12.3 Universal Response Envelope

ALL responses use this envelope. `Results` is **always an array** — even for single items, deletes, or errors.

```json
{
    "Status": {
        "IsSuccess": true,
        "IsFailed": false,
        "Code": 200,
        "Message": "OK",
        "Timestamp": "2026-04-02T10:30:00Z"
    },
    "Attributes": {
        "RequestedAt": "http://localhost:8080/api/v1/transactions",
        "RequestDelegatedAt": "",
        "HasAnyErrors": false,
        "IsSingle": false,
        "IsMultiple": true,
        "IsEmpty": false,
        "TotalRecords": 47,
        "PerPage": 10,
        "TotalPages": 5,
        "CurrentPage": 2
    },
    "Results": [
        {
            "TransactionId": 1,
            "PluginSlug": "my-plugin",
            "Amount": 29.99,
            "StatusName": "Pending",
            "IsActive": true,
            "CreatedAt": "2026-04-02T10:30:00Z"
        }
    ],
    "Navigation": {
        "NextPage": "http://localhost:8080/api/v1/transactions?page=3&perPage=10",
        "PrevPage": "http://localhost:8080/api/v1/transactions?page=1&perPage=10",
        "CloserLinks": []
    }
}
```

### 12.4 Envelope Section Reference

| Section | Type | Present | Description |
|---------|------|---------|-------------|
| `Status` | object | ✅ Always | `IsSuccess`, `IsFailed`, `Code`, `Message`, `Timestamp` |
| `Attributes` | object | ✅ Always | Shape flags (`IsSingle`, `IsMultiple`, `IsEmpty`), pagination, error flag |
| `Results` | array | ✅ Always | Payload — always an array. Delete = `[]`. Single = `[{...}]` |
| `Navigation` | object\|null | ⚙️ Conditional | Pagination links (paginated lists only) |
| `Errors` | object\|null | ⚙️ Conditional | Error details (when `HasAnyErrors` is `true`) |

### 12.5 Error Response Example

```json
{
    "Status": {
        "IsSuccess": false,
        "IsFailed": true,
        "Code": 404,
        "Message": "Transaction not found",
        "Timestamp": "2026-04-02T14:00:00Z"
    },
    "Attributes": {
        "HasAnyErrors": true,
        "IsSingle": false,
        "IsMultiple": false,
        "IsEmpty": true,
        "TotalRecords": 0
    },
    "Results": [],
    "Errors": {
        "BackendMessage": "Transaction not found",
        "DelegatedServiceErrorStack": [],
        "Backend": ["handlers.go:92 handleGetTransaction"],
        "Frontend": []
    }
}
```

### 12.6 URL Paths vs JSON Keys

| Context | Convention | Example |
|---------|-----------|---------|
| URL paths (slugs) | **kebab-case lowercase** | `/api/v1/blog-posts/my-first-post` |
| Query parameters | **PascalCase** | `?StatusName=Pending&IsActive=1` |
| JSON request keys | **PascalCase** | `{"PluginSlug": "my-plugin"}` |
| JSON response keys | **PascalCase** | `{"TransactionId": 42}` |

### 12.7 TypeScript Envelope Type

```typescript
interface ApiResponse<T> {
    Status: {
        IsSuccess: boolean;
        IsFailed: boolean;
        Code: number;
        Message: string;
        Timestamp: string;
    };
    Attributes: {
        RequestedAt: string;
        RequestDelegatedAt: string;
        HasAnyErrors: boolean;
        IsSingle: boolean;
        IsMultiple: boolean;
        IsEmpty: boolean;
        TotalRecords: number;
        PerPage: number;
        TotalPages: number;
        CurrentPage: number;
    };
    Results: T[];
    Navigation?: {
        NextPage: string | null;
        PrevPage: string | null;
        CloserLinks: string[];
    };
    Errors?: {
        BackendMessage: string;
        DelegatedServiceErrorStack: string[];
        Backend: string[];
        Frontend: string[];
    };
}
```

---

## 13. Testing Strategy

### 13.1 Two-Tier Approach

| Tier | What It Tests | Database | Speed |
|------|--------------|----------|-------|
| **Unit tests** | Schema creation, migrations, constraints, column types | In-memory SQLite (`:memory:`) | Fast (ms) |
| **Integration tests** | Full CRUD operations, views, relationships, ORM queries | In-memory SQLite (`:memory:`) | Fast (ms) |

### 13.2 Unit Test — Schema Validation

```go
func TestTransactionSchema(t *testing.T) {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    defer db.Close()

    err = RunMigrations(db)
    require.NoError(t, err)

    rows, err := db.Query("PRAGMA table_info(Transaction)")
    require.NoError(t, err)
    defer rows.Close()

    columns := make(map[string]string)
    for rows.Next() {
        var cid int
        var name, colType string
        var notNull, pk int
        var dfltValue sql.NullString
        rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
        columns[name] = colType
    }

    assert.Equal(t, "INTEGER", columns["TransactionId"])
    assert.Equal(t, "TEXT", columns["PluginSlug"])
    assert.Contains(t, columns, "CreatedAt")
}
```

### 13.3 Unit Test — FK Enforcement

```go
func TestForeignKeyEnforcement(t *testing.T) {
    db, _ := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
    defer db.Close()
    RunMigrations(db)

    _, err := db.Exec(
        "INSERT INTO Transaction (TransactionId, StatusTypeId) VALUES (1, 999)",
    )
    assert.Error(t, err, "FK constraint should reject invalid StatusTypeId")
}
```

### 13.4 Integration Test — CRUD via ORM

```go
func TestTransactionCRUD(t *testing.T) {
    db := testutil.NewTestDB(t) // in-memory + migrated + seeded
    repo := NewTransactionRepo(db)

    // Create
    tx := Transaction{PluginSlug: "my-plugin", StatusTypeId: 1, FileTypeId: 1, Amount: 29.99}
    id, err := repo.Insert(tx)
    require.NoError(t, err)
    assert.Greater(t, id, int64(0))

    // Read
    found, err := repo.FindById(id)
    require.NoError(t, err)
    assert.Equal(t, "my-plugin", found.PluginSlug)

    // Read via view
    details, err := repo.FindDetailById(id)
    require.NoError(t, err)
    assert.Equal(t, "Pending", details.StatusName)
}
```

### 13.5 Reusable Test Helper

```go
// testutil/db.go
func NewTestDB(t *testing.T) *sql.DB {
    t.Helper()
    db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })
    require.NoError(t, RunMigrations(db))
    require.NoError(t, SeedLookupTables(db))
    return db
}
```

### 13.6 What to Test

| Category | Tests |
|----------|-------|
| Schema | Table creation, column types, constraints, NOT NULL, UNIQUE, DEFAULT |
| Foreign keys | FK enforcement, cascade behavior |
| Views | Expected columns, correct JOIN output |
| CRUD | Insert, select, update, delete through ORM |
| Migrations | Idempotent (running twice doesn't break), version tracking |
| Edge cases | NULL handling, empty results, boundary values |

---

## 14. Migration Conventions

### 14.1 Per-Database Migration Folders (Split DB)

```
migrations/
├── transactions/
│   ├── 001_create_status_types.sql
│   ├── 002_create_transactions.sql
│   └── 003_add_transaction_items.sql
├── snapshots/
│   ├── 001_create_snapshots.sql
│   └── 002_add_snapshot_meta.sql
└── auth/
    ├── 001_create_users.sql
    └── 002_create_roles.sql
```

### 14.2 Migration Rules

1. Migrations are **numbered sequentially** per database (not globally)
2. Each migration targets **one database only**
3. Migration file names use **snake_case** with numeric prefix
4. Migrations are **forward-only** — no down migrations
5. Migrations MUST be **idempotent** (safe to run multiple times)
6. Views are created/updated via migrations (never manually)
7. When underlying tables change, update dependent views in the same migration

---

## 15. Schema Documentation Template

Every table MUST be documented:

```markdown
### TableName (singular)

**Purpose:** [What this table stores]  
**Expected volume:** [N rows in 10 years]

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| TableNameId | INTEGER | PK, AUTOINCREMENT | Primary key |
| ForeignTableId | INTEGER | FK → ForeignTable, NOT NULL | References ForeignTable |
| Name | TEXT | NOT NULL | Human-readable name |
| IsActive | BOOLEAN | NOT NULL DEFAULT 1 | Active status |
| CreatedAt | TEXT | NOT NULL DEFAULT CURRENT_TIMESTAMP | ISO 8601 timestamp |
```

---

## 16. AI Implementation Checklist

When creating or modifying a database schema, follow in order:

| # | Step | Rule |
|---|------|------|
| 1 | Name everything PascalCase | Tables, columns, indexes, views |
| 2 | Table names = **singular** | `User` not `Users`, `Transaction` not `Transactions` |
| 3 | PK = `{TableName}Id` | `UserId`, `TransactionId` — never bare `Id` |
| 4 | PK type = INTEGER AUTOINCREMENT | SMALLINT for lookup tables, BIGINT for 2B+ rows |
| 5 | FK column = exact PK name | `AgentSiteId` in both parent and child tables |
| 6 | Declare `FOREIGN KEY` constraint | Never rely on naming alone (same-DB only) |
| 7 | Boolean = `Is`/`Has` + positive | `IsActive`, `HasLicense` — never negative |
| 8 | Boolean = NOT NULL DEFAULT | No nullable booleans |
| 9 | No UUID | Unless distributed + public + non-guessable (all 3) |
| 10 | Right-size all columns | SMALLINT for lookup PKs, CHAR(2) for country codes |
| 11 | Extract repeated values | Lookup table + FK, SMALLINT PK |
| 12 | Create views for joins | `Vw` prefix, business layer queries views |
| 13 | ORM-only in business layer | No raw SQL outside migrations/views |
| 14 | Set SQLite PRAGMAs | WAL, foreign_keys=ON, busy_timeout=5000 |
| 15 | REST API = PascalCase JSON | Response keys match DB columns, use envelope |
| 16 | Unit test schema | In-memory SQLite, verify tables/columns/constraints |
| 17 | Integration test CRUD | In-memory SQLite, verify ORM operations |

---

## 17. Complete Schema Example

Copy-paste-ready SQL demonstrating all conventions:

```sql
-- LOOKUP TABLES (SMALLINT PKs, singular names)
CREATE TABLE StatusType (
    StatusTypeId SMALLINT PRIMARY KEY,
    Name         TEXT NOT NULL UNIQUE
);

CREATE TABLE FileType (
    FileTypeId SMALLINT PRIMARY KEY,
    Name       TEXT NOT NULL UNIQUE
);

CREATE TABLE Role (
    RoleId SMALLINT PRIMARY KEY,
    Name   TEXT NOT NULL UNIQUE
);

-- ENTITY TABLES (INTEGER PKs, AUTOINCREMENT, singular names)
CREATE TABLE AgentSite (
    AgentSiteId INTEGER PRIMARY KEY AUTOINCREMENT,
    SiteName    TEXT NOT NULL,
    SiteUrl     TEXT NOT NULL,
    IsActive    BOOLEAN NOT NULL DEFAULT 1,
    CreatedAt   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE User (
    UserId     INTEGER PRIMARY KEY AUTOINCREMENT,
    Name       TEXT NOT NULL,
    Email      TEXT NOT NULL UNIQUE,
    IsActive   BOOLEAN NOT NULL DEFAULT 1,
    IsVerified BOOLEAN NOT NULL DEFAULT 0,
    HasLicense BOOLEAN NOT NULL DEFAULT 0,
    CreatedAt  TEXT NOT NULL DEFAULT (datetime('now')),
    DeletedAt  TEXT NULL
);

CREATE TABLE Transaction (
    TransactionId INTEGER PRIMARY KEY AUTOINCREMENT,
    AgentSiteId   INTEGER NOT NULL,
    StatusTypeId  SMALLINT NOT NULL,
    FileTypeId    SMALLINT NOT NULL,
    PluginSlug    TEXT NOT NULL,
    Amount        REAL NOT NULL DEFAULT 0,
    IsActive      BOOLEAN NOT NULL DEFAULT 1,
    CreatedAt     TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (AgentSiteId)  REFERENCES AgentSite(AgentSiteId),
    FOREIGN KEY (StatusTypeId) REFERENCES StatusType(StatusTypeId),
    FOREIGN KEY (FileTypeId)   REFERENCES FileType(FileTypeId)
);

-- JUNCTION TABLE (N-to-M, singular name)
CREATE TABLE UserRole (
    UserRoleId INTEGER PRIMARY KEY AUTOINCREMENT,
    UserId     INTEGER NOT NULL,
    RoleId     SMALLINT NOT NULL,
    UNIQUE (UserId, RoleId),
    FOREIGN KEY (UserId) REFERENCES User(UserId),
    FOREIGN KEY (RoleId) REFERENCES Role(RoleId)
);

-- INDEXES (Idx{Table}_{Column})
CREATE INDEX IdxTransaction_CreatedAt    ON Transaction(CreatedAt);
CREATE INDEX IdxTransaction_StatusTypeId ON Transaction(StatusTypeId);
CREATE INDEX IdxUserRole_UserId          ON UserRole(UserId);

-- VIEWS (Vw prefix, singular)
CREATE VIEW VwTransactionDetail AS
SELECT
    t.TransactionId, t.PluginSlug, t.Amount, t.IsActive, t.CreatedAt,
    st.Name AS StatusName, ft.Name AS FileTypeName,
    a.SiteName AS AgentSiteName, a.SiteUrl AS AgentSiteUrl
FROM Transaction t
INNER JOIN StatusType st ON t.StatusTypeId = st.StatusTypeId
INNER JOIN FileType ft   ON t.FileTypeId = ft.FileTypeId
LEFT JOIN AgentSite a    ON t.AgentSiteId = a.AgentSiteId;

CREATE VIEW VwUserRoleSummary AS
SELECT
    u.UserId, u.Name AS UserName, u.Email,
    u.IsActive, u.IsVerified, u.HasLicense,
    r.Name AS RoleName
FROM User u
INNER JOIN UserRole ur ON u.UserId = ur.UserId
INNER JOIN Role r      ON ur.RoleId = r.RoleId;

-- SEED DATA
INSERT INTO StatusType (StatusTypeId, Name) VALUES (1,'Pending'),(2,'Complete'),(3,'Failed');
INSERT INTO FileType (FileTypeId, Name) VALUES (1,'Plugin'),(2,'Theme'),(3,'MuPlugin');
INSERT INTO Role (RoleId, Name) VALUES (1,'Admin'),(2,'Editor'),(3,'Viewer');
```

---

*Consolidated database conventions — v3.2.0 — 2026-04-16*
