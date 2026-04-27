---
name: Error stack traces convention
description: errtrace.{Wrap,Wrapf,New,Errorf} is mandatory (3 lints in fail mode); UI errors flow through errlog.ReportError → ring buffer → JSONL persistence + errors tail CLI.
type: preference
---
# Error stack trace convention

Every error return in production code goes through `internal/errtrace`,
and every UI-surface error is additionally captured into the
`internal/ui/errlog` ring so the user can review traces in
**Diagnostics → Error Log** (or via `email-read errors tail`).

> **Status:** Phase 4.4 of the error-trace logging upgrade is shipped.
> All three guardrail linters run with `LINT_MODE=fail` in `run.sh` /
> `run.ps1` and CI; any new violation breaks the build.
> Spec: [`spec/03-error-manage/04-app-error-log-view/00-overview.md`](mem://../../spec/03-error-manage/04-app-error-log-view/00-overview.md).
> Rollout log: `mem://features/02-error-trace-rollout`.

## Rules (enforced)

1. **No `fmt.Errorf`** for production error returns. Use
   `errtrace.Wrap(err, "msg")` (cause + context) or
   `errtrace.Wrapf(err, "msg %s", x)` (formatted context).
   Enforced by `linter-scripts/check-no-fmt-errorf.sh`.
2. **No bare `return err`** at a package boundary — always wrap with
   one short context message so the trace gets a frame at the surface
   that observed the failure. `errtrace.Wrap` is nil-safe, so success
   paths stay nil. Enforced by
   `linter-scripts/check-no-bare-return-err.sh`.
3. **No `errors.New`** in production code. Use `errtrace.New("msg")`
   for sentinel / no-cause errors. Do not pass a pre-formatted message
   via `Wrap(nil, ...)` — `Wrap` returns nil on nil err. Enforced by
   `linter-scripts/check-no-errors-new.sh`.
4. **`errors.Is` / `errors.As` still work** — `errtrace.Traced` uses
   `%w`-style `Unwrap`. Wrapping does not break sentinel matching.
5. **Format only at the edges.** The only places that call
   `errtrace.Format(err)` are:
   - `cmd/email-read/main.go` (stderr on exit)
   - the watcher per-poll / per-message loggers
   - `internal/ui/errlog/errlog.go::ReportError` (when populating
     `Entry.Trace`)

## UI surface rule (Phase 3+)

Every UI handler that observes an error must call:

```go
errlog.ReportError(component, err)
```

before (or instead of) surfacing it in a status banner. This is what
populates the ring buffer that drives the sidebar badge, the
first-error toast, the persisted `data/error-log.jsonl`, and the
`email-read errors tail` CLI. `component` is a short tag like
`"emails"`, `"watcher"`, `"settings"` — free-form but kept short
because the view renders it as a chip.

`ReportError` is nil-safe: passing a nil `err` is a no-op.

## Canonical examples

### Wrap with cause

```go
mc, err := mailclient.Dial(opts.Account)
if err != nil {
    return nil, errtrace.Wrap(err, "dial imap")
}
```

Output (rendered by `errtrace.Format`):

```
error: dial imap: imap login user@host: EOF
  at internal/watcher/watcher.go:122 (watcher.pollOnce)
  at internal/mailclient/mailclient.go:70 (mailclient.Dial)
```

### Sentinel / no-cause

```go
if name == "" {
    return errtrace.New("name required")
}
```

### UI handler — wrap + report

```go
if err := svc.LoadThread(uid); err != nil {
    werr := errtrace.Wrap(err, "loadThread")
    errlog.ReportError("emails", werr)
    return werr
}
```

The `Wrap` adds the surface frame; `ReportError` makes the entry
visible in **Diagnostics → Error Log** and writes it to
`data/error-log.jsonl`.

## Why this exists

The CLI ships as a binary to non-developers. They paste logs into bug
reports — we must know which call failed without rerunning anything.
Per-frame `file:line` is non-negotiable. Phase 4 added on-disk
persistence and the `errors tail` CLI so the user can forward the raw
trace chain even when the desktop UI isn't running.

## Related

- `mem://index.md` — Core line: errtrace mandatory; UI errors via `errlog.ReportError`.
- `mem://features/02-error-trace-rollout` — full slice-by-slice rollout log.
- `spec/03-error-manage/04-app-error-log-view/00-overview.md` — formal AC for the desktop surface (AC-ELV-01 … AC-ELV-12).
- `README.md` §9 "Reporting a bug" — user-facing instructions.
