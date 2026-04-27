# 01 — Sandbox lacks Go toolchain

## Description
The Lovable sandbox has no preinstalled `go` binary on `$PATH`, so the
canonical `go test ./...` command fails immediately. CI runs that need
to verify Go code cannot use the standard invocation.

## Impact
Every verification step in this project (`go vet`, `go test`,
`go build`) requires a different command form. AI sessions that don't
know this waste tool calls discovering it.

## Workaround (current)
Use `nix run nixpkgs#go -- <args>`:

```bash
nix run nixpkgs#go -- vet -tags nofyne ./...
nix run nixpkgs#go -- test -tags nofyne ./...
nix run nixpkgs#go -- test -tags nofyne -race -count=2 ./...
```

The `-tags nofyne` flag is mandatory in this sandbox because the Fyne UI
needs cgo + a display server (see issue 03).

## Status
🔄 Workaround in place — codified in `mem://go-verification-path` and
`mem://workflow/01-status`. Not blocking work.
