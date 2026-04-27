# 07 — `0 enabled rules` blocked incognito open even when browser was ready

**Status:** solved in v0.17.0
**Severity:** High
**Area:** 03-rules
**Opened:** 2026-04-22
**Resolved:** 2026-04-22
**Spec links:** [../../02-features/03-rules/](../../02-features/03-rules/), [../../02-features/05-watch/](../../02-features/05-watch/), [../../02-features/03-rules/01-backend.md](../../02-features/03-rules/01-backend.md)
**Source:** `.lovable/solved-issues/07-zero-rules-default-seed.md`

---

## Symptom

```
[admin] browser ready: /Applications/Google Chrome.app/.../Google Chrome (incognito flag="--incognito")
[admin] ✉ 1 new message(s)
    rules: 0 enabled rules — nothing to evaluate (add one in data/config.json)
```

User saw the email arrive, the file saved with the new readable name (per issue 06), but no link opened in incognito. Cause: `data/config.json` had `Rules: []`, and we required the user to hand-edit JSON to add one.

This was a regression of UX, not of code: issue 05 fixed the silent-failure problem by *telling* the user "0 enabled rules", but didn't give them an easy way to fix it.

## Root cause

- The "happy path" required hand-editing JSON — but the project's stated DX target is *zero JSON editing* for first-run.
- No CLI command existed to add a rule non-interactively.
- No bootstrap defaults existed; an empty `Rules: []` was treated as a deliberate user choice rather than an unconfigured state.

## Fix (v0.17.0)

### 1. Auto-seed `default-open-any-url`

Both `runWatch` and `runRead` (CLI) now seed a default rule when `countEnabledRules(cfg.Rules) == 0`:

| Field | Value |
|-------|-------|
| `Name` | `default-open-any-url` |
| `UrlRegex` | `https?://[^\s<>"'\)\]]+` |
| `FromRegex` | (empty — match any sender) |
| `SubjectRegex` | (empty) |
| `BodyRegex` | (empty) |
| `Enabled` | `true` |

The seed writes back to `config.json` atomically (tmp + fsync + rename, see Settings backend) so the file becomes self-documenting.

### 2. `rules add` non-interactive command

For custom rules:

```
email-read rules add --name X --url-regex '...'
```

…with optional `--from-regex`, `--subject-regex`, `--body-regex`, `--disabled`.

### 3. Clear seed log line

Logged a clear line so the user knows the seed happened and where the file lives:

```
ℹ no enabled rules found — seeded default rule "default-open-any-url" in /path/to/data/config.json
```

### 4. Files

- `internal/cli/cli.go` — seed before building engine in `runWatch`.
- `internal/cli/read.go` — same seed in `runRead`.
- `internal/cli/rules_export.go` — `countEnabledRules` helper + `rules add`.
- `cmd/email-read/main.go` — version → `0.17.0`.

## Spec encoding

- `spec/21-app/02-features/03-rules/01-backend.md` §Default seed — formalises the seed regex, the trigger condition (`countEnabledRules == 0`), and the persistence requirement.
- `spec/21-app/02-features/03-rules/02-frontend.md` §Empty state — the GUI rules list shows a one-click "Seed default rule" CTA when the list is empty, mirroring the CLI behaviour.
- `spec/21-app/02-features/05-watch/01-backend.md` §Pre-flight checks — the rule-count check at watcher startup now distinguishes "0 rules and seed disabled" from "0 rules and seed allowed but failed to write" (the latter is `ER-RUL-21305` / `ErrRuleSeedDefault`). *(Slice #157: corrected from `ER-RUL-21260` — that number falls in the MAIL block `21200–21299`, not Rules `21300–21399`. The seed-failure semantic is already covered by the existing registered code `ER-RUL-21305 ErrRuleSeedDefault` in `internal/errtrace/codes.yaml`; no new registry row needed.)*

## What NOT to repeat

- **Do not** require JSON hand-editing for first-run UX. If a feature has a sensible default, seed it on first use (and tell the user, in plain text, that you did).
- **Do not** rely on hand-editing for *any* config that has a CLI/GUI alternative. Either provide the CLI subcommand or add the GUI form — preferably both.
- **Do** treat an empty config array as "unconfigured" until the user has explicitly disabled the seed (a future `BootstrapDefaults: false` setting, currently always `true`).

## Iteration count

1 — designed in a single round once the issue 05 + 06 fixes exposed the gap.
