# 06 — Tools — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, view-model, theming, interaction, lifecycle, and streaming-output wiring** for the Tools view — a single sidebar route that hosts four sub-tool tabs: **Read**, **Export CSV**, **Diagnose**, **OpenUrl**. Lives in `internal/ui/views/tools.go` — the only file in `internal/ui` permitted to compose Tools widgets.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.6
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.6
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`
- Sibling consumers of `core.Tools.OpenUrl`: `02-features/03-rules/02-frontend.md`, `02-features/05-watch/02-frontend.md`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/tools.go
package views

type ToolsVM struct {
    svc        *core.Tools
    accSvc     *core.Accounts
    nav        NavRouter

    // Shared
    aliasOptions   binding.StringList   // mirrors AccountsVM.PickerSnapshot
    activeTab      binding.String       // "Read" | "Export" | "Diagnose" | "OpenUrl"

    // Read sub-tool
    readForm       *ReadFormState
    readOutput     binding.StringList   // streaming log lines (cap 2000)
    readRunning    binding.Bool
    readReport     binding.Untyped      // *core.ReadResult (final)

    // Export sub-tool
    exportForm     *ExportFormState
    exportProgress binding.Untyped      // *core.ExportProgress (latest only)
    exportRunning  binding.Bool
    exportReport   binding.Untyped      // *core.ExportReport (final)

    // Diagnose sub-tool
    diagAlias      binding.String
    diagForce      binding.Bool
    diagSteps      binding.UntypedList  // []*core.DiagnosticsStep (always 5; pre-seeded Pending)
    diagRunning    binding.Bool
    diagReport     binding.Untyped      // *core.DiagnosticsReport (final)

    // OpenUrl sub-tool
    openForm       *OpenUrlFormState
    openRunning    binding.Bool
    openReport     binding.Untyped      // *core.OpenUrlReport (final)
    openRecent     binding.UntypedList  // []*core.OpenedUrlRow (latest 100)

    // Internal
    cancelRead     context.CancelFunc
    cancelExport   context.CancelFunc
    cancelDiag     context.CancelFunc
    cancelOpen     context.CancelFunc
    accSub         func()               // unsubscribe handle for AccountEvent
    clock          Clock
    log            Logger
}

type ReadFormState struct {
    Alias  binding.String
    Limit  binding.Int                  // 1..500; default 10
    Errors binding.Untyped              // map[string]string
}

type ExportFormState struct {
    Alias     binding.String
    From      binding.Untyped           // *time.Time
    To        binding.Untyped           // *time.Time
    OutPath   binding.String
    Overwrite binding.Bool
    Errors    binding.Untyped
}

type OpenUrlFormState struct {
    Url      binding.String
    Origin   binding.String             // pinned to "Manual" in this tab; not user-editable
    Errors   binding.Untyped
}
```

### 1.1 Constructor

```go
func NewToolsVM(svc *core.Tools, accSvc *core.Accounts, nav NavRouter, clock Clock, log Logger) *ToolsVM
```

- Pure assembly; no service calls.
- Pre-seeds `diagSteps` with 5 `Pending` placeholders so the checklist UI renders immediately.

### 1.2 Buffer caps

```go
const (
    ToolsReadOutputCap   = 2000  // raw-log style; matches Watch's WatchRawLogCap for consistency
    ToolsOpenRecentCap   = 100   // matches RecentOpenedUrls default Limit
    ToolsDiagStepCount   = 5     // constant: DnsLookup, TcpConnect, TlsHandshake, ImapLogin, InboxSelect
)
```

`ToolsReadOutputCap` MUST equal `WatchRawLogCap` from `05-watch/02-frontend.md` §1.2 (consistency report cross-checks this).

---

## 2. Widget Tree

### 2.1 Top-level layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ToolsView (container.NewBorder)                                         │
│ ┌─ Top: ToolsHeader (height 56 px) ───────────────────────────────────┐ │
│ │  Tools                                                              │ │
│ │  Run one-shot operations. Each tool runs in isolation.              │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌─ Center: container.NewAppTabs ──────────────────────────────────────┐ │
│ │  ┌─[ Read ]─[ Export CSV ]─[ Diagnose ]─[ OpenUrl ]──────────────┐  │ │
│ │  │  Active tab content (form pane + output pane stacked vertically)│ │
│ │  └────────────────────────────────────────────────────────────────┘  │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

