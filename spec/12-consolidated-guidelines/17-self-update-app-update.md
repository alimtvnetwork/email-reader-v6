# Consolidated: Self-Update & App Update — Complete Reference

**Version:** 3.2.0  
**Updated:** 2026-04-16  
**Source Module:** [`spec/14-self-update-app-update/`](../14-self-update-app-update/00-overview.md)

---

## Purpose

This is the **standalone consolidated reference** for CLI self-update functionality and release distribution. An AI reading only this file must be able to implement the complete update system without consulting source specs.

---

## Core Problem

A running binary **cannot overwrite itself** on Windows. The entire update architecture exists to work around this constraint using a rename-first strategy.

---

## Two Update Strategies

### Strategy 1: Source-Based (Build from Repo)

When the user has the source repository locally:

```
1. Resolve source repo path
2. Pull latest changes (git pull)
3. Resolve dependencies (go mod tidy)
4. Build binary (go build with ldflags)
5. Rename-first deploy
6. Verify version
7. Cleanup artifacts
```

### Strategy 2: Binary-Based (Download Pre-Built)

When no source repo is available:

```
1. Locate <binary>-updater on PATH
2. Execute: <binary>-updater --install-dir <path>
3. Updater queries GitHub API for latest version
4. Downloads install script
5. Executes install (handles download + verify + install)
```

The `<binary> update` command auto-detects which strategy to use based on whether a source repo path can be resolved.

---

## Deploy Path Resolution (3-Tier Priority)

| Priority | Source | When Used |
|----------|--------|-----------|
| 1 | CLI flag (`--deploy-path`) | Explicit override |
| 2 | Global PATH lookup | Binary already installed |
| 3 | Config file default | First-time install |

### Repo Path Resolution (5-Tier Priority)

| Priority | Method | Description |
|----------|--------|-------------|
| 1 | `--repo-path` flag | CLI flag passed by user |
| 2 | Embedded constant | Compiled into binary via `-ldflags -X` |
| 3 | Database lookup | Previously saved path from prior update |
| 4 | Interactive prompt | Ask user to provide the path |
| 5 | Updater fallback | Delegate to `<binary>-updater` (binary-based update) |

Each resolved repo path is **saved to the database** for future use.

### Special Cases

- Symlink resolution: Follow symlinks to find the real binary path
- Nested directory detection: `<binary>/<binary>.exe` pattern → resolve to parent
- `installed-dir` utility command: Print resolved install directory

---

## Rename-First Deploy — Complete Specification

Windows file lock workaround with rollback safety:

```
Step 1: Rename running <binary>.exe → <binary>.exe.old
Step 2: Copy new binary → <binary>.exe (destination now free)
Step 3: Verify new binary works (execute --version)
Step 4: If verify fails → rollback (rename .old back)
Step 5: If verify succeeds → .old is cleanup candidate
```

### Retry Logic

| Parameter | Value |
|-----------|-------|
| Max retries | 5 |
| Retry delay | 500ms |
| Retry applies to | Rename and copy operations |
| Previous approach | 20 retries with copy-only (replaced) |

### PATH Sync

When the deploy target differs from the PATH location, the script must sync:
1. Copy binary to deploy target
2. Update PATH to point to deploy target (if different from current PATH entry)

### Implementation Available In

| Platform | Script |
|----------|--------|
| PowerShell | `run.ps1 -Update` |
| Bash | `run.sh --update` |

---

## Build Scripts (Cross-Platform)

Both `run.ps1` and `run.sh` implement a 4-step pipeline:

```
[1/4] Pull latest changes
[2/4] Resolve dependencies (go mod tidy)
[3/4] Build binary (go build with ldflags)
[4/4] Deploy (rename-first)
```

### Key Features

| Feature | Detail |
|---------|--------|
| JSON config loading | Reads build config from `config.json` |
| Git pull | With conflict resolution (abort merge, force pull) |
| Source file validation | Checks `main.go` exists before building |
| Data folder copy | Copies `data/` to deploy directory |
| Colored logging | Consistent prefix scheme: `[1/4]`, `[OK]`, `[FAIL]` |
| -ldflags embedding | `Version`, `RepoPath`, `CommitSHA`, `BuildDate` |

---

## Handoff Mechanism (Windows Self-Replacement)

### Flow

```
1. Running binary detects Windows OS
2. Copies itself to <binary>-update-<PID>.exe (temp worker)
3. Launches worker with: <worker> update-runner --repo-path <path> --deploy-path <path>
4. Parent waits SYNCHRONOUSLY (cmd.Run(), NOT cmd.Start())
5. Worker generates temp PowerShell script
6. PowerShell script calls: run.ps1 -Update
7. run.ps1 performs rename-first deploy on the ORIGINAL binary path
8. Worker exits → parent resumes → prints result
```

