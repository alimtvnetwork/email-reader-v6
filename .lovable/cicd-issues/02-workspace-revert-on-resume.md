# 02 — Workspace files revert between sessions

## Description
Files outside `.lovable/` may revert to a previous state between AI
sessions. This means an earlier session's edits to `internal/...` or
`spec/...` cannot be assumed to persist.

## Impact
Any "did the previous slice land?" assumption must be verified by
re-reading the file on disk. Memory files in `.lovable/` are durable;
project files are not guaranteed to be.

## Workaround
Always verify on disk with `code--view` or `grep` before assuming
earlier work persisted. Codified in `mem://workspace-revert-on-resume`.

## Status
🔄 Workaround in place. Sandbox infrastructure issue, not project bug.