```go
func (vm *ToolsVM) Build() fyne.CanvasObject {
    header := vm.buildHeader()
    tabs := container.NewAppTabs(
        container.NewTabItemWithIcon("Read",       theme.MailComposeIcon(),  vm.buildReadTab()),
        container.NewTabItemWithIcon("Export CSV", theme.DocumentSaveIcon(), vm.buildExportTab()),
        container.NewTabItemWithIcon("Diagnose",   theme.HelpIcon(),         vm.buildDiagnoseTab()),
        container.NewTabItemWithIcon("OpenUrl",    theme.MailForwardIcon(),  vm.buildOpenUrlTab()),
    )
    tabs.OnSelected = func(item *container.TabItem) { vm.activeTab.Set(item.Text) }
    return container.NewBorder(header, nil, nil, nil, tabs)
}
```

### 2.2 Read tab (`buildReadTab`)

| Region        | Widget                              | Binding                  | Behavior                                    |
|---------------|-------------------------------------|--------------------------|---------------------------------------------|
| Form          | `*widget.Form` with `Alias` (Select), `Limit` (Entry, validate int 1..500) | `vm.readForm.*` | Inline field errors via `vm.readForm.Errors` |
| Action row    | `*widget.Button` "Run" / "Cancel"   | toggles via `vm.readRunning` | See §4.1 |
| Output panel  | `*widget.List` virtualized monospace 12 pt | `vm.readOutput`    | Scroll auto-sticks to bottom unless user scrolled up >64 px |
| Report panel  | `*widget.Card` "Result"             | `vm.readReport`          | Hidden until `readReport != nil`; shows `UidsScanned`, `len(Emails)`, `DurationMs` |

### 2.3 Export tab (`buildExportTab`)

| Region        | Widget                              | Binding                  | Behavior                                    |
|---------------|-------------------------------------|--------------------------|---------------------------------------------|
| Form          | `*widget.Form` with `Alias` (Select), `From` / `To` (date pickers), `OutPath` (Entry + Browse… button), `Overwrite` (Check) | `vm.exportForm.*` | Browse button opens `dialog.NewFileSave` defaulting to `~/Documents/email-read-export-{alias}-{ts}.csv` |
| Action row    | "Run" / "Cancel"                    | `vm.exportRunning`       | See §4.2 |
| Progress      | `*widget.ProgressBar` + `*widget.Label` | `vm.exportProgress` | `ProgressBar.SetValue(RowsWritten / TotalRows)` (≥0); label "{Phase}: {RowsWritten}/{TotalRows}" |
| Report        | `*widget.Card` "Result"             | `vm.exportReport`        | Shows `OutPath` (clickable → `core.Tools.OpenUrl` with `file://` scheme — but **wait**: scheme is forbidden by §5.1 of overview; instead opens an OS file-manager via `internal/browser.RevealInFileManager`) |

### 2.4 Diagnose tab (`buildDiagnoseTab`)

| Region        | Widget                              | Binding                  | Behavior                                    |
|---------------|-------------------------------------|--------------------------|---------------------------------------------|
| Form          | `Alias` (Select) + `Force` (Check "Bypass cache") | `vm.diagAlias`, `vm.diagForce` | — |
| Action row    | "Run" / "Cancel"                    | `vm.diagRunning`         | See §4.3 |
| Checklist     | `*widget.List` of 5 step rows       | `vm.diagSteps`           | See §2.4.1 |
| Cache notice  | `*widget.Label`                     | shown iff `report.Cached` | "Result from cache (≤ 60 s old). Tick 'Bypass cache' to force re-run." |
| Report        | `*widget.Card`                      | `vm.diagReport`          | Shows `OverallOk`, `DurationMs`, `Cached`   |

#### 2.4.1 Step row widget (`diagStepRow`)

```
┌─ HBox ──────────────────────────────────────────────────────────────┐
│ [● status icon]  StepName                  Detail / Err.Message      │
└──────────────────────────────────────────────────────────────────────┘
```

