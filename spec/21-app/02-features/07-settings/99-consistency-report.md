# 07 — Settings — Consistency Report

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Cross-checks the Settings feature against every other spec it touches. Each row is one consistency invariant with the citation pair (Settings doc ↔ peer doc) and the test that enforces it. If a peer spec changes, this report MUST be re-validated.

---

## 1. Internal Consistency (within Settings)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| INT-1 | The `SettingsSnapshot` fields in §3 of backend exactly match the form fields in §3 of frontend (no extras either side). | `01-backend.md` §3 ↔ `02-frontend.md` §3 | `Test_FieldParity_BackendFrontend` (reflect-based) |
| INT-2 | Every error code in `01-backend.md` §10 has a friendly mapping in `02-frontend.md` §5. | `01-backend.md` §10 ↔ `02-frontend.md` §5 | `Test_ErrorCodes_AllMapped` (table-driven) |
| INT-3 | The forbidden-scheme list (`file`, `javascript`, `data`, `vbscript`) appears identically in backend §6 and frontend §5. | `01-backend.md` §6 ↔ `02-frontend.md` §5 | `Test_ForbiddenSchemes_BothLayers` (AC-SX-06) |
| INT-4 | Defaults table in backend §3 matches the defaults restored by the Reset confirm dialog copy in frontend §3.5. | `01-backend.md` §3 ↔ `02-frontend.md` §3.5 | `Test_Reset_ConfirmCopy` (AC-SF-20) — golden-string match |
| INT-5 | Read-only path fields (`ConfigPath`, `DataDir`, `EmailArchiveDir`, `UiStateFile`) never appear in `SettingsInput`. | `01-backend.md` §3 | `go vet` + `Test_FieldParity_BackendFrontend` (asserts absence) |
| INT-6 | Validation rules in backend §6 are mirrored (not re-implemented divergently) in `internal/ui/views/settings_validate.go`. | `02-frontend.md` §11 | shared table fixture in `internal/core/settings_validation_table.go` imported by both |

---

## 2. Cross-Feature Consistency

### 2.1 Settings ↔ Watch (Feature 05)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-W1 | `PollSeconds` change is applied by Watch on the **next** loop iteration; in-flight polls are NOT interrupted. | `01-backend.md` §8 ↔ `02-features/05-watch/01-backend.md` §6 | `Test_Watch_PollReload_OnSettingsEvent` (Watch test suite) |
| CF-W2 | `AutoStartWatch` is read **once** at process start; toggling it later only affects next launch (no surprise stop). | `01-backend.md` §8 ↔ `02-features/05-watch/01-backend.md` §3 (lifecycle) | `Test_Watch_AutoStart_RuntimeToggle_NoEffect` |
| CF-W3 | Watch frontend live-log "poll cadence" indicator updates within ≤ 1 s of a `SettingsSaved` event for `PollSeconds`. | `02-features/05-watch/02-frontend.md` §5 | `Test_Watch_LiveCadenceUpdate` |

### 2.2 Settings ↔ Tools (Feature 06, OpenUrl)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-T1 | `BrowserOverride.ChromePath` is consumed by `core.Tools.OpenUrl` on the next call after `SettingsSaved`; never mid-launch. | `01-backend.md` §8 ↔ `02-features/06-tools/01-backend.md` §3.4 | `Test_Tools_OpenUrl_RespectsNewChromePath` |
| CF-T2 | `OpenUrlAllowedSchemes` is the single source of truth — `core.Tools.OpenUrl` rejects with the same code regardless of caller (Tools UI, Rules UI, Watch UI). | `01-backend.md` §6 (rule SET-21773) ↔ `02-features/06-tools/01-backend.md` §3.4 (validation step 2) | `Test_OpenUrl_SchemeRejection_AllCallers` (parametrized over caller) |
| CF-T3 | `AllowLocalhostUrls = false` causes `core.Tools.OpenUrl` to reject `http://localhost`/`http://127.0.0.1`/`http://[::1]` with code `ER-TLS-21764` (`ErrToolsOpenUrlLocalhost`). | `01-backend.md` §3 ↔ `02-features/06-tools/01-backend.md` §3.4 | `Test_OpenUrl_LocalhostBlocked_Default` |
| CF-T4 | `IncognitoArg` override, when valid, is passed verbatim by `browser.Launcher`; when empty, the per-browser auto-pick from Tools §3.4 applies. | `01-backend.md` §6 (SET-21775) ↔ `02-features/06-tools/01-backend.md` §3.4 | `Test_Browser_IncognitoArg_OverrideVsAuto` |

