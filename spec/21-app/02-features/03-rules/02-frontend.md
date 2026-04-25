# 03 — Rules — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, view-model, theming, interaction, and lifecycle** for the Rules view. Lives in `internal/ui/views/rules.go` — the only file in `internal/ui` permitted to compose Rules widgets.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.3
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/rules.go
package views

type RulesVM struct {
    svc       *core.Rules
    watch     *core.Watch
    nav       NavRouter

    rules     binding.UntypedList   // []core.RuleWithStat (sorted by Order)
    selected  binding.Untyped       // *core.RuleWithStat (form right pane)
    formSpec  binding.Untyped       // *core.RuleSpec (edit buffer)
    fieldErrs binding.Untyped       // map[string]string (field → error msg)
    isDirty   binding.Bool
    isLoading binding.Bool
    loadErr   binding.String
    sub       <-chan core.WatchEvent
    cancelSub func()
}

func NewRulesVM(svc *core.Rules, watch *core.Watch, nav NavRouter) *RulesVM

func (vm *RulesVM) Refresh(ctx context.Context)
func (vm *RulesVM) SelectRule(name string)
func (vm *RulesVM) NewRule()                                // resets form to defaults
func (vm *RulesVM) SaveForm(ctx context.Context)            // Create or Update
func (vm *RulesVM) DeleteSelected(ctx context.Context)
func (vm *RulesVM) ToggleEnabled(ctx context.Context, name string)
func (vm *RulesVM) RenameSelected(ctx context.Context, newName string)
func (vm *RulesVM) Reorder(ctx context.Context, namesInOrder []string)
func (vm *RulesVM) DryRun(ctx context.Context, sample core.EmailSample) errtrace.Result[core.RuleMatch]
func (vm *RulesVM) AttachLive(ctx context.Context)
func (vm *RulesVM) DetachLive()
```

**Rules:**
- VM holds **no Fyne widgets** — only `binding.*` and a small navigation router interface.
- All long calls happen off the UI goroutine; results pushed via `binding.*.Set`.
- VM is the **only** consumer of `core.Rules` from `internal/ui`.
- Form validation runs **client-side** for empty/syntax checks before calling `SaveForm`; the server is the source of truth and re-validates.

---

## 2. Widget Tree

```
RulesView (container.NewBorder)
├── Top:    Toolbar (container.NewHBox)
│           ├── widget.NewButtonWithIcon("New rule", theme.ContentAddIcon(), vm.OnNewRule)
│           ├── widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), vm.OnRefresh)
│           ├── layout.NewSpacer()
│           └── widget.NewLabel("{Count} rules · {EnabledCount} enabled")
├── Center: container.NewHSplit(0.45)
│           ├── Left:  RuleListPane    (container.NewBorder)
│           │         ├── Top:    SearchEntry "Filter by name…" (debounced 200 ms)
│           │         └── Center: widget.List with drag handles  // RuleRow template
│           └── Right: RuleFormPane    (container.NewBorder)
│                     ├── Top:    FormHeader (Name input + Rename button + Enabled switch)
│                     ├── Center: container.NewVScroll(FormFields)
│                     │           ├── widget.NewSelect(Action, vm.OnActionChange)
│                     │           ├── widget.NewEntry "FromRegex" (with FieldErrLabel)
│                     │           ├── widget.NewEntry "SubjectRegex" (with FieldErrLabel)
│                     │           ├── widget.NewMultiLineEntry "BodyRegex" (with FieldErrLabel)
│                     │           ├── widget.NewEntry "UrlRegex" (with FieldErrLabel)
│                     │           └── widget.NewEntry "Order" (numeric)
│                     └── Bottom: FormActionBar
│                                 ├── widget.NewButton("Dry-run…", vm.OnOpenDryRunDialog)
│                                 ├── layout.NewSpacer()
│                                 ├── widget.NewButton("Cancel", vm.OnDiscard)         // disabled when !isDirty
│                                 └── widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), vm.OnSave)
└── Bottom: StatusBar (container.NewHBox)
            ├── widget.NewLabel("Updated {RelativeTime}")
            ├── layout.NewSpacer()
            └── widget.NewProgressBarInfinite()  // visible only while isLoading
