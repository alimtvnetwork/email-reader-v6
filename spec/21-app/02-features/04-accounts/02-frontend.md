# 04 — Accounts — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, view-model, theming, interaction, and lifecycle** for the Accounts view. Lives in `internal/ui/views/accounts.go` — the only file in `internal/ui` permitted to compose Accounts widgets. Also defines the **sidebar account-picker** binding (rendered by `internal/ui/sidebar.go` but fed exclusively by `AccountsVM.PickerSnapshot`).

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.4
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/accounts.go
package views

type AccountsVM struct {
    svc       *core.Accounts
    nav       NavRouter

    accounts   binding.UntypedList   // []core.AccountView (sorted by Order)
    selected   binding.Untyped       // *core.AccountView (form right pane)
    formSpec   binding.Untyped       // *core.AccountSpec (edit buffer; Password held in-memory only)
    formMode   binding.String        // "Closed" | "Add" | "Edit"
    fieldErrs  binding.Untyped       // map[string]string (field → error msg)
    isDirty    binding.Bool
    isLoading  binding.Bool
    isTesting  binding.Bool          // true while TestConnection in flight
    testResult binding.Untyped       // *core.TestConnectionResult (last attempt)
    suggestRes binding.Untyped       // *core.ImapDefaults (last SuggestImap)
    loadErr    binding.String
    sub        <-chan core.AccountEvent
    cancelSub  func()
}

func NewAccountsVM(svc *core.Accounts, nav NavRouter) *AccountsVM

func (vm *AccountsVM) Refresh(ctx context.Context)
func (vm *AccountsVM) SelectAccount(alias string)
func (vm *AccountsVM) NewAccount()                                     // resets form to defaults, formMode="Add"
func (vm *AccountsVM) EditSelected()                                   // copies selected into formSpec, formMode="Edit"
func (vm *AccountsVM) DiscardForm()                                    // formMode="Closed", clears formSpec + fieldErrs
func (vm *AccountsVM) SuggestImapForCurrent(ctx context.Context)       // debounced 300 ms after EmailAddr edit
func (vm *AccountsVM) TestConnectionForCurrent(ctx context.Context)    // explicit "Test" button
func (vm *AccountsVM) SaveForm(ctx context.Context)                    // Add or Update (re-runs TestConnection internally)
func (vm *AccountsVM) RemoveSelected(ctx context.Context)              // after RemoveConfirmDialog
func (vm *AccountsVM) RenameSelected(ctx context.Context, newAlias string)
func (vm *AccountsVM) Reorder(ctx context.Context, aliasesInOrder []string)
func (vm *AccountsVM) PickerSnapshot() []core.AccountView              // sidebar consumer (lock-free read)
func (vm *AccountsVM) AttachLive(ctx context.Context)
func (vm *AccountsVM) DetachLive()
```

**Rules:**
- VM holds **no Fyne widgets** — only `binding.*` and a small navigation router interface.
- All long calls (`svc.*`) happen off the UI goroutine via `go func()` + `binding.*.Set` on completion. The UI goroutine is never blocked on network I/O.
- VM is the **only** consumer of `core.Accounts` from `internal/ui` (sidebar reads via `PickerSnapshot`, never directly).
- The **plaintext password** lives only in `formSpec.Password` and is wiped (`= ""`) immediately after a successful `SaveForm` or any `DiscardForm`. Never bound to a `binding.String` (would risk leaking via reflection / debug dumps).
- Form validation runs **client-side** for empty/format checks before calling `SaveForm`; the server is the source of truth and re-validates.

---

## 2. Widget Tree

```
AccountsView (container.NewBorder)
├── Top:    Toolbar (container.NewHBox)
│           ├── widget.NewButtonWithIcon("Add account", theme.ContentAddIcon(), vm.OnNewAccount)
│           ├── widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), vm.OnRefresh)
│           ├── layout.NewSpacer()
│           └── widget.NewLabel("{Count} accounts · {ConnectedCount} connected")
├── Center: container.NewHSplit(0.50)
│           ├── Left:  AccountListPane    (container.NewBorder)
│           │         ├── Top:    SearchEntry "Filter by alias / email…" (debounced 200 ms)
│           │         └── Center: widget.List with drag handles  // AccountRow template
│           └── Right: AccountFormPane    (container.NewMax)
│                     ├── EmptyState (visible when formMode == "Closed" && selected == nil)
│                     │   └── widget.NewLabel("Select an account or click + Add account")
│                     ├── DetailPane     (visible when formMode == "Closed" && selected != nil)
│                     │   ├── DetailHeader (Alias + Connected badge + Edit + Rename + Remove)
│                     │   └── DetailFields (read-only labels: EmailAddr, Host, Port, UseTls, LastSeenUid, LastConnectedAt, LastConnectError)
│                     └── FormPane       (visible when formMode != "Closed")
│                         ├── Top:    FormHeader ({Add account | Edit alias} + Discard X)
│                         ├── Center: container.NewVScroll(FormFields)
│                         │           ├── widget.NewEntry "Alias"        (with FieldErrLabel)         — disabled in Edit mode
│                         │           ├── widget.NewEntry "EmailAddr"    (with FieldErrLabel)         — triggers SuggestImap on debounced edit
│                         │           ├── PasswordEntry "Password"       (with FieldErrLabel)         — empty in Edit mode = "keep existing"
│                         │           ├── ImapDefaultsBadge              (shows Source: Builtin/Unknown + reveal-manual link)
│                         │           ├── widget.NewEntry "Host"         (with FieldErrLabel)         — collapsed when Source==Builtin unless user expands
│                         │           ├── widget.NewEntry "Port"         (numeric, with FieldErrLabel)
│                         │           └── widget.NewCheck("Use TLS", …)
│                         └── Bottom: FormActionBar
│                                     ├── widget.NewButton("Test connection", vm.OnTestConnection)   — disabled while isTesting
│                                     ├── TestResultBadge                                            — green ✓ / red ✗ + latency
│                                     ├── layout.NewSpacer()
│                                     ├── widget.NewButton("Cancel", vm.OnDiscard)                   — disabled when !isDirty
│                                     └── widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), vm.OnSave)
└── Bottom: StatusBar (container.NewHBox)
            ├── widget.NewLabel("Updated {RelativeTime}")
            ├── layout.NewSpacer()
            └── widget.NewProgressBarInfinite()  // visible only while isLoading || isTesting
