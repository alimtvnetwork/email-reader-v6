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
| 2 | `Result[T]` API   | Every `core.*` method returns `errtrace.Result[T]`                                                                 | Most methods return `(T, error)`; refactor in progress.                                                     |
| 3 | Settings backend  | `core.Settings` service with atomic `Save` (tmp + fsync + rename) and `Subscribe`                                  | Not yet implemented — drives Task series outside this authoring round.                                       |
| 4 | Design tokens     | `ColorWatchDot{Ok,Warn,Err}` + sidebar tokens registered in `01-tokens.md`                                         | Fyne theme partially uses tokens; full migration tracked.                                                   |
| 5 | Issues regressions | Each of the 8 solved issues has a regression test (`Regress_Issue0N_*`)                                            | Test names defined in AC-PROJ §1.7; tests themselves to be written.                                          |
| 6 | Doctor + Tools UI  | `email-read doctor` rune-dump + Tools card                                                                         | Doctor exists in CLI; Tools UI card to be built per `06-tools/02-frontend.md`.                              |
| 7 | Pre-existing UI    | The current `internal/ui/` and `cmd/email-read-ui/` are treated as throwaway per `99-cr` v1.0; spec is authoritative. | Carry-over rule from v1.0 still applies.                                                                |
| 8 | Fn-length linter   | AC-PROJ-20: every Go fn ≤ 15 statements (`linter-scripts/check-fn-length.sh`)                                                                                                                                                | **12 violations** (down from 21 after linter calibration on 2026-04-25 — paren-depth tracking suppressed false-positives from multi-line call expressions; `add_account_form.go` and `add_rule_form.go` refactored). Remaining: `internal/cli/cli.go` (`runWatch` 18, `runAdd` 30); `internal/core/{accounts.go:AddAccount(18), diagnose.go:Diagnose(23), read.go:ReadEmail(28)}`; `internal/exporter/exporter.go:ExportCSV(18)`; `internal/mailclient/mailclient.go:SaveRaw(17)`; `internal/store/store.go` (anon @ 254, 17); `internal/ui/views/dashboard.go:BuildDashboard(16)`; `internal/ui/views/emails.go:BuildEmails(21)`; `internal/watcher/watcher.go` (`Run` 21, `pollOnce` 26). |
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
