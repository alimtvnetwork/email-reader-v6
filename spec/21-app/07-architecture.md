# 07 — Architecture

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **single architectural source of truth** for the Go/Fyne application. Defines the package layout, the dependency graph (a strict DAG), the public API surface of `internal/core`, and the rules every package must obey.

Implementers and AI agents MUST treat the dependency graph in §3 as **enforceable**: any import that violates it is a build-blocking defect.

Cross-references:
- Coding rules: [`spec/21-app/04-coding-standards.md`](./04-coding-standards.md)
- Logging: [`spec/21-app/05-logging-strategy.md`](./05-logging-strategy.md)
- Errors: [`spec/21-app/06-error-registry.md`](./06-error-registry.md)
- Guidelines: `spec/12-consolidated-guidelines/02-coding-guidelines.md`, `13-app.md`, `23-generic-cli.md`

---

## 1. Top-Level Repository Layout

```
.
├── cmd/
│   ├── email-read/            # CLI entrypoint (no Fyne)
│   │   └── main.go
│   └── email-read-ui/         # Fyne GUI entrypoint
│       └── main.go
├── internal/
│   ├── core/                  # Pure business logic. NO Fyne, NO os.Exit, NO log to stdout.
│   ├── cli/                   # Cobra/flag dispatch + CLI-only formatters
│   ├── config/                # Seedable config loader (PascalCase JSON)
│   ├── errtrace/              # Error wrapping + Result[T] envelope
│   ├── store/                 # SQLite (modernc.org/sqlite, no CGO) + migrations
│   ├── mailclient/            # IMAP/SMTP transport
│   ├── imapdef/               # IMAP command definitions / capabilities
│   ├── rules/                 # Rule engine (pure)
│   ├── browser/               # OS-level URL/file open
│   ├── exporter/              # CSV / JSON / NDJSON writers
│   ├── watcher/               # Long-running poll loop + heartbeat
│   └── ui/                    # Fyne widgets, themes, layout — sole Fyne importer
├── spec/                      # Specifications (this folder)
├── linters/                   # External linter configs
├── linter-scripts/            # Linter runner scripts
└── data/                      # Runtime: db, logs, exports (gitignored)
```

### 1.1 Forbidden locations

- ❌ `pkg/` — all internal code lives in `internal/`.
- ❌ Top-level `main.go` — entrypoints live under `cmd/`.
- ❌ `internal/common`, `internal/utils`, `internal/helpers` — junk-drawer packages are banned. Helpers belong in the package that owns the type.

---

## 2. Package Roles & Boundaries

| Package              | Role                                                          | May import                                                          | May NOT import                              |
|----------------------|---------------------------------------------------------------|---------------------------------------------------------------------|---------------------------------------------|
| `cmd/email-read`     | CLI entrypoint                                                | `internal/cli`, `internal/config`, `internal/errtrace`              | `internal/ui`, `fyne.io/*`                  |
| `cmd/email-read-ui`  | GUI entrypoint                                                | `internal/ui`, `internal/config`, `internal/errtrace`               | `internal/cli`                              |
| `internal/cli`       | Subcommand dispatch, flag parsing, text formatters            | `internal/core`, `internal/errtrace`, `internal/exporter`           | `internal/ui`, `fyne.io/*`                  |
| `internal/ui`        | Fyne widgets, themes, layout, view-models                     | `internal/core`, `internal/errtrace`, `fyne.io/fyne/v2/*`           | `internal/cli`                              |
| `internal/core`      | Business logic (Dashboard, Emails, Rules, Accounts, …)        | `internal/store`, `internal/mailclient`, `internal/rules`, `internal/errtrace`, `internal/config` | `internal/ui`, `internal/cli`, `fyne.io/*`, `os.Exit`, `fmt.Print*` |
| `internal/store`     | SQLite access + migrations                                    | `internal/errtrace`, `modernc.org/sqlite`                           | `internal/core`, `internal/ui`              |
| `internal/mailclient`| IMAP/SMTP transport                                           | `internal/errtrace`, `internal/imapdef`                             | `internal/core`, `internal/store`, `internal/ui` |
| `internal/imapdef`   | IMAP command/capability constants                             | (stdlib only)                                                       | everything else                             |
| `internal/rules`     | Pure rule evaluator                                           | `internal/errtrace`                                                 | `internal/store`, `internal/mailclient`     |
| `internal/browser`   | OS browser/file open                                          | `internal/errtrace`                                                 | everything else                             |
| `internal/exporter`  | CSV / JSON / NDJSON writers                                   | `internal/errtrace`                                                 | `internal/store`, `internal/ui`             |
| `internal/watcher`   | Poll loop, heartbeat, event bus                               | `internal/core`, `internal/errtrace`, `internal/config`             | `internal/ui`, `internal/cli`               |
| `internal/config`    | Seedable JSON config loader                                   | `internal/errtrace`                                                 | every business package                      |
| `internal/errtrace`  | `Wrap`, `New`, `Format`, `Frames`, `Result[T]`                | (stdlib only)                                                       | every other internal package                |

