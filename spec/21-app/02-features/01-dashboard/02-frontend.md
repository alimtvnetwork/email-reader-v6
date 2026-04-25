# 01 — Dashboard — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, layout, theming, interaction, and view-model** for the Dashboard view. Lives in `internal/ui/views/dashboard.go` (only file in `internal/ui` allowed to compose Dashboard widgets).

This is the **only** spec layer permitted to import `fyne.io/fyne/v2/*` for the Dashboard feature.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md)
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/dashboard.go
package views

type DashboardVM struct {
    svc       *core.Dashboard
    watch     *core.Watch
    summary   binding.Untyped         // *core.DashboardSummary
    activity  binding.UntypedList     // []core.ActivityRow (cap 20)
    health    binding.UntypedList     // []core.AccountHealthRow
    loadErr   binding.String
    isLoading binding.Bool
    sub       <-chan core.WatchEvent
    cancelSub func()
}

func NewDashboardVM(svc *core.Dashboard, watch *core.Watch) *DashboardVM
func (vm *DashboardVM) Refresh(ctx context.Context)        // debounced 500 ms
func (vm *DashboardVM) AttachLive(ctx context.Context)     // subscribes to WatchEvent
func (vm *DashboardVM) DetachLive()                        // unsubscribes, releases ch
```

**Rules:**
- VM holds **no Fyne widgets** — only Fyne `binding.*` data primitives.
- All long calls happen off the UI goroutine; results are pushed via `binding.*.Set` (Fyne marshals to UI thread).
- VM is the **only** caller of `core.Dashboard` from `internal/ui`.

---

## 2. Widget Tree

```
DashboardView (container.NewBorder)
├── Top:    SummaryHeader              (container.NewGridWithColumns(4))
│           ├── StatCard "Accounts"
│           ├── StatCard "Rules"        (Enabled / Total)
│           ├── StatCard "Emails"       (Unread / Total)
│           └── StatCard "LastPoll"     (relative time + HealthBadge)
├── Center: container.NewHSplit(0.55)
│           ├── Left:  AccountHealthList   (widget.List)
│           └── Right: RecentActivityList  (widget.List)
└── Bottom: ActionBar                   (container.NewHBox)
            ├── widget.NewLabel("Updated {RelativeTime}")
            ├── layout.NewSpacer()
            └── widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), vm.OnRefresh)
```

### 2.1 `StatCard` (custom widget)

```go
type StatCard struct {
    widget.BaseWidget
    Title    string
    Primary  binding.String   // "1,247"
    Secondary binding.String  // "of 12,500" (optional)
    Health   binding.String   // "Healthy" | "Warning" | "Error" | ""
}
```

Renders inside `canvas.Rectangle` with border-radius `theme.Padding()*1.5`, background `theme.BackgroundColor()`, and a 1px stroke `theme.SeparatorColor()`. The `Health` binding drives a 6×6 dot in the top-right corner using `--Status-{Healthy|Warning|Error}` tokens.

### 2.2 `AccountHealthList`

`widget.List` with item template:

```
[●] {Alias}                        {EmailsStored} stored · {UnreadCount} unread
    Last poll: {RelativeTime}      {ConsecutiveFailures > 0 ? "⚠ N failures" : ""}
```

Row click → `vm.OnAccountClicked(alias)` → emits navigation event consumed by `internal/ui/nav.go` (`NavigateTo("emails", QueryParams{"Alias": alias})`).

### 2.3 `RecentActivityList`

`widget.List` capped at 20 rows. Item template:

```
{KindIcon} {RelativeTime} · {Alias}
    {Message}{ErrorCode > 0 ? "  [E" + ErrorCode + "]" : ""}
