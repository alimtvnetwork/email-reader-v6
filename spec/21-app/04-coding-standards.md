# 04 — Coding Standards (Central Reference)

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Single source of truth for **how `email-read` Go code is written**. Every feature spec under `spec/21-app/02-features/*` cites this file by section number. An AI implementing any feature must read this file once, then re-read only the cited sub-sections per task.

This file does **not** restate the consolidated guidelines — it pins the **subset that applies to this Go-only, Fyne-UI, SQLite, single-binary product** and adds project-specific decisions.

---

## Citation Map (Source of Truth)

| Topic | Authoritative source | What this file does |
|-------|---------------------|---------------------|
| Naming (PascalCase keys) | `spec/12-consolidated-guidelines/02-coding-guidelines.md` §1.1–§1.7 | Pins Go-specific application |
| Boolean principles P1–P8 | `spec/12-consolidated-guidelines/02-coding-guidelines.md` §2 | Pins positive-only rule for Go |
| Code style (braces/nesting/15-line) | `spec/12-consolidated-guidelines/02-coding-guidelines.md` §3 | Pins limits + exemptions |
| Strict typing | `spec/12-consolidated-guidelines/02-coding-guidelines.md` §5–§7 | Bans `interface{}`/`any`/casts |
| Error wrapping | `spec/12-consolidated-guidelines/03-error-management.md` §Go apperror | Maps to `internal/errtrace` (this project's wrapper) |
| DB naming (PascalCase, singular) | `spec/12-consolidated-guidelines/18-database-conventions.md` §1–§6 | Pins schema & query rules |
| CLI structure | `spec/12-consolidated-guidelines/23-generic-cli.md` §Project Structure | Pins `cmd/` + `internal/` layout |

---

## 1. File & Folder Layout (Hard Rule)

```
email-reader/
├── cmd/
│   ├── email-read/main.go            # CLI entrypoint — Cobra dispatch only
│   └── email-read-ui/main.go         # Fyne entrypoint — wires app, no logic
├── internal/
│   ├── core/                         # ✅ ONLY API surface for cmd/* and ui/*
│   │   ├── accounts.go   accounts_test.go
│   │   ├── dashboard.go  dashboard_test.go
│   │   ├── diagnose.go   diagnose_test.go
│   │   ├── emails.go     emails_test.go
│   │   ├── export.go     export_test.go
│   │   ├── read.go       read_test.go
│   │   └── rules.go      rules_test.go
│   ├── errtrace/                     # Error wrapping with file:line stacks
│   ├── config/                       # config.json load/save
│   ├── store/                        # SQLite (modernc.org/sqlite, no CGO)
│   ├── mailclient/                   # IMAP client
│   ├── imapdef/                      # IMAP server defaults per provider
│   ├── rules/                        # Regex rule engine
│   ├── browser/                      # Chrome incognito launcher
│   ├── watcher/                      # Poll loop + event fan-out
│   ├── exporter/                     # CSV export
│   ├── cli/                          # Cobra command builders
│   └── ui/                           # Fyne views (UI-only — no business logic)
│       ├── app.go nav.go sidebar.go state.go
│       └── views/<feature>.go + <feature>_format.go (+ _test.go)
├── linters/                          # phpcs / golangci / sonarqube / stylecop
└── linter-scripts/                   # validate-guidelines.{go,py}, run.{ps1,sh}
```

### Hard rules

1. **`internal/core` is the only surface** `cmd/*` and `internal/ui/*` may import. Lower packages are private to core. Enforced by `golangci-lint` `depguard` rule (see `linters/golangci-lint/.golangci.yml`).
2. **No Fyne imports outside `internal/ui/` or `cmd/email-read-ui`.** Pure formatters live in `internal/ui/views/*_format.go` and are unit-testable without Fyne.
3. **No CGO ever.** SQLite stays on `modernc.org/sqlite`. See `.lovable/strictly-avoid.md`.
4. **One file per Cobra command** in `internal/cli/`.
5. **Test file lives next to source**: `accounts.go` ↔ `accounts_test.go`.

---

## 2. Naming Conventions

### 2.1 Go identifiers

| Symbol kind | Convention | Example |
|---|---|---|
| Exported type / func / const | PascalCase | `WatchState`, `RunWatch`, `DefaultPollSeconds` |
| Unexported type / func | camelCase | `pollOnce`, `mailboxStats` |
| Acronyms | Full uppercase if exported | `IMAPClient`, `URLOpener`, `DBPath`, `IDList` |
| Receivers | 1–3 letter abbreviation | `func (w *Watcher) Tick(...)` — NOT `func (this *Watcher)` |
| Test functions | `Test{Type}_{Behavior}` | `TestWatcher_HeartbeatLogsEveryPoll` |

**Banned:**
- `Db`, `Url`, `Api`, `Id` for exported names (use `DB`, `URL`, `API`, `ID`).
- `this`, `self` as receiver names.
- Single-letter names except for: `i` (loop), `r` (reader), `w` (writer), `tx` (transaction), `ctx` (context).

### 2.2 PascalCase string keys (everywhere)

Per consolidated §1.1, ALL string keys are PascalCase. In this product:

| Surface | Example |
|---|---|
| `config.json` keys | `"Accounts"`, `"WatchPollSeconds"`, `"ChromePath"` |
| Log fields (structured) | `"Alias"`, `"Uid"`, `"MessagesCount"`, `"UidNext"` |
| SQLite columns | `EmailId`, `Uid`, `ReceivedAt`, `IsRead` |
| Event type names | `"EmailFetched"`, `"RuleMatched"`, `"BrowserLaunched"` |
| CSV header row | `EmailId,Alias,Subject,From,ReceivedAt` |

**Go struct rule**: omit `json:"..."` tags — Go marshals PascalCase by default. Only add tags for `omitempty` or `json:"-"`.

```go
// ✅ CORRECT
type Email struct {
    EmailId    int64
    Alias      string
    Subject    string
    ReceivedAt time.Time
}

// ❌ FORBIDDEN — redundant tags
type Email struct {
    EmailId int64 `json:"EmailId"`
}
```

### 2.3 Boolean naming (P1, P2 — positive only)

| ❌ Forbidden | ✅ Required |
|---|---|
| `disabled`, `notReady` | `isDisabled`? **No** → `isPaused` / `isPending` |
| `hasNoRules` | `isRuleListEmpty` |
| `isInvalid` | `isValid` (invert the check) |
| `isUnverified` | `isVerified` |
| `IsDisabled` (DB column) | `IsEnabled` |
| `noBrowser bool` | `hasBrowser bool` |

### 2.4 Variable naming

- Singular for one item: `account`, `email`, `rule`.
- Plural for collections: `accounts`, `emails`, `rules`.
- Maps suffixed with `By`: `ruleByAlias`, `accountByEmailAddress`.
- Loop var = singular of collection: `for _, rule := range rules`.

### 2.5 File naming (Go)

`snake_case.go`. Test file = `<source>_test.go`. Format helper = `<source>_format.go`.

---

## 3. Code Style — Hard Limits

| Metric | Limit | Linter rule |
|---|---|---|
| Function body | ≤ **15 lines** (excluding signature, comments, blank lines, `if err != nil { return ... }` blocks) | `funlen` |
| Struct/interface | ≤ **120 lines** | `lll` (custom) |
| File | < **300 lines** (hard max 400) | `funlen` + manual review |
| Function parameters | ≤ **3** (use a struct param if more) | `revive: max-params` |
| Cyclomatic complexity | ≤ **10** | `gocyclo` |
| Cognitive complexity | ≤ **10** | `gocognit` |
| Nesting depth | **2 levels max** (function body → one control structure) | `nestif` |
| Operands per condition | **2 max**; never mix `&&` and `\|\|` in one expression | `gocognit` + review |

### 3.1 Braces always required

```go
// ❌ FORBIDDEN
if err != nil return err

// ✅ REQUIRED
if err != nil {
    return err
}
```

### 3.2 Zero nested `if`

```go
// ❌ FORBIDDEN
if account != nil {
    if account.IsActive {
        ...
    }
}

// ✅ REQUIRED — extracted positive guard
if isAccountUsable(account) {
    ...
}

// helper:
func isAccountUsable(a *Account) bool {
    if a == nil { return false }
    return a.IsActive
}
```

### 3.3 No redundant `else` after return/throw/break/continue

```go
// ❌ FORBIDDEN
if err != nil {
    return err
} else {
    return process(value)
}

// ✅ REQUIRED
if err != nil {
    return err
}

return process(value)
```

### 3.4 Blank line before `return` (when preceded by other code) and after `}` (unless next line is `if`/`else`/`case`/`}`)

### 3.5 No raw negation on existence/system calls (P3)

```go
// ❌ FORBIDDEN
if !strings.Contains(s, "@") { ... }

// ✅ REQUIRED
if isAtSignMissing(s) { ... }

func isAtSignMissing(s string) bool {
    return !strings.Contains(s, "@")
}
```

### 3.6 No `else` chain — early return only

### 3.7 Constants & magic values (consolidated §8)

Every literal used in a comparison, switch, or key lookup MUST be a named const or typed enum.

```go
// ❌ FORBIDDEN
if event.Type == "EmailFetched" { ... }

// ✅ REQUIRED
const EventTypeEmailFetched EventType = "EmailFetched"
if event.Type == EventTypeEmailFetched { ... }
```

Group constants in `internal/core/constants.go` (per feature) or in feature-local `consts.go`.

---

## 4. Strict Typing (No `interface{}` / No `any` / No Casts)

### 4.1 Public APIs

- **No `interface{}` or `any` in exported function signatures.**
- **No type assertions** (`x.(T)`) in business logic. Allowed only at parse boundaries (JSON decode, IMAP envelope decode) with immediate validation.
- **Explicit param/return types always** — no naked returns in exported funcs.

### 4.2 Single return value via `errtrace.Result[T]`

This project uses `internal/errtrace` (not `apperror` from other projects). The wrapper is functionally equivalent.

```go
// ❌ FORBIDDEN — dual return in core layer
func GetAccount(alias string) (*Account, error) { ... }

// ✅ REQUIRED — Result[T]
func GetAccount(alias string) errtrace.Result[*Account] { ... }
```

**Exemption:** the lowest-level adapters (`internal/store`, `internal/mailclient`, stdlib calls) MAY return `(T, error)` and are immediately wrapped at the `internal/core` boundary.

### 4.3 No inline return types

```go
// ❌ FORBIDDEN
func parsePoll() struct{ Count int; UidNext int } { ... }

// ✅ REQUIRED
type MailboxStats struct {
    MessagesCount int
    UidNext       int
}

func parsePoll() MailboxStats { ... }
```

### 4.4 Generics over `interface{}`

```go
// ✅ Generic helper
func first[T any](items []T, predicate func(T) bool) (T, bool) { ... }
```

---

## 5. Error Handling (Maps to `internal/errtrace`)

Full registry & log lines: see [`06-error-registry.md`](./06-error-registry.md). This section pins the **wrapping rules**.

### 5.1 Every error must be wrapped at every layer crossing

```go
// internal/store/store.go (lowest layer — may return raw)
func (s *Store) InsertEmail(e Email) error { ... }

// internal/core/emails.go (boundary — wraps)
func SaveEmail(e Email) errtrace.Result[Email] {
    if err := store.InsertEmail(e); err != nil {
        return errtrace.Wrap(err, ErrEmailInsert, "insert email").
            WithContext("Alias", e.Alias).
            WithContext("Uid", e.Uid)
    }

    return errtrace.Ok(e)
}
```

### 5.2 Every wrap MUST include

| Field | Source | Required? |
|---|---|---|
| Error code | `ErrXxx` const from `06-error-registry.md` | ✅ |
| Operation | Short verb phrase (`"insert email"`) | ✅ |
| File:line | Auto-captured by `errtrace.Wrap` | ✅ (automatic) |
| Context | At minimum `Alias`, plus entity ID (`Uid`, `RuleId`, etc.) | ✅ |

### 5.3 🔴 CODE RED — forbidden patterns

| Pattern | Why banned |
|---|---|
| `_, err := f()` then ignore `err` | Silent failure |
| `return errors.New("x")` in core layer | No code, no context |
| `return fmt.Errorf("...: %w", err)` in core layer | No code |
| `panic()` outside `cmd/*/main.go` startup | Crashes UI |
| Empty `if err != nil { }` | Swallowed error |

### 5.4 Error code ranges (project-owned)

This project's reserved range from `spec/12-consolidated-guidelines/03-error-management.md`:

| Range | Prefix | Layer |
|---|---|---|
| `21000–21099` | `ER-CFG` | `internal/config` |
| `21100–21199` | `ER-STO` | `internal/store` |
| `21200–21299` | `ER-MAIL` | `internal/mailclient` |
| `21300–21399` | `ER-RUL` | `internal/rules` |
| `21400–21499` | `ER-WCH` | `internal/watcher` |
| `21500–21599` | `ER-BRW` | `internal/browser` |
| `21600–21699` | `ER-EXP` | `internal/exporter` |
| `21700–21799` | `ER-COR` | `internal/core` (cross-cutting) |
| `21800–21899` | `ER-CLI` | `internal/cli` |
| `21900–21999` | `ER-UI` | `internal/ui` |
| `22100–22199` | `ER-MIG` | `internal/store/migrate` (allocated Slice #155 — moved out of CLI's 21800-block to resolve range collision; see `spec/23-app-database/03-migrations.md` §9) |
| `22200–22299` | `ER-ACC` | `internal/core/accounts*.go` (allocated Slice #158 — Accounts feature had no canonical block; original spec refs at `21430/21431` collided with the Watcher block. See `spec/21-app/02-features/04-accounts/`.) |

> If new error needed, add to [`06-error-registry.md`](./06-error-registry.md) first, then reference the constant.

---

## 6. Logging (Maps to `05-logging-strategy.md`)

Every function in `internal/core` and `internal/watcher` MUST log:

1. **Entry** at `DEBUG` level with `Operation` + key context fields.
2. **Exit** at `INFO` (success) or `ERROR` (failure) with `DurationMs`.
3. **Heartbeat** for any loop — see watcher invariant (`.lovable/strictly-avoid.md`).

Format (line-delimited JSON to stderr, human-readable to stdout):

```
INFO  2026-04-25T10:14:03.221Z core.SaveEmail Alias=ab Uid=12345 DurationMs=42
ERROR 2026-04-25T10:14:03.300Z core.SaveEmail Alias=ab Uid=12345 ErrCode=ER-DB-21108 File=internal/store/store.go:184 Msg="insert email: UNIQUE constraint failed"
```

Full schema in [`05-logging-strategy.md`](./05-logging-strategy.md).

---

## 7. SQLite & Database Rules (Pins `04-database-conventions/` + `18-database-conventions.md`)

### 7.1 Naming (mandatory)

| Object | Rule | Example |
|---|---|---|
| Table | PascalCase, **singular** | `Email`, `WatchState`, `OpenedUrl`, `Account`, `Rule` |
| Primary key | `{TableName}Id` | `EmailId`, `RuleId`, `AccountId` |
| Foreign key column | Exact same name as referenced PK | `Email.AccountId` → `Account.AccountId` |
| Boolean column | `Is`/`Has` prefix, **positive**, `NOT NULL DEFAULT` | `IsRead`, `HasAttachment` |
| Index | `Idx{Table}_{Column}` | `IdxEmail_ReceivedAt` |
| View | `Vw{Name}` (singular) | `VwEmailDetail` |
| Timestamp | `TEXT` ISO-8601 (`YYYY-MM-DDTHH:MM:SSZ`) | `ReceivedAt`, `CreatedAt` |

### 7.2 Required PRAGMAs (every connection)

```go
db.Exec(`PRAGMA journal_mode=WAL`)
db.Exec(`PRAGMA foreign_keys=ON`)
db.Exec(`PRAGMA busy_timeout=5000`)
```

### 7.3 ORM rule

Raw SQL is allowed in `internal/store` only. `internal/core` MUST call `store.*` repository methods. No SQL strings in `cmd/*` or `internal/ui/*`.

### 7.4 Struct tags

```go
type Email struct {
    EmailId    int64  `db:"EmailId"`
    AccountId  int64  `db:"AccountId"`
    Uid        uint32 `db:"Uid"`
    Subject    string `db:"Subject"`
    ReceivedAt string `db:"ReceivedAt"`
    IsRead     bool   `db:"IsRead"`
}
```

`db:` tags required. `json:` tags omitted.

---

## 8. Testing Standards

### 8.1 Coverage targets

| Package | Target |
|---|---|
| `internal/core/*` | ≥ **90%** |
| `internal/errtrace`, `internal/rules`, `internal/imapdef` | **100%** |
| `internal/store` | ≥ **85%** |
| `internal/ui/views/*_format.go` | ≥ **90%** (pure functions only) |
| `internal/ui/*` (Fyne widgets) | Smoke test only |

### 8.2 Test style

- **Table-driven** for any function with > 1 input shape.
- One `_test.go` per source file.
- No `time.Sleep` in tests — use injectable clocks.
- No network in tests — mock `internal/mailclient` via interface.
- `go test -tags nofyne ./...` MUST pass on a sandbox without Fyne.

### 8.3 Build tags

| Tag | Purpose |
|---|---|
| (default) | Full build including Fyne UI |
| `nofyne` | Skips `internal/ui` and `cmd/email-read-ui` — used by CI / sandbox |

---

## 9. Cobra CLI Conventions (Pins `23-generic-cli.md`)

### 9.1 One file per command

`internal/cli/<command>.go` — exports `New<Command>Cmd(coreDeps Deps) *cobra.Command`.

### 9.2 Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error |
| `2` | Usage / invalid args |
| `3` | Configuration error (`ER-CFG-*`) |
| `4` | Database error (`ER-DB-*`) |
| `5` | Network / IMAP error (`ER-MAIL-*`) |

### 9.3 Output

| Stream | Content |
|---|---|
| `stdout` | Primary output (table / JSON / CSV) |
| `stderr` | Logs, progress, errors |

`--format=table\|csv\|json\|markdown`. Default `table`. Same data shape across formats.

### 9.4 Verbose

`--verbose` (`-v`) toggles DEBUG logging to stderr. Never on by default.

---

## 10. Versioning Rule (Project-Specific)

Every code change bumps **at least the minor version** of both binaries:

- `cmd/email-read/main.go` → `Version = "x.y.z"`
- `cmd/email-read-ui/main.go` → `Version = "x.y.z"`

Both constants MUST stay in lockstep. Enforced by `linter-scripts/validate-guidelines.go`.

---

## 11. Forbidden Patterns (Project-Specific)

| Pattern | Reason |
|---|---|
| `import "C"` anywhere | No CGO (sandbox + cross-compile) |
| `time.Sleep` in production code | Use `time.Ticker` / context |
| `os.Exit` outside `cmd/*/main.go` | Breaks tests |
| `fmt.Println` in `internal/*` | Use logger |
| `panic()` outside startup | Crashes Fyne UI |
| `recover()` outside watcher's top-level loop | Hides bugs |
| Direct `*sql.DB` access outside `internal/store` | ORM rule |
| Fyne import outside `internal/ui` / `cmd/email-read-ui` | Layering rule |
| `interface{}` / `any` in exported core APIs | Strict typing rule |

---

## 12. Linter Configuration

| Linter | Config | Enforces |
|---|---|---|
| `golangci-lint` | `linters/golangci-lint/.golangci.yml` | All Go rules above |
| Custom validator | `linter-scripts/validate-guidelines.go` | PascalCase, version lockstep, layering, error registry |

CI fails on any `golangci-lint run` violation. Local: `linter-scripts/run.sh` (Linux/macOS) or `linter-scripts/run.ps1` (Windows).

---

## 13. PR Checklist (Cite This in Every Feature Spec's Acceptance Criteria)

- [ ] Every new exported func has `errtrace.Result[T]` return (no `(T, error)`)
- [ ] Every error wrapped with code from `06-error-registry.md` + `Alias` context
- [ ] Every function ≤ 15 lines (excluding error wraps)
- [ ] Zero nested `if`; no `!` on existence checks
- [ ] All booleans `is`/`has` prefixed and **positive**
- [ ] All string keys (config / logs / events / DB) PascalCase
- [ ] No `interface{}`/`any` in exported signatures
- [ ] Test file present; `go test -tags nofyne ./...` passes
- [ ] No Fyne import outside `internal/ui` / `cmd/email-read-ui`
- [ ] Version bumped in both `main.go` files
- [ ] `linter-scripts/run.sh` (or `.ps1`) clean

---

## Cross-References

| Reference | Location |
|---|---|
| Fundamentals | [01-fundamentals.md](./01-fundamentals.md) |
| Logging strategy | [05-logging-strategy.md](./05-logging-strategy.md) |
| Error registry | [06-error-registry.md](./06-error-registry.md) |
| Architecture | [07-architecture.md](./07-architecture.md) |
| DB schema | [../23-app-database/00-overview.md](../23-app-database/00-overview.md) |
| Fyne design system | [../24-app-design-system-and-ui/00-overview.md](../24-app-design-system-and-ui/00-overview.md) |
| Consolidated coding guide | [../12-consolidated-guidelines/02-coding-guidelines.md](../12-consolidated-guidelines/02-coding-guidelines.md) |
| Consolidated error mgmt | [../12-consolidated-guidelines/03-error-management.md](../12-consolidated-guidelines/03-error-management.md) |
| Consolidated DB conventions | [../12-consolidated-guidelines/18-database-conventions.md](../12-consolidated-guidelines/18-database-conventions.md) |
| Consolidated CLI guide | [../12-consolidated-guidelines/23-generic-cli.md](../12-consolidated-guidelines/23-generic-cli.md) |