```

### 2.1 `RuleRow` (custom widget — list item template)

```
[≡] [●] {Name}                    {Action}    {MatchCount}×  {RelativeTime LastMatchedAt}
        {Snippet of UrlRegex (60ch, --Text-Secondary)}
```

- `[≡]` = drag handle (Fyne `desktop.Draggable`); drop-reorder triggers `vm.Reorder`.
- `[●]` = enabled dot; click toggles `Enabled`. Color: `--Status-Healthy` when enabled, `--Border-Subtle` when disabled.
- Row click selects + populates form pane.
- Long-press / right-click (desktop) → context menu: **Rename**, **Duplicate**, **Delete**.
- Disabled rows render at 60 % opacity.

### 2.2 `FieldErrLabel` (custom widget)

A red `widget.Label` that shows `fieldErrs[FieldName]` directly under the entry. Hidden when no error. Color: `--Status-Danger`. Updates reactively from the `fieldErrs` binding.

### 2.3 `DryRunDialog` (modal)

```
DryRunDialog (dialog.NewCustom)
├── Header: "Dry-run rule: {Name}"
├── Body:   container.NewForm
│           ├── widget.NewEntry "FromAddr"
│           ├── widget.NewEntry "Subject"
│           └── widget.NewMultiLineEntry "BodyText" (rows=10)
├── Result: ResultPanel (visible after Run)
│           ├── widget.NewLabel("✓ Matched · {DurationMicro}µs"  | "✗ No match")
│           ├── PerFieldGrid: From / Subject / Body each ✓ or ✗
│           └── UrlList: widget.List of ExtractedUrls (when Action == OpenUrl)
└── Buttons: [Run] [Close]
```

Calling **Run** invokes `vm.DryRun(ctx, sample)`. Result rendered in place; the dialog stays open so the user can tweak the sample. **Writes nothing** — visually labelled "Read-only — does not affect any email or counter."

### 2.4 `RenameDialog` (modal)

```
RenameDialog
├── widget.NewEntry "New name" (pre-filled with current Name)
├── FieldErrLabel
└── Buttons: [Cancel] [Rename]
```

Click **Rename** invokes `vm.RenameSelected(ctx, newName)`. On `21314 RuleRenameTargetTaken`, the field error is set; dialog stays open.

### 2.5 `DeleteConfirmDialog` (modal)

```
"Delete rule {Name}?"
"Hit-counter history (MatchCount={N}, last matched {RelativeTime}) will be lost.
 Past OpenedUrl entries are kept for audit."