### Critical Rules

- **Synchronous wait**: Parent uses `cmd.Run()` to block until worker completes
- **Output piped**: `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr` — user sees all output
- **PID-based naming**: Worker binary includes parent PID to avoid collisions

### Unix

No handoff needed — `bash run.sh --update` directly replaces the binary.

### Binary-Based Fallback

When no source repo is found:

```go
func tryUpdaterFallback() bool {
    updaterPath, err := exec.LookPath("<binary>-updater")
    if err != nil {
        return false  // updater not found on PATH
    }
    cmd := exec.Command(updaterPath, "run")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    return cmd.Run() == nil
}
```

---

## Updater Binary (`<binary>-updater`)

### Purpose

Separate Go module that downloads and installs updates when the main binary cannot replace itself.

### Module Structure

```
<binary>-updater/
├── go.mod                  # Independent module
├── main.go
├── cmd/root.go             # CLI command definitions
├── updater/
│   ├── download.go         # GitHub API + asset download
│   ├── install.go          # Rename-first deploy logic
│   └── verify.go           # Post-install version verification
├── version/version.go      # Embedded version constants
├── winres.json             # Windows icon/version resources
└── assets/icon-updater.png # Distinct icon from main binary
```

### CLI Interface

```
<binary>-updater [flags]
  --install-dir <path>    Target directory (required)
  --version <ver>         Specific version (default: latest)
  --repo <owner/repo>     GitHub repository (embedded at build time)
  --binary-name <name>    Name of binary to update (embedded)
  --skip-checksum         Skip SHA-256 verification (not recommended)
  --verbose               Enable detailed output
```

### Update Flow

```
1. Resolve target version (flag → GitHub API latest)
2. Construct download URL
3. Download binary archive + checksums.txt
4. Verify SHA-256 (unless --skip-checksum)
5. Rename-first deploy to install-dir
6. Verify: execute <binary> version
7. Print result
```

---

## Cleanup

Post-update artifact removal:

| Artifact | Location | Created By |
|----------|----------|-----------|
| `<binary>-update-<PID>.exe` | Same dir or temp dir | Handoff mechanism |
| `<binary>-update-*.ps1` | Temp dir | PowerShell script generation |
| `<binary>.exe.old` | Deploy directory | Rename-first strategy |

### Rules

- **Explicit `update-cleanup` subcommand** for manual cleanup
- **Auto-cleanup** after successful update (best-effort, never fails the update)
- **`.old` files are the rollback safety net** — never deleted during deploy, only after verification
- Cleanup scans both the binary directory and temp directory

---

## Release Assets

### Naming Convention

```
<binary>-<os>-<arch>.<ext>
```

| OS | Extension |
|----|-----------|
| Windows | `.zip` |
| Linux/macOS | `.tar.gz` |

### Standard Release (9 files)

| # | File | Description |
|---|------|-------------|
| 1-6 | `<binary>-{os}-{arch}.{ext}` | 6 compressed archives |
| 7 | `checksums.txt` | SHA-256 checksums of all archives |
| 8 | `install.ps1` | PowerShell installer |
| 9 | `install.sh` | Bash installer |

**Archive contents:** Single flat binary — no nested directories.

---

## Checksums & Verification

| Rule | Detail |
|------|--------|
| Algorithm | SHA-256 exclusively — never MD5/SHA-1 |
| Source | Generated from **compressed archives**, not raw binaries |
| Requirement | **Mandatory** — never optional or skippable |
| macOS fallback | `shasum -a 256` when `sha256sum` unavailable |
| Format | `sha256hash  filename` (two-space separator) |
| TOCTOU | Checksumming archives (not raw files) prevents time-of-check issues |

---

## Release Versioning

### Version Resolution (3-Tier)

| Priority | Source | Example |
|----------|--------|---------|
| 1 | Explicit argument | `--version 1.3.0` |
| 2 | Bump flag | `--bump minor` |
| 3 | Current source constant | Read from `version.go` |

### Rules

- SemVer 2.0.0 with `v` prefix normalization (`v1.2.0` → `1.2.0`)
- Atomic version sync: source constant + CHANGELOG + metadata in same commit
- Release branch strategy: `main → release/x.y.z → tag → merge back`
- Optional `latest.json` for programmatic version queries

