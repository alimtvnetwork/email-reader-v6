---
id: 20260427-160000-errlog-persist-nil-writer-race
title: errlog Persistence panics on `-race -count=2` (nil bufio.Writer)
status: open
severity: medium
discovered_by: Slice #189 (canonical race-stress sweep `go test -tags nofyne -race -count=2 ./...`)
discovered_at: 2026-04-27
---

## Symptom

Running the canonical race-stress invocation:

```bash
nix run nixpkgs#go -- test -tags nofyne -race -count=2 ./...
```

…fails in `internal/ui/errlog` with a nil-pointer panic on the second
iteration:

```
panic: runtime error: invalid memory address or nil pointer dereference
bufio.(*Writer).Write(0x0, …)
github.com/lovable/email-read/internal/ui/errlog.(*Persistence).Write(…)
  /dev-server/internal/ui/errlog/persist.go:97 +0x172
github.com/lovable/email-read/internal/ui/errlog.TestReportError_FillsTraceFromErrtrace(…)
  /dev-server/internal/ui/errlog/errlog_test.go:82 +0x2d9
```

`count=1` passes; `count=2` reliably crashes. Smells like package-level
state (a `*Persistence` or its `*bufio.Writer`) surviving across the
two test runs without being re-initialised — first run closes the file
and nils the writer, second run reuses the stale handle.

## Suggested triage

1. Audit `internal/ui/errlog/persist.go` and `errlog.go` for any
   package-level singletons or once.Do guards that wouldn't re-arm
   between test iterations.
2. Add a `reset` test helper (already used by similar packages) and
   call it from `TestMain` / `t.Cleanup`.
3. Re-run `-race -count=2` to confirm green.

## Out of scope for the discovering slice

Slice #189's job was the race sweep itself, not the bug. Filing here so
the next `next` command can pick it up as a focused fix slice.
