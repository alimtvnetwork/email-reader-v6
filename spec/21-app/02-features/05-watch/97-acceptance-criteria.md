# 05 — Watch — Acceptance Criteria

**Version:** 1.0.1
**Updated:** 2026-04-27
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Binary, machine-checkable acceptance criteria for the **Watch** feature (overview / backend / frontend). A merged build is shippable iff **every** check below passes in CI. Each row maps to one or more concrete tests in `internal/{watcher,core,ui/views}/*_test.go` and benchmarks in `*_bench_test.go`.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend: [`./01-backend.md`](./01-backend.md)
- Frontend: [`./02-frontend.md`](./02-frontend.md)
- Logging (heartbeat invariant 🔴): [`../../05-logging-strategy.md`](../../05-logging-strategy.md) §6.4
- Error registry: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21400–21499`
- Solved-issue regression: `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`

---

---

## Sandbox feasibility legend (added Slice #184 — see `mem://workflow/progress-tracker.md`)

A fresh AI picking up an unchecked row should consult the `**Sandbox:**` tag immediately under
each section header to decide whether the row is implementable in the Lovable sandbox or must
be deferred to a workstation/CI runner.

| Tag | Meaning | Implementable in sandbox? |
|---|---|---|
| 🟢 **headless** | Go unit/integration tests, AST scanners, log greps, spec-doc edits. Verified via `nix run nixpkgs#go -- test -tags nofyne ./...`. | **Yes** — preferred sandbox work. |
| 🟡 **cgo-required** | Fyne canvas widget tests, real driver behaviour. Needs cgo + GL/X11. See `mem://workflow/canvas-harness-starter.md` (Slice #180). | **No** — defer to workstation; planned. |
| 🔴 **needs bench / E2E infra** | p95 perf gates (bench infra) or multi-process IMAP+browser E2E. See `mem://workflow/{bench,e2e}-harness-starter.md` (Slices #178/#179). | **No** — defer to CI runner; planned. |
| ⚪ **N/A** | Manual sign-off checklist; no automated test possible. | **No** — human reviewer. |

A section may carry **two** tags when its rows split (e.g. `🟢 + 🔴`); pick the right tag per row by reading the row itself.

## 1. Functional (must-pass)

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| #   | Criterion                                                                                                  | Test                                                       |
|-----|------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|
| F-01 | `Start(opts)` for an Idle alias transitions `Idle → Starting → Watching` and emits `StatusChanged` for each step. | `Watch_Start_TransitionsThroughStarting`                  |
| F-02 | `Start` for an alias already in `runners` returns `21401 WatchAlreadyStarted`; **no second runner spawned**. | `Watch_Start_RejectsDuplicate_NoSecondRunner`              |
| F-03 | `Stop(alias)` completes IMAP `LOGOUT` + ctx cancel + channel close within **2 s** (95th percentile over 50 runs). | `Watch_Stop_CompletesUnder2s_p95`                          |
| F-04 | `Stop` on an Idle alias returns success (idempotent) and **does not** log ERROR.                            | `Watch_Stop_Idempotent_NoError`                            |
| F-05 | `Status(alias)` returns the current `WatchState` snapshot in **≤ 5 ms**; never blocks on the loop.          | `Watch_Status_NonBlocking_Under5ms`                        |
| F-06 | `Subscribe(ctx)` returns a buffered channel of cap **256**; closing `ctx` closes the channel within 100 ms. | `Watch_Subscribe_BufferedAndClosesOnCtx`                   |
| F-07 | `StatusAll` returns one `WatchState` per alias known to `core.Accounts.List`, including Idle ones.          | `Watch_StatusAll_IncludesIdleAccounts`                     |
| F-08 | A real new email is detected via `UID > LastSeenUid`, persisted via `core.Emails.Insert`, and emitted as `WatchEvent{NewMail}` within **5 s** of arrival (Poll mode @ 3 s). | `Watch_NewMail_E2E_Under5s` (integration)              |
| F-09 | After processing, `WatchState.LastSeenUid` is updated **atomically** (single SQL UPDATE per cycle).         | `Watch_PollCycle_SingleCursorUpdate`                       |
| F-10 | A poison-email failure (`processEmail` returns error) emits `21403 WatcherProcessEmail` WARN and **continues** the cycle for remaining UIDs. | `Watch_PoisonEmail_DoesNotKillCycle`                  |
| F-11 | Rule match per new email triggers `core.Rules.EvaluateAll`; on match emits `WatchEvent{RuleMatched}` AND calls `core.Rules.BumpStat`. | `Watch_RuleMatch_EmitsEventAndBumpsStat`              |
| F-12 | For `Action == OpenUrl`, every extracted URL is passed to `core.Tools.OpenUrl` (one call per URL), writing one `OpenedUrl` row each. | `Watch_RuleMatch_OpenUrl_AuditTrailComplete`              |
| F-13 | `OnAccountRemoved(alias)` for the active alias auto-stops that runner within **1 s**; `WatchEvent{WatchStop}` emitted. | `Watch_OnAccountRemoved_AutoStops`                       |
| F-14 | `RestartOnAccountUpdate(alias)` cleanly stops + starts the runner; `LastSeenUid` is preserved across the restart. | `Watch_RestartOnAccountUpdate_PreservesCursor`             |

---

## 2. Heartbeat Invariant 🔴 (must-pass — single-handedly blocks merge)

**Sandbox:** 🟢 **headless** (most rows) + 🔴 **E2E** for cross-process timing — see `mem://workflow/e2e-harness-starter.md` (Slice #179).

| #   | Criterion                                                                                                  | Test                                                       |
|-----|------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|
| H-01 | Every poll cycle emits **one** `INFO` log line per `05-logging-strategy.md` §6.4, even when `NewCount == 0`. | `Watcher_HeartbeatLogEveryCycle_EvenWhenIdle`              |
| H-02 | Every poll cycle emits **one** `WatchEvent{Kind: PollHeartbeat}`; 10 cycles → exactly 10 events.            | `Watcher_HeartbeatEventEveryCycle_Exact10of10`             |
| H-03 | The heartbeat log line carries the full `PollStat` structure: `MessagesCount`, `UidNext`, `LastUid`, `NewCount`, `DurationMs`. | `Watcher_HeartbeatLog_HasFullPollStat`                |
| H-04 | The Watch view's Raw log tab renders 10 heartbeat lines after 10 cycles (cards tab unchanged, `pollCounter == 10`). | `WatchVM_Heartbeat_AppendsRawLine_NoCard` (frontend)   |
| H-05 | A defensive 30 s "no heartbeat seen" sentinel emits `21423 WatcherHeartbeatMissed` ERROR and auto-restarts the loop (must never fire if H-01 holds — pure backstop). | `Watcher_HeartbeatMissed_DefensiveAutoRestart`        |
| H-06 | IDLE-mode wakeups that reveal no new mail STILL emit a heartbeat (parity with Poll mode).                   | `Watcher_IdleWakeup_NoNewMail_StillHeartbeats`             |

> **Regression sentinel:** if H-01 or H-02 fails, the build fails immediately — this is the exact regression from `solved-issues/02-watcher-silent-on-healthy-idle.md`.

---

## 3. Reconnect Backoff Ladder

**Sandbox:** 🟢 **headless** (most rows) + 🔴 **E2E** for cross-process timing — see `mem://workflow/e2e-harness-starter.md` (Slice #179).

| #   | Criterion                                                                                                  | Test                                                       |
|-----|------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|
| R-01 | A connection drop transitions `Watching → Reconnecting` and emits `WatchEvent{Reconnecting}` with `BackoffStep == 0`, `BackoffUntil == now+1s`. | `Watcher_FirstDrop_BackoffStep0_1s`             |
| R-02 | The full backoff ladder is exactly **1 s, 2 s, 5 s, 10 s, 30 s, 60 s**. Step 6 = exhausted.                | `Watcher_BackoffLadder_ExactSequence_1_2_5_10_30_60`       |
| R-03 | Backoff exhaustion (step 6) emits `WatchEvent{ReconnectExhausted}` + `21418` ERROR; status transitions to `Error`. | `Watcher_BackoffExhausted_TransitionsToError`         |
| R-04 | The backoff counter resets to 0 on the **first successful poll cycle** after reconnect (NOT on successful LOGIN). | `Watcher_BackoffResets_OnFirstSuccessfulPoll_NotLogin` |
| R-05 | A connection that LOGINs successfully then immediately drops continues to climb the ladder (no reset).      | `Watcher_LoginThenImmediateDrop_KeepsClimbing`             |
| R-06 | `Stop` during backoff cancels the wait clock immediately (≤ 100 ms); status transitions to `Idle`.          | `Watcher_StopDuringBackoff_CancelsWait_Under100ms`         |
| R-07 | `Start` after `Error` resumes cleanly; backoff counter starts fresh at 0.                                   | `Watcher_StartAfterError_ResumesFresh`                     |
| R-08 | Frontend reconnect banner shows live countdown (`backoffSecs` decrements 1 Hz from `BackoffUntil - now`).   | `WatchVM_Reconnecting_ShowsBannerWithCountdown`            |

---

## 4. Mode Negotiation (IDLE vs Poll)

**Sandbox:** 🟢 **headless** (most rows) + 🔴 **E2E** for cross-process timing — see `mem://workflow/e2e-harness-starter.md` (Slice #179).

| #   | Criterion                                                                                                  | Test                                                       |
|-----|------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|
| M-01 | `WatchModeAuto` attempts IMAP IDLE; on success `WatchState.Mode == Idle`.                                  | `Watcher_AutoMode_PrefersIdleWhenSupported`                |
| M-02 | `WatchModeAuto` falls back to Poll on `ER-MAIL-21209 ErrMailIdleUnsupported`; emits `21414 WatcherIdleStartFailed` INFO; `WatchState.Mode == Poll`. | `Watcher_AutoMode_FallsBackToPoll_OnIdleUnsupported`  |
| M-03 | `WatchModeIdle` (forced) returns `21414` if server lacks IDLE; status → `Error` (no fallback).             | `Watcher_ForcedIdle_FailsHard_NoFallback`                  |
| M-04 | `WatchModePoll` (forced) never opens IDLE even when supported.                                              | `Watcher_ForcedPoll_NeverOpensIdle`                        |
| M-05 | IDLE notify timeout (no NOOP within 29 min) emits `21415 WatcherIdleNotifyTimeout`, treated as disconnect → backoff. | `Watcher_IdleNotifyTimeout_TriggersBackoff`           |

---

## 5. Event Fan-out (subscriber semantics)

**Sandbox:** 🟢 **headless** (most rows) + 🔴 **E2E** for cross-process timing — see `mem://workflow/e2e-harness-starter.md` (Slice #179).

| #   | Criterion                                                                                                  | Test                                                       |
|-----|------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|
| E-01 | Every `WatchEvent` is delivered exactly once to **every** subscriber (no duplicates, no losses on the happy path). | `Bus_FanOut_DeliversOncePerSubscriberPerEvent`        |
| E-02 | Subscriber channel cap = **256**; backend never blocks publishing.                                          | `Bus_PublishNeverBlocks_EvenWith0Subscribers`              |
| E-03 | A slow subscriber (consumes at 1 Hz while watcher emits at 100 Hz) drops oldest events; emits `21420 WatcherSubscriberSlow` WARN; watcher unaffected. | `Bus_SlowSubscriber_DropsOldest_WatcherUnaffected`    |
| E-04 | A panicking subscriber callback is recovered; subscriber is removed; emits `21421 WatcherSubscriberPanic` ERROR; loop continues. | `Bus_SubscriberPanic_Removed_LoopContinues`           |
| E-05 | Closing the subscribe ctx closes its channel within **100 ms**; no goroutine leak (`goleak.VerifyNone`).   | `Bus_CtxCancel_ClosesChannel_NoLeak`                       |
| E-06 | Three concurrent subscribers (Watch view + Dashboard recent-events + CLI) all see identical event streams over a 100-event run. | `Bus_ThreeConcurrentSubscribers_IdenticalStreams`     |

---

## 6. Frontend (UI contract)

**Sandbox:** 🟡 **cgo-required** — needs Fyne canvas harness; see `mem://workflow/canvas-harness-starter.md` (Slice #180).

| #   | Criterion                                                                                                  | Test                                                       |
|-----|------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|
| U-01 | `Attach()` to first paint of header ≤ **80 ms** (single round-trip: `Status` + `Subscribe`).               | `BenchmarkWatchAttach`                                     |
| U-02 | Cards tab capped at **200** items; oldest evicted O(1) via ring buffer (no slice copy).                    | `WatchVM_CardCap_DropsOldestAt200` + `BenchmarkAppendCard` |
| U-03 | Raw log capped at **2000** lines; oldest evicted O(1).                                                     | `WatchVM_RawLogCap_DropsOldestAt2000`                      |
| U-04 | Pause toggles autoscroll only — events keep accumulating into the buffer (still capped).                   | `WatchVM_Pause_StopsAutoscroll_NotEventConsumption`        |
| U-05 | Clear empties both buffer and binding without unsubscribing; subsequent events still append.                | `WatchVM_Clear_EmptiesBufferKeepsSubscription`             |
| U-06 | `RuleMatched` event for an existing card mutates the card in place (badge appended); no new card created. | `WatchVM_RuleMatched_AttachesBadgeInPlace`                 |
| U-07 | Tab switches (Cards ↔ Raw log) **never** cause re-subscription; `Subscribe` called exactly once per Attach. | `WatchVM_TabSwitch_DoesNotResubscribe`                    |
| U-08 | Status dot color map is **exhaustive** over `WatchStatus` (lint-enforced + reflection test).               | `WatchVM_StatusChanged_UpdatesDotAndLabel_AllSixStates`    |
| U-09 | `onEvent` switch is exhaustive over `WatchEventKind` (lint-enforced + reflection test).                    | `WatchVM_ExhaustiveEventKindSwitch`                        |
| U-10 | Hyperlink clicks in cards route through `core.Tools.OpenUrl` (NEVER `os/exec` directly).                   | `WatchVM_HyperlinkClickGoesViaCoreTools`                   |
| U-11 | No hex color literals in `views/watch.go` (AST scan).                                                      | `WatchVM_NoHardcodedColors`                                |
| U-12 | Sidebar dot updates via `internal/ui/sidebar.go`'s **separate** persistent subscription, not from WatchVM. | `Sidebar_Dot_UpdatesAfterWatchViewDetached`                |
| U-13 | Memory ceiling: `vm.cards + vm.logBuf` ≤ **2 MiB** after 1 h synthetic 10 Hz event stream.                 | `TestWatchMemoryCeiling`                                   |
| U-14 | `goleak.VerifyNone(t)` passes after `Detach()` (no leaked goroutines).                                     | `WatchVM_Detach_CancelsSubscription_NoLeak`                |
| U-15 | Already-watching toast text = `"Already watching this account"` — no error stack visible.                  | `WatchVM_AlreadyWatching_ShowsFriendlyToast`               |

---

## 7. Performance (CI-gated benchmarks)

**Sandbox:** 🔴 **needs bench infra** — see `mem://workflow/bench-infra-starter.md` (Slice #178).

| #   | Op                                                       | Budget        | Bench                              |
|-----|----------------------------------------------------------|---------------|------------------------------------|
| P-01 | One poll cycle (FETCH + cursor update + heartbeat)       | ≤ **150 ms** p95 (100 messages, no new) | `BenchmarkPollCycle`     |
| P-02 | Per-email persist (`core.Emails.Insert` + rule eval)     | ≤ **20 ms** p95                          | `BenchmarkProcessEmail`  |
| P-03 | `WatchVM.Attach` to first paint                          | ≤ **80 ms**                              | `BenchmarkWatchAttach`   |
| P-04 | Append one card                                          | ≤ **4 ms**                               | `BenchmarkAppendCard`    |
| P-05 | Append one raw-log line                                  | ≤ **1 ms**                               | `BenchmarkAppendRawLine` |
| P-06 | Header tick (uptime + backoff secs)                      | ≤ **2 ms**                               | `BenchmarkHeaderTick`    |
| P-07 | Bus fan-out, 1 publish → 3 subscribers                   | ≤ **0.2 ms** p95                         | `BenchmarkBusFanOut3`    |
| P-08 | `Stop` end-to-end (LOGOUT + ctx + channel close)         | ≤ **2 s** p95 (the SLO)                  | `BenchmarkStopP95`       |

---

## 8. Code Quality

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| #   | Criterion                                                                                       | How verified                       |
|-----|-------------------------------------------------------------------------------------------------|------------------------------------|
| Q-01 | Zero `interface{}` / `any` in any new file under `internal/watcher`, `internal/core/watch.go`, `internal/ui/views/watch.go`. | `linters/no-empty-interface.sh` |
| Q-02 | Every public method returns `errtrace.Result[T]`; no naked `error` returns from the API surface. | `linters/result-only.sh`           |
| Q-03 | Every function ≤ **15 lines** of body (excluding signature, braces, comments).                   | `linters/fn-length.sh` (≤ 15)      |
| Q-04 | No `time.Sleep` in production code; all waits are `select { ctx.Done(); clock.After }`.          | `linters/no-time-sleep.sh`         |
| Q-05 | No `os/exec`, `net/http`, `database/sql` in `internal/ui/views/watch.go` (forbidigo).            | `golangci-lint forbidigo`          |
| Q-06 | All identifiers PascalCase (variables, fields, types, constants) per `04-coding-standards.md` §1.1. | `linters/pascalcase.sh`         |
| Q-07 | `golangci-lint exhaustive` passes for `WatchStatus` map and `WatchEventKind` switch.             | `golangci-lint run`                |
| Q-08 | `goleak.VerifyNone(t)` in `TestMain` of every test package touching the watcher.                 | Test code review                   |

---

## 9. Security & PII

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| #   | Criterion                                                                                       | Test                                              |
|-----|-------------------------------------------------------------------------------------------------|---------------------------------------------------|
| S-01 | IMAP password is **never** logged in any code path (heartbeat, errors, debug).                  | `Logging_NeverContainsPassword` (regex over logs) |
| S-02 | Email body / snippet content is **never** logged at INFO level (only `EmailId`, `Alias`, hashes). | `Logging_NeverContainsEmailBody`                |
| S-03 | Card snippet truncated to 160 chars and stripped of OTP-looking patterns before render.         | `WatchCardItem_SnippetRedactsOtp`                 |
| S-04 | Hyperlinks always open via `core.Tools.OpenUrl` (incognito mode by default per Tools spec).     | `WatchVM_HyperlinkClickGoesViaCoreTools`          |
| S-05 | Subscriber callbacks run in their own goroutine — a malicious subscriber cannot exfiltrate the watcher's stack. | `Bus_SubscriberRunsInOwnGoroutine`           |

---

## 10. Logging

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| #   | Criterion                                                                                       | Test                                              |
|-----|-------------------------------------------------------------------------------------------------|---------------------------------------------------|
| L-01 | Every poll cycle emits one INFO line with all fields per `05-logging-strategy.md` §6.4.         | `Logging_HeartbeatStructure_FullFields` (= H-03)  |
| L-02 | Every backoff transition emits one WARN line with `Step`, `WaitSecs`, `LastErrorCode`.          | `Logging_BackoffWarn_HasStepAndWait`              |
| L-03 | `Reconnecting → Watching` (success after backoff) emits one INFO `WatcherReconnected`.          | `Logging_ReconnectSuccess_OneInfoLine`            |
| L-04 | Every error log line carries `TraceId` matching the corresponding `WatchEvent.TraceId`.         | `Logging_TraceIdMatchesEvent`                     |
| L-05 | Log redaction tested by injecting password/body strings and asserting absence in captured output. | `Logging_RedactionTest`                          |

---

## 11. Database (cursor & state)

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| #   | Criterion                                                                                       | Test                                              |
|-----|-------------------------------------------------------------------------------------------------|---------------------------------------------------|
| D-01 | `WatchState` row exists per alias; created lazily on first `Start`.                              | `Store_WatchState_LazyCreate`                     |
| D-02 | `LastSeenUid` is `INTEGER NOT NULL DEFAULT 0`; positive boolean `IsActive` per `18-database-conventions.md`. | `Schema_WatchState_PositiveBooleans`        |
| D-03 | One UPDATE per poll cycle (no per-email cursor write).                                          | `Watch_PollCycle_SingleCursorUpdate` (= F-09)     |
| D-04 | Cursor update failure (`21417`) emits WARN; cycle continues; cursor recovers next cycle.        | `Watch_CursorUpdateFails_NextCycleRecovers`       |
| D-05 | `WatchState` row is preserved when account is renamed (FK update, not delete+insert).           | `Store_WatchState_PreservedOnRename`              |
| D-06 | Account deletion cascades to `WatchState` row (FK `ON DELETE CASCADE`).                         | `Store_WatchState_CascadeOnAccountDelete`         |

---

## 12. Atomicity & Safety

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

| #   | Criterion                                                                                       | Test                                              |
|-----|-------------------------------------------------------------------------------------------------|---------------------------------------------------|
| A-01 | `Start` is idempotent under concurrent calls (mutex-protected `runners` map).                   | `Watch_Start_ConcurrentCallers_OnlyOneRunner`     |
| A-02 | `Stop` after partial `Start` failure (LOGIN succeeded, SELECT failed) cleanly closes the IMAP conn. | `Watch_PartialStartFailure_StopCleansUp`      |
| A-03 | App shutdown calls `StopAll()`; every runner stops within **2 s** in parallel (not sequential). | `Watch_StopAll_ParallelUnder2s`                   |
| A-04 | Per-email processing is wrapped in `recover()`; a panic in `processEmail` does NOT kill the runner. | `Watch_ProcessEmailPanic_RunnerSurvives`      |
| A-05 | Cursor write + email insert are NOT in a single SQL tx (intentional — email is the source of truth, cursor is a hint). | `Watch_NoCursorEmailTx_DocumentedDecision` |

---

## 13. Accessibility

**Sandbox:** 🟡 **cgo-required** — needs Fyne canvas harness; see `mem://workflow/canvas-harness-starter.md` (Slice #180).

| #   | Criterion                                                                                       | How verified                       |
|-----|-------------------------------------------------------------------------------------------------|------------------------------------|
| X-01 | Status dot is decorative; `statusLabel` carries semantic state for screen readers.              | Manual a11y audit + golden snapshot |
| X-02 | All buttons have visible text (no icon-only).                                                   | AST scan asserts `SetText` per button |
| X-03 | Card hyperlinks have aria-label "Open `{url}` in browser (opens incognito)".                    | Snapshot test of card aria attrs   |
| X-04 | Reduced motion (`fyne.CurrentApp().Settings().ReducedMotion()`) disables pulse animations.      | `WatchVM_ReducedMotion_NoPulse`    |
| X-05 | Focus order: alias picker → start/stop → tabs → toolbar → list. Predictable Tab navigation.     | Manual + snapshot of focus chain   |

---

## 14. Sign-off

**Sandbox:** ⚪ **N/A** — manual sign-off checklist; no automated gate.

A merge into `main` requires **all** of the following on the PR:

- [ ] CI green: every test in §1–§13 passes; every benchmark within budget.
- [ ] `golangci-lint run` clean (exhaustive, forbidigo, no-empty-interface, fn-length).
- [ ] `goleak.VerifyNone(t)` passes in `TestMain` of `internal/watcher`, `internal/core`, `internal/ui/views`.
- [ ] **Heartbeat regression sentinel** (H-01, H-02, H-04) green — single-handedly blocks merge.
- [ ] Code owner review by Watch lead.
- [ ] Security lead sign-off on §9 (PII redaction).
- [ ] Spec deltas (if any) merged to `00-overview.md` / `01-backend.md` / `02-frontend.md` in the **same** PR.

> Any criterion failing = build is **not shippable**, even if all others pass. There are no "soft" criteria here.

---

**End of `05-watch/97-acceptance-criteria.md`**
