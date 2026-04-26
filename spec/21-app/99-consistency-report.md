# Consistency Report — 21-app (Project-Wide)

**Version:** 2.0.0
**Updated:** 2026-04-25
**Status:** Final — supersedes v1.0.0
**Scope:** End-of-authoring sweep across `spec/21-app/`, `spec/23-app-database/`, and `spec/24-app-design-system-and-ui/`. Per-feature `99-consistency-report.md` files remain authoritative for their own area; this file consolidates **cross-feature** consistency findings and the merge-readiness checklist.

---

## 1. Module Health (top-level)

| Criterion                                       | Status                                                                 |
|-------------------------------------------------|------------------------------------------------------------------------|
| `00-overview.md` present                        | ✅                                                                     |
| `01-fundamentals.md` present                    | ✅                                                                     |
| `04-coding-standards.md` present                | ✅ (Task #03)                                                          |
| `05-logging-strategy.md` present                | ✅ (Task #04)                                                          |
| `06-error-registry.md` present                  | ✅ (Task #05) — 11 prefixes, 63 codes registered                       |
| `07-architecture.md` present                    | ✅ (Task #06)                                                          |
| `97-acceptance-criteria.md` present             | ✅ — v2.0.0, 35 AC-PROJ rows + 700-criterion roll-up                   |
| `99-consistency-report.md` present              | ✅ — this file                                                         |
| `02-features/` complete (7 features × 5 files)  | ✅ — Dashboard, Emails, Rules, Accounts, Watch, Tools, Settings        |
| `03-issues/solved/` populated                   | ✅ — 8 issues migrated (Task #34); `pending/` empty                    |
| Lowercase kebab-case naming                     | ✅                                                                     |
| Unique numeric sequence prefixes                | ✅                                                                     |
| Cross-spec deps (`23-app-database`, `24-app-design-system-and-ui`) authored | ✅ (Tasks #32, #33)                              |

**Health Score:** 100/100 (A+)

---

## 2. File Inventory (final)

### 2.1 `spec/21-app/`

| #     | Path                                                       | Status            | Authored in task |
|-------|------------------------------------------------------------|-------------------|------------------|
| 00    | `00-overview.md`                                           | ✅ Present         | Pre-existing     |
| 01    | `01-fundamentals.md`                                       | ✅ Present         | Pre-existing     |
| 02    | `02-features/00-overview.md`                               | ✅ Present         | Pre-existing     |
| 02.01 | `02-features/01-dashboard/{00,01,02,97,99}.md`             | ✅ 5/5             | #07–10           |
| 02.02 | `02-features/02-emails/{00,01,02,97,99}.md`                | ✅ 5/5             | #11–14           |
| 02.03 | `02-features/03-rules/{00,01,02,97,99}.md`                 | ✅ 5/5             | #15–18           |
| 02.04 | `02-features/04-accounts/{00,01,02,97,99}.md`              | ✅ 5/5             | #19–22           |
| 02.05 | `02-features/05-watch/{00,01,02,97,99}.md`                 | ✅ 5/5             | #23–26           |
| 02.06 | `02-features/06-tools/{00,01,02,97,99}.md`                 | ✅ 5/5             | #27–30           |
| 02.07 | `02-features/07-settings/{00,01,02,97,99}.md`              | ✅ 5/5             | #31              |
| 03    | `03-issues/00-overview.md`                                 | ✅ Rewritten       | #34              |
| 03.s  | `03-issues/solved/01..08-*.md`                             | ✅ 8/8 migrated    | #34              |
| 03.p  | `03-issues/pending/00-overview.md`                         | ✅ (empty seed)    | #34              |
| 04    | `04-coding-standards.md`                                   | ✅ Present         | #03              |
| 05    | `05-logging-strategy.md`                                   | ✅ Present         | #04              |
| 06    | `06-error-registry.md`                                     | ✅ Present         | #05              |
| 07    | `07-architecture.md`                                       | ✅ Present         | #06              |
| 97    | `97-acceptance-criteria.md`                                | ✅ Rewritten v2.0  | #35              |
| 99    | `99-consistency-report.md`                                 | ✅ Rewritten v2.0  | #35 (this file)  |
| —     | `legacy/spec.md`                                           | 📦 Archived        | Pre-existing     |
| —     | `legacy/plan-cli.md`                                       | 📦 Archived        | Pre-existing     |
| —     | `legacy/plan-fyne-ui.md`                                   | 📦 Archived        | Pre-existing     |

### 2.2 Cross-cutting specs

| Path                                                                          | Files                                                                                                         | Authored in task |
|-------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|------------------|
| `spec/23-app-database/`                                                       | `00-overview` · `01-schema` · `02-queries` · `03-migrations` · `04-retention-and-vacuum` · `97-ac` · `99-cr` | #32              |
| `spec/24-app-design-system-and-ui/`                                           | `00-overview` · `01-tokens` · `02-theme-implementation` · `03-layout-and-shell` · `04-components` · `05-accessibility` · `97-ac` · `99-cr` | #33              |

**Total active spec files (21-app + 23 + 24):** 12 + 35 (features) + 9 (issues) + 7 (db) + 8 (ds) = **71**.
**Legacy:** 3.

---

## 3. Naming & Convention Audit

| Rule (from `spec/12-consolidated-guidelines/`)                                  | Status   | Evidence / Notes                                                                                                                                                  |
|---------------------------------------------------------------------------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **PascalCase config / DB keys** (02-coding §1.1, 18-database §1)                | ✅       | `Account`, `Rule`, `Email`, `WatchState`, `OpenedUrl`, `RuleStat` — all singular PascalCase; field keys (`Alias`, `LastUid`, `PasswordB64`) match.                |
| **Singular table names** (18-database §1)                                       | ✅       | All 4 + 1 tables singular. Legacy plurals renamed via `M004` (see `spec/23-app-database/03-migrations.md`).                                                       |
| **Lowercase kebab-case file names**                                             | ✅       | All 71 files conform.                                                                                                                                             |
| **15-line function rule** (02-coding §3)                                        | ✅ (spec)| AC-PROJ-20 in `97-acceptance-criteria.md` enforces; `linter-scripts/check-fn-length.sh` is the CI gate. Code may currently violate — that is an *implementation* delta, not a spec consistency issue. |
| **Strict typing — no `any`/`interface{}`** (02-coding §6)                       | ✅ (spec)| Every backend spec uses concrete types or generics; zero `interface{}` in spec text.                                                                              |
| **`errtrace.Wrap` boundaries + `Result[T]` envelope** (03-error §2, §4)         | ✅ (spec)| Every `core.*` API in `02-features/*/01-backend.md` returns `errtrace.Result[T]`.                                                                                 |
| **`ER-XXX-NNNNN` error codes from registry ranges** (03-error §6)               | ✅       | 11 prefixes registered (`ER-CFG/STO/MAIL/RUL/WCH/BRW/EXP/COR/CLI/UI/UNKNOWN`), all in non-overlapping 21000–21999 bands. Watch range 21400–21499 matches `06-error-registry.md`. |
| **Heartbeat invariant** (16-app-design §6.4 / 05-logging-strategy §6.4)         | ✅       | Codified in Watch §2 (H-01), AC-PROJ-14, regression test AC-PROJ-24 (issue 02).                                                                                   |
| **Sidebar tokens, type scale, z-index** (16-app-design)                         | ✅       | All listed in `spec/24-app-design-system-and-ui/01-tokens.md`; `ColorWatchDot{Ok,Warn,Err}` added (closes Watch OI-1).                                            |
| **CLI subcommand structure + exit codes** (23-generic-cli)                      | ✅       | CLI surface documented in feature backends (`runWatch`, `runRead`, `rules add`, `add-quick`, `doctor`); exit-code mapping in `06-error-registry.md`.              |

---

## 4. Cross-Reference Validation

### 4.1 Forward-references resolved

| Reference (originally forward-pointing)                                                       | Source                                                              | Resolution                                                                          |
|-----------------------------------------------------------------------------------------------|---------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| `core.Tools.OpenUrl(ctx, url)` from Watch frontend §2.3.1                                     | `02-features/05-watch/02-frontend.md`                               | ✅ Defined in `02-features/06-tools/01-backend.md` (Tasks #27–30). Watch OI-2 closed. |
| `ColorWatchDot{Ok,Warn,Err}` from Watch frontend §3                                           | `02-features/05-watch/02-frontend.md`                               | ✅ Defined in `spec/24-app-design-system-and-ui/01-tokens.md` (Task #33). Watch OI-1 closed. |
| `Q-EMAIL-GET-BY-UID` from Emails backend                                                      | `02-features/02-emails/01-backend.md`                               | ✅ Defined in `spec/23-app-database/02-queries.md` (Task #32).                         |
| `core.Settings.Subscribe(ctx)` from Watch frontend (live `PollSeconds` reload)                | `02-features/05-watch/01-backend.md`                                | ✅ Defined in `02-features/07-settings/01-backend.md` (Task #31).                      |
| `config.SanitizePassword` from Accounts backend                                               | `02-features/04-accounts/01-backend.md`                             | ✅ Defined in same file §Password Sanitization, plus issue 03 spec encoding.          |
| Issue regression coverage                                                                     | `97-acceptance-criteria.md` §1.7                                    | ✅ All 8 solved issues map to AC-PROJ-23–30.                                          |

### 4.2 Internal-link sweep

All `./` and `../` links inside `spec/21-app/`, `spec/23-app-database/`, and `spec/24-app-design-system-and-ui/` were authored as file-relative. Run `node .lovable/linter-scripts/generate-dashboard-data.cjs` (or `linter-scripts/check-internal-links.sh` once added — see AC-PROJ-33) to confirm zero broken links in CI. Manual spot-checks during authoring: clean.

### 4.3 Error-code reference check

Every `ER-XXX-NNNNN` cited in any feature `01-backend.md` or `97-acceptance-criteria.md` was cross-checked against `06-error-registry.md`:

- ✅ All references resolve.
- ✅ No reference uses an unregistered code.
- ✅ No registered code is unreferenced (every registered code has at least one spec citation or is on the explicitly-reserved list).

### 4.4 Named-query reference check

Every `Q-*` cited in any backend spec was cross-checked against `spec/23-app-database/02-queries.md`:

- ✅ 14 named queries registered, all referenced ≥ 1× from a feature backend.
- ✅ Zero dangling `Q-*` references.

---

## 5. Open Issues — Project-Wide

| #   | Issue                                                                                                                          | Owner             | Status                                                                                              |
|-----|--------------------------------------------------------------------------------------------------------------------------------|-------------------|-----------------------------------------------------------------------------------------------------|
| OI-1 (Watch) | `ColorWatchDot*` design tokens                                                                                       | Design system     | ✅ **Closed** by Task #33                                                                            |
| OI-2 (Watch) | `core.Tools.OpenUrl` signature                                                                                       | Tools feature     | ✅ **Closed** by Tasks #27–30                                                                        |
| —   | _None outstanding._                                                                                                            | —                 | —                                                                                                   |

`spec/21-app/03-issues/pending/` is empty. AC-PROJ-35 is therefore green.

---

## 6. Implementation deltas (spec ≠ code) — informational

The following are known places where the **spec is ahead of the implementation**. They are NOT spec inconsistencies — they are the work the spec was written to drive. Captured here so the next implementation round has a checklist:

| # | Area              | Spec says                                                                                                          | Code currently                                                                                              |
|---|-------------------|--------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------|
| 1 | Tables            | Singular PascalCase (`Email`, `WatchState`, `OpenedUrl`, …)                                                        | Some legacy plural names exist; renamed by migration `M004` once that migration runs.                       |
| 2 | `Result[T]` API   | Every `core.*` method returns `errtrace.Result[T]`                                                                 | ✅ **DONE (7/7)** — 2026-04-26: foundations + all 7 `core.*` files migrated. **Foundations:** `internal/errtrace/result.go` (Result[T], Ok/Err, HasError/Value/Error/PropagateError, Coded carrier, WrapCode, WithContext); `internal/errtrace/codes.go` (all ErrXxx Code constants from 06-error-registry.md). **Migrated:** `core.ExportCSV` → `Result[string]`; `core.AddAccount/ListAccounts/GetAccount/RemoveAccount`; `core.LoadDashboardStats` → `Result[DashboardStats]`; `core.Diagnose` → `Result[struct{}]`; `core.ListEmails`/`GetEmail`/`CountEmails`; `core.AddRule`/`ListRules`/`GetRule`/`SetRuleEnabled`/`RemoveRule`; `core.ReadEmail` → `Result[struct{}]` (uses ErrConfigOpen, ErrConfigAccountMissing, ErrDbOpen, ErrDbQueryEmail, ErrBrowserNotFound, ErrCoreContextCancelled with WithContext("alias",…)/("uid",…)). **Callers updated:** all CLI runners (`runRead`, `runRulesList`, `runRulesToggle`, `newRulesAddCmd`, `runAddQuick`, `runAdd`, `runList`, `runRemove`, `runDiagnose`, `runExportCsv`); all UI seams (`accounts.go`, `add_account_form.go`, `dashboard.go`, `emails.go`, `rules.go`, `add_rule_form.go`). All packages compile + tests green (errtrace, core, cli, watcher, exporter, store, config, rules, mailclient, browser, imapdef, ui, ui/views). |
| 3 | Settings backend  | `core.Settings` service with atomic `Save` (tmp + fsync + rename) and `Subscribe`                                  | 🟢 **MVP done (2026-04-26)** — `internal/core/settings.go` + `settings_types.go` + `settings_validate.go` + `settings_extension.go`. Public API `NewSettings(clock) → Result[*Settings]` with `Get / Save / ResetToDefaults / DetectChrome / Subscribe`. **Live consumers wired:** **CF-W1** poll cadence (watcher.Options.PollSecondsCh, applied on next tick); **CF-T1** browser launcher live ChromePath via `browser.Launcher.Reload(cfg)`; **CF-D1** Dashboard "Auto-start watcher" indicator (`internal/ui/views/dashboard.go::newAutoStartIndicator`); **Theme live-switch** (Delta #4 (e)) via `internal/ui/app.go::startThemeLiveConsumer`. **CF-W3 (2026-04-26):** Watch view scaffold landed at `internal/ui/views/watch.go` (`BuildWatch`) per `02-features/05-watch/02-frontend.md` §2.1: status header (alias + ○ Idle placeholder + disabled ▶ Start) / Cards|Raw-log AppTabs (honest empty states pending `core.Watch` event bus) / footer with **live cadence label** — `newCadenceIndicator` + `forwardCadenceEvents` subscribe to `core.Settings`, render `cadence: every N s`, and update on every Save / Reset event; on Settings setup failure shows neutral `cadence: unknown`. NavWatch routed in `internal/ui/app.go::viewFor` (replaces the v0.26 placeholder). Tests in `internal/ui/views/watch_test.go`: `Test_ForwardCadenceEvents_UpdatesLabel` (2 events, asserts last value + ctx-done) + `Test_FormatCadence` (locks "every N s" format) + `Test_AliasLabel_EmptyFallback` (header empty-state). 14 packages green; fn-length linter still 0/0 (53 files scanned). **Schema deviation:** keeps legacy camelCase JSON keys; PascalCase migration deferred to Delta #1. **Outstanding (CF-W2):** AutoStartWatch read-once-at-process-start regression test — to be added when an auto-start handler exists. **Watch view real-time (cards/raw-log/status dot/counters/start-stop):** awaits `core.Watch` service + `internal/eventbus` per `02-features/05-watch/01-backend.md` (the largest remaining backend). |
| 4 | Design tokens     | `ColorWatchDot{Ok,Warn,Err}` + sidebar tokens + Fyne adapter + AST guards (`02-theme-implementation.md` §2/§6) | 🟢 **MVP+adapter+all token groups+sizes/density/fonts+selection-alpha+tnum done (2026-04-26)** — `internal/ui/theme/`: `tokens.go` (typed `ColorName` enum, **37 token constants** across §2.1–§2.8), `palette_dark.go` + `palette_light.go` (key-parallel `map[ColorName]color.NRGBA` with **all 74 spec values verbatim** — both palettes parity-checked by AST-T5 + `Test_Palettes_Parity`), `theme.go` (public `Apply / Active / Color`; goroutine-safe via RWMutex; unknown token → ER-UI-21900 fallback to `ColorForeground`, dedup-logged once per name). **§3/§4 size scale:** `sizes.go` adds typed `SizeName` enum + **15 size token constants** (7 typography + 8 spacing) with all spec values verbatim, `Size(SizeName) float32` accessor, no-panic unknown→0 fallback. **§8 density resolver:** `Density` enum (`DensityComfortable` default / `DensityCompact`); `SetDensity / ActiveDensity` under the same RWMutex as the theme mode; Compact applies the spec ×0.875 multiplier with `math.Round`. **§3 fonts:** `fonts.go` declares `embed.FS` over `all:fonts` and exposes `TextFont() / TextMonospaceFont() fyne.Resource` with nil-fallback to Fyne defaults when the OFL .ttf payloads have not yet been committed. **§2 selection alpha-blend (2026-04-26):** `alpha.go::AlphaBlend(name, factor) color.NRGBA` returns the active-mode color with its alpha scaled by `factor` (clamped to [0,1] via `scaleAlpha`); `fyne_theme.go::Color()` now special-cases `ColorNameSelection` to return `ColorPrimary` at α=0.30 inline (no global state mutation during render) — replaces the placeholder solid-Primary mapping called out in the previous Outstanding list. The selection band now correctly tints the underlying surface instead of opaquely covering it. **§3.2 tabular numerics:** `tnum.go` exposes `TabularNumFeature = "tnum"` + `TabularNumFeatures() []string` (defensive copy on every call) — wiring contract for the future `text.NewWithFeatures` swap when Fyne ships post-v2.6 feature-flag support; numeric-column widgets (Email count, OpenedUrl count, Watch counters) read this list rather than hard-coding the tag. **§2.6 raw-log / §2.7 badges / §2.8 code-surface tokens** all wired with full RGB lock-in tests. **Fyne adapter:** `fyne_theme.go` (build-tagged `!nofyne`) implements `fyne.Theme` with the §2 color routing table (10 entries + selection special-case), the §3/§4 `fyneSizeRoute` (7 entries — Text→TextBody, HeadingText→TextPageTitle, SubHeadingText→TextSectionTitle, CaptionText→TextCaption, Padding→Spacing2, InnerPadding→Spacing3, InlineIcon→TextCaption), and `Font(s)` routing monospace→JetBrains Mono / regular→Inter (variable axis covers weights/italics) with nil-fallback. `fyne_apply.go::ApplyToFyne(mode)` installs the theme on the running app. **Bootstrap wired:** `internal/ui/app.go::Run` reads `core.Settings.Get().Theme` and calls `theme.ApplyToFyne` BEFORE `BuildShell`. **AST guards:** AST-T1, AST-T2, AST-T4, AST-T5 — auto-validate new groups land in both palettes. **New error code:** `ER-UI-21900 ErrUiThemeUnknownToken`. **Tests (20 cases — 4 added today):** Palettes_Parity, Color_ResolvesPerMode, Color_KnownTokens, Color_RawLogBadgeCodeTokens, Color_UnknownFallback, Apply_RejectsInvalid, Apply_ConcurrentSafe, Size_ComfortableScale, Size_CompactScale, Size_DensityToggleNoStateLeak, Size_UnknownReturnsZero, AllSizeNames_CoversBase, Fonts_FallbackWhenAbsent, **AlphaBlend_PrimarySelection** (NEW — locks the §2 spec value for both Dark and Light: ColorPrimary @ α=0.30 → A=77 in both modes), **AlphaBlend_ClampsOutOfRange** (NEW — negative→0, >1→identity, ==1→identity), **AlphaBlend_UnknownFallsBackToForeground** (NEW — preserves Color()'s no-panic contract under alpha math), **TabularNumFeatures** (NEW — locks the `["tnum"]` slice and proves the defensive-copy contract: caller mutation does not leak), plus the 4 AST guards. 14 packages green under `-tags nofyne`; fn-length linter still **0/0** (68 files). **Settings live theme-switch consumer:** `internal/ui/app.go::startThemeLiveConsumer` + `internal/ui/theme_live_test.go`. **Outstanding:** (a) drop the actual Inter-Variable.ttf + JetBrainsMono-Variable.ttf binary assets under `internal/ui/theme/fonts/` (wiring + fallback are done; only the OFL payloads remain); (b) Settings UI control to surface the Density toggle (backend resolver wired + tested, just no Settings form field yet — deferred per §8 "Today only Comfortable ships"). |
| 5 | Issues regressions | Each of the 8 solved issues has a regression test (`Regress_Issue0N_*`)                                            | ✅ **DONE** — 7 `Regress_Issue0N_*` Go tests added 2026-04-25 across 5 packages: `internal/config/regress_test.go` (issues 01, 03), `internal/watcher/regress_test.go` (issues 02, 04, 08), `internal/rules/regress_test.go` (issue 05), `internal/mailclient/regress_test.go` (issue 06), `internal/cli/regress_test.go` (issue 07). All 8 AC-PROJ-23–30 invariants are now locked at the code level. |
| 6 | Doctor + Tools UI  | `email-read doctor` rune-dump + Tools card | 🟢 **MVP done (2026-04-26)** — `core.Doctor` backend (`internal/core/doctor.go`) + Doctor / Diagnose Tools tabs. **OpenUrl sub-tool MVP:** `core.Tools` slim slice (`internal/core/tools.go` + `tools_redact.go`) — full §5.1 validation pipeline (8 codes `ER-TLS-21760..21767`), §5.2 redaction, per-key in-memory dedup, incognito launch via `browser.Launcher.Open`. **ReadOnce + ExportCsv slices:** `internal/core/tools_read.go::ReadOnce(ctx, ReadSpec, chan<- string) → Result[ReadResult]` per §2.1 (Limit ∈ [1,500] default 10, alias resolution, mailclient.Dial seam, streaming progress, watcher cursor never written); `internal/core/tools_export.go::ExportCsv(ctx, ExportSpec, chan<- ExportProgress) → Result[ExportReport]` per §2.2 (Counting → Writing → Flushing → Done phases; wraps `exporter.ExportCSV`; `Overwrite` gate → `ER-TLS-21753`). Both wired into `BuildTools` — all 4 sub-tool placeholders are now real forms. **CachedDiagnose + RecentOpenedUrls (2026-04-26):** `internal/core/tools_diagnose.go` adds two services on top of the existing `core.Diagnose` backend per spec §2.3 + §2.5. (a) **`Tools.CachedDiagnose(ctx, DiagnoseSpec, emit) → Result[DiagnosticsReport]`** — wraps `Diagnose` with a per-Tools, per-alias, in-memory `diagnoseCache` (60 s TTL, `sync.Mutex`-guarded `map[string]diagCacheEntry`). On cache miss: captures every `DiagnoseEvent` from the live run, stores `{Alias, StoredAt: now, Events}`, returns `Cached: false`. On cache hit (StoredAt + 60 s ≥ now): replays the captured event slice to `emit` so the UI renders identically to a live run, returns `Cached: true` (zero IMAP traffic). `Force: true` evicts the entry and re-runs. Failures are NOT cached (lazy-evict-then-run path). The cache field is allocated lazily via `sync.Once` on `Tools.diagCache()` so existing `NewTools` callers don't break ABI. Backend errors wrap to `ER-TLS-21752 ToolsDiagnoseAborted`; ctx-cancel checks pre-flight. (b) **`Tools.RecentOpenedUrls(ctx, OpenedUrlListSpec) → Result[[]OpenedUrlRow]`** — read-only audit accessor backed by the live `OpenedUrls` table. Validates `Limit ∈ [1,1000]` (default 100), defaults `Before = now`, runs `SELECT Id, EmailId, RuleName, Url, OpenedAt FROM OpenedUrls WHERE OpenedAt < ? ORDER BY OpenedAt DESC, Id DESC LIMIT ?`, scans into `[]OpenedUrlRow` via the testable `scanOpenedUrlRows` helper (rows-iterator interface). `Alias` and `Origin` filter fields are accepted but **inert** until Delta #1 (PascalCase migration adds those columns); documented in source so callers don't silently rely on them. **Tests (9 cases):** `TestCachedDiagnose_MissThenHitReplaysEvents` (3 captured events replayed verbatim + `Cached: true`), `TestCachedDiagnose_TtlExpiryRefetches` (61 s clock advance → 2 live runs), `TestCachedDiagnose_ForceBypassesCache`, `TestCachedDiagnose_WrapsBackendError` (wraps to `ErrToolsDiagnoseAborted`), `TestCachedDiagnose_CtxCancelled` (pre-cancelled ctx short-circuits), `TestCachedDiagnose_FailureNotCached` (failures don't poison cache; 2 calls = 2 live runs), `TestValidateOpenedUrlListSpec` (defaults + bounds), `TestScanOpenedUrlRows_HappyPath`, `TestScanOpenedUrlRows_ScanError`. **Quality bars:** 14 packages green; fn-length linter still **0/0** (64 files). **Outstanding:** (i) `AccountEvent{Updated\|Removed}` → cache invalidation hook (waits on `core.Accounts.Subscribe` channel — separate refactor); (ii) `ExportCsv` slice 2 — per-alias / date-range filtering with streaming SELECT + per-256-row progress ticks; (iii) Delta #1 PascalCase OpenedUrls schema → activates Alias / Origin / OriginalUrl / IsDeduped / IsIncognito / TraceId columns. |
| 7 | Pre-existing UI    | The current `internal/ui/` and `cmd/email-read-ui/` are treated as throwaway per `99-cr` v1.0; spec is authoritative. | Carry-over rule from v1.0 still applies.                                                                |
| 8 | Fn-length linter   | AC-PROJ-20: every Go fn ≤ 15 statements (`linter-scripts/check-fn-length.sh`)                                                                                                                                                | ✅ **PASS** — 0 violations (down from 21 → 12 → 11 → 9 → 8 → 7 → 6 → 5 → 4 → 3 → 2 → 1 → 0). 13 fns refactored on 2026-04-25: `BuildAddAccountForm`, `BuildAddRuleForm`, `runAdd`, `watcher.Run`, `watcher.pollOnce`, `core.ReadEmail`, `core.Diagnose`, `BuildEmails`, `cli.runWatch`, `core.AddAccount`, `exporter.ExportCSV`, `mailclient.SaveRaw`, `store.ListEmails`, `BuildDashboard` — all now ≤15 statements via extracted helpers. |
| 9 | Internal-link linter | AC-PROJ-33: every `./` / `../` Markdown link resolves (`linter-scripts/check-internal-links.sh`)                                                                                                                              | ✅ **PASS** — 322/322 links resolve across 72 spec files (verified 2026-04-25, post Task #35).                                                                                                                                                                                                                                                                                                                                                                       |

---

## 7. Migration Notes (cumulative)

| Date       | Action                                                                                                                                        |
|------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| 2026-04-25 | Renamed `spec/21-golang-email-reader/` → `spec/21-app/`; folded `spec/22-fyne-ui/plan.md` → `legacy/plan-fyne-ui.md`.                          |
| 2026-04-25 | Authored 7 features × 5 files = 35 feature spec files (Tasks #07–31).                                                                         |
| 2026-04-25 | Authored `spec/23-app-database/` (7 files, Task #32) and `spec/24-app-design-system-and-ui/` (8 files, Task #33).                              |
| 2026-04-25 | Migrated 8 solved issues from `.lovable/solved-issues/` → `spec/21-app/03-issues/solved/`; closed Watch OI-1 / OI-2 (Task #34).               |
| 2026-04-25 | Rewrote `97-acceptance-criteria.md` (v1.0 → v2.0): 35 AC-PROJ rows + 700-criterion roll-up + sign-off ladder (Task #35).                       |
| 2026-04-25 | Rewrote `99-consistency-report.md` (v1.0 → v2.0): full inventory + cross-ref validation + open-issues table + implementation deltas (Task #35). |
| 2026-04-25 | Implemented `linter-scripts/check-internal-links.sh` (AC-PROJ-33) and `linter-scripts/check-fn-length.sh` (AC-PROJ-20); both runnable, both produced real findings (links 322/322 PASS; fn-length 21 violations — see §6 Delta #8). |
| 2026-04-25 | Calibrated `check-fn-length.sh` (paren-depth tracking) and refactored `internal/ui/views/add_account_form.go` (`BuildAddAccountForm` 38 → ≤ 15) + `add_rule_form.go` (`BuildAddRuleForm` 33 → ≤ 15) by extracting widget construction, submit handlers, and message formatters into focused helpers. Linter now reports **12 real violations** (down from 21). |
| 2026-04-25 | Refactored `internal/cli/cli.go:runAdd` (30 → ≤ 15) by extracting `addAccountDefaults` struct + `resolveAddDefaults` / `promptAddIdentity` / `promptAddServer` helpers. Linter now reports **11 real violations**. |
| 2026-04-25 | Refactored `internal/watcher/watcher.go`: `Run` 21 → ≤ 15 (extracted `logStartupBanner` + `pollState` struct + `logPollError` + `handlePollOK`) and `pollOnce` 26 → ≤ 15 (extracted `connectAndSelect`, `handleBaseline`, `fetchAndCheckEmpty`, `processBatch`, `processMessage`, `persistMessage`, `evaluateRules`, `launchMatches`, `finalizeBatch`, `loadWatchState`, `logPollDone`, `advanceCursor`). Linter now reports **9 real violations**. |
| 2026-04-25 | Refactored `internal/core/read.go:ReadEmail` (28 → ≤ 15) by extracting `loadEmailDetail`, `ensureSeededRule`, `buildEngineAndLauncher`, `rowToMessage`, `evaluateMatches`, `openMatches`, `openOneMatch`. Linter now reports **8 real violations**. |

---

## 8. Final Sign-off Checklist (spec authoring)

- [x] Every feature folder has all 5 required files.
- [x] `spec/23-app-database/` and `spec/24-app-design-system-and-ui/` are authored.
- [x] Every solved issue is migrated to `03-issues/solved/` with normalized frontmatter.
- [x] `pending/` is empty (or every entry has a tracking link).
- [x] Watch OI-1 and OI-2 are closed and cross-referenced.
- [x] Project-wide `97-acceptance-criteria.md` v2.0.0 enumerates 35 AC-PROJ rows and indexes 700 total criteria.
- [x] Project-wide `99-consistency-report.md` v2.0.0 (this file) lists every spec file and audits every cross-spec reference.
- [x] Tasklist `.lovable/memory/workflow/02-spec-21-app-tasklist.md` reflects #35 as the final task.

> The spec authoring round is **complete**. Implementation work tracked under §6 is the next phase and lives outside this consistency report.

---

## 9. Validation History

| Date       | Version | Action                                                                                                                                                                                    |
|------------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 2026-04-25 | 1.0.0   | Initial scaffolding consistency report (Task #06).                                                                                                                                        |
| 2026-04-25 | 2.0.0   | **Full rewrite** (Task #35). Final inventory of 71 active files; closed OI-1/OI-2; audited every error-code, named-query, and forward-reference; recorded 7 known implementation deltas.   |
