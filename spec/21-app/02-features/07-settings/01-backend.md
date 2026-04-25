# 07 — Settings — Backend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the `core.Settings` service: the canonical `Settings` struct, the `config.json` schema slice it owns, the atomic write protocol, validation rules, the `SettingsEvent` fan-out (live-reload of poll interval, theme, browser overrides), Chrome auto-detection, and the error registry block `21770–21789`.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.7
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md) (PascalCase keys §1.1, ≤15-line fns §3, no `any`/`interface{}` §6)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.7
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21770–21789` (Settings) + wrapped `ER-CFG-21000..21009` (config IO), `ER-COR-21704` (path escape)
- DB conventions: `spec/12-consolidated-guidelines/18-database-conventions.md`
- Watcher consumer: [`../05-watch/01-backend.md`](../05-watch/01-backend.md) §6 (poll interval reload)
- Browser consumer: [`../06-tools/01-backend.md`](../06-tools/01-backend.md) §3.4 (chrome path/incognito)
- UI consumer: [`./02-frontend.md`](./02-frontend.md)

---

## 1. Service Definition

```go
// Package core — file: internal/core/settings.go
package core

type Settings struct {
    store     store.Store        // not used today; reserved for future per-profile rows
    cfg       config.Manager     // owns config.json read/write + atomic swap
    bus       eventbus.Publisher // emits SettingsEvent
    paths     paths.Validator    // ER-COR-21704 guard for ChromePath / DataDir display
    chrome    browser.Detector   // pure function: probes config / env / OS / PATH
    clock     Clock
    log       Logger

    mu        sync.RWMutex       // guards lastApplied snapshot
    lastApplied SettingsSnapshot // last successfully written values
}

type SettingsConfig struct {
    PollSecondsMin uint16 // 1
    PollSecondsMax uint16 // 60
}
```

**Construction.** `NewSettings(deps SettingsDeps) errtrace.Result[*Settings]` — fails if `cfg`, `bus`, `paths`, `chrome`, `clock`, `log` are nil (`ER-COR-21770`). On success, calls `cfg.LoadSnapshot()` once and caches `lastApplied`.

---

## 2. Public API

All methods return `errtrace.Result[T]` per `03-error-management.md` §4.

```go
// Read the current persisted Settings.
func (s *Settings) Get(ctx context.Context) errtrace.Result[SettingsSnapshot]

// Write — validates, persists atomically, emits SettingsEvent.
// MUST NOT touch Accounts[] or Rules[] arrays in config.json.
func (s *Settings) Save(ctx context.Context, in SettingsInput) errtrace.Result[SettingsSnapshot]

// Reset only PollSeconds, BrowserOverride, Theme to defaults.
// Accounts and Rules are untouched.
func (s *Settings) ResetToDefaults(ctx context.Context) errtrace.Result[SettingsSnapshot]

// Pure (no IO except OS detection); safe to call from UI thread.
func (s *Settings) DetectChrome(ctx context.Context) errtrace.Result[ChromeDetection]

// Live-reload subscription — receivers MUST drain promptly.
func (s *Settings) Subscribe(ctx context.Context) (<-chan SettingsEvent, func())
```

---

## 3. Domain Types

PascalCase keys, no abbreviations, no `interface{}` (per `02-coding-guidelines.md` §1.1, §6).

```go
type SettingsSnapshot struct {
    PollSeconds        uint16            // 1..60
    BrowserOverride    BrowserOverride
    Theme              ThemeMode         // Dark | Light | System
    OpenUrlAllowedSchemes []string       // canonical lower-case, sorted, deduped — defaults: ["https"]
    AllowLocalhostUrls bool              // default: false
    AutoStartWatch     bool              // default: true
    ConfigPath         string            // read-only, absolute
    DataDir            string            // read-only, absolute
    EmailArchiveDir    string            // read-only, absolute
    UiStateFile        string            // read-only, absolute
    UpdatedAt          time.Time
}