Status icon mapping (token-resolved, no hex):

| Status   | Icon (token)            | Color (token)        |
|----------|-------------------------|----------------------|
| Pending  | `ColorDiagStepPending`  | grey 400             |
| Running  | `ColorDiagStepRunning`  | blue 400 (spinner)   |
| Pass     | `ColorDiagStepPass`     | green 500            |
| Fail     | `ColorDiagStepFail`     | red 500              |
| Skipped  | `ColorDiagStepSkipped`  | grey 300             |

Detail column shows `Detail` for Pass and `Err.Code: Err.Message` for Fail.

### 2.5 OpenUrl tab (`buildOpenUrlTab`)

The most security-sensitive UI in the app. Layout makes the audit trail visible.

| Region            | Widget                                | Binding                  | Behavior                                  |
|-------------------|---------------------------------------|--------------------------|-------------------------------------------|
| Notice            | `*widget.Card` (info)                 | static                   | "Every URL opens **incognito** and is recorded in the audit log." |
| Form              | `*widget.Entry` URL (multi-line off)  | `vm.openForm.Url`        | Validate live: length ≤ 8192, scheme ∈ {http, https}; show inline error |
| Origin (read-only) | `*widget.Label` "Origin: Manual"     | static                   | NOT user-editable — `OriginCard`/`OriginRule` only ever come from caller code |
| Action row        | "Open" / "Cancel"                     | `vm.openRunning`         | See §4.4 |
| Result panel      | `*widget.Card` "Last open"            | `vm.openReport`          | Shows redacted `Url`, `Origin`, `BrowserBinary`, `Deduped`, `OpenedAt` |
| Recent panel      | `*widget.Card` "Recent (last 100)"    | `vm.openRecent`          | `*widget.List` of `OpenedUrlRow`s; each row clickable to copy `Url` to clipboard (NEVER reopens — audit-only view) |

**Critical UI invariant**: the Recent panel must NEVER offer a "re-open" button. If the user wants to re-open, they paste again — that is intentional friction so dedup behaviour is observable.

### 2.6 Header (`buildHeader`)

Static `*widget.Card` with title "Tools" and subtitle. No live bindings; no actions.

---

## 3. Theming

### 3.1 New tokens

```go
const (
    ColorDiagStepPending  = "diagStepPending"
    ColorDiagStepRunning  = "diagStepRunning"
    ColorDiagStepPass     = "diagStepPass"
    ColorDiagStepFail     = "diagStepFail"
    ColorDiagStepSkipped  = "diagStepSkipped"
    ColorOpenUrlSafe      = "openUrlSafe"      // green badge for OriginRule
    ColorOpenUrlManual    = "openUrlManual"    // amber badge for OriginManual
    ColorOpenUrlDeduped   = "openUrlDeduped"   // blue badge "deduped"
    ColorReadOutputDim    = "readOutputDim"    // monospace log lines
    ColorExportProgress   = "exportProgress"   // progress bar fill
)
```

All status / origin / dedup colour usage routes through the token table — no hex literals (lint Q-11).

### 3.2 Status icon map (exhaustive over `DiagnosticsStepStatus`)

```go
var diagStatusToken = map[core.DiagnosticsStepStatus]string{
    core.DiagnosticsStepPending: ColorDiagStepPending,
    core.DiagnosticsStepRunning: ColorDiagStepRunning,
    core.DiagnosticsStepPass:    ColorDiagStepPass,
    core.DiagnosticsStepFail:    ColorDiagStepFail,
    core.DiagnosticsStepSkipped: ColorDiagStepSkipped,
}
```

`golangci-lint exhaustive` enforces that every enum value has a token; adding a new status without updating this map fails the build.

---

## 4. Interactions & Events

### 4.1 Read — Run / Cancel