```

### 2.1 `AccountRow` (custom widget — list item template)

```
[≡] [●] {Alias}                    {EmailAddr}            UID {LastSeenUid}    {RelativeTime LastConnectedAt}
        {Host}:{Port} {TLS|STARTTLS}                                           {LastConnectError? in --Status-Danger}
```

- `[≡]` = drag handle (Fyne `desktop.Draggable`); drop-reorder triggers `vm.Reorder`.
- `[●]` = connected dot; **read-only** indicator (NOT a toggle — accounts are not "disabled", they are removed).
  - `--Status-Healthy` when `IsConnected == true`.
  - `--Status-Danger` when `LastConnectError != ""`.
  - `--Border-Subtle` when never connected (fresh add, watcher hasn't polled yet).
- Row click selects + populates `DetailPane` (read-only) on the right. Form does NOT auto-open on click — user must press **Edit**.
- Long-press / right-click (desktop) → context menu: **Edit**, **Rename**, **Remove**, **Copy email address**.
- Currently-selected sidebar account: `--Surface-Selected` background highlight on the matching row.

### 2.2 `PasswordEntry` (custom widget)

A `widget.Entry` with `Password = true` (mask). Additions:
- Eye toggle on the right edge (Fyne `widget.NewButtonWithIcon(theme.VisibilityIcon(), …)`) that flips `Password = false` for **3 s** then auto-re-masks. Requires explicit click each time (no "remember unmasked" preference).
- `placeholder` differs by mode: `"Enter password"` in Add, `"Leave blank to keep existing"` in Edit.
- The widget's text value is read directly into `formSpec.Password` on Save and **immediately zeroed** (`entry.SetText("")`) after the call returns, regardless of success.
- No clipboard auto-clear (out of scope for v1). Right-click → Cut/Copy is left enabled (Fyne default) — explicit user action.

### 2.3 `ImapDefaultsBadge` (custom widget)

Renders next to the `EmailAddr` field after `SuggestImap` returns. States:

| `Source`           | Badge                                                                  |
|--------------------|------------------------------------------------------------------------|
| `Builtin`          | `widget.NewLabel("Auto-detected: {Host}:{Port} {TLS}")` in `--Status-Info` color, with `widget.NewHyperlink("change", … reveals manual fields)` |
| `Unknown`          | `widget.NewLabel("We don't know this provider — fill in Host/Port manually")` in `--Status-Warning`; manual fields auto-revealed |
| (no result yet)    | hidden                                                                 |

### 2.4 `FieldErrLabel` (custom widget)

A red `widget.Label` that shows `fieldErrs[FieldName]` directly under the entry. Hidden when no error. Color: `--Status-Danger`. Updates reactively from the `fieldErrs` binding. Same component used by Rules / Settings views — keep in `internal/ui/widgets/fielderr.go`.

### 2.5 `TestResultBadge` (custom widget)

Renders the last `testResult`:

| State                                     | Rendering                                                |
|-------------------------------------------|----------------------------------------------------------|
| `nil` (never tested)                      | hidden                                                   |
| `Ok == true`                              | `✓ Connected · {LatencyMs} ms` in `--Status-Healthy`     |
| `Ok == false, ErrorCode == ER-MAIL-21201` | `✗ Wrong email or password` in `--Status-Danger`         |
| `Ok == false, ErrorCode == ER-MAIL-21200` | `✗ Cannot reach {Host}:{Port}` in `--Status-Danger`     |
| `Ok == false, ErrorCode == ER-MAIL-21207` | `✗ TLS handshake failed` in `--Status-Danger`           |
| `Ok == false, ErrorCode == ER-MAIL-21208` | `✗ Timeout after 5 s` in `--Status-Danger`              |
| `Ok == false, other`                      | `✗ {ErrorMsg}` truncated to 80 chars in `--Status-Danger` |

The badge is cleared whenever any field in the form is edited (stale-result guard).

### 2.6 `RemoveConfirmDialog` (modal)

```
┌─ Remove account "{Alias}"? ─────────────────────────┐
│                                                       │
│  This will:                                           │
│   • Remove credentials for {EmailAddr}.               │
│   • Delete the watcher cursor (LastSeenUid={N}).     │
│   • Keep your existing {EmailCount} stored emails    │
│     (they will appear under "Unknown account").      │
│                                                       │
│  This cannot be undone.                               │
│                                                       │
│           [Cancel]  [Remove]   ← danger color        │
└───────────────────────────────────────────────────────┘
```

The `EmailCount` is fetched on dialog-open via a single `SELECT COUNT(*) FROM Email WHERE Alias = ?` (cheap, indexed). The dialog blocks until user choice.

### 2.7 `RenameDialog` (modal)

A simple `dialog.NewForm` with one field "New alias" + the same alias-validation rules as the form (live `FieldErrLabel`). On confirm calls `vm.RenameSelected`. On error keeps the dialog open with the field-level error highlighted.

### 2.8 `DiscardChangesDialog` (modal)

Triggered when user navigates away (sidebar nav, view switch, app close) while `isDirty == true`. Three buttons: **Save & continue**, **Discard**, **Cancel** (stay on view). Same shape as Rules' `DiscardChangesDialog` — keep the dialog component shared in `internal/ui/widgets/dirtyguard.go`.

---

## 3. Theming

All colors via tokens from `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`. **No hard-coded colors.**

| Element                               | Token                                  |
|---------------------------------------|----------------------------------------|
| Pane background                       | `--Surface-Default`                    |
| Selected row                          | `--Surface-Selected`                   |
| Drag handle hover                     | `--Surface-Hover`                      |
| Connected dot                         | `--Status-Healthy`                     |
| Disconnected dot (error)              | `--Status-Danger`                      |
| Never-connected dot                   | `--Border-Subtle`                      |
| Field error text                      | `--Status-Danger`                      |
| Auto-detect badge background          | `--Status-Info-Subtle`                 |
| Unknown-provider badge background     | `--Status-Warning-Subtle`              |
| TestResult success                    | `--Status-Healthy`                     |
| TestResult failure                    | `--Status-Danger`                      |
| Remove button (in confirm dialog)     | `--Action-Danger`                      |
| Form pane border                      | `--Border-Default`                     |
| Read-only field text                  | `--Text-Secondary`                     |

Type scale: `Heading-3` for `FormHeader`/`DetailHeader`, `Body-Default` for fields, `Caption` for `LastConnectError` snippet and `RelativeTime`.

Padding: outer pane `--Spacing-4`, between fields `--Spacing-3`, intra-field label-to-input `--Spacing-2`. `HSplit` divider uses Fyne default.

---

## 4. Interactions & Events

### 4.1 Add flow

1. User clicks **+ Add account** → `vm.NewAccount()`: `formMode = "Add"`, `formSpec = &AccountSpec{UseTls: true, Order: 0}`, focus on `Alias` field.
2. User types `Alias`, then `EmailAddr`.
3. **Debounced 300 ms** after the user stops typing in `EmailAddr`, `vm.SuggestImapForCurrent` runs:
   - On `Source == Builtin`: `formSpec.Host/Port/UseTls` populated; `ImapDefaultsBadge` shows green; manual fields stay collapsed.
   - On `Source == Unknown`: badge shows yellow; manual fields auto-expand.
   - User-entered manual values are **never** overwritten by SuggestImap (guarded by `formSpec._hostUserEdited` boolean).
4. User types `Password`. **No** debounced TestConnection — only explicit button or Save triggers network I/O.
5. User clicks **Test connection** (optional pre-check) → `vm.TestConnectionForCurrent`: `isTesting = true`, button disabled, `TestResultBadge` updates on completion. No persistence.
6. User clicks **Save** → `vm.SaveForm` → `core.Accounts.Add` (which re-runs TestConnection server-side):
   - Success: `formMode = "Closed"`, password wiped, list refreshed via `AccountEvent{Added}`, toast `"Account {Alias} added"`.
   - Failure with field-level error code: populate `fieldErrs[Field]`, focus offending field, keep form open.
   - Failure with `21720 AccountAddTestConnectionFailed`: populate `TestResultBadge`, toast `"Could not connect — see error below"`.

### 4.2 Edit flow

1. User selects a row → `DetailPane` shows read-only.
2. User clicks **Edit** → `vm.EditSelected`: `formMode = "Edit"`, `formSpec` deep-copied from selected (Password left blank).
3. **Alias field is disabled** in Edit mode (use Rename for that). The disabled label shows `"Use Rename to change the alias"` as tooltip.
4. User edits any field. The dirty-tracker compares `formSpec` to the original snapshot (excluding Password — non-empty Password always counts as dirty).
5. **Save** behavior:
   - If only `Order` changed → no TestConnection (matches backend `core.Accounts.Update` rule §2.6).
   - If `Host`/`Port`/`UseTls` changed OR `Password != ""` → re-runs TestConnection server-side.
   - If `Password == ""` → backend preserves existing password.

### 4.3 Rename flow

1. User clicks **Rename** in DetailHeader → `RenameDialog`.
2. On confirm → `vm.RenameSelected` → `core.Accounts.Rename`. Sidebar picker updates via `AccountEvent{Renamed}` within 1 s.

### 4.4 Remove flow

1. User clicks **Remove** in DetailHeader → `RemoveConfirmDialog` (with `EmailCount` lookup).
2. On confirm → `vm.RemoveSelected` → `core.Accounts.Remove`.
3. On success: row disappears (`AccountEvent{Removed}` triggers list re-bind), `selected = nil`, `DetailPane → EmptyState`, toast `"Account {Alias} removed"`.

### 4.5 Reorder flow

1. User drags `[≡]` handle on a row → Fyne emits `desktop.DragEvent`.
2. **Optimistic reorder**: VM reorders `accounts` binding immediately for visual feedback.
3. On drop, **debounced 300 ms**, `vm.Reorder` calls `core.Accounts.SetOrder`.
4. On error: rollback to pre-drag order, toast `"Reorder failed — {ErrorMessage}"`.

### 4.6 Sidebar account-picker binding

`internal/ui/sidebar.go` does NOT call `core.Accounts.List`. Instead:
1. At app boot, `mainWindow.go` calls `accountsVM.AttachLive(ctx)` once (process-lifetime).
2. Sidebar reads `accountsVM.PickerSnapshot()` whenever it rebuilds — a `O(N)` slice copy guarded by an `RWMutex`.
3. Sidebar subscribes to a separate `binding.Untyped` (`vm.accounts`) for change notifications; on every change, sidebar rebuilds its list of `widget.RadioGroup` options.
4. Selecting an account in the sidebar emits `nav.SetActiveAccount(alias)` — this is consumed by Emails / Watch / Dashboard views, NOT by AccountsView (the AccountsView highlights the selected row but does not auto-focus).

---

## 5. Lifecycle

| Hook        | Action                                                                            |
|-------------|-----------------------------------------------------------------------------------|
| `OnShow`    | `vm.Refresh` (List); `vm.AttachLive` (Subscribe to `AccountEvent`).               |
| `OnHide`    | If `isDirty`, show `DiscardChangesDialog`; on Discard/Save proceed; on Cancel veto the hide. Then `vm.DetachLive`. |
| `OnAppQuit` | Same dirty guard as `OnHide`. If user picks Discard, password buffers wiped before exit. |

`AttachLive` deliberately stays alive for the **entire app session**, not only when the view is visible — the sidebar picker depends on it. `DetachLive` is called only on app shutdown.

---

## 6. Live-Update Reactions

| Event source                                     | VM reaction                                                              |
|--------------------------------------------------|--------------------------------------------------------------------------|
| `AccountEvent{Added, Alias}`                     | Append `AccountView` to `accounts`; if user is on AccountsView, scroll to it and show toast. |
| `AccountEvent{Removed, Alias}`                   | Remove from `accounts`; if `selected.Alias == Alias`, clear selection.   |
| `AccountEvent{Renamed, PrevAlias, Alias}`        | In-place rename in `accounts`; preserve scroll position.                 |
| `AccountEvent{Updated, Alias}`                   | In-place merge new fields; if formMode=="Edit" on this row, prompt user via `DiscardChangesDialog` ("This account was edited elsewhere"). |
| `AccountEvent{Reordered}`                        | Re-fetch `accounts` (single `core.Accounts.List` call).                  |
| `WatchEvent{AccountConnected, Alias}` (from `core.Watch`) | Flip dot + clear `LastConnectError` for the matching row.       |
| `WatchEvent{AccountConnectError, Alias, Err}`    | Flip dot to red + populate `LastConnectError` for the matching row.      |

Overflow handling: `AccountEvent` channel cap is 64; if `WARN AccountEventOverflow` fires (backend §8), VM responds by issuing a **full `Refresh()`** to recover canonical state. Documented because UI must not silently desync from `config.json`.

---

## 7. Performance Budgets

| Operation                                             | Budget        | Measured by                                                 |
|-------------------------------------------------------|---------------|-------------------------------------------------------------|
| Initial render of 50 accounts                         | ≤ **40 ms**   | `AccountsView.Show` to first frame (manual stopwatch in test) |
| Single row optimistic reorder                         | ≤ **16 ms**   | One frame at 60 Hz                                          |
| Live event → row dot color change                     | ≤ **40 ms**   | From `bus.Publish` to `Canvas.Refresh`                      |
| `SuggestImap` debounce                                | **300 ms**    | After last keystroke in `EmailAddr`                         |
| `TestConnection` UI freeze                            | **0 ms**      | Must run in goroutine; UI never blocks                      |
| Sidebar `PickerSnapshot` read                         | ≤ **1 ms**    | RWMutex-guarded slice copy                                  |

---

## 8. Accessibility

- Every interactive widget has a `Hint` (Fyne tooltip) — at minimum: Alias entry, Password entry (incl. eye toggle), Test button, Save button, drag handle, dots.
- Tab order: Alias → EmailAddr → Password → (eye-toggle) → ImapDefaults change-link → Host → Port → UseTls → Test → Cancel → Save.
- The connected dot has an off-screen `widget.Label` ("Connected" / "Disconnected — {Error}" / "Not yet polled") for screen-reader semantics.
- High-contrast mode (Fyne `theme.HighContrast`) renders without losing the connected/disconnected distinction (verified in QA — uses both color AND a small icon `✓`/`✗`/`—`).
- Keyboard: `Cmd/Ctrl+N` triggers **+ Add account** when AccountsView is focused; `Cmd/Ctrl+S` saves the form; `Esc` discards (with dirty-guard).

---

## 9. Testing Contract

File: `internal/ui/views/accounts_test.go`. UI tests use `fyne.io/fyne/v2/test` + a fake `*core.Accounts`.

Required test cases:

1. `Render_Empty_ShowsEmptyState`.
2. `Render_FiftyAccounts_FirstFrameUnder40ms`.
3. `NewAccount_OpensFormWithDefaults_FocusOnAlias`.
4. `EmailAddrEdit_DebouncedSuggestImap_PopulatesHost` — fake suggester returns `Builtin` for `gmail.com`.
5. `EmailAddrEdit_UnknownDomain_ExpandsManualFields`.
6. `UserEditedHost_NotOverwrittenBySuggest`.
7. `TestConnection_ButtonClick_RunsOffUiGoroutine_BadgeUpdates` — asserts UI thread not blocked (test framework checks `runtime.NumGoroutine` delta).
8. `Save_TestConnectionFails_KeepsFormOpen_PopulatesBadge`.
9. `Save_Success_WipesPasswordEntry_ClosesForm` — asserts `passwordEntry.Text == ""` after save.
10. `Edit_AliasFieldDisabled`.
11. `Edit_BlankPassword_NotMarkedDirtyUntilOtherFieldChanges`.
12. `Edit_OnlyOrderChanged_NoTestConnectionCalled` — fake dialer asserts zero `Login` calls.
13. `Reorder_Optimistic_RollsBackOnError`.
14. `RemoveConfirm_ShowsEmailCount_FromCountQuery`.
15. `Rename_DialogValidatesAliasFormat`.
16. `LiveEvent_Added_AppendsRowAndShowsToast`.
17. `LiveEvent_Removed_ClearsSelectionIfMatched`.
18. `LiveEvent_Renamed_PreservesScrollPosition`.
19. `WatchEvent_ConnectError_FlipsDotRed_PopulatesLabel`.
20. `EventChannelOverflow_TriggersFullRefresh`.
21. `OnHide_Dirty_ShowsDiscardDialog_VetoesNavOnCancel`.
22. `OnAppQuit_DiscardWipesPasswordBuffers` — asserts password byte-slice zeroed (memory inspection via `unsafe.Slice` test helper).
23. `Sidebar_PickerSnapshot_LockFreeRead_NoBlockingDuringLiveUpdate` — concurrent reader stress test.
24. `HighContrastTheme_DotStillDistinguishable` — golden-image test (light + dark + HC).
25. `KeyboardShortcuts_CmdN_OpensAddForm`, `CmdS_SavesForm`, `Esc_DiscardsWithGuard`.

Fakes:
- `core.FakeAccounts` (replays scripted `Result[T]` per method).
- `mailclient.FakeDialer` (scripted Login outcome).
- `imapdef.FakeSuggester` (table-driven).
- `eventbus.NewMemory()` for `AccountEvent` injection.
- `test.NewWindow` from `fyne.io/fyne/v2/test`.

---

## 10. Compliance Checklist

- [x] VM holds zero Fyne widgets — only `binding.*` and a `NavRouter` interface.
- [x] All `core.Accounts` calls run off the UI goroutine.
- [x] No hard-coded colors — all via tokens from `16-app-design-system-and-ui.md`.
- [x] Plaintext password never bound to a `binding.String`; wiped after Save/Discard.
- [x] `PasswordEntry.Text` zeroed (`SetText("")`) after every Save attempt, regardless of outcome.
- [x] No `interface{}` / `any` in any new code (lint-enforced).
- [x] Sidebar reads via lock-free `PickerSnapshot()`; never imports `core.Accounts` directly.
- [x] All long operations show `isLoading` or `isTesting` — UI thread never blocked.
- [x] Dirty-guard on `OnHide` / `OnAppQuit` shared with Rules / Settings via `internal/ui/widgets/dirtyguard.go`.
- [x] Live event channel overflow triggers full `Refresh` — UI never silently desyncs.
- [x] Accessibility: every interactive widget has a `Hint`; HC theme verified by golden-image test.
- [x] Cites 02-coding, 16-app-design-system-and-ui, 13-app.

---

**End of `04-accounts/02-frontend.md`**