type SettingsInput struct {
    PollSeconds        uint16
    BrowserOverride    BrowserOverride
    Theme              ThemeMode
    OpenUrlAllowedSchemes []string
    AllowLocalhostUrls bool
    AutoStartWatch     bool
}

type BrowserOverride struct {
    ChromePath   string // "" → auto-detect at launch time
    IncognitoArg string // "" → auto-pick per detected browser family
}

type ThemeMode uint8
const (
    ThemeDark   ThemeMode = 1
    ThemeLight  ThemeMode = 2
    ThemeSystem ThemeMode = 3
)

type ChromeDetection struct {
    Path   string
    Source ChromeDetectionSource // Config | Env | OsDefault | Path | NotFound
}

type ChromeDetectionSource uint8
const (
    ChromeFromConfig    ChromeDetectionSource = 1
    ChromeFromEnv       ChromeDetectionSource = 2
    ChromeFromOsDefault ChromeDetectionSource = 3
    ChromeFromPath      ChromeDetectionSource = 4
    ChromeNotFound      ChromeDetectionSource = 5
)

type SettingsEvent struct {
    Kind     SettingsEventKind
    Snapshot SettingsSnapshot
    At       time.Time
}

type SettingsEventKind uint8
const (
    SettingsSaved        SettingsEventKind = 1
    SettingsResetApplied SettingsEventKind = 2
)
```

**Defaults** (used by `ResetToDefaults` and as `cfg` fallback values):

| Field | Default |
|---|---|
| `PollSeconds` | `3` |
| `BrowserOverride.ChromePath` | `""` |
| `BrowserOverride.IncognitoArg` | `""` |
| `Theme` | `ThemeDark` |
| `OpenUrlAllowedSchemes` | `["https"]` |
| `AllowLocalhostUrls` | `false` |
| `AutoStartWatch` | `true` |

---

## 4. config.json Schema Slice (PascalCase)

`Settings` owns exactly these top-level objects in `config.json`. Anything else (Accounts, Rules, schemaVersion) is read but never mutated.

```json
{
  "Watch": { "PollSeconds": 3, "AutoStart": true },
  "Browser": {
    "ChromePath": "",
    "IncognitoArg": "",
    "OpenUrlAllowedSchemes": ["https"],
    "AllowLocalhostUrls": false
  },
  "Ui": { "Theme": "Dark" }
}
```

`Theme` is serialized as `"Dark" | "Light" | "System"` (string, not uint) to keep `config.json` human-editable. The internal `ThemeMode` enum is the in-memory form only.

---

## 5. Save Pipeline

`Save` is a 7-step pipeline. Every step short-circuits on first error via `errtrace.Wrap`.

1. **Normalize** — lower-case `OpenUrlAllowedSchemes`, dedupe, sort; trim `BrowserOverride.*`.
2. **Validate** — see §6. Errors use codes `21771–21777`.
3. **Snapshot read** — `cfg.LoadSnapshot()` returns the full current `config.json` parse tree.
4. **Patch** — replace only the four owned objects; leave `Accounts`, `Rules`, and unknown keys byte-identical.
5. **Atomic write** — `cfg.WriteAtomic(snapshot)` (write to `config.json.tmp`, fsync, rename). Wrapped error `ER-CFG-21001`.
6. **Cache** — under `s.mu.Lock()`, set `lastApplied = newSnapshot` (with `UpdatedAt = clock.Now()`).
7. **Publish** — non-blocking `bus.Publish(SettingsEvent{Kind: SettingsSaved, ...})`. A full subscriber buffer is logged at WARN (`SettingsEventDropped`) but never fails `Save`.

`ResetToDefaults` runs the same pipeline with `Kind = SettingsResetApplied` and only resets the three categories listed in `00-overview.md`.

---

## 6. Validation Rules

| Code | Field | Rule | Error message (logger field `code`) |
|---|---|---|---|
| `ER-SET-21771` | `PollSeconds` | `1 ≤ v ≤ 60` | `poll seconds out of range` |
| `ER-SET-21772` | `Theme` | one of `Dark`/`Light`/`System` | `unknown theme mode` |
| `ER-SET-21773` | `OpenUrlAllowedSchemes` | non-empty; each matches `^[a-z][a-z0-9+\-.]*$`; no `file`, `javascript`, `data`, `vbscript` | `disallowed url scheme` |
| `ER-SET-21774` | `BrowserOverride.ChromePath` | when non-empty: absolute path, exists, executable, passes `paths.Validator` | `chrome path invalid` |
| `ER-SET-21775` | `BrowserOverride.IncognitoArg` | when non-empty: matches `^--?[a-zA-Z][a-zA-Z0-9\-]*$` | `incognito arg malformed` |
| `ER-SET-21776` | `AllowLocalhostUrls` | bool, no extra rule (kept for symmetry) | n/a |
| `ER-SET-21777` | composite | `AllowLocalhostUrls=true` requires `OpenUrlAllowedSchemes` to include `http` (UI hint) | `localhost requires http scheme` |

Read-only display fields (`ConfigPath`, `DataDir`, `EmailArchiveDir`, `UiStateFile`) are never accepted from `SettingsInput` — they are populated from `cfg` on every `Get`.

---

## 7. Chrome Detection

`DetectChrome` is a pure function (modulo OS calls); no IO into `config.json`.

Probe order (first hit wins):

1. `BrowserOverride.ChromePath` from current snapshot → `ChromeFromConfig`.
2. Env var `MAILPULSE_CHROME` → `ChromeFromEnv`.
3. OS-default well-known locations (table per platform: `/Applications/Google Chrome.app/...`, `C:\Program Files\Google\Chrome\Application\chrome.exe`, `/usr/bin/google-chrome`, etc.) → `ChromeFromOsDefault`.
4. `exec.LookPath("google-chrome")`, then `chromium`, `microsoft-edge`, `brave-browser` → `ChromeFromPath`.
5. Otherwise `ChromeNotFound` (NOT an error — UI shows a guidance banner).

Detection MUST NOT launch the browser; only `os.Stat` + `exec.LookPath`.

---

## 8. Live-Reload Fan-Out

`SettingsEvent` consumers (declared in their respective backend specs):

| Consumer | Reacts to | Behavior |
|---|---|---|
| `core.Watcher` | `PollSeconds` change | Applies to next loop iteration; in-flight poll is **not** interrupted. |
| `core.Tools` (OpenUrl) | `BrowserOverride`, `OpenUrlAllowedSchemes`, `AllowLocalhostUrls` | Cached on next call; never mid-launch. |
| `internal/ui/theme` | `Theme` change | Calls `fyne.CurrentApp().Settings().SetTheme(...)` on UI goroutine. |
| `internal/ui/views/dashboard` | `AutoStartWatch` change | Updates the "Auto-start on launch" indicator only; does not start/stop the running watcher. |

`AutoStartWatch` is **read once at process start** by `cmd/mailpulse/main.go` to decide whether to auto-start the watcher; toggling it later only affects the next launch.

---

## 9. Concurrency

- `Save` and `ResetToDefaults` are serialized by an internal `sync.Mutex` so two concurrent saves never interleave step 4–5.
- `Get` holds `s.mu.RLock()` only for the cached snapshot copy (zero IO).
- `Subscribe` returns a buffered channel (cap 4); the cancel func closes the unsubscribe signal — events emitted after cancel are dropped silently.

---

## 10. Error Registry — Block 21770–21789

| Code | Layer | Recovery |
|---|---|---|
| `ER-SET-21770` | construct | nil dependency — fix wiring |
| `ER-SET-21771..21777` | validate | inline UI message; no retry |
| `ER-SET-21778` | persist | wraps `ER-CFG-21001`; UI shows "could not save" + retry button |
| `ER-SET-21779` | persist | snapshot diff detected concurrent edit (file mtime changed during write) — auto-reload + re-prompt |
| `ER-SET-21780` | detect | `os.Stat` denied — surface as warning, NOT failure |
| `ER-SET-21781` | event | subscriber buffer full — WARN log only |
| `ER-SET-21782..21789` | reserved | future use |

All wraps use `apperror.Wrap(err, "ER-SET-NNNNN", "...")` per `03-error-management.md` §3.

---

## 11. Logging

Per `05-logging-strategy.md` §6.7. Every Settings log entry carries:

- `component=settings`
- `op` ∈ `get | save | reset | detect_chrome | publish`
- `trace_id` (from ctx)
- On `save` / `reset`: `dirty_fields=[…]` (names only, never values)
- On `detect_chrome`: `source` (enum name), never the full path at INFO

**Never logged** (PII / fingerprinting risk): full `ChromePath`, env-var values, IncognitoArg contents. Logged only at DEBUG with the `redact=true` field gate.

---

## 12. Testing Contract

All tests live in `internal/core/settings_test.go` (unit) and `internal/core/settings_integration_test.go` (with real tempdir `config.json`). 31 required cases:

**Get / construction (4):**
1. nil dep → `ER-SET-21770`.
2. Fresh load returns defaults when `config.json` lacks the four owned objects.
3. `Get` after `Save` returns the saved snapshot byte-equivalent.
4. `Get` returns absolute paths even when `cfg` holds relative ones.

**Validation (10):** one positive + one negative for each of `21771..21777`, plus the composite `21777`.

**Save pipeline (8):**
12. Save preserves unknown top-level keys in `config.json` byte-identically.
13. Save preserves `Accounts` array byte-identically (golden-file diff).
14. Save preserves `Rules` array byte-identically (golden-file diff).
15. Atomic write: kill -9 simulation between `tmp` write and `rename` leaves original intact.
16. Concurrent `Save` × 2 produces exactly one of the two snapshots, never a merge.
17. `Save` emits exactly one `SettingsEvent` with `Kind = SettingsSaved`.
18. `ResetToDefaults` emits `Kind = SettingsResetApplied` and does NOT mutate `Accounts`/`Rules`.
19. `Save` with no dirty fields still writes (debounce is a UI concern, not core).

**DetectChrome (5):**
20. Config path takes precedence over env.
21. Env takes precedence over OS-default.
22. PATH lookup honors fallback browser order: chrome → chromium → edge → brave.
23. `ChromeNotFound` is NOT an error.
24. `os.Stat` permission denied → `ER-SET-21780` WARN only, returns `ChromeNotFound`.

**Subscribe (2):**
25. Subscribe → Save → event delivered within 50 ms.
26. Cancel func stops delivery; later Save does not panic.

**Anti-features (2):**
27. AST scan: no other package writes `config.json` directly (only `internal/config`).
28. AST scan: `Settings.Save` body never references `Accounts` or `Rules` field names.

**Logging (2):**
29. `dirty_fields` log field never contains a value, only field names.
30. `ChromePath` value never appears at INFO or above (regex scan of test logger output).

**Race (1):**
31. `go test -race` clean across all of the above.

---

## 13. File Layout

```
internal/core/
  settings.go                 // service + Save/Get/Reset (≤15-line fns)
  settings_types.go           // structs + enums + defaults
  settings_validate.go        // §6 rules, one fn per code
  settings_event.go           // bus integration
  settings_test.go
  settings_integration_test.go

internal/config/
  manager.go                  // LoadSnapshot / WriteAtomic — owned by config pkg, NOT settings
```

`internal/core/settings.go` is the only file in `internal/core` permitted to import `internal/config`. Verified by AST test in §12.