```go
func (vm *ToolsVM) handleReadRun() {
    if vm.readRunning.Get() { vm.handleReadCancel(); return }
    spec, valid := vm.validateReadForm()
    if !valid { return }
    vm.readRunning.Set(true)
    ctx, cancel := context.WithCancel(vm.rootCtx)
    vm.cancelRead = cancel
    progress := make(chan string, 64)
    go vm.drainReadProgress(progress)         // appends to vm.readOutput
    go func() {
        defer vm.readRunning.Set(false)
        if r := vm.svc.ReadOnce(ctx, spec, progress); r.IsErr() {
            vm.showError(r.Err()); return
        }
        vm.readReport.Set(r.Ok())
    }()
}

func (vm *ToolsVM) handleReadCancel() {
    if vm.cancelRead != nil { vm.cancelRead(); vm.cancelRead = nil }
}
```

- Run button text/colour swaps to "Cancel" (danger token) while `readRunning == true`.
- The `progress` channel is closed by `core.Tools.ReadOnce` (callee-closes contract from backend §2.1); `drainReadProgress` exits cleanly on close.

### 4.2 Export — Run / Cancel

Identical pattern to Read, with `ExportProgress` channel (cap 8). On Cancel, the partial file is removed best-effort by the backend (per backend §7); no UI-side cleanup.

### 4.3 Diagnose — Run / Cancel

Same pattern. Pre-seeds `vm.diagSteps` with 5 `Pending` rows on Run; the streaming channel updates each row in place by index.

```go
func (vm *ToolsVM) drainDiagSteps(ch <-chan core.DiagnosticsStep) {
    i := 0
    for step := range ch {
        // Find the matching seeded row by Name; fall back to index if needed.
        idx := vm.findDiagStepIdx(step.Name)
        vm.diagSteps.SetValue(idx, &step)
        i++
    }
}
```

### 4.4 OpenUrl — Open / Cancel

```go
func (vm *ToolsVM) handleOpenRun() {
    if vm.openRunning.Get() { vm.handleOpenCancel(); return }
    raw, _ := vm.openForm.Url.Get()
    spec := core.OpenUrlSpec{Url: raw, Origin: core.OpenUrlOriginManual} // pinned
    vm.openRunning.Set(true)
    ctx, cancel := context.WithCancel(vm.rootCtx)
    vm.cancelOpen = cancel
    go func() {
        defer vm.openRunning.Set(false)
        r := vm.svc.OpenUrl(ctx, spec)
        if r.IsErr() {
            vm.showError(r.Err())   // Friendly mapping of 21760..21767 per §4.4.1
            return
        }
        vm.openReport.Set(r.Ok())
        vm.refreshRecent(ctx)         // re-query RecentOpenedUrls to include this row
    }()
}
```

#### 4.4.1 Friendly error mapping for OpenUrl errors

| Code  | Toast text                                                            |
|-------|-----------------------------------------------------------------------|
| 21760 | "URL is empty."                                                       |
| 21761 | "URL is too long (max 8192 chars)."                                   |
| 21762 | "URL is malformed."                                                   |
| 21763 | "Scheme `{scheme}` is not allowed. Use http or https."                |
| 21764 | "Localhost URLs are blocked. Enable in Settings → Browser."           |
| 21765 | "Private-IP URLs are blocked. Enable in Settings → Browser."          |
| 21766 | "No browser found. Set a Browser override in Settings."               |
| 21767 | "Browser launch failed. See log for OS error."                        |
| 21768 | "Browser opened, but audit insert failed. Check Recent panel."        |

**No raw stack traces** ever surface in toasts; the structured `apperror` log carries the trace.

### 4.5 Tab switching

`tabs.OnSelected` updates `vm.activeTab` only. It does NOT cancel any in-flight sub-tool — running operations continue in the background. The action button on each tab reflects its own running state.

### 4.6 Keyboard shortcuts (registered via `internal/ui/shortcuts.go`, scope = ToolsView focused)

| Key              | Action                            |
|------------------|-----------------------------------|
| `Cmd/Ctrl+Enter` | Run/Cancel current tab            |
| `Cmd/Ctrl+1..4`  | Switch to tab 1..4                |
| `Cmd/Ctrl+L`     | Clear current tab's output panel  |

---

## 5. Lifecycle

### 5.1 `Attach(ctx context.Context)`

