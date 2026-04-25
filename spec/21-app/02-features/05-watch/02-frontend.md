# 05 — Watch — Frontend

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Defines the **Fyne widget tree, view-model, theming, interaction, lifecycle, and live-update wiring** for the Watch view — the headline real-time surface of `email-read`. Lives in `internal/ui/views/watch.go` — the only file in `internal/ui` permitted to compose Watch widgets. Also defines the **sidebar status dot** binding consumed by `internal/ui/sidebar.go`.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend contract: [`./01-backend.md`](./01-backend.md)
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.5
- Logging (heartbeat invariant 🔴): [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.4
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Design system: `spec/12-consolidated-guidelines/16-app-design-system-and-ui.md`
- Sibling consumer: `02-features/01-dashboard/02-frontend.md` (Recent Events feed reads the same `WatchEvent` stream — must stay in sync)
- Regression to never repeat: `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`

---

## 1. View-Model

```go
// Package views — file: internal/ui/views/watch.go
package views

type WatchVM struct {
    svc        *core.Watch
    accSvc     *core.Accounts
    nav        NavRouter

    // ---------- Bindings exposed to widgets ----------
    activeAlias    binding.String        // the alias currently shown in this view
    status         binding.Untyped       // *core.WatchState (full snapshot, refreshed on every StatusChanged)
    statusKind     binding.String        // string(core.WatchStatus) — for status-dot color binding
    statusLabel    binding.String        // "Watching · alias@domain · IDLE · uptime 03:42"
    backoffSecs    binding.Int           // remaining seconds until next reconnect attempt; 0 when not Reconnecting
    cardItems      binding.UntypedList   // []*WatchCardItem (newest first; capped 200)
    rawLogLines    binding.StringList    // []string (newest at end; capped 2000)
    pollCounter    binding.Int           // total PollHeartbeat events seen this session
    newMailCounter binding.Int           // total NewMail events seen this session
    matchCounter   binding.Int           // total RuleMatched events seen this session
    isPaused       binding.Bool          // raw-log autoscroll paused
    isStarting     binding.Bool          // ▶ Start in flight
    isStopping     binding.Bool          // ■ Stop in flight
    aliasOptions   binding.StringList    // for header alias picker; mirrors AccountsVM.PickerSnapshot

    // ---------- Internal ----------
    sub        <-chan core.WatchEvent     // current Subscribe() channel; nil when detached
    cancelSub  context.CancelFunc         // cancels sub goroutine
    cards      []*WatchCardItem           // in-memory ring buffer (cap 200)
    logBuf     []string                   // in-memory ring buffer (cap 2000)
    cardIndex  map[int64]*WatchCardItem   // EmailId → card (for in-place RuleMatched badge)
    clock      Clock                      // injectable for tests
    log        Logger                     // structured logger (package zerolog)
}

type WatchCardItem struct {                // one structured card; one new email = one card
    Time        time.Time
    Alias       string
    EmailId     int64
    From        string                    // formatted "Name <addr>" or just addr
    Subject     string                    // truncated to 120 chars in widget; full in tooltip
    Snippet     string                    // first 160 chars from EmailSummary.SnippetText
    RuleMatches []WatchCardRuleMatch      // 0..N — appended in-place when RuleMatched arrives
    TraceId     string
}

type WatchCardRuleMatch struct {
    RuleName string
    Action   string                       // "OpenUrl" | "Notify" | … (from core.Rules)
    Urls     []string                     // for Action == OpenUrl
}
```

### 1.1 Constructor

```go
func NewWatchVM(svc *core.Watch, accSvc *core.Accounts, nav NavRouter, clock Clock, log Logger) *WatchVM
```

- Pure assembly — **must not** call `Status` / `Subscribe` / dial IMAP.
- All side effects deferred to `Attach()` (§5.1).

### 1.2 Buffer caps (constants — single source of truth)

```go
const (
    WatchCardCap      = 200    // §5 of overview / §7 below
    WatchRawLogCap    = 2000   // §5 of overview
    WatchSubChanCap   = 256    // mirrors backend Subscribe() buffer
    WatchHeaderTickMs = 1000   // 1 Hz refresh of "uptime" + backoff countdown
)
```

The Cards/RawLog caps **must** match the values in `00-overview.md` §5 and `97-acceptance-criteria.md`. A drift here is a spec inconsistency.

---

## 2. Widget Tree

The Watch view is a vertical split: **status header** (fixed height 96 px) + **two-tab content** (Cards | Raw log).

### 2.1 Top-level layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│ WatchView (container.NewBorder)                                         │
│ ┌─ Top: WatchStatusHeader (Border layout, height 96 px) ──────────────┐ │
│ │  [● dot] [Status label]                  [Alias picker ▾] [▶ Start] │ │
│ │  [Reconnect banner — visible only when Status==Reconnecting]        │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌─ Center: container.NewAppTabs (fills remaining) ────────────────────┐ │
│ │  ┌─[ Cards ]─[ Raw log ]──────────────────────────────────────────┐ │ │
│ │  │  Tab content swaps; both subscribe to the SAME WatchVM stream │ │ │
│ │  └────────────────────────────────────────────────────────────────┘ │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌─ Bottom: WatchFooter (Border layout, height 28 px) ─────────────────┐ │
│ │  polls=NNN  newMail=NN  matches=NN          [traceId of last event] │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

Composition (Fyne):

```go
func (vm *WatchVM) Build() fyne.CanvasObject {
    header := vm.buildHeader()           // §2.2
    tabs   := container.NewAppTabs(
        container.NewTabItemWithIcon("Cards",   theme.DocumentIcon(),  vm.buildCardsTab()),  // §2.3
        container.NewTabItemWithIcon("Raw log", theme.FileTextIcon(),  vm.buildRawLogTab()), // §2.4
    )
    tabs.SetTabLocation(container.TabLocationTop)
    footer := vm.buildFooter()           // §2.5
    return container.NewBorder(header, footer, nil, nil, tabs)
}
```

### 2.2 Status header (`buildHeader`)

| Widget                       | Type                       | Binding                      | Behavior                                                   |
|------------------------------|----------------------------|------------------------------|------------------------------------------------------------|
| `statusDot`                  | `*canvas.Circle` (12 × 12) | `vm.statusKind`              | Color per §3.2 (Idle grey / Watching green / Reconnecting amber / Error red / Starting+Stopping pulsing blue) |
| `statusLabel`                | `*widget.Label` bold 14 pt | `vm.statusLabel`             | "Watching · `alias` · IDLE · uptime 03:42" (rebuilt every 1 s by `headerTicker`) |
| `aliasPicker`                | `*widget.Select`           | `vm.aliasOptions` + `vm.activeAlias` | Disabled when `isStarting \|\| isStopping`. Switching while `Watching` shows confirm strip (§4.4). |
| `startBtn`                   | `*widget.Button` Primary   | enabled when `status.Status ∈ {Idle, Error}` | Click → `vm.handleStart()` (§4.1) |
| `stopBtn`                    | `*widget.Button` Danger    | enabled when `status.Status ∈ {Watching, Reconnecting, Starting}` | Click → `vm.handleStop()` (§4.2) |
| `reconnectBanner`            | `*widget.Card` (warning)   | visible iff `status.Status == Reconnecting` | "Reconnecting in `{backoffSecs}` s (step `{BackoffStep+1}`/6) — last error: `{LastError}`" |
| `errorBanner`                | `*widget.Card` (danger)    | visible iff `status.Status == Error`        | "Watcher stopped: `{LastErrorCode}` `{LastError}`" + retry CTA = ▶ Start |

`startBtn` and `stopBtn` are mutually exclusive; only one is visible at a time (`canvas.Hide`/`Show`). Never disable the only visible button silently — show a spinner instead while `isStarting`/`isStopping`.

### 2.3 Cards tab (`buildCardsTab`)

| Widget                | Type                                       | Binding              | Behavior                                                |
|-----------------------|--------------------------------------------|----------------------|---------------------------------------------------------|
| `cardsScroll`         | `*container.Scroll` (vertical)             | —                    | Wraps the list; auto-scroll to top on new card unless user is scrolled mid-list (detect via `cardsScroll.Offset.Y > 64`). |
| `cardsList`           | `*widget.List` virtualized                 | `vm.cardItems`       | Renders one `cardRow` per `WatchCardItem`; cap 200; oldest auto-evicted at the **list level**, not by binding remove (avoids flicker). |
| `cardRow` (per item)  | `container.NewVBox` inside `*widget.Card`  | item-bound           | See §2.3.1 |
| `cardsEmpty`          | `*widget.Label` centered, dim              | shown when `len(cardItems) == 0` | "No new mail yet — heartbeats stream to the Raw log tab." |

Empty-state and list are stacked in a `container.NewMax` so swapping is `Show()`/`Hide()` only — no re-layout cost.

#### 2.3.1 `cardRow` widget tree (per `WatchCardItem`)

```
┌─ widget.Card (no title; padding 12 px) ─────────────────────────────┐
│ ┌─ row: HBox ───────────────────────────────────────────────────┐   │
│ │ [from: bold 13pt, ellipsis]  [time: monospace dim, right]    │   │
│ └───────────────────────────────────────────────────────────────┘   │
│ ┌─ subject: 14pt regular, max 2 lines ──────────────────────────┐   │
│ ┌─ snippet: 12pt dim, max 2 lines ──────────────────────────────┐   │
│ ┌─ matches: HBox (only if RuleMatches not empty) ───────────────┐   │
│ │ [● badge "rule:{Name}"] [↗ link 1] [↗ link 2] …                │   │
│ └───────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

- **Badge** uses the `theme.RuleMatchBadge` color token (§3).
- Each `↗ link` is a `*widget.Hyperlink` — clicking does **not** call the OS browser directly; it calls `core.Tools.OpenUrl(ctx, url)` (the same path the rule auto-fired) so audit row goes through `OpenedUrl`. Never bypass `core.Tools`.
- Cards are **append-once, mutate-in-place** for `RuleMatched`. Lookup via `vm.cardIndex[EmailId]` is O(1).

### 2.4 Raw log tab (`buildRawLogTab`)

| Widget          | Type                           | Binding             | Behavior                                                          |
|-----------------|--------------------------------|---------------------|-------------------------------------------------------------------|
| `rawLogToolbar` | `*widget.Toolbar`              | —                   | `[⏸ Pause]` (toggles `isPaused`), `[🗑 Clear]`, `[💾 Copy all]`     |
| `rawLogScroll`  | `*container.Scroll` vertical   | —                   | Sticks to bottom unless `isPaused == true` OR user scrolled up (>64 px from bottom). |
| `rawLogList`    | `*widget.List` virtualized     | `vm.rawLogLines`    | One row = one line, monospace 12 pt, copy-on-double-click.        |

- `Pause` only suspends **autoscroll**, not the underlying subscription — events keep accumulating into `vm.logBuf` (still capped at 2000).
- `Clear` empties `vm.logBuf` AND `vm.rawLogLines.Set([]string{})` but does not unsubscribe.
- `Copy all` joins lines with `\n` and writes to the clipboard via `fyne.CurrentApp().Clipboard()`.
- Each row is a flat string assembled in `formatRawLine(ev WatchEvent) string`:
  ```
  15:04:05.000 [PollHeartbeat]  poll: messages=1842 uidNext=9012 new=0 (87 ms)
  15:04:08.123 [NewMail]        from=alice@x.test subject="Order #42 confirmation"
  15:04:08.140 [RuleMatched]    rule="bank-otp" emailId=12345 action=OpenUrl urls=1
  15:04:11.001 [PollHeartbeat]  poll: messages=1843 uidNext=9013 new=0 (91 ms)
  15:04:14.870 [Reconnecting]   step=2 wait=5s reason=ER-MAIL-21208
  ```
  Format is **deterministic** (same as `05-logging-strategy.md` §6.4) so test fixtures can match exact strings.

### 2.5 Footer (`buildFooter`)

| Widget          | Binding              | Format                                       |
|-----------------|----------------------|----------------------------------------------|
| `pollLbl`       | `vm.pollCounter`     | `polls={N}`                                  |
| `newMailLbl`    | `vm.newMailCounter`  | `newMail={N}`                                |
| `matchLbl`      | `vm.matchCounter`    | `matches={N}`                                |
| `traceIdLbl`    | last event's TraceId | monospace, dim, right-aligned, copy on click |

Counters are session-scoped (reset on `Detach()` / app restart); the persistent totals live on the Dashboard.

---

## 3. Theming

### 3.1 Token usage

All colors come from `internal/ui/theme` — no hex literals. New tokens introduced for Watch:

```go
const (
    ColorWatchDotIdle         = "watchDotIdle"          // grey 500
    ColorWatchDotWatching     = "watchDotWatching"      // green 500
    ColorWatchDotReconnecting = "watchDotReconnecting"  // amber 500 (pulses 1 Hz)
    ColorWatchDotError        = "watchDotError"         // red 500
    ColorWatchDotStarting     = "watchDotStarting"      // blue 400 (pulses 2 Hz)
    ColorWatchDotStopping     = "watchDotStopping"      // blue 300 (pulses 2 Hz)
    ColorRuleMatchBadge       = "ruleMatchBadge"        // accent purple
    ColorRawLogHeartbeat      = "rawLogHeartbeat"       // dim grey 400
    ColorRawLogNewMail        = "rawLogNewMail"         // foreground 100%
    ColorRawLogError          = "rawLogError"           // red 400
)
```

Token resolution must be added to `internal/ui/theme/tokens.go` and **mirrored** in `spec/24-app-design-system-and-ui/` (Task #33). No widget hard-codes a `color.RGBA`.

### 3.2 Status dot color map

```go
var watchDotColor = map[core.WatchStatus]string{
    core.WatchStatusIdle:         ColorWatchDotIdle,
    core.WatchStatusStarting:     ColorWatchDotStarting,
    core.WatchStatusWatching:     ColorWatchDotWatching,
    core.WatchStatusReconnecting: ColorWatchDotReconnecting,
    core.WatchStatusStopping:     ColorWatchDotStopping,
    core.WatchStatusError:        ColorWatchDotError,
}
```

Lookup is **total** (every `WatchStatus` value covered). Adding a new status without updating this map is a compile-time error via `golangci-lint` exhaustive check (config in `linters/`).

### 3.3 Pulse animation

Reconnecting / Starting / Stopping dots pulse via `canvas.NewColorRGBAAnimation` with `fyne.AnimationEaseInOut`. Single source: `internal/ui/anim/pulse.go` — Watch must not roll its own.

---

## 4. Interactions & Events

### 4.1 Start (▶)

```go
func (vm *WatchVM) handleStart() {
    if vm.isStarting.Get() || vm.isStopping.Get() { return }
    vm.isStarting.Set(true)
    alias, _ := vm.activeAlias.Get()
    go func() {
        defer vm.isStarting.Set(false)
        opts := core.WatchOptions{Alias: alias} // backend fills defaults from config
        if r := vm.svc.Start(ctx, opts); r.IsErr() {
            vm.showError(r.Err())   // toast — never silent
            return
        }
        // On success: backend will publish StatusChanged → Status will update via subscription;
        // we do NOT optimistically set Status here.
    }()
}
```

- **No optimistic UI** — status changes only via the subscription stream. Source-of-truth is the backend.
- Errors render via `widget.NewModalPopUp` with `Code` + `Message` from `apperror.Wrap`.
- `Start` while already `Watching` for the same alias → backend returns `21401 WatchAlreadyStarted`; the toast surfaces the friendly message "Already watching this account" (no scary stack).

### 4.2 Stop (■)

```go
func (vm *WatchVM) handleStop() {
    vm.isStopping.Set(true)
    alias, _ := vm.activeAlias.Get()
    go func() {
        defer vm.isStopping.Set(false)
        if r := vm.svc.Stop(ctx, alias); r.IsErr() {
            vm.showError(r.Err()); return
        }
    }()
}
```

- The 2-second `Stop` SLO is enforced by the backend; the spinner is purely informational.
- After `Stop` completes, the next `StatusChanged` (`Watching → Stopping → Idle`) drives the UI back to Idle.

### 4.3 Pause / Clear / Copy (raw log)

| Control      | Handler                                                                            |
|--------------|------------------------------------------------------------------------------------|
| `⏸ Pause`    | `vm.isPaused.Set(!cur)` — autoscroll stops; events still accumulate.               |
| `🗑 Clear`   | `vm.logBuf = vm.logBuf[:0]; vm.rawLogLines.Set([]string{})` — does NOT unsubscribe. |
| `💾 Copy all`| Clipboard write of `strings.Join(vm.logBuf, "\n")`.                                |

### 4.4 Alias switch while Watching

If user changes `aliasPicker` while `status.Status ∈ {Watching, Reconnecting, Starting}`:

1. Show a confirm strip docked below the header: "Stop watching `{OldAlias}` first?" `[Stop & switch]` `[Cancel]`.
2. `[Cancel]` — revert `aliasPicker` selection to `OldAlias`; no side effects.
3. `[Stop & switch]` — call `Stop(ctx, OldAlias)` then on success update `vm.activeAlias` to `NewAlias`. Do **not** auto-start the new one (deliberate — user must press ▶).

If `status.Status ∈ {Idle, Error, Stopping}`, the switch is immediate with no confirm.

### 4.5 Tab focus (within `container.NewAppTabs`)

`OnSelected` does **not** detach/attach the subscription — both tabs share the single `vm.sub`. Switching tabs is purely a Fyne `Show()`/`Hide()` toggle on the children. This guarantees no event is lost between tab switches.

### 4.6 Keyboard shortcuts (registered via `internal/ui/shortcuts.go`, scope = WatchView focused)

| Key        | Action            |
|------------|-------------------|
| `Cmd/Ctrl+Enter` | ▶ Start (if eligible) |
| `Cmd/Ctrl+.`     | ■ Stop  (if eligible) |
| `Cmd/Ctrl+L`     | Toggle Pause          |
| `Cmd/Ctrl+K`     | Clear raw log         |
| `Cmd/Ctrl+1`     | Switch to Cards tab   |
| `Cmd/Ctrl+2`     | Switch to Raw log tab |

---

## 5. Lifecycle

### 5.1 `Attach(ctx context.Context)`

Called by `MainWindow` when the Watch view becomes the active route.

```go
func (vm *WatchVM) Attach(ctx context.Context) errtrace.Result[Unit] {
    subCtx, cancel := context.WithCancel(ctx)
    vm.cancelSub = cancel

    // 1. Read alias options + initial status (single round-trip).
    if r := vm.refreshAliasOptions(ctx); r.IsErr() { return r }
    if r := vm.refreshStatus(ctx); r.IsErr() { return r }

    // 2. Subscribe to the canonical event stream.
    sub := vm.svc.Subscribe(subCtx)
    if sub.IsErr() { return errtrace.Err[Unit](sub.Err()) }
    vm.sub = sub.Ok()

    // 3. Start the consumer goroutine — single goroutine drains the channel.
    go vm.runEventLoop(subCtx)

    // 4. Start the 1 Hz header ticker (uptime + backoff countdown).
    go vm.runHeaderTicker(subCtx)

    return errtrace.Ok(Unit{})
}
```

### 5.2 `Detach()`

Called when navigating away (NOT when switching tabs within Watch view).

```go
func (vm *WatchVM) Detach() {
    if vm.cancelSub != nil { vm.cancelSub(); vm.cancelSub = nil }
    vm.sub = nil
    // Buffers retained — re-Attach restores the visible state instantly.
    // Sidebar dot continues to update via a SEPARATE persistent subscription
    // owned by sidebar.go (see §6.3).
}
```

### 5.3 Goroutine inventory (must be exact — no other goroutines may be spawned by views/watch.go)

| Goroutine          | Lifetime                           | Purpose                                          |
|--------------------|------------------------------------|--------------------------------------------------|
| `runEventLoop`     | `Attach` → `Detach`                | Drains `vm.sub`; dispatches per `WatchEventKind` |
| `runHeaderTicker`  | `Attach` → `Detach`                | 1 Hz ticker for uptime + backoff countdown       |
| `handleStart` (ad-hoc) | Single shot per click          | `svc.Start` call                                 |
| `handleStop` (ad-hoc)  | Single shot per click          | `svc.Stop` call                                  |

Tests assert leak-free shutdown via `goleak.VerifyNone(t)`.

---

## 6. Live-Update Reactions

### 6.1 Event dispatch table (single source of truth)

```go
func (vm *WatchVM) onEvent(ev core.WatchEvent) {
    switch ev.Kind {
    case core.WatchEventKindStatusChanged:        vm.applyStatusChange(ev)
    case core.WatchEventKindNewMail:              vm.appendCard(ev); vm.appendRawLine(ev); vm.newMailCounter.Set(vm.newMailCounter.Get()+1)
    case core.WatchEventKindRuleMatched:          vm.attachRuleBadge(ev); vm.appendRawLine(ev); vm.matchCounter.Set(vm.matchCounter.Get()+1)
    case core.WatchEventKindPollHeartbeat:        vm.appendRawLine(ev); vm.pollCounter.Set(vm.pollCounter.Get()+1)
    case core.WatchEventKindWatchStart,
         core.WatchEventKindWatchStop,
         core.WatchEventKindAccountConnected,
         core.WatchEventKindAccountConnectError,
         core.WatchEventKindReconnecting,
         core.WatchEventKindReconnectExhausted:   vm.appendRawLine(ev)
    default:
        vm.log.Warn().Str("kind", string(ev.Kind)).Msg("unknown WatchEventKind — ignored")
    }
}
```

The switch is **exhaustive** for every `WatchEventKind` declared in `00-overview.md` §4.1. `golangci-lint exhaustive` is enabled; adding a new kind without updating this switch is a compile failure.

### 6.2 Heartbeat invariant 🔴 (UI obligation)

The Cards tab does **not** render heartbeat events as cards (would be visual noise) but MUST:

1. Append the heartbeat line to the Raw log tab (always — even when paused; only autoscroll suspends).
2. Increment `vm.pollCounter` (visible in footer).

If a UI test ever asserts that no heartbeat appears in the raw log over a 10-cycle window, it has detected the regression from `solved-issues/02-watcher-silent-on-healthy-idle.md` and must fail loudly.

### 6.3 Sidebar dot (separate subscription)

`internal/ui/sidebar.go` owns a **persistent** `core.Watch.Subscribe()` that lives for the entire app session. It listens for `StatusChanged` events and updates the sidebar's per-alias status dot. This subscription is **independent** of the Watch view's subscription — closing the Watch view never blacks out the sidebar dot.

The sidebar dot binding contract:

```go
type SidebarWatchDotBinding interface {
    Color(alias string) string                  // returns one of the ColorWatchDot* tokens
    Subscribe(fn func(alias string, status core.WatchStatus)) (unsub func())
}
```

WatchVM **must not** drive the sidebar dot directly — it goes via the same fan-out. Single source of truth = `core.Watch`.

### 6.4 Slow-subscriber handling (UI side)

If the UI's `vm.sub` channel ever fills (cap 256), the **backend** drops the oldest event with `WARN WatchSubscriberSlow` (per `00-overview.md` §3 / `01-backend.md` §6). The UI is therefore guaranteed never to block the watcher.

To detect *itself* being slow, `runEventLoop` records the time between `<-vm.sub` recv and `onEvent` return. If > 50 ms over 10 consecutive events, it logs `WARN WatchUiRenderSlow` (single line, throttled). It does **not** retry, drop, or skip events on the UI side — that's the backend's job.

### 6.5 Account events

A separate sub via `core.Accounts.Subscribe()` updates `vm.aliasOptions` on `AccountEvent{Added/Removed/Renamed}`. If `Removed` matches `vm.activeAlias`, the view auto-clears `activeAlias` and shows the empty-state ("Pick an account to watch"). The backend has already auto-Stopped the watcher per `00-overview.md` §7.

---

## 7. Performance Budgets

| Operation                                          | Budget       | How enforced                                        |
|----------------------------------------------------|--------------|-----------------------------------------------------|
| `Attach()` to first paint of header                | **≤ 80 ms**  | `BenchmarkWatchAttach` (testing/benchmark)          |
| Append one card (best case, no scroll repaint)     | **≤ 4 ms**   | `BenchmarkAppendCard`                               |
| Append one raw-log line                            | **≤ 1 ms**   | `BenchmarkAppendRawLine`                            |
| Per-tick header refresh (uptime + backoff secs)    | **≤ 2 ms**   | `BenchmarkHeaderTick`                               |
| Card buffer eviction (200 → 199 then push)         | **O(1)**     | Ring-buffer impl + bench                            |
| Raw-log buffer eviction (2000 → 1999 then push)    | **O(1)**     | Ring-buffer impl + bench                            |
| Memory ceiling for `vm.cards + vm.logBuf`          | **≤ 2 MiB**  | `TestWatchMemoryCeiling` after 1 h synthetic stream |

Ring-buffer impl lives in `internal/util/ringbuf/` (typed generic, no `any`). Slice-append + `[1:]` is forbidden (O(n) copy under load).

---

## 8. Accessibility

- **Status dot is decorative** — the `statusLabel` carries the semantic state ("Watching", "Reconnecting in 5 s", …). Screen readers see the label.
- All buttons have `widget.Button.SetText` (no icon-only buttons in Watch).
- Card hyperlinks have `aria-label` text = "Open `{url}` in browser (opens incognito)".
- Color-blind safety: status dot is **always** accompanied by the textual label, never used alone to convey state.
- Focus order: `aliasPicker → startBtn/stopBtn → tabs → toolbar (raw log) → list`. Tab navigation must be predictable.
- Reduced motion: when `fyne.CurrentApp().Settings().ReducedMotion()` is true, pulse animations (§3.3) are static.

---

## 9. Testing Contract

All tests live in `internal/ui/views/watch_test.go` and use the headless Fyne test driver (`fyne.io/fyne/v2/test`).

### 9.1 Required tests

| #  | Test name                                                    | Asserts                                                                                                  |
|----|--------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| 1  | `WatchVM_Attach_FetchesInitialStatusAndAliases`              | `Attach` calls `Status` + alias picker populated; one `Subscribe` opened.                                |
| 2  | `WatchVM_StatusChanged_UpdatesDotAndLabel_AllSixStates`      | Inject each `WatchStatus` enum value → `statusKind` and `statusLabel` reflect it; coverage = 6/6.        |
| 3  | `WatchVM_Heartbeat_AppendsRawLine_NoCard` 🔴                 | Inject 10 `PollHeartbeat` events → `rawLogLines` length += 10, `cardItems` unchanged, `pollCounter`==10. |
| 4  | `WatchVM_NewMail_AppendsCardAndRawLine`                      | One `NewMail` → one new card (newest first), one raw log line, `newMailCounter`==1.                      |
| 5  | `WatchVM_RuleMatched_AttachesBadgeInPlace`                   | After `NewMail{EmailId:42}` then `RuleMatched{EmailId:42}` → same card has 1 `WatchCardRuleMatch`, no new card created. |
| 6  | `WatchVM_CardCap_DropsOldestAt200`                           | Inject 250 `NewMail` → `len(cardItems) == 200` and items[0] is newest.                                   |
| 7  | `WatchVM_RawLogCap_DropsOldestAt2000`                        | Inject 2500 events → `len(rawLogLines) == 2000` and last line matches the 2500th event.                  |
| 8  | `WatchVM_Pause_StopsAutoscroll_NotEventConsumption`          | Pause; inject 100 events → buffer grows to 100, scroll offset unchanged.                                 |
| 9  | `WatchVM_Clear_EmptiesBufferKeepsSubscription`               | Clear → `rawLogLines` empty; subsequent inject still appends.                                            |
| 10 | `WatchVM_HandleStart_DisablesUIWhileInFlight`                | Mock `Start` with 200 ms latency; `isStarting` true during, false after; spinner visible.                |
| 11 | `WatchVM_AlreadyWatching_ShowsFriendlyToast`                 | `Start` returns `21401 WatchAlreadyStarted` → toast text = "Already watching this account"; no stack.    |
| 12 | `WatchVM_Reconnecting_ShowsBannerWithCountdown`              | Inject `Reconnecting` with `BackoffUntil = now+5s` → banner visible, `backoffSecs` decrements 5→0.       |
| 13 | `WatchVM_BackoffExhausted_ShowsErrorBanner_StartReenabled`   | Inject `ReconnectExhausted` → error banner visible, ▶ Start re-enabled, ■ Stop hidden.                   |
| 14 | `WatchVM_AliasSwitchWhileWatching_ShowsConfirm`              | Switch picker while `Watching` → confirm strip rendered; Cancel reverts; Stop&switch calls `Stop`.       |
| 15 | `WatchVM_AccountRemoved_ClearsActiveAlias`                   | Inject `AccountEvent{Removed, Alias}` matching `activeAlias` → `activeAlias` cleared, empty state shown. |
| 16 | `WatchVM_TabSwitch_DoesNotResubscribe`                       | Switch Cards↔Raw log 5× → `Subscribe` called exactly once.                                               |
| 17 | `WatchVM_Detach_CancelsSubscription_NoLeak`                  | `goleak.VerifyNone(t)` passes after Detach.                                                              |
| 18 | `WatchVM_ExhaustiveEventKindSwitch`                          | Reflect over `WatchEventKind` constants → every value is handled by `onEvent`'s switch.                  |
| 19 | `WatchVM_RawLineFormat_DeterministicAcrossEventKinds`        | Snapshot test of `formatRawLine` for each kind matches golden file.                                      |
| 20 | `WatchVM_HyperlinkClickGoesViaCoreTools`                     | Card link click → `core.Tools.OpenUrl` called once with the URL; **never** `os/exec` directly.           |
| 21 | `WatchVM_NoHardcodedColors`                                  | AST scan asserts no `color.RGBA{...}` literals in `views/watch.go`.                                      |

### 9.2 Lint gates

- `golangci-lint exhaustive` on the `onEvent` switch and `watchDotColor` map.
- `forbidigo` blocks `os/exec`, `net/http`, `database/sql`, `fmt.Sprintf("%v"...)` in this file.
- `interfacebloat` keeps `WatchVM` cohesive (no growth without justification).

---

## 10. Compliance Checklist

- [x] PascalCase identifiers throughout (`04-coding-standards.md` §1.1).
- [x] No `any`/`interface{}` — `binding.Untyped` is wrapped in typed accessors only.
- [x] No imports of `internal/watcher`, `internal/mailclient`, `internal/store`, `internal/eventbus` from this file (verified by `linters/no-internal-from-views.sh`).
- [x] No hex color literals — every color via `internal/ui/theme` token.
- [x] Heartbeat invariant 🔴 enforced by Test #3 + Test #18 (exhaustive switch).
- [x] Single source of truth: every UI state derives from the backend event stream — no optimistic updates, no parallel polling.
- [x] Functions ≤ 15 lines (long handlers split into helpers; `Build()` composes only).
- [x] All errors flow through `apperror.Wrap` and surface via `vm.showError` toast (no `panic`, no `log.Fatal`).
- [x] Buffer caps (`WatchCardCap`, `WatchRawLogCap`) match `00-overview.md` §5.
- [x] Sidebar dot updates via `internal/ui/sidebar.go`'s **separate** persistent subscription, not from this view.
- [x] Goroutine inventory exact (4 patterns, leak-free per `goleak`).
- [x] Audit-safe: every link click goes via `core.Tools.OpenUrl` so `OpenedUrl` row is written.

---

**End of `05-watch/02-frontend.md`**
