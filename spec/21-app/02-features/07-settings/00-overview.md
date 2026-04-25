# Feature 07 — Settings

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI only

---

## Purpose

Edit the parts of `config.json` that are not per-account or per-rule (paths, watch interval, browser overrides, theme). Replaces hand-editing JSON.

## Fields

| Field | Source | Default | Notes |
|---|---|---|---|
| Config path | read-only | `data/config.json` | display only, opens folder on click |
| Data dir | read-only | `data/` | display only |
| Email archive dir | read-only | `email/` | display only |
| Poll interval (s) | `watch.pollSeconds` | `3` | min 1, max 60 |
| Chrome path override | `browser.chromePath` | `""` | file picker; "Detect" button reruns auto-detect |
| Incognito flag override | `browser.incognitoArg` | `""` | text input; placeholder shows the auto-pick |
| Theme | UI state | `dark` | `dark` / `light` / `system` |
| UI state file | read-only | `data/ui-state.json` | display only |

A "Reset to defaults" button restores poll interval, browser overrides, and theme (does NOT touch accounts/rules).

## Layout

Single vertical form, grouped sections (Paths, Watcher, Browser, Appearance). "Save" button bottom-right, "Reset to defaults" bottom-left. Save is disabled until any field is dirty.

## Backend (core API)

New, narrow:

```go
func GetSettings(ctx context.Context) (Settings, error)
func SaveSettings(ctx context.Context, s Settings) error    // validates ranges
func DetectChrome() (path string, source string, err error) // path + how it was found
```

`SaveSettings` MUST NOT touch `accounts` or `rules` arrays in `config.json` — only the named scalar fields above.

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-S1 | Saving a poll interval outside 1–60 shows an inline range error and does not write. |
| AC-S2 | Theme switch applies immediately without restart. |
| AC-S3 | "Detect" button shows the resolved Chrome path and which source matched (config / env / OS default / PATH). |
| AC-S4 | Clicking a path-display row opens the OS file manager at that location. |
| AC-S5 | After Save, all open Watch tabs respect the new poll interval on the next cycle. |