```go
func (vm *ToolsVM) Attach(ctx context.Context) errtrace.Result[Unit] {
    vm.rootCtx = ctx
    if r := vm.refreshAliasOptions(ctx); r.IsErr() { return r }
    if r := vm.refreshRecent(ctx);       r.IsErr() { return r }
    vm.accSub = vm.accSvc.Subscribe(ctx, vm.onAccountEvent)
    return errtrace.Ok(Unit{})
}
```

### 5.2 `Detach()`

```go
func (vm *ToolsVM) Detach() {
    // Cancel ALL in-flight sub-tools — Detach is the user navigating away.
    for _, c := range []*context.CancelFunc{&vm.cancelRead, &vm.cancelExport, &vm.cancelDiag, &vm.cancelOpen} {
        if *c != nil { (*c)(); *c = nil }
    }
    if vm.accSub != nil { vm.accSub(); vm.accSub = nil }
    // Buffers retained — re-Attach restores the visible state instantly.
}
```

Cancellation latency is bounded by the backend's 500 ms SLO (per backend §10).

### 5.3 Goroutine inventory (must be exact)

| Goroutine            | Lifetime                            | Purpose                              |
|----------------------|-------------------------------------|--------------------------------------|
| `drainReadProgress`  | Run → channel close                 | Append streaming lines to `readOutput` |
| `drainExportProgress`| Run → channel close                 | Update `exportProgress` binding      |
| `drainDiagSteps`     | Run → channel close                 | Update step rows in place            |
| Sub-tool runner (×4) | Run → svc method returns            | Calls `core.Tools.{ReadOnce,ExportCsv,Diagnose,OpenUrl}` |
| `accountEvent` callback | Attach → Detach (via `accSub`)   | Refreshes `aliasOptions`             |

Tests assert leak-free shutdown via `goleak.VerifyNone(t)`.

---

## 6. Live-Update Reactions

| Trigger                                           | Action                                                    |
|---------------------------------------------------|-----------------------------------------------------------|
| `AccountEvent{Added/Removed/Renamed}`             | Refresh `aliasOptions` (all four sub-tool dropdowns)      |
| `AccountEvent{Updated, Alias}`                    | (Backend invalidates Diagnose cache; UI no-op)            |
| `AccountEvent{Removed, Alias}` matches active form alias | Clear that form's alias field; show inline notice    |
| Read progress channel item                        | `vm.readOutput.Append(line)` (cap-evict from front)       |
| Export progress channel item                      | `vm.exportProgress.Set(&p)`; bar + label update           |
| Diagnose step channel item                        | `vm.diagSteps.SetValue(idx, &step)` in place              |
| OpenUrl `Deduped` result                          | Toast "Already opened in last `{N}`s — see Recent panel"; Recent panel highlights the dedup row |

---

## 7. Performance Budgets

| Operation                                          | Budget       | Bench                              |
|----------------------------------------------------|--------------|------------------------------------|
| `Attach()` to first paint                          | ≤ **80 ms**  | `BenchmarkToolsAttach`             |
| Tab switch                                         | ≤ **8 ms**   | `BenchmarkToolsTabSwitch`          |
| Read output line append                            | ≤ **1 ms**   | `BenchmarkReadAppend`              |
| Export progress bar update                         | ≤ **1 ms**   | `BenchmarkExportProgress`          |
| Diagnose step row in-place mutate                  | ≤ **2 ms**   | `BenchmarkDiagStepMutate`          |
| OpenUrl Recent refresh (100 rows)                  | ≤ **20 ms**  | `BenchmarkOpenRecentRefresh`       |
| Memory ceiling for `readOutput + openRecent`       | ≤ **1 MiB**  | `TestToolsMemoryCeiling`           |

Ring-buffer impl reused from `internal/util/ringbuf/` (typed generic; no `any`) for `readOutput`. `openRecent` is a bounded `binding.UntypedList` re-fetched on demand.

---

## 8. Accessibility

- All step-status icons accompanied by step name + status text — colour never carries semantic load alone.
- Cancel button has `aria-label` "Cancel running operation".
- OpenUrl's incognito notice uses both colour AND text ("incognito" word always present).
- Reduced motion: spinners use static frame when `fyne.CurrentApp().Settings().ReducedMotion()`.
- Focus order per tab: form fields → action button → output panel.

---

