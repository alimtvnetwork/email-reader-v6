---
name: Error stack traces convention
description: All error returns must use internal/errtrace.Wrap/Wrapf/New so logs show file:line of every wrap site.
type: preference
---
# Error stack trace convention

The CLI uses `internal/errtrace` for every error path so logs reveal the exact
file:line where an error was reported and every wrap site between origin and
top-level handler.

## Rules

1. Never use `fmt.Errorf("...: %w", err)` for production error returns.
   Use `errtrace.Wrap(err, "msg")` or `errtrace.Wrapf(err, "msg %s", x)`.
2. Never `return err` straight up at a package boundary — always wrap with
   one short context message so the trace has a frame.
3. For sentinel/no-cause errors use `errtrace.New("msg")`. Do not pass a
   pre-formatted message via `Wrapf(nil, ...)` — Wrap returns nil on nil err.
4. The only place that calls `errtrace.Format(err)` is `cmd/email-read/main.go`
   (printed to stderr on exit) and the watcher logger (per-poll error and
   per-message error logs).
5. `errors.Is` / `errors.As` continue to work because Traced uses `%w`-style
   Unwrap.

## Canonical example

```go
mc, err := mailclient.Dial(opts.Account)
if err != nil {
    return nil, errtrace.Wrap(err, "dial imap")
}
```

Output:
```
error: dial imap: imap login user@host: EOF
  at internal/watcher/watcher.go:122 (watcher.pollOnce)
  at internal/mailclient/mailclient.go:70 (mailclient.Dial)
```

## Why
User cannot debug from message-only errors. Per-frame file:line is required
because the CLI is shipped as a binary to non-developers — they paste logs and
we need to know which call failed without rerunning anything.