```json
{
  "Version": "1.3.0",
  "ReleasedAt": "2026-04-16T12:00:00Z",
  "DownloadUrl": "https://github.com/.../releases/download/v1.3.0/"
}
```

---

## Cross-Compilation

### 6-Target Build Matrix

| OS | Arch | Binary Name |
|----|------|-------------|
| windows | amd64 | `<binary>-windows-amd64.exe` |
| windows | arm64 | `<binary>-windows-arm64.exe` |
| linux | amd64 | `<binary>-linux-amd64` |
| linux | arm64 | `<binary>-linux-arm64` |
| darwin | amd64 | `<binary>-darwin-amd64` |
| darwin | arm64 | `<binary>-darwin-arm64` |

### Build Rules

| Rule | Detail |
|------|--------|
| `CGO_ENABLED=0` | Fully static binaries, no C dependencies |
| `-ldflags -X` | Embeds Version, RepoPath, CommitSHA, BuildDate |
| `.exe` extension | Mandatory for Windows only |
| Output directory | All binaries to `dist/` — single source of truth |
| Never rebuild | All packaging/publishing operates on existing artifacts |

---

## Network Requirements

| Requirement | Detail |
|-------------|--------|
| Connectivity check | `requireOnline()` before any network operation |
| GitHub API | Used for version queries and release downloads |
| Timeout | 30s for API calls, 5m for binary downloads |
| Retry | 3 attempts with exponential backoff for downloads |

---

## Config File

Update configuration stored in a config file:

```json
{
  "RepoPath": "/home/user/projects/my-cli",
  "DeployPath": "/usr/local/bin",
  "LastUpdateCheck": "2026-04-16T12:00:00Z",
  "AutoUpdate": false
}
```

---

## Update Command Workflow Summary

### `<binary> update`

```
1. requireOnline() — verify internet
2. resolveRepoPath() — 5-tier priority cascade
3. If source found → source-based update
4. If no source → try updater fallback
5. If no updater → error and exit
```

### `<binary> update-cleanup`

```
1. Scan binary dir for *.old files
2. Scan temp dir for <binary>-update-* files
3. Delete all found artifacts
4. Report results
```

---

## Key Rules

1. ALL self-update content belongs in `spec/14-self-update-app-update/`
2. CI/CD pipeline specs (GitHub Actions) belong in `spec/13-cicd-pipeline-workflows/`
3. Rename-first is mandatory on Windows — never attempt to overwrite a running binary
4. Updates are always synchronous — user sees all output in the terminal
5. Never leave the system without a working binary — always rollback on failure
6. Binaries are built exactly once — no stage may trigger a rebuild
7. Checksum verification is mandatory — never optional or skippable
8. `.old` files are rollback safety — never delete during deploy

---

## File Inventory

| # | File | Description |
|---|------|-------------|
| 01 | `01-self-update-overview.md` | Problem statement, strategies, command flow |
| 02 | `02-deploy-path-resolution.md` | 3-tier deploy target resolution |
| 03 | `03-rename-first-deploy.md` | Rename-first with retry and rollback |
| 04 | `04-build-scripts.md` | Cross-platform build scripts |
| 05 | `05-handoff-mechanism.md` | Windows self-replacement flow |
| 06 | `06-cleanup.md` | Post-update artifact removal |
| 07 | `07-release-assets.md` | Asset naming and packaging |
| 08 | `08-checksums-verification.md` | SHA-256 generation and verification |
| 09 | `09-release-versioning.md` | Version resolution, tagging, changelog |
| 10 | `10-cross-compilation.md` | 6-target static build matrix |
| 11 | `11-release-pipeline.md` | End-to-end CI/CD release workflow |
| 12 | `12-install-scripts.md` | One-liner installer generation |
| 13 | `13-updater-binary.md` | Standalone updater binary architecture |
| 14 | `14-network-requirements.md` | Connectivity checks, timeouts, retry |
| 15 | `15-config-file.md` | Update configuration storage |
| 16 | `16-update-command-workflow.md` | Step-by-step update/cleanup commands |

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Full Source | `../14-self-update-app-update/00-overview.md` |
| CI/CD Pipeline Workflows | `../13-cicd-pipeline-workflows/00-overview.md` |
| Install Script Generation | `../13-cicd-pipeline-workflows/04-install-script-generation.md` |
| Code Signing | `../13-cicd-pipeline-workflows/05-code-signing.md` |
| Installation Flow | `../13-cicd-pipeline-workflows/08-installation-flow.md` |

---

*Consolidated self-update & app update — v3.2.0 — 2026-04-16*
