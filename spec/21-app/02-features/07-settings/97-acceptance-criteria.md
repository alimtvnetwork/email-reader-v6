# 07 — Settings — Acceptance Criteria

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Binary, machine-checkable acceptance criteria for the Settings feature (overview / backend / frontend). Each item maps to ≥1 automated test in the test files referenced below. Manual-only checks are flagged `[manual]` and must appear in the QA checklist before release.

Source documents:
- [`./00-overview.md`](./00-overview.md) — fields table, AC-S1..AC-S5
- [`./01-backend.md`](./01-backend.md) — API §2, validation §6, save pipeline §5, registry §10, tests §12
- [`./02-frontend.md`](./02-frontend.md) — VM §1, lifecycle §2, widget tree §3, save flow §4, tests §10

Test locations:
- `internal/core/settings_test.go` — backend unit
- `internal/core/settings_integration_test.go` — backend integration with tempdir `config.json`
- `internal/ui/views/settings_test.go` — frontend (Fyne test driver)
- `linters/ast/settings_no_arrays.go` — AST guard
- `linters/ast/settings_no_direct_color.go` — AST guard


<!-- sandbox-feasibility-legend v1 -->

## Sandbox feasibility legend

Each criterion below is tagged for the implementing AI so it can pick sandbox-implementable rows first:

| Tag | Meaning | Where it runs |
|---|---|---|
| 🟢 | **headless** — pure Go logic, AST scanner, SQL, registry, lint rule, errtrace, code-quality check | Sandbox: `nix run nixpkgs#go -- test -tags nofyne ./...` |
| 🟡 | **cgo-required** — Fyne canvas / widget render / focus ring / hover / pulse / pixel contrast / screen-reader runtime | Workstation only (CGO + display server) |
| 🔴 | **bench / E2E** — perf gate (`P-*`), benchmark, race detector under UI, multi-process integration | CI infra only |

