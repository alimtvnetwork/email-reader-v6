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
| 3 | Settings backend  | `core.Settings` service with atomic `Save` (tmp + fsync + rename) and `Subscribe`                                  | 🟢 **MVP done (2026-04-26)** — `internal/core/settings.go` + `settings_types.go` + `settings_validate.go` + `settings_extension.go`. Public API `NewSettings(clock) → Result[*Settings]` with `Get / Save / ResetToDefaults / DetectChrome / Subscribe`. **Atomic write** via single tmp+rename combining typed config + new `settings` extension block (preserves Accounts/Rules byte-for-byte). **Pub/sub:** buffered cap-4 channels with non-blocking publish + cancel-closes-channel semantics. **Chrome detection:** §7 probe order (Config → Env `EMAIL_READ_CHROME` → OS defaults → PATH → NotFound). **Live-reload consumer (CF-W1, 2026-04-26):** `watcher.Options.PollSecondsCh <-chan int` + extracted `runLoop` / `applyPollReload` (clamps 1..60, logs `⚙ poll cadence reloaded`, no-op when unchanged). CLI bridge in `cli.startPollReloadBridge` subscribes to `core.Settings`, fan-outs `SettingsEvent.Snapshot.PollSeconds` onto a buffered cap-4 int channel; cancel via `defer`. New cadence applies on **next** tick — in-flight polls never interrupted. **Tests:** 11 settings cases + 5 `Test_ApplyPollReload_ClampsAndLogs` cases (race-clean). 13 packages green; fn-length linter still 0/0. **Schema deviation:** keeps legacy camelCase JSON keys (`pollSeconds`, `chromePath`) for backward compat; PascalCase migration deferred to Delta #1. **Outstanding (CF-W2/W3, CF-T1, CF-D1):** UI live indicators (cadence label, dashboard auto-start dot) and Tools `OpenUrl` ChromePath consumer remain — those land with Delta #4/#6. |
| 4 | Design tokens     | `ColorWatchDot{Ok,Warn,Err}` + sidebar tokens registered in `01-tokens.md`                                         | 🟢 **MVP done (2026-04-26)** — new `internal/ui/theme/` package: `tokens.go` (typed `ColorName` enum, 26 token constants for §2.1 surfaces, §2.2 brand, §2.3 status, §2.4 sidebar 6×, §2.5 watch dots 6×), `palette_dark.go` + `palette_light.go` (key-parallel `map[ColorName]color.NRGBA` with all 52 spec values verbatim), `theme.go` (public `Apply / Active / Color`; Apply is goroutine-safe via RWMutex; unknown token → ER-UI-21900 fallback to `ColorForeground`, dedup-logged once per name; ThemeSystem currently routes to ThemeDark — OS detection tracked OI-DS-2). **Build-tag-free:** entire token surface compiles + tests under `-tags nofyne`; the Fyne adapter (`fyne_theme.go`) is the only file that will need `!nofyne`. **New error code:** `ER-UI-21900 ErrUiThemeUnknownToken` added to `internal/errtrace/codes.go` (slot was reserved in spec §06 but missing from registry). **Tests:** 6 cases in `theme_test.go` — Palettes_Parity (both maps cover every AllColorNames entry), Color_ResolvesPerMode (Dark≠Light for representative tokens), Color_KnownTokens (locks 5 spec RGBs for sidebar+WatchDot), Color_UnknownFallback (no-panic contract), Apply_RejectsInvalid (ThemeMode(0) → ER-SET-21772, no state mutation), Apply_ConcurrentSafe (race-clean read/write hammer). 14 packages green; fn-length linter still 0/0. **Outstanding:** (a) Fyne `appTheme` adapter routing built-in `theme.ColorName` → our tokens (`02-theme-implementation.md` §2 table); (b) AST guard `ast_test.go` to ban raw `color.NRGBA{}` literals in feature views (§6); (c) full §2.6–2.9 token groups (raw-log, badges, code) — land with the raw-log viewer feature; (d) density resolver + Compact mode (deferred per §8). |
| 5 | Issues regressions | Each of the 8 solved issues has a regression test (`Regress_Issue0N_*`)                                            | ✅ **DONE** — 7 `Regress_Issue0N_*` Go tests added 2026-04-25 across 5 packages: `internal/config/regress_test.go` (issues 01, 03), `internal/watcher/regress_test.go` (issues 02, 04, 08), `internal/rules/regress_test.go` (issue 05), `internal/mailclient/regress_test.go` (issue 06), `internal/cli/regress_test.go` (issue 07). All 8 AC-PROJ-23–30 invariants are now locked at the code level. |
| 6 | Doctor + Tools UI  | `email-read doctor` rune-dump + Tools card                                                                         | Doctor exists in CLI; Tools UI card to be built per `06-tools/02-frontend.md`.                              |
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
