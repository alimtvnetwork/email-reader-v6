# Acceptance Criteria — 21-app

**Version:** 1.0.0
**Updated:** 2026-04-25

---

## Overview

Consolidated, testable acceptance criteria across all features. Each row links to the originating feature spec for context.

---

## Dashboard (D)

| # | Criterion | Source |
|---|-----------|--------|
| AC-D1 | Dashboard renders within 100ms of cold open on a 10k-email DB. | `02-features/01-dashboard/00-overview.md` |
| AC-D2 | Counts refresh every 5s while visible. | `02-features/01-dashboard/00-overview.md` |
| AC-D3 | Recent-events list shows latest 5 events newest-first. | `02-features/01-dashboard/00-overview.md` |
| AC-D4 | "▶ Start" disabled when zero accounts configured. | `02-features/01-dashboard/00-overview.md` |
| AC-D5 | Clicking a count card navigates to the matching view, preserving alias. | `02-features/01-dashboard/00-overview.md` |

## Emails (E)

| # | Criterion | Source |
|---|-----------|--------|
| AC-E1 | Email list virtualizes — 50k rows scroll smoothly. | `02-features/02-emails/00-overview.md` |
| AC-E2 | Body renders plain text first; HTML fallback to stripped text. | `02-features/02-emails/00-overview.md` |
| AC-E3 | Search filters list within 200ms (debounced). | `02-features/02-emails/00-overview.md` |
| AC-E4 | Link button label = URL truncated to 60 chars; tooltip = full URL. | `02-features/02-emails/00-overview.md` |
| AC-E5 | Manual link open records `RuleName = "manual"` in `OpenedUrls`. | `02-features/02-emails/00-overview.md` |
| AC-E6 | Switching alias clears the detail pane to a placeholder. | `02-features/02-emails/00-overview.md` |

## Rules (R)

| # | Criterion | Source |
|---|-----------|--------|
| AC-R1 | Toggling Enabled persists immediately and the next poll respects it. | `02-features/03-rules/00-overview.md` |
| AC-R2 | Invalid regex highlights the field with the compiler error. | `02-features/03-rules/00-overview.md` |
| AC-R3 | Duplicate rule name rejected with inline error. | `02-features/03-rules/00-overview.md` |
| AC-R4 | Delete requires typing the rule name to confirm. | `02-features/03-rules/00-overview.md` |
| AC-R5 | CLI `rules list` and UI table show identical rows after any mutation. | `02-features/03-rules/00-overview.md` |

## Accounts (A)

| # | Criterion | Source |
|---|-----------|--------|
| AC-A1 | Email input auto-fills host/port/TLS via `imapdef`. | `02-features/04-accounts/00-overview.md` |
| AC-A2 | Wrong-password submit shows IMAP error inline; nothing written. | `02-features/04-accounts/00-overview.md` |
| AC-A3 | Account removal clears its `WatchState` rows. | `02-features/04-accounts/00-overview.md` |
| AC-A4 | Sidebar picker reflects add/remove within 1s. | `02-features/04-accounts/00-overview.md` |
| AC-A5 | Password field masked; never logged. | `02-features/04-accounts/00-overview.md` |

## Watch (W)

| # | Criterion | Source |
|---|-----------|--------|
| AC-W1 | Test email produces a card + raw log line within 5s. | `02-features/05-watch/00-overview.md` |
| AC-W2 | Stop completes IMAP LOGOUT in 2s and flips status to idle. | `02-features/05-watch/00-overview.md` |
| AC-W3 | Switching alias while watching shows a confirm strip. | `02-features/05-watch/00-overview.md` |
| AC-W4 | Heartbeat lines `messages=N uidNext=M` always present. | `02-features/05-watch/00-overview.md` |
| AC-W5 | Card link click records `RuleName = "manual"`. | `02-features/05-watch/00-overview.md` |
| AC-W6 | Connection drop → status `error`, error card, exponential backoff retry. | `02-features/05-watch/00-overview.md` |

## Tools (T)

| # | Criterion | Source |
|---|-----------|--------|
| AC-T1 | Tool output streams (not batched). | `02-features/06-tools/00-overview.md` |
| AC-T2 | Export CSV defaults to `./data/export-<ts>.csv`. | `02-features/06-tools/00-overview.md` |
| AC-T3 | Diagnose halts at first failed step. | `02-features/06-tools/00-overview.md` |
| AC-T4 | Running tool does not block UI; button becomes "Cancel". | `02-features/06-tools/00-overview.md` |
| AC-T5 | Diagnose result cached 60s per alias. | `02-features/06-tools/00-overview.md` |

## Settings (S)

| # | Criterion | Source |
|---|-----------|--------|
| AC-S1 | Poll interval outside 1–60 shows inline error. | `02-features/07-settings/00-overview.md` |
| AC-S2 | Theme switch applies immediately without restart. | `02-features/07-settings/00-overview.md` |
| AC-S3 | "Detect" shows resolved Chrome path + match source. | `02-features/07-settings/00-overview.md` |
| AC-S4 | Path-display rows open OS file manager. | `02-features/07-settings/00-overview.md` |
| AC-S5 | New poll interval honored on next cycle without restart. | `02-features/07-settings/00-overview.md` |

---

## Total

35 testable criteria across 7 features.