---

## 3. Dependency Graph (Enforceable DAG)

```
                ┌─────────────────────┐         ┌─────────────────────┐
                │  cmd/email-read     │         │  cmd/email-read-ui  │
                └─────────┬───────────┘         └─────────┬───────────┘
                          │                               │
                          ▼                               ▼
                ┌─────────────────────┐         ┌─────────────────────┐
                │   internal/cli      │         │    internal/ui      │
                └─────────┬───────────┘         └─────────┬───────────┘
                          │                               │
                          └───────────────┬───────────────┘
                                          ▼
                              ┌───────────────────────┐
                              │    internal/core      │ ◄── internal/watcher
                              └───────────┬───────────┘
                                          │
                  ┌───────────────────────┼───────────────────────┐
                  ▼                       ▼                       ▼
       ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────┐
       │  internal/store    │  │ internal/mailclient│  │  internal/rules    │
       └────────────────────┘  └─────────┬──────────┘  └────────────────────┘
                                         ▼
                              ┌────────────────────┐
                              │  internal/imapdef  │
                              └────────────────────┘

       (everyone)  ─────────►  internal/errtrace, internal/config, internal/browser, internal/exporter
```

### 3.1 Invariants

1. `internal/core` is the **only** caller of `internal/store` and `internal/mailclient` from above.
2. `internal/cli` and `internal/ui` are **siblings** — neither imports the other.
3. `internal/errtrace` has **zero** internal imports (stdlib only) — it is the bottom of the DAG.
4. `internal/watcher` consumes `internal/core` and is consumed by `internal/ui` only via channels/callbacks (no direct widget calls from watcher).
5. There are **no cycles**. CI runs `go list -deps` checks against this graph.

---

## 4. `internal/core` Public API Surface

`internal/core` exposes **seven service structs**, one per feature spec under `02-features/`. Each method returns `errtrace.Result[T]` for single values or `(T, error)` only when `T` is `void`-equivalent (use `Result[Unit]`).

### 4.1 `core.Dashboard`

```go
type Dashboard struct{ /* injected: Store, Clock */ }

func NewDashboard(s store.Store, c Clock) *Dashboard
func (d *Dashboard) Summary(ctx context.Context) errtrace.Result[DashboardSummary]
func (d *Dashboard) RecentActivity(ctx context.Context, limit int) errtrace.Result[[]ActivityRow]
func (d *Dashboard) AccountHealth(ctx context.Context) errtrace.Result[[]AccountHealthRow]
```

### 4.2 `core.Emails`

```go
type Emails struct{ /* Store, MailClient, Rules */ }

func (e *Emails) List(ctx context.Context, q EmailQuery) errtrace.Result[EmailPage]
func (e *Emails) Get(ctx context.Context, alias string, uid uint32) errtrace.Result[Email]
func (e *Emails) MarkRead(ctx context.Context, alias string, uids []uint32) errtrace.Result[Unit]
func (e *Emails) Delete(ctx context.Context, alias string, uids []uint32) errtrace.Result[Unit]
func (e *Emails) Refresh(ctx context.Context, alias string) errtrace.Result[RefreshReport]
```

### 4.3 `core.Rules`

```go
type Rules struct{ /* Store, RuleEngine */ }

func (r *Rules) List(ctx context.Context) errtrace.Result[[]Rule]
func (r *Rules) Create(ctx context.Context, rule RuleSpec) errtrace.Result[Rule]
func (r *Rules) Update(ctx context.Context, id RuleId, rule RuleSpec) errtrace.Result[Rule]
func (r *Rules) Delete(ctx context.Context, id RuleId) errtrace.Result[Unit]
func (r *Rules) DryRun(ctx context.Context, id RuleId, sample EmailSample) errtrace.Result[RuleMatch]
```

### 4.4 `core.Accounts`

```go
type Accounts struct{ /* Store, MailClient */ }

func (a *Accounts) List(ctx context.Context) errtrace.Result[[]Account]
func (a *Accounts) Add(ctx context.Context, spec AccountSpec) errtrace.Result[Account]
func (a *Accounts) Remove(ctx context.Context, alias string) errtrace.Result[Unit]
func (a *Accounts) TestConnection(ctx context.Context, alias string) errtrace.Result[ConnectionReport]
```

### 4.5 `core.Watch`

