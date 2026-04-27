# 05 — No bench/perf infra (blocks AC-DBP / AC-SP)

## Description
Performance acceptance rows (AC-DBP-01..06 for database p95, AC-SP-01..05
for settings p95) require a stable benchmark harness with goroutine-leak
detection (`goleak`) and reproducible perf gates. The sandbox has no
such infrastructure.

## Impact
- 11 acceptance rows blocked (6 AC-DBP + 5 AC-SP).
- Bucket #9 in `mem://workflow/progress-tracker` stays at ~70% pending
  this infra.

## Workaround
None for closing the rows; they remain in `coverageGapAllowlist` with a
documented blocker note.

Per-feature benches that DO exist (Dashboard.Summary, Emails.List/MarkRead,
Rules, Accounts, Watch) provide spot coverage but don't satisfy p95 gates.

## Status
🚫 Blocked on bench infra. Tracked as deferred backlog item.