[Cancel] [Delete]
```

---

## 3. States

| State                | Trigger                                | Visible UI                                              |
|----------------------|----------------------------------------|---------------------------------------------------------|
| `Loading`            | First mount / Refresh in flight        | Translucent scrim over list pane                        |
| `Loaded`             | `List` returned successfully           | Full widget tree as §2                                  |
| `Error`              | `List` returned error                  | `ErrorPanel{ErrorCode, Message, RetryButton}` in list   |
| `Empty`              | Loaded, `len(rules) == 0`              | `EmptyState{Icon, Title="No rules yet", CTA="New rule"}`|
| `FormCreating`       | `vm.NewRule` invoked                   | Form pane shown with defaults; FormHeader = "New rule"  |
| `FormEditing`        | Row selected                           | Form pane shown with selected rule's data               |
| `FormDirty`          | `isDirty == true`                      | Cancel button enabled; Save button highlighted          |
| `FormFieldErr`       | Server returned `21312`/`21316`/etc.   | `FieldErrLabel` shown under offending field             |
| `DryRunRunning`      | DryRun dialog Run in flight            | Result panel shows spinner; Run button disabled         |
| `DryRunComplete`     | DryRun returned                        | Result panel populated                                  |

`FormDirty` is computed by deep-equal between `formSpec` and the `selected` rule's spec.

---

## 4. Theming Tokens

Defined in `internal/ui/theme/theme.go`, sourced from `16-app-design-system-and-ui.md`.

| Token                  | Use                                          |
|------------------------|----------------------------------------------|
| `--Surface-Card`       | List + form pane background                  |
| `--Surface-Selected`   | Selected row background                      |
| `--Surface-Drag`       | Row background during drag                   |
| `--Border-Subtle`      | Pane borders, disabled enabled-dot           |
| `--Status-Healthy`     | Enabled dot                                  |
| `--Status-Danger`      | FieldErrLabel text, Delete button hover      |
| `--Status-Info`        | "Read-only" badge in DryRunDialog            |
| `--Text-Primary`       | Rule name                                    |
| `--Text-Secondary`     | UrlRegex snippet, MatchCount, RelativeTime   |
| `--Spacing-Form-Row`   | Form field vertical gap                      |
| `--Radius-Card`        | Pane border-radius                           |

**Hard rule:** No hex colors in `views/rules.go`. Lint rule `no-hex-in-views` enforces this.

---

## 5. Lifecycle Hooks

```go
func (v *RulesView) OnShow()  // vm.Refresh + vm.AttachLive
func (v *RulesView) OnHide()  // PromptIfDirty → vm.DetachLive
```

If `isDirty` is true on `OnHide`, a non-blocking toast offers **Save** / **Discard**; the tab still hides immediately and the change is **discarded** if the user takes no action within 5 s. (No silent data loss without warning.)

`AttachLive` subscribes to `WatchEvent.Kind == RuleMatched`; matching rules' rows update `MatchCount` + `LastMatchedAt` in place without a full refresh.

---

## 6. Interaction Rules

| User action                                | Effect                                                                       |
|--------------------------------------------|------------------------------------------------------------------------------|
| Click **New rule**                         | `vm.NewRule`; form pane shows defaults; focus on Name field                  |
| Click row                                  | `vm.SelectRule(name)`; if `isDirty`, prompt before discarding                |
| Click `[●]` enabled dot                    | `vm.ToggleEnabled(name)`; optimistic flip; rollback on error                 |
| Drag `[≡]` and drop                        | `vm.Reorder(newOrder)`; debounced 300 ms; optimistic; rollback on error      |
| Right-click row → **Rename**               | Opens `RenameDialog`                                                         |
| Right-click row → **Duplicate**            | `vm.NewRule` pre-filled with copy; Name = `"{Name} copy"`                    |
| Right-click row → **Delete**               | Opens `DeleteConfirmDialog`                                                  |
| Type in any regex field                    | Marks `isDirty`; clears matching `fieldErrs` entry                           |
| Click **Save**                             | `vm.SaveForm`; on field-error, set `fieldErrs` and focus offending field     |
| Click **Cancel**                           | Reverts `formSpec` to last-loaded spec                                       |
| Click **Dry-run…**                         | Opens `DryRunDialog` with the current `formSpec` (even if unsaved)           |
| Press `Ctrl+S`                             | Same as Save                                                                 |
| Press `Ctrl+N`                             | Same as New rule                                                             |
| Press `F5`                                 | `vm.Refresh`                                                                 |
| Press `Esc`                                | Closes any open dialog                                                       |
| `WatchEvent.RuleMatched` for visible rule  | Increment that row's `MatchCount`; bump `LastMatchedAt` to event time        |

Drag reorder is **list-only**; no cross-pane drag.

---

## 7. Confirmation & Toasts

| Trigger                          | Confirmation                            | Success toast                       | Failure toast                                          |
|----------------------------------|-----------------------------------------|-------------------------------------|--------------------------------------------------------|
| Save (Create)                    | (no modal)                              | "Rule {Name} created"               | Field-error highlighting + "Save failed. Code E{Code}" |
| Save (Update)                    | (no modal)                              | "Rule {Name} updated"               | same                                                   |
| Delete                           | `DeleteConfirmDialog` (warns hit-history)| "Rule {Name} deleted"               | "Delete failed. Code E{Code}. Retry?"                  |
| Toggle Enabled                   | (no modal)                              | (silent)                            | "Toggle failed. Code E{Code}."                         |
| Reorder                          | (no modal)                              | (silent)                            | "Reorder failed. Code E{Code}."                        |
| Rename                           | `RenameDialog`                          | "Renamed to {NewName}"              | Field-error in dialog                                  |
| Rename atomicity failure (21319) | (no modal)                              | —                                   | "Rename failed; state restored"                        |
| OnHide with dirty form           | Toast w/ Save/Discard buttons           | "Changes saved" / "Changes discarded" | "Save failed; changes kept locally"                  |

---

## 8. Performance Budgets

| Metric                                          | Budget    | Measured by                              |
|-------------------------------------------------|-----------|------------------------------------------|
| Cold mount → first paint                        | < 100 ms  | `views/rules_bench_test.go`              |
| `Refresh` round-trip (200 rules)                | < 40 ms   | same                                     |
| Form field-error highlight after server reply   | < 16 ms   | same                                     |
| Drag-reorder visual feedback                    | < 16 ms   | same (60 FPS)                            |
| DryRun round-trip (100 KB body)                 | < 30 ms   | same (15 ms server + 15 ms render)       |
| Live `RuleMatched` row update                   | < 16 ms   | same                                     |
| Memory footprint (200 rules + dialog)           | < 3 MB    | `go test -memprofile`                    |

---

## 9. Accessibility

- `RuleRow` exposes role `"button"`, `aria-label = "Rule {Name}, action {Action}, {Enabled ? 'enabled' : 'disabled'}, {MatchCount} matches"`.
- Drag handle exposes role `"button"`, `aria-label = "Drag to reorder rule {Name}"`, plus keyboard reorder via `Ctrl+ArrowUp/Down` when focused.
- Enabled dot exposes role `"switch"` with `aria-checked`.
- Form fields expose `aria-invalid="true"` when their `fieldErrs` entry is set; `FieldErrLabel` is referenced via `aria-describedby`.
- Focus order: New rule → Refresh → SearchEntry → list rows → form Name → Action → FromRegex → SubjectRegex → BodyRegex → UrlRegex → Order → Dry-run → Cancel → Save.
- Screen-reader announcement on `Loaded`: `"Showing {Count} rules, {EnabledCount} enabled."`.
- Screen-reader announcement on save: `"Rule {Name} {Created|Updated}."`.
- Screen-reader announcement on field error: `"{FieldName} is invalid: {ErrorMessage}."`.

---

## 10. Testing Contract

File: `internal/ui/views/rules_test.go`. Smoke tests only (per `04-coding-standards.md` §7).

Required test cases:

1. `Mount_RendersToolbarListAndForm`.
2. `NewRule_ResetsFormToDefaults`.
3. `SelectRule_PopulatesForm`.
4. `EditField_SetsIsDirty`.
5. `Save_ValidationErr_HighlightsField` — fake returns 21312 with `Field="UrlRegex"`.
6. `Save_Success_ShowsToast_RefreshesList`.
7. `ToggleEnabled_OptimisticFlip_RollbackOnError`.
8. `DragReorder_DebouncedAt300ms_PersistsOnDrop`.
9. `DryRunDialog_RunsAndShowsResult_WithoutWritingState`.
10. `RenameDialog_TargetTaken_ShowsFieldError_StaysOpen`.
11. `DeleteConfirm_RemovesRowOnConfirm_NoOpOnCancel`.
12. `WatchEvent_RuleMatched_IncrementsRowCount_NoFullRefresh`.
13. `OnHide_WithDirtyForm_ShowsSaveDiscardToast`.
14. `Error_ShowsErrorPanel_PreservesPreviousRows`.

Fakes:
- `core.FakeRules` implementing the same method set with controllable returns.
- `core.FakeWatch` with controllable event channel.
- `MockNavRouter` recording calls.

---

## 11. Compliance Checklist

- [x] Single Fyne-importing file (`internal/ui/views/rules.go`).
- [x] VM holds no widgets, only `binding.*` + interfaces.
- [x] No hex colors; all colors via theme tokens.
- [x] PascalCase widget/struct/binding names.
- [x] Subscription lifecycle paired (`OnShow`/`OnHide`).
- [x] Optimistic UI for toggle/reorder with rollback on error.
- [x] DryRun visually labelled read-only; never writes.
- [x] Dirty-form protection on tab hide (no silent loss).
- [x] Field-level error highlighting via `fieldErrs` + `FieldErrLabel`.
- [x] Performance budgets stated and benchmark-enforced.
- [x] Accessibility roles + announcements + keyboard reorder defined.
- [x] PII (sample `BodyText`) never written to log lines.
- [x] Cites `16-app-design-system-and-ui.md`.

---

**End of `03-rules/02-frontend.md`**