See also: [`mem://design/schema-naming-convention.md`](mem://design/schema-naming-convention.md), `.lovable/cicd-issues/03-fyne-canvas-needs-cgo.md`, `.lovable/cicd-issues/05-no-bench-infra.md`.

---
---

## A. Functional — Backend

| # | Criterion | Test ID |
|---|---|---|
| AC-SB-01 | 🟡 `Get` on a fresh `config.json` (none of the four owned objects present) returns the documented defaults from `01-backend.md` §3. | `Test_Get_Defaults` |
| AC-SB-02 | 🟡 `Get` after `Save(in)` returns a snapshot byte-equivalent to `in` after normalization. | `Test_Get_AfterSave_Roundtrip` |
| AC-SB-03 | 🟢 `Save` with `PollSeconds = 0` returns `ER-SET-21771`; `config.json` is unchanged. | `Test_Save_PollSeconds_Zero` |
| AC-SB-04 | 🟢 `Save` with `PollSeconds = 61` returns `ER-SET-21771`. | `Test_Save_PollSeconds_TooHigh` |
| AC-SB-05 | 🟢 `Save` with `Theme = "purple"` returns `ER-SET-21772`. | `Test_Save_Theme_Unknown` |
| AC-SB-06 | 🟢 `Save` with scheme `"javascript"` returns `ER-SET-21773`. | `Test_Save_Scheme_Forbidden_Javascript` |
| AC-SB-07 | 🟢 `Save` with scheme `"file"` returns `ER-SET-21773`. | `Test_Save_Scheme_Forbidden_File` |
| AC-SB-08 | 🟢 `Save` with non-existent `ChromePath` returns `ER-SET-21774`. | `Test_Save_ChromePath_Missing` |
| AC-SB-09 | 🟢 `Save` with `IncognitoArg = "; rm -rf /"` returns `ER-SET-21775`. | `Test_Save_IncognitoArg_Injection` |
| AC-SB-10 | 🟢 `Save` with `AllowLocalhostUrls = true` and schemes `["https"]` returns `ER-SET-21777`. | `Test_Save_Localhost_Without_Http` |
| AC-SB-11 | 🟡 `Save` preserves `Accounts` array byte-identically (golden-file diff over 5 mixed accounts). | `Test_Save_Accounts_Untouched` |
| AC-SB-12 | 🟡 `Save` preserves `Rules` array byte-identically (golden-file diff over 12 mixed rules). | `Test_Save_Rules_Untouched` |
| AC-SB-13 | 🟡 `Save` preserves unknown top-level keys (`schemaVersion`, `_comment`, …). | `Test_Save_UnknownKeys_Untouched` |
| AC-SB-14 | 🟡 Atomic write: simulating SIGKILL between tmp-write and rename leaves the original `config.json` intact and parseable. | `Test_Save_AtomicCrash` |
| AC-SB-15 | 🟡 Two concurrent `Save` calls produce exactly one of the two snapshots — never a merge. | `Test_Save_ConcurrentRaces` |
| AC-SB-16 | 🟡 `ResetToDefaults` resets only `PollSeconds`, `BrowserOverride`, `Theme`, `OpenUrlAllowedSchemes`, `AllowLocalhostUrls`, `AutoStartWatch`; leaves `Accounts`/`Rules` byte-identical. | `Test_Reset_DoesNotTouchAccountsRules` |
| AC-SB-17 | 🟡 `Save` emits exactly one `SettingsEvent{Kind: SettingsSaved}`. | `Test_Save_EventEmitted` |
| AC-SB-18 | 🟡 `ResetToDefaults` emits `SettingsEvent{Kind: SettingsResetApplied}`. | `Test_Reset_EventEmitted` |
| AC-SB-19 | 🟡 `Subscribe` cancel func stops delivery; later `Save` does not panic and does not deadlock. | `Test_Subscribe_CancelClean` |
| AC-SB-20 | 🟡 `DetectChrome` returns `ChromeFromConfig` when `ChromePath` is set. | `Test_Detect_ConfigPrecedence` |
| AC-SB-21 | 🟡 `DetectChrome` returns `ChromeFromEnv` when `MAILPULSE_CHROME` is set and config is empty. | `Test_Detect_EnvPrecedence` |
| AC-SB-22 | 🟡 `DetectChrome` returns `ChromeNotFound` (NOT an error) when nothing is found. | `Test_Detect_NotFound` |
| AC-SB-23 | 🟢 `os.Stat` permission denied during detection logs `ER-SET-21780` at WARN and returns `ChromeNotFound`. | `Test_Detect_PermissionDenied` |
| AC-SB-24 | 🔴 `go test -race ./internal/core/...` passes for every test above. | `make test-race-settings` |

## B. Functional — Frontend

| # | Criterion | Test ID |
|---|---|---|
| AC-SF-01 | 🟡 Initial render shows the four cards in order: Paths, Watcher, Browser, Appearance. | `Test_Render_CardOrder` |
| AC-SF-02 | 🟡 Save button is disabled on first render. | `Test_Render_SaveDisabled` |
| AC-SF-03 | 🟡 Read-only path rows are not `*widget.Entry`. | `Test_Render_PathsReadOnly` |
| AC-SF-04 | 🟡 Typing `0` in Poll Seconds sets `errPoll` and disables Save within one frame. | `Test_Validate_PollZero` |
| AC-SF-05 | 🟡 Typing `30` clears `errPoll` and enables Save (when dirty). | `Test_Validate_PollValid_Enables` |
| AC-SF-06 | 🟡 Adding `javascript` to schemes sets `errSchemes`. | `Test_Validate_Scheme_Js` |
| AC-SF-07 | 🟡 Toggling Allow-localhost without `http` sets `errSchemes` (composite). | `Test_Validate_Localhost_Composite` |
| AC-SF-08 | 🟡 Editing and reverting clears `dirty`. | `Test_Dirty_RevertClears` |
| AC-SF-09 | 🟡 Whitespace-only edit to ChromePath does NOT mark dirty. | `Test_Dirty_WhitespaceIgnored` |
| AC-SF-10 | 🟡 Rapid double-click on Save calls `svc.Save` exactly once. | `Test_Save_Debounced` |
| AC-SF-11 | 🟡 Successful Save updates `loaded` and disables Save again. | `Test_Save_Success_RedisableSave` |
| AC-SF-12 | 🟢 Failed Save (`ER-SET-21778`) shows banner with Retry; does not clear `loaded`. | `Test_Save_Failure_BannerRetry` |
| AC-SF-13 | 🟡 Tap "Detect" calls `svc.DetectChrome` once and updates the detected label. | `Test_Detect_OneCall` |
| AC-SF-14 | 🟡 Detect never overwrites user-typed `ChromePath`. | `Test_Detect_PreservesUserInput` |
| AC-SF-15 | 🟡 External `SettingsEvent` while clean updates non-dirty bindings in place. | `Test_LiveReload_Clean` |
| AC-SF-16 | 🟡 External `SettingsEvent` while dirty leaves local bindings alone and shows reload banner. | `Test_LiveReload_DirtyConflict` |
| AC-SF-17 | 🟡 Selecting Light theme immediately calls `theme.Apply(ThemeLight)`. | `Test_Theme_LivePreview` |
| AC-SF-18 | 🟡 Discarding via Leave-confirm restores the previous theme. | `Test_Theme_DiscardRestores` |
| AC-SF-19 | 🟡 Leave with dirty fields shows confirm dialog and blocks navigation until resolved. | `Test_Leave_DirtyBlocks` |
| AC-SF-20 | 🟡 Reset confirm dialog explicitly mentions Accounts/Rules are NOT affected. | `Test_Reset_ConfirmCopy` |
| AC-SF-21 | 🟡 `[manual]` Tab order traverses cards top-to-bottom. | QA checklist |

## C. Cross-Feature Acceptance (the original AC-S1..AC-S5 from `00-overview.md`)

| # | Original | Maps to |
|---|---|---|
| AC-S1 | Saving poll outside 1–60 shows inline error and does not write. | AC-SB-03, AC-SB-04, AC-SF-04 |
| AC-S2 | Theme switch applies immediately without restart. | AC-SF-17 |
| AC-S3 | "Detect" shows resolved Chrome path + source. | AC-SB-20, AC-SB-21, AC-SF-13 |
| AC-S4 | Clicking a path row opens OS file manager. | `Test_Paths_OpenInFileManager` (frontend) |
| AC-S5 | After Save, open Watch tabs respect new poll interval on next cycle. | `Test_Watch_PollReload_OnSettingsEvent` in `02-features/05-watch/01-backend_test.go` (cross-feature) |

## D. Security / Anti-Feature Invariants

| # | Criterion | Test ID |
|---|---|---|
| AC-SX-01 | 🟡 AST scan: only `internal/config` writes `config.json`. | `Test_AST_OnlyConfigWritesFile` |
| AC-SX-02 | 🟡 AST scan: `Settings.Save` body never references identifiers `Accounts` or `Rules`. | `Test_AST_Save_NoAccountsRulesRefs` |
| AC-SX-03 | 🟡 AST scan: `internal/ui/views/settings.go` contains no `color.RGBA{` / `color.NRGBA{` literals. | `Test_AST_Settings_NoDirectColor` |
| AC-SX-04 | 🟡 Log scan: `ChromePath` value never appears at level ≥ INFO across all Settings tests. | `Test_LogScan_NoChromePathLeak` |
| AC-SX-05 | 🟢 Log scan: `IncognitoArg` value never appears at any level. | `Test_LogScan_NoIncognitoArgLeak` |
| AC-SX-06 | 🟡 Forbidden-scheme list (`file`, `javascript`, `data`, `vbscript`) is rejected by both backend (§6) and frontend (§5) — verified via shared table-driven test fixture. | `Test_ForbiddenSchemes_BothLayers` |

## E. Performance

| # | Criterion | Threshold |
|---|---|---|
| AC-SP-01 | 🔴 `Get` cold (first call after process start) | ≤ 25 ms p95 on a 50 KB `config.json` |
| AC-SP-02 | 🔴 `Get` warm (cached snapshot) | ≤ 1 ms p95 |
| AC-SP-03 | 🔴 `Save` end-to-end (validate + atomic write + event publish) | ≤ 50 ms p95 on a 50 KB file |
| AC-SP-04 | 🔴 `DetectChrome` total | ≤ 100 ms p95 (probes + `os.Stat`, no `exec`) |
| AC-SP-05 | 🟡 UI Save click → button re-enabled | ≤ 80 ms p95 (excludes IO) |

## F. Definition of Done

All AC-SB-*, AC-SF-* (except `[manual]`), AC-SX-*, AC-SP-* automated tests pass on `linux/amd64`, `darwin/arm64`, `windows/amd64`. `[manual]` items signed off by QA on at least one OS. `make spec-check` (defined in root Makefile) reports zero TODOs in `02-features/07-settings/`.
