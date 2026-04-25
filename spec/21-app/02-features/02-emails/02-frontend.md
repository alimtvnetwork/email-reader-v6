# 02 — Emails — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, view-model, theming, interaction, and lifecycle** for the Emails view. Lives in `internal/ui/views/emails.go` — the only file in `internal/ui` permitted to compose Emails widgets.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.2
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/emails.go
package views

type EmailsVM struct {
    svc       *core.Emails
    watch     *core.Watch
    nav       NavRouter

    query     binding.Untyped         // *core.EmailQuery
    page      binding.Untyped         // *core.EmailPage
    selected  binding.Untyped         // *core.EmailDetail (nil = none)
    counts    binding.Untyped         // *core.EmailCounts
    selection binding.UntypedList     // []uint32 (UIDs checked in list)
    isLoading binding.Bool
    loadErr   binding.String
    newBadge  binding.Int             // arrived-since-last-list count

    undoStack []core.DeleteReceipt    // cap 1
    sub       <-chan core.WatchEvent
    cancelSub func()
}

func NewEmailsVM(svc *core.Emails, watch *core.Watch, nav NavRouter) *EmailsVM

func (vm *EmailsVM) Refresh(ctx context.Context)            // re-runs List
func (vm *EmailsVM) PollNow(ctx context.Context)            // delegates to Refresh service method
func (vm *EmailsVM) OpenEmail(ctx context.Context, alias string, uid uint32)
func (vm *EmailsVM) MarkSelectedRead(ctx context.Context, read bool)
func (vm *EmailsVM) DeleteSelected(ctx context.Context)
func (vm *EmailsVM) UndoLastDelete(ctx context.Context)
func (vm *EmailsVM) ApplyFilter(q core.EmailQuery)          // debounced 300 ms
func (vm *EmailsVM) AttachLive(ctx context.Context)
func (vm *EmailsVM) DetachLive()
```

**Rules:**
- VM holds **no Fyne widgets** — only `binding.*` and a small navigation router interface.
- All long calls happen off the UI goroutine; results pushed via `binding.*.Set`.
- VM is the **only** consumer of `core.Emails` from `internal/ui`.
- `query` is persisted to `Settings.LastEmailQuery` on every change (debounced 1 s).

---

## 2. Widget Tree

```
EmailsView (container.NewBorder)
├── Top:    Toolbar (container.NewVBox)
│           ├── FilterBar (container.NewHBox)
│           │   ├── widget.NewSelect(aliases, vm.OnAliasChange)
│           │   ├── widget.NewEntry()  // Search; placeholder "Search subject, from, body"
│           │   ├── widget.NewCheck("Unread only", vm.OnUnreadToggle)
│           │   ├── widget.NewCheck("Include deleted", vm.OnIncludeDeletedToggle)
│           │   └── layout.NewSpacer()
│           └── ActionBar (container.NewHBox)
│               ├── widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), vm.OnPollNow)
│               ├── NewBadge "{newBadge} new" (hidden when 0)
│               ├── widget.NewButton("Mark read",   vm.OnMarkReadSelected)
│               ├── widget.NewButton("Mark unread", vm.OnMarkUnreadSelected)
│               ├── widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), vm.OnDeleteSelected)
│               ├── widget.NewButton("Undo", vm.OnUndoDelete)   // disabled when undoStack empty
│               ├── layout.NewSpacer()
│               └── CountsLabel "Total {N} · Unread {U} · Deleted {D}"
├── Center: container.NewHSplit(0.40)
│           ├── Left:  EmailListPane    (container.NewBorder)
│           │         ├── Top:    SelectAllCheck (widget.Check)
│           │         ├── Center: widget.List         // EmailRow template
│           │         └── Bottom: PaginationBar
│           │                     ├── widget.NewButton("Prev", vm.OnPrevPage)
│           │                     ├── widget.NewLabel("{Offset+1}–{Offset+Len} of {TotalCount}")
│           │                     └── widget.NewButton("Next", vm.OnNextPage)
│           └── Right: EmailDetailPane  (container.NewBorder)
│                     ├── Top:    HeaderTable (Subject / From / To / Cc / Received)
│                     ├── Center: BodyTabs (container.NewAppTabs)
│                     │           ├── "Text"  → widget.RichText (BodyText)
│                     │           └── "HTML"  → widget.RichTextFromMarkdown(SafeHtml(BodyHtml))
│                     └── Bottom: DetailActionBar
│                                 ├── widget.NewButtonWithIcon("Open .eml", theme.FileIcon(), vm.OnOpenEml)
│                                 ├── widget.NewButton("Open URLs…", vm.OnOpenUrls)
│                                 ├── layout.NewSpacer()
│                                 └── MatchedRulesLabel "Rules: {names}"
└── Bottom: StatusBar (container.NewHBox)
            ├── widget.NewLabel("Updated {RelativeTime}")
            ├── layout.NewSpacer()
            └── widget.NewProgressBarInfinite()  // visible only while isLoading
