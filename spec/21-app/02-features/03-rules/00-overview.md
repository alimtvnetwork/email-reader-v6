# Feature 03 — Rules

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI + CLI (`rules list/enable/disable`)

---

## Purpose

Manage regex rules that decide which incoming emails trigger Chrome-incognito link opens. The CLI already implements list/enable/disable; the UI adds an inline add/edit form and a live-toggle table.

## User stories

- I see every rule in a table with Name, Enabled toggle, From regex, Subject regex, URL regex.
- I flip the toggle and the change persists to `config.json` immediately.
- I click "+ Add rule" and an inline form appears in the same pane (not a modal). I fill it and submit; the table reloads.
- I click a rule's "Delete" button → an inline confirm strip ("Type the name to confirm: [____] [Delete]").

## Layout

```
┌─ Rules ──────────────────────────────────────────┐
│ [+ Add rule]                                     │
│ ┌─────────────────────────────────────────────┐  │
│ │ ✓ open-magic-links   noreply@.*  sign.?in   │  │
│ │ ✗ disabled-rule      alice@.*    .*          │  │
│ │ ✓ another            .*          token=...   │  │
│ └─────────────────────────────────────────────┘  │
│                                                   │
│  [inline form appears here when adding/editing]  │
└───────────────────────────────────────────────────┘
```

## Backend (core API)

`internal/core/rules.go`:

```go
func ListRules(ctx context.Context) ([]Rule, error)
func AddRule(ctx context.Context, r Rule) error           // validates regex compile
func UpdateRule(ctx context.Context, name string, r Rule) error
func RemoveRule(ctx context.Context, name string) error
func SetRuleEnabled(ctx context.Context, name string, enabled bool) error
```

`AddRule` and `UpdateRule` MUST `regexp.Compile` every pattern before persisting; on failure return a typed error so the UI shows an inline field-level message.

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-R1 | Toggling Enabled writes `config.json` and the next watch poll respects the new state without restart. |
| AC-R2 | Submitting an invalid regex highlights the offending field in red with the compiler error message. |
| AC-R3 | Rule names are unique; submitting a duplicate name shows an inline error and does not write. |
| AC-R4 | Deleting a rule requires typing the rule name to confirm (no double-click trap). |
| AC-R5 | The CLI `rules list` and the UI table show identical rows immediately after any mutation. |
