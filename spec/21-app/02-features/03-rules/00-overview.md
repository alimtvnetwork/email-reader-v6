# 03 — Rules — Overview

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **Rules** feature is the automation surface of `email-read`. A rule is a **named, regex-based predicate over an email** plus an **action** (open URL, mark read, tag). The watcher evaluates enabled rules against every newly-stored email and the UI surfaces matches in the Emails view via `MatchedRules`.

This feature owns: rule CRUD, validation (regex compile-check at save time), enable/disable toggle, dry-run against a sample email, and per-rule hit counters.

Cross-references:
- Architecture: [`../../07-architecture.md`](../../07-architecture.md) §4.3
- Coding standards: [`../../04-coding-standards.md`](../../04-coding-standards.md)
- Logging: [`../../05-logging-strategy.md`](../../05-logging-strategy.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21300–21399
- Guidelines: `spec/12-consolidated-guidelines/13-app.md`, `16-app-design-system-and-ui.md`

---

## 1. Scope

### In scope
1. List all configured rules (enabled + disabled), sorted by user-defined `Order`.
2. Create a rule: `Name`, `FromRegex`, `SubjectRegex`, `BodyRegex`, `UrlRegex`, `Enabled`, `Action`, `Order`.
3. Update a rule by `Name` (rename via separate `Rename` op).
4. Delete a rule (hard delete from `config.json`).
5. Enable / disable toggle (one-click, no save dialog).
6. Reorder rules (drag handle in UI; first match wins for `MarkRead` / `Tag` actions; `OpenUrl` actions are union, see §4.2).
7. **Dry-run**: evaluate one rule against a user-provided `EmailSample` and show what would match, without writing any state.
8. **Hit counter**: per-rule `LastMatchedAt` + `MatchCount` (persisted in DB, read-only in UI).
9. Regex syntax validation at save time — never persist a rule with an uncompilable pattern.

### Out of scope
- Conditional logic (AND/OR/NOT trees beyond implicit AND between regex fields). Deferred to v2.
- Time-of-day or sender-domain whitelists as first-class fields (use `FromRegex`).
- Server-side rule push (IMAP `SIEVE`). Out of project scope.
- Per-account rule scoping (`Alias` filter). Deferred to v2; all rules currently apply globally.

---

## 2. User Stories

| #  | As a … | I want to …                                                          | So that …                                                  |
|----|--------|----------------------------------------------------------------------|------------------------------------------------------------|
| 1  | User   | see all my rules at a glance with their enabled state                | I can audit what is firing                                 |
| 2  | User   | add a new rule by filling in regex fields                            | I can automate handling of a new sender                    |
| 3  | User   | get an immediate error when my regex is invalid                      | I am not told later by the watcher logs                    |
| 4  | User   | disable a rule without deleting it                                   | I can pause a rule while debugging                         |
| 5  | User   | reorder rules                                                        | I can control which mark-read/tag rule wins ties           |
| 6  | User   | dry-run a rule against a sample email                                | I can verify the regexes match what I expect               |
| 7  | User   | see when a rule last matched and how many times                      | I know which rules are still useful                        |
| 8  | User   | delete a rule entirely                                               | I can clean up obsolete automation                         |
| 9  | User   | rename a rule                                                        | The hit-count history follows it                           |

---

## 3. Dependencies

| Dependency             | Why                                                                  |
|------------------------|----------------------------------------------------------------------|
| `core.Rules`           | All CRUD + dry-run                                                   |
| `internal/rules`       | (transitive) regex engine — `Engine.New`, `Engine.Evaluate`          |
| `internal/config`      | (transitive) reads/writes `config.json` `Rules[]`                    |
| `internal/store`       | (transitive) reads/writes `RuleStat` table for hit counters          |
| `core.Watch`           | Subscribes to `WatchEvent.Kind == RuleMatched` for live counter bump |
| `internal/ui/theme`    | Tokens for enabled/disabled badge, danger (delete confirmation)      |

The view **must not** import `internal/rules`, `internal/store`, or `internal/config` directly.

---

## 4. Data Model

All names PascalCase (per `04-coding-standards.md` §1.1).

### 4.1 Core types

```go
type Rule struct {
    Name         string       // unique, case-sensitive identifier
    Order        int          // ascending; ties broken by Name asc
    Enabled      bool
    FromRegex    string       // empty = match any
    SubjectRegex string       // empty = match any
    BodyRegex    string       // empty = match any
    UrlRegex     string       // empty = no URL extraction
    Action       RuleAction
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type RuleSpec struct {           // input shape for Create/Update
    Name         string
    Order        int
    Enabled      bool
    FromRegex    string
    SubjectRegex string
    BodyRegex    string
    UrlRegex     string
    Action       RuleAction
}

type RuleAction string  // PascalCase enum
const (
    RuleActionOpenUrl  RuleAction = "OpenUrl"
    RuleActionMarkRead RuleAction = "MarkRead"
    RuleActionTag      RuleAction = "Tag"      // tag name = rule name
)

type RuleId string                 // currently == Rule.Name; reserved for future numeric ID

type EmailSample struct {           // input for DryRun
    FromAddr  string
    Subject   string
    BodyText  string
}

type RuleMatch struct {
    RuleName     string
    Matched      bool
    MatchedFromAddr  bool
    MatchedSubject   bool
    MatchedBody      bool
    ExtractedUrls    []string      // only when Action == OpenUrl
    EvaluatedAt      time.Time
    DurationMicro    int           // perf telemetry for the dry-run
}

type RuleStat struct {              // persisted projection
    RuleName       string
    LastMatchedAt  time.Time
    MatchCount     int64
}
```

### 4.2 Action semantics

| Action       | When `Matched == true`, the watcher will …                                              |
|--------------|------------------------------------------------------------------------------------------|
| `OpenUrl`    | For every `ExtractedUrls[i]`, call `core.Tools.OpenUrl` and write to `OpenedUrl` table. **Union across rules** (every matching rule contributes URLs; deduped per `(EmailId, Url)` by the existing unique index `IxOpenedUrlsUnique`). |
| `MarkRead`   | Single `UPDATE Email SET IsRead = 1 …`. **First-match-wins** by `Order` — only the lowest-Order matching `MarkRead` rule fires. |
| `Tag`        | Insert into `EmailTag(EmailId, TagName)` (new table — see §5.2). **First-match-wins**. |

### 4.3 Default values

```go
RuleSpec{
    Order:   max(existing.Order)+10,   // gaps for easy reordering
    Enabled: true,
    Action:  RuleActionOpenUrl,
}
```

---

## 5. Refresh & Live-Update

| Trigger                                              | Action                                                  |
|------------------------------------------------------|---------------------------------------------------------|
| Tab opened                                           | `core.Rules.List` once                                  |
| Rule created / updated / deleted                     | Optimistic UI update; server-confirmed re-list on success |
| Toggle Enabled                                       | Optimistic flip; rollback on error                      |
| Drag-reorder row                                     | Optimistic reorder; persist on drop (debounced 300 ms)  |
| `WatchEvent.Kind == RuleMatched` for visible rule    | Increment `MatchCount` in-place; bump `LastMatchedAt`   |
| Tab loses focus                                      | Unsubscribe from `WatchEvent` channel                   |

The Rules view never executes a rule itself — only `core.Watch` evaluates rules in production. The view's **DryRun** path is read-only and writes nothing to the DB.

---

## 6. Acceptance Snapshot (full criteria in `97-acceptance-criteria.md`)

A merged Rules build is shippable iff:

1. `List` returns ≤ **20 ms** with 200 rules.
2. `Create` rejects an uncompilable regex synchronously with code `21302 RuleInvalidRegex` and field-level error highlighting in the UI.
3. `DryRun` of one rule against a 100 KB body completes in ≤ **15 ms**.
4. Toggling Enabled is **idempotent** (re-issue is a no-op, not a 2× write).
5. Deleting a rule removes it from `config.json` **and** drops the corresponding `RuleStat` row in the same transaction.
6. Reorder is a single transaction — concurrent reorder + create cannot interleave to break `Order` uniqueness invariant ("ascending integers, no duplicates").
7. `MatchedRules` projection in Emails view stays consistent: a rule rename updates Emails badges within one `List` refresh.
8. Zero `interface{}` / `any` in any new code (lint-enforced).

---

## 7. Open Questions

None. Confidence: Production-Ready.

---

**End of `03-rules/00-overview.md`**