```

### 2.1 `EmailRow` (custom widget — list item template)

```
[☐] [●] {FromAddr (32ch)}    {Subject (truncated)}      {RelativeTime (right)}
        {Snippet (90ch, --Text-Secondary)}              [{MatchedRules badges}]
```

- `[☐]` = selection checkbox (binds to `selection`).
- `[●]` = unread dot (visible iff `IsRead == false`); color `--Status-Info`.
- Soft-deleted rows render with 60 % opacity and a strike-through subject; only visible when `IncludeDeleted` filter is on.
- Row click (not on checkbox) → `vm.OpenEmail(alias, uid)` → loads detail pane.

### 2.2 `NewBadge` (custom widget)

Pill with `--Surface-Accent` background, `--Text-OnAccent` text. Click → `vm.Refresh(ctx)` (clears badge, re-runs List).

---

## 3. States

| State        | Trigger                              | Visible UI                                              |
|--------------|--------------------------------------|---------------------------------------------------------|
| `Loading`    | First mount / Refresh in flight      | Translucent scrim over list pane (detail unaffected)    |
| `Loaded`     | `List` returned successfully         | Full widget tree as §2                                  |
| `Error`      | `List` returned error                | `ErrorPanel{ErrorCode, Message, RetryButton}` in list   |
| `Empty`      | Loaded, `len(Rows) == 0`             | `EmptyState{Icon, Title="No emails", Body, CTA="Refresh"}` |
| `DetailLoading` | `Get` in flight                   | Spinner overlay on detail pane only                     |
| `DetailEmpty`| No row selected                      | `EmptyState{Title="Select an email"}` in detail pane    |

`Loading` does **not** clear list rows — preserves spatial context.

---

## 4. Theming Tokens

Defined in `internal/ui/theme/theme.go`, sourced from `16-app-design-system-and-ui.md`.

| Token                  | Use                                          |
|------------------------|----------------------------------------------|
| `--Surface-Card`       | List + detail pane background                |
| `--Surface-Selected`   | Selected row background                      |
| `--Surface-Accent`     | NewBadge background                          |
| `--Border-Subtle`      | Pane borders                                 |
| `--Status-Info`        | Unread dot                                   |
| `--Status-Danger`      | Delete button hover, danger confirmations    |
| `--Text-Primary`       | Subject, From                                |
| `--Text-Secondary`     | Snippet, RelativeTime, headers labels        |
| `--Text-OnAccent`      | NewBadge text                                |
| `--Spacing-Row`        | List row vertical padding                    |
| `--Radius-Card`        | Pane border-radius                           |

**Hard rule:** No hex colors in `views/emails.go`. Lint rule `no-hex-in-views` enforces this.

---

## 5. Lifecycle Hooks

```go
func (v *EmailsView) OnShow()  // RestoreLastQuery → vm.Refresh + vm.AttachLive
func (v *EmailsView) OnHide()  // vm.DetachLive + persist current query
```

Wired in `internal/ui/nav.go`. Failure to detach is a leak — the watcher's event bus logs `WARN WatchSubscriberLeak` if `OnHide` is skipped.

`OnShow` consumes optional `NavParams{"Alias": "..."}` (e.g., from a Dashboard click) and pre-applies the alias filter.

---

## 6. Interaction Rules

| User action                                | Effect                                                                      |
|--------------------------------------------|-----------------------------------------------------------------------------|
| Type in Search                             | `ApplyFilter` debounced 300 ms; resets `Offset` to 0                        |
| Change Alias select                        | `ApplyFilter` immediately; resets `Offset` to 0; clears selection           |
| Toggle "Unread only"                       | `ApplyFilter` immediately                                                   |
| Toggle "Include deleted"                   | `ApplyFilter` immediately                                                   |
| Click row (not checkbox)                   | `OpenEmail` → loads detail pane                                             |
| Click row checkbox                         | Toggles UID in `selection`                                                  |
| Click `SelectAllCheck`                     | Selects/clears all currently visible UIDs only                              |
| Click **Mark read** / **Mark unread**      | Optimistic flip; rollback on error toast                                    |
| Click **Delete**                           | Confirmation dialog ("Delete N emails locally?"), then optimistic remove    |
| Click **Undo**                             | `Undelete(undoStack[0].AffectedIds)`; clears stack                          |
| Click **Refresh**                          | `vm.PollNow` (calls `Emails.Refresh` → `Watch.PollOnce`)                    |
| Click **Open .eml**                        | `core.Tools.OpenUrl("file://" + FilePath)`                                  |
| Click **Open URLs…**                       | Dialog listing URLs from BodyText/BodyHtml; click → `Tools.OpenUrl`         |
| Press `F5`                                 | `vm.PollNow`                                                                |
| Press `Ctrl+F`                             | Focus the Search entry                                                      |
| Press `Delete` with selection              | Same as Delete button                                                       |
| Press `R` with detail open                 | Toggle Read state of the open email                                         |
| Press `Esc`                                | Closes any open dialog / clears Search if focused                           |
| `WatchEvent.Kind == EmailStored` for active alias | Increment `newBadge`; do not auto-refetch                            |

No drag-and-drop. No double-click bindings. No context menus in v1.

---

## 7. Confirmation & Toasts

| Trigger                  | Confirmation                                  | Success toast                  | Failure toast                                       |
|--------------------------|-----------------------------------------------|--------------------------------|-----------------------------------------------------|
| Delete N emails          | Modal: "Delete {N} emails locally?" / Cancel  | "Deleted {N}. Undo"            | "Delete failed. Code E{ErrorCode}. Retry?"          |
| Mark read N emails       | (no modal)                                    | "Marked {N} as read"           | "Update failed. Code E{ErrorCode}."                 |
| Refresh failure          | (no modal)                                    | —                              | `ErrorPanel` in list (preserves rows)               |

The "Undo" toast is dismissed automatically after 5 s; clicking it before then triggers `UndoLastDelete`.

---

## 8. Performance Budgets

| Metric                                        | Budget    | Measured by                                    |
|-----------------------------------------------|-----------|------------------------------------------------|
| Cold mount → first paint                      | < 120 ms  | `views/emails_bench_test.go`                   |
| `ApplyFilter` round-trip (warm DB, 100k rows) | < 80 ms   | same                                           |
| `OpenEmail` round-trip                        | < 30 ms   | same                                           |
| Bulk mark-read of 500 (UI feedback)           | < 200 ms  | optimistic flip < 16 ms; SQL ≤ 150 ms          |
| Live `WatchEvent` badge increment             | < 16 ms   | same                                           |
| Memory footprint (1 page of 50 rows)          | < 4 MB    | `go test -memprofile`                          |

---

## 9. Accessibility

- `EmailRow` exposes role `"button"`, `aria-label = "Email from {FromAddr}, subject {Subject}, {RelativeTime}, {IsRead ? 'read' : 'unread'}"`.
- Selection checkboxes expose role `"checkbox"`, `aria-checked` reflects state.
- Toolbar buttons expose `aria-label` matching their text.
- Focus order: Alias select → Search → Unread toggle → Include deleted toggle → Refresh → list rows → pagination → detail body.
- Screen-reader announcement on `Loaded`: `"Showing {Len} of {TotalCount} emails for {AliasOrAll}."`.
- Screen-reader announcement on bulk action: `"Marked {N} as read."` / `"Deleted {N}. Undo available."`.

---

## 10. Testing Contract

File: `internal/ui/views/emails_test.go`. Smoke tests only (per `04-coding-standards.md` §7).

Required test cases:

1. `Mount_RendersToolbarAndPanes`.
2. `ApplyFilter_DebouncedAt300ms`.
3. `OpenEmail_PopulatesDetailPane`.
4. `BulkSelect_ThenMarkRead_OptimisticFlip`.
5. `Delete_ShowsConfirmation_ThenRemovesRows`.
6. `Undo_RestoresDeletedRows`.
7. `Refresh_DelegatesToVmPollNow`.
8. `WatchEvent_EmailStored_IncrementsBadge_NoAutoFetch`.
9. `OnHide_DetachesLive_PersistsQuery`.
10. `Error_ShowsErrorPanel_PreservesPreviousRows`.

Fakes:
- `core.FakeEmails` implementing the same method set with controllable return values.
- `core.FakeWatch` with a controllable event channel.
- `MockNavRouter` recording calls.

---

## 11. Compliance Checklist

- [x] Single Fyne-importing file (`internal/ui/views/emails.go`).
- [x] VM holds no widgets, only `binding.*` + interfaces.
- [x] No hex colors; all colors via theme tokens.
- [x] PascalCase widget/struct/binding names.
- [x] Subscription lifecycle paired (`OnShow`/`OnHide`).
- [x] Optimistic UI for bulk ops with rollback on error.
- [x] Live update increments badge; never auto-refetches.
- [x] Performance budgets stated and benchmark-enforced.
- [x] Accessibility roles + announcements defined.
- [x] PII (BodyText, full address) never written to log lines.
- [x] Cites `16-app-design-system-and-ui.md`.

---

**End of `02-emails/02-frontend.md`**