## 9. Testing Contract

All tests in `internal/ui/views/tools_test.go` using the headless Fyne test driver.

### 9.1 Required tests

1. `ToolsVM_Attach_LoadsAliasesAndRecent`
2. `ToolsVM_TabSwitch_PreservesInFlightOperations`
3. `ToolsVM_Read_ValidationBlocksRun_LimitOutOfRange`
4. `ToolsVM_Read_StreamingLinesAppearMidRun`
5. `ToolsVM_Read_Cancel_StopsUnder500ms`
6. `ToolsVM_Read_OutputCappedAt2000`
7. `ToolsVM_Export_BrowseDefaultsToDocumentsPath`
8. `ToolsVM_Export_ProgressBarReflectsRowsWritten`
9. `ToolsVM_Export_PathExistsTriggersOverwriteConfirm`
10. `ToolsVM_Export_Cancel_RemovesPartialFile`
11. `ToolsVM_Diag_PreSeeds5PendingRows`
12. `ToolsVM_Diag_StepsUpdateInPlaceByName`
13. `ToolsVM_Diag_FailMarksRemainingSkipped`
14. `ToolsVM_Diag_CacheHit_ShowsCacheNotice`
15. `ToolsVM_Diag_ForceBypassesCache`
16. `ToolsVM_OpenUrl_OriginPinnedManual_NotEditable`
17. `ToolsVM_OpenUrl_LiveValidation_BlocksJavascriptScheme`
18. `ToolsVM_OpenUrl_HappyPath_RecentRefreshesIncludingNewRow`
19. `ToolsVM_OpenUrl_Deduped_ShowsToastAndHighlightsRecentRow`
20. `ToolsVM_OpenUrl_RecentPanel_HasNoReopenButton` (anti-feature lint)
21. `ToolsVM_OpenUrl_FriendlyErrorMapping_AllCodes_21760_to_21768`
22. `ToolsVM_NoHardcodedColors`
23. `ToolsVM_DiagStatusTokenMap_ExhaustiveOverEnum`
24. `ToolsVM_Detach_CancelsAllSubTools_NoLeak`
25. `ToolsVM_Header_RendersStaticOnly_NoServiceCalls`
26. `ToolsVM_Export_NoBrowserLaunchOnReport` (verifies "Reveal in file manager" path, NOT `core.Tools.OpenUrl` with `file://` — that would be 21763)

### 9.2 Lint gates

- `golangci-lint exhaustive` on `diagStatusToken` map.
- `forbidigo` blocks `os/exec`, `net/http`, `database/sql`, browser binary literals (`xdg-open`, `open`, `start`).
- `linters/no-internal-from-views.sh` — no direct imports of `internal/exporter`, `internal/mailclient`, `internal/store`, `internal/browser`.
- AST scan: `core.Tools.OpenUrl` is the **only** way this view triggers a browser launch (test #26 reinforces).

---

## 10. Compliance Checklist

- [x] PascalCase identifiers throughout (§1).
- [x] No `any` / `interface{}` — `binding.Untyped` wrapped in typed accessors (§1).
- [x] No imports of `internal/exporter`, `internal/mailclient`, `internal/store`, `internal/browser` from this file (lint).
- [x] No hex color literals — every color via `internal/ui/theme` token (§3).
- [x] All four sub-tools follow Run/Cancel + streaming-channel pattern uniformly (§4).
- [x] OpenUrl tab pins `Origin = Manual` — caller cannot spoof `OriginRule`/`OriginCard` (§4.4).
- [x] Recent panel has no re-open affordance (anti-feature; test #20).
- [x] Cancellation propagates to backend within 500 ms (per backend SLO; §5.2).
- [x] Goroutine inventory exact (5 patterns, leak-tested).
- [x] No raw error stacks in user-facing toasts (§4.4.1).
- [x] `ToolsReadOutputCap == WatchRawLogCap` (cross-feature consistency anchor; §1.2).
- [x] Friendly error mapping covers every code in `21760..21768` (test #21).
- [x] Diagnose checklist pre-seeds 5 Pending rows so layout doesn't shift on first stream (§4.3).

---

**End of `06-tools/02-frontend.md`**
