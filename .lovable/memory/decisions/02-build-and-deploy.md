# Build & deploy decisions

## `run.ps1` design
One-command bootstrap on Windows. Steps it performs in order:

1. `git pull --ff-only` (skippable with `-SkipPull` switch — useful for offline rebuilds).
2. `go build -o email-reader-cli/email-read.exe ./cmd/email-read`.
3. Ensure `email-reader-cli/data/` and `email-reader-cli/email/` exist.
4. Idempotently add the `email-reader-cli/` directory to the **User PATH** via `[Environment]::SetEnvironmentVariable('Path', ..., 'User')`.
5. Update `$env:Path` in the current session so the just-installed `email-read` is callable immediately (though new terminal windows still need to be reopened to inherit the persisted change).

### Why User PATH (not Machine PATH)
- No admin elevation required.
- Per-user install matches the personal-tool nature of the CLI.

### Idempotency check
- Splits current User PATH on `;`, checks if the target dir is already present (case-insensitive), only appends when missing. Safe to run repeatedly.

## Version bump policy
- User preference: **every code change bumps at least the minor version** of the CLI.
- The single source of truth is the `Version` constant in `cmd/email-read/main.go`.
- Current: `0.8.0` (after Step 10 README work).

## Why no sandbox build
- The Lovable sandbox has no Go toolchain by default and the spec is explicit: builds happen on the user's Windows machine. We do not attempt `go build` here — we only write source.