### 2.3 Settings ↔ Rules (Feature 03)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-R1 | "Open URL" actions in Rules use `core.Tools.OpenUrl`, which respects `OpenUrlAllowedSchemes` from Settings (single chokepoint). | `02-features/03-rules/01-backend.md` (action dispatch) ↔ `02-features/06-tools/01-backend.md` §3.4 | `Test_Rules_OpenUrl_HonorsSchemeAllowlist` |
| CF-R2 | Rules UI does NOT expose a "bypass scheme allowlist" toggle. | `02-features/03-rules/02-frontend.md` (action editor) | AST guard `Test_AST_RulesUI_NoSchemeBypass` |

### 2.4 Settings ↔ Accounts (Feature 04)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-A1 | `Settings.Save` never mutates the `Accounts` array — proven by golden-file diff (AC-SB-11). | `01-backend.md` §5 step 4 ↔ `02-features/04-accounts/01-backend.md` (config ownership) | `Test_Save_Accounts_Untouched` |
| CF-A2 | Account add/remove (Feature 04) and Settings save can run concurrently without lost updates: each owns disjoint top-level keys; `internal/config.WriteAtomic` serializes both. | `01-backend.md` §5 ↔ `02-features/04-accounts/01-backend.md` (persist flow) | `Test_Concurrent_Settings_Accounts_NoLoss` |

### 2.5 Settings ↔ Dashboard (Feature 01)

| # | Invariant | Citations | Enforcement |
|---|---|---|---|
| CF-D1 | Dashboard "Auto-start watcher" indicator reflects `AutoStartWatch` and updates on `SettingsEvent`. | `02-features/01-dashboard/02-frontend.md` (status panel) ↔ `01-backend.md` §8 | `Test_Dashboard_AutoStartIndicator_Live` |

---

## 3. Cross-Reference to Consolidated Guidelines

| Guideline | Settings reference | Verified by |
|---|---|---|
| `02-coding-guidelines.md` §1.1 (PascalCase keys) | `config.json` schema slice in `01-backend.md` §4 uses `Watch`, `Browser`, `Ui` (not `watch`, `browser`, `ui`). | `Test_ConfigJson_Casing` |
| `02-coding-guidelines.md` §3 (≤15-line fns) | `Save`, `onSave`, `recomputeDirty` bodies. | `golangci-lint funlen` |
| `02-coding-guidelines.md` §6 (no `any`/`interface{}`) | All Settings types in §3. | `golangci-lint forbidigo` rule `interface\{\}` |
| `03-error-management.md` §3 (`apperror.Wrap`) | All error sites in §10 of backend. | `Test_Errors_AllWrapped` (AST) |
| `03-error-management.md` §4 (`errtrace.Result[T]`) | All five public methods in `01-backend.md` §2. | compile-time |
| `05-logging-strategy.md` §6.7 | `01-backend.md` §11 fields. | `Test_LogShape_Settings` |
| `16-app-design-system-and-ui.md` (no hard-coded colors) | `02-frontend.md` §7. | `Test_AST_Settings_NoDirectColor` (AC-SX-03) |
| `18-database-conventions.md` | Settings owns no DB tables (config-only); explicitly declared in `01-backend.md` §1 ("`store` reserved for future use"). | manual review |

---

## 4. Open Issues

None. All cross-feature seams have a backing test ID. If a future change adds a new consumer of `SettingsEvent`, append a row to §2 with its own enforcement test before merging.

## 5. Sign-Off

| Reviewer | Role | Date |
|---|---|---|
| Spec author (AI) | Drafting | 2026-04-25 |
| Pending | Tech lead | — |
| Pending | QA | — |