```go
type Watch struct{ /* Store, MailClient, EventBus */ }

func (w *Watch) Start(ctx context.Context, opts WatchOptions) errtrace.Result[Unit]
func (w *Watch) Stop(ctx context.Context) errtrace.Result[Unit]
func (w *Watch) Status(ctx context.Context) errtrace.Result[WatchStatus]
func (w *Watch) Subscribe() <-chan WatchEvent  // closed on Stop
```

### 4.6 `core.Tools`

Canonical surface — see `02-features/06-tools/01-backend.md` §2 for full contracts.

```go
type Tools struct{ /* Store, Mailcli, Exporter, Browser, Accounts, Paths, Cfg, Clock, EventBus */ }

// Public API (5 methods)
func (t *Tools) ReadOnce(ctx context.Context, spec ReadSpec, progress chan<- string) errtrace.Result[ReadResult]
func (t *Tools) ExportCsv(ctx context.Context, spec ExportSpec, progress chan<- ExportProgress) errtrace.Result[ExportReport]
func (t *Tools) Diagnose(ctx context.Context, spec DiagnoseSpec, progress chan<- DiagnosticsStep) errtrace.Result[DiagnosticsReport]
func (t *Tools) OpenUrl(ctx context.Context, spec OpenUrlSpec) errtrace.Result[OpenUrlReport]
func (t *Tools) RecentOpenedUrls(ctx context.Context, spec OpenedUrlListSpec) errtrace.Result[[]OpenedUrlRow]

// Internal (event-bus subscriber, 1 method)
func (t *Tools) OnAccountUpdate(ctx context.Context, alias string) errtrace.Result[Unit]
```

### 4.7 `core.Settings`

```go
type Settings struct{ /* Config, Store */ }

func (s *Settings) Get(ctx context.Context) errtrace.Result[Settings]
func (s *Settings) Update(ctx context.Context, patch SettingsPatch) errtrace.Result[Settings]
func (s *Settings) ResetToDefaults(ctx context.Context) errtrace.Result[Unit]
```

---

## 5. Cross-Cutting Types

```go
package core

type Unit struct{}                 // for void-equivalent results

type Clock interface {
    Now() time.Time
}

type EventBus interface {
    Publish(WatchEvent)
    Subscribe() <-chan WatchEvent
}
```

All exported field names use **PascalCase**. JSON tags are PascalCase. SQLite column names are PascalCase. (See `04-coding-standards.md` §1.1.)

---

## 6. Lifecycle & Composition Root

Both `cmd/email-read/main.go` and `cmd/email-read-ui/main.go` follow the same composition order:

```
1. config.Load()                  → returns Config
2. errtrace.InstallSignalHandlers (FATAL on SIGTERM/SIGINT)
3. store.Open(config.DbPath)      → returns Store
4. mailclient.New(config.MailRc)  → returns MailClient
5. rules.NewEngine()              → returns RuleEngine
6. core.NewDashboard / Emails / Rules / Accounts / Watch / Tools / Settings
7. cli.Run(services) OR ui.Run(services)
```

No package-level globals. No `init()` side effects beyond constant registration. The composition root is the **only** place dependencies are wired.

---

## 7. Testability Contract

- Every `internal/core` service accepts its dependencies via constructor → trivial fakes in tests.
- `internal/store` ships an in-memory variant (`store.NewMemory()`) used by core tests.
- `internal/mailclient` ships a `mailclient.Fake` honoring the same interface.
- `internal/ui` widgets accept a `core.*` service interface (not the concrete struct) so tests can drive widgets with stubs.

Coverage targets (from `04-coding-standards.md`):
- `internal/core`: ≥ 90 %
- `internal/store`, `internal/rules`, `internal/exporter`: ≥ 85 %
- `internal/ui`: smoke tests only (Fyne `test.NewApp()`)

---

## 8. CI Enforcement

The CI pipeline (defined in `linter-scripts/run.sh`) runs:

1. `go vet ./...`
2. `golangci-lint run` (config in `linters/golangci-lint/.golangci.yml`)
3. `linter-scripts/validate-guidelines.go` — checks PascalCase keys, 15-line function limit, banned imports.
4. `go test -race -cover ./...`
5. **Architecture check**: a `go list -deps` script walks every package and fails if any import violates the table in §2.

A failing architecture check is a **release blocker**.

---

## 9. Compliance Checklist

- [x] Cites `02-coding-guidelines.md`, `13-app.md`, `23-generic-cli.md` from `spec/12-consolidated-guidelines/`.
- [x] Cross-references `04-coding-standards.md`, `05-logging-strategy.md`, `06-error-registry.md`.
- [x] Defines a strict DAG with no cycles.
- [x] Bans `internal/common`, `internal/utils`, `pkg/`, top-level `main.go`.
- [x] All identifiers PascalCase.
- [x] No mention of Supabase, React, Tailwind, or any web stack.
- [x] Production-Ready / Ambiguity: None.

---

**End of `07-architecture.md`**