```

`KindIcon` map (PascalCase enum → Fyne icon resource):

| ActivityKind     | Icon                          |
|------------------|-------------------------------|
| `PollStarted`    | `theme.MediaPlayIcon()`       |
| `PollSucceeded`  | `theme.ConfirmIcon()`         |
| `PollFailed`     | `theme.ErrorIcon()`           |
| `EmailStored`    | `theme.MailComposeIcon()`     |
| `RuleMatched`    | `theme.SearchIcon()`          |

---

## 3. States

| State          | Trigger                              | Visible UI                                        |
|----------------|--------------------------------------|---------------------------------------------------|
| `Loading`      | First mount / Refresh in flight      | `widget.NewProgressBarInfinite()` over content    |
| `Loaded`       | `Summary` returned successfully      | Full widget tree as §2                            |
| `Error`        | `Summary` returned error             | `ErrorPanel{ErrorCode, Message, RetryButton}`     |
| `Empty`        | Loaded but `Accounts == 0`           | `EmptyState{Title, Body, CTA="Add account"}`      |
| `Stale`        | `time.Since(GeneratedAt) > 60 s`     | Subtle "Stale — refresh" badge in ActionBar       |

`Loading` does **not** clear the previous content — it overlays a translucent scrim so the user keeps spatial context.

---

## 4. Theming Tokens (read-only consumers)

Defined in `internal/ui/theme/theme.go`, sourced from `16-app-design-system-and-ui.md`:

| Token                    | Use                                       |
|--------------------------|-------------------------------------------|
| `--Surface-Card`         | StatCard background                       |
| `--Border-Subtle`        | StatCard stroke                           |
| `--Status-Healthy`       | Green dot                                 |
| `--Status-Warning`       | Amber dot                                 |
| `--Status-Error`         | Red dot                                   |
| `--Text-Primary`         | StatCard primary number                   |
| `--Text-Secondary`       | StatCard caption                          |
| `--Spacing-Card`         | Padding inside StatCard                   |
| `--Radius-Card`          | StatCard border-radius                    |

**Hard rule:** No hex colors in `views/dashboard.go`. Lint check `linters/golangci-lint/.golangci.yml` rule `no-hex-in-views` enforces this.

---

## 5. Lifecycle Hooks

```go
func (v *DashboardView) OnShow()  // calls vm.Refresh(ctx) + vm.AttachLive(ctx)
func (v *DashboardView) OnHide()  // calls vm.DetachLive()
```

Wired in `internal/ui/nav.go` tab switcher. Failure to detach is a leak — `internal/watcher`'s event bus tracks open subscriptions and logs `WARN WatchSubscriberLeak` if a tab is hidden without `DetachLive`.

---

## 6. Interaction Rules

| User action                  | Effect                                                                |
|------------------------------|-----------------------------------------------------------------------|
| Click **Refresh** button     | `vm.Refresh(ctx)` (debounced 500 ms; spinner overlay shows)           |
| Click `AccountHealthList` row| Navigate to Emails tab with `Alias` filter                            |
| Click `RecentActivityList` row of kind `PollFailed` | Open `ErrorDetailDialog{ErrorCode, Trace}`     |
| Press `F5`                   | Same as Refresh (registered via `desktop.CustomShortcut`)             |
| Press `Esc` while dialog open| Close dialog                                                          |

No drag-and-drop. No context menus. No double-click bindings.

---

## 7. Performance Budgets

| Metric                                  | Budget    | Measured by                          |
|-----------------------------------------|-----------|--------------------------------------|
| Cold mount → first paint                | < 100 ms  | `views/dashboard_bench_test.go`      |
| `Refresh` round-trip (warm DB)          | < 50 ms   | same                                 |
| Live `WatchEvent` insert                | < 16 ms   | `internal/ui/views/dashboard_bench_test.go` |
| Memory footprint (cap 20 activity rows) | < 2 MB    | manual `go test -memprofile`         |

---

## 8. Accessibility

- All `StatCard` instances expose `widget.Accessible` with role `"region"` and an `aria-label` of `"{Title} statistic: {Primary}"`.
- All clickable rows expose role `"button"`.
- Focus order: Refresh button → AccountHealthList → RecentActivityList → StatCards.
- Screen-reader announcement on `Loaded`: `"Dashboard updated, {Accounts} accounts, {EmailsTotal} emails."` (announced via `widget.AccessibleAnnounce`).

---

## 9. Testing Contract

File: `internal/ui/views/dashboard_test.go`. Smoke tests only (per `04-coding-standards.md` §7):

1. `Mount_RendersAllStatCards` — uses `test.NewApp()`, asserts 4 StatCards present.
2. `Refresh_ShowsOverlayThenClears`.
3. `Error_ShowsErrorPanelWithRetry`.
4. `Empty_ShowsAddAccountCTA`.
5. `OnHide_DetachesLiveSubscription` — asserts `vm.cancelSub` called.
6. `WatchEvent_PrependsToActivityList_CapsAt20`.

Fakes:
- `core.FakeDashboard` implementing the same method set.
- `core.FakeWatch` with a controllable event channel.

---

## 10. Compliance Checklist

- [x] Single Fyne-importing file (`internal/ui/views/dashboard.go`).
- [x] VM holds no widgets, only `binding.*`.
- [x] No hex colors; all colors via theme tokens.
- [x] PascalCase widget/struct names.
- [x] Subscription lifecycle paired (`OnShow`/`OnHide`).
- [x] Performance budgets stated and benchmark-enforced.
- [x] Accessibility roles + announcements defined.
- [x] Cites `16-app-design-system-and-ui.md`.

---

**End of `01-dashboard/02-frontend.md`**
