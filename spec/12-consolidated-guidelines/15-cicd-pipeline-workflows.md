# Consolidated: CI/CD Pipeline Workflows — Complete Reference

**Version:** 3.2.0  
**Updated:** 2026-04-16  
**Source Module:** [`spec/13-cicd-pipeline-workflows/`](../13-cicd-pipeline-workflows/00-overview.md)

---

## Purpose

This is the **standalone consolidated reference** for all CI/CD pipeline specifications. An AI reading only this file must be able to implement any pipeline described here without consulting source specs.

---

## Platform & Shared Conventions

| Convention | Rule |
|-----------|------|
| Platform | GitHub Actions, `ubuntu-latest` |
| Action versions | Pinned to exact tags (`@v6`), **never** `@latest` or `@main` |
| Tool versions | Pinned to exact versions, **never** `@latest` |
| CI concurrency | Cancel superseded runs, except release branches |
| Release concurrency | **Never cancel** — every commit runs to completion |
| Version resolution | Derived from Git ref (tag or branch), never hardcoded |
| Checksums | SHA-256 for all release assets |
| Permissions | Minimum required (`read` for CI, `write` for releases) |
| Working directory | Use `working-directory:` key, never `cd` |
| Build-once rule | Compile once, then compress/checksum/publish — no rebuilds |

### Trigger Patterns

```yaml
# CI Pipeline
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

# Release Pipeline
on:
  push:
    branches: ["release/**"]
    tags: ["v*"]

# Scheduled Scans
on:
  schedule:
    - cron: "0 9 * * 1"  # Weekly Monday 9:00 UTC
  workflow_dispatch:       # Manual trigger
```

### Concurrency Control

```yaml
# CI — cancel old runs on same branch (except release branches)
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: ${{ !startsWith(github.ref, 'refs/heads/release/') }}

# Release — NEVER cancel
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false
```

### Permissions

| Pipeline | Permission | Reason |
|----------|-----------|--------|
| CI | `contents: read` | Only reads source code |
| Release | `contents: write` | Creates GitHub Releases, uploads assets |

---

## Pipeline Archetype 1: Browser Extension Deploy

For Node.js/pnpm projects that produce Chrome/Chromium extensions.

### CI Pipeline

```
Lint → Test → SDK Build → Parallel Module Builds → Extension Assembly
```

- Diamond dependency graph: SDK builds first, modules build in parallel, extension assembles last
- Each module is a separate build target with its own `pnpm build`
- Source maps included in CI builds

### Release Pipeline

```
Same build graph → Source Map Removal → ZIP Packaging → GitHub Release
```

- Source maps stripped from production builds
- `.zip` archives of extension and standalone components
- Asset naming: `{component}-v{version}.zip`

---

## Pipeline Archetype 2: Go Binary Deploy

For Go projects that produce cross-compiled CLI binaries.

### CI Pipeline — 4-Phase Structure

```
Phase 1: SHA Dedup Gate → Phase 2: Lint + Vulncheck → Phase 3: Test Matrix → Phase 4: Cross-Compile + Summary
```

#### Phase 1: SHA-Based Dedup Gate

```yaml
- name: Check for changes
  id: dedup
  run: |
    CURRENT_SHA=$(git rev-parse HEAD)
    LAST_SHA=$(cat .last-ci-sha 2>/dev/null || echo "")
    if [ "$CURRENT_SHA" = "$LAST_SHA" ]; then
      echo "skip=true" >> $GITHUB_OUTPUT
    else
      echo "skip=false" >> $GITHUB_OUTPUT
    fi
```

Skips redundant validation via **step-level conditionals** (not job-level `if`) to ensure the job always succeeds for branch protection rules.

#### Phase 2: Lint + Vulnerability Check

- `golangci-lint` with pinned version
- `govulncheck` for Go vulnerability scanning
- Stdlib vulns → warn only; third-party vulns → fail

#### Phase 3: Test Matrix

- Unit tests across multiple Go versions
- Race detector enabled: `go test -race ./...`
- Coverage reporting

#### Phase 4: Cross-Compile

- 6 targets: `windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- `CGO_ENABLED=0` for fully static binaries
- `-ldflags -X` embeds `Version`, `RepoPath`, `CommitSHA`, `BuildDate`
- Output to `dist/` directory

### Release Pipeline — 9-Stage Strict Order

```
1. Checkout → 2. Setup Go → 3. Resolve Version → 4. Build All Targets →
5. Compress → 6. Checksum → 7. Generate Install Scripts → 8. Extract Changelog → 9. Publish
```

#### Version Resolution

```bash
# From tag: refs/tags/v1.2.0 → 1.2.0
VERSION="${GITHUB_REF#refs/tags/v}"
```

#### Compression

| OS | Format | Tool |
|----|--------|------|
| Windows | `.zip` | `zip` |
| Linux/macOS | `.tar.gz` | `tar czf` (preserves permissions) |

#### Checksum Generation

```bash
cd dist && sha256sum *.zip *.tar.gz > checksums.txt
```

Generated from **compressed archives**, not raw binaries (TOCTOU prevention).

#### Prerelease Detection

Versions containing `-` (e.g., `1.2.0-beta.1`) are marked prerelease and **NOT** set as latest.

#### Release Asset Set (Standard: 9 files)

| # | File | Description |
|---|------|-------------|
| 1-6 | `<binary>-<os>-<arch>.<ext>` | 6 compressed archives |
| 7 | `checksums.txt` | SHA-256 checksums |
| 8 | `install.ps1` | PowerShell installer |
| 9 | `install.sh` | Bash installer |

---

## Install Script Generation

### PowerShell (`install.ps1`) Flow

```
1. Print banner → 2. Detect arch → 3. Download .zip + checksums.txt →
4. Verify SHA-256 → 5. Rename existing (rename-first) →
6. Extract to install dir → 7. Clean .old file →
8. Register PATH (Registry + Profile + Git Bash) → 9. Print summary
```

- Install location: `$env:LOCALAPPDATA\<binary>\`
- PATH registration: Windows Registry (User), PowerShell `$PROFILE`, Git Bash profiles

### Bash (`install.sh`) Flow

```
1. Print banner → 2. Detect OS/arch → 3. Select download tool (curl/wget) →
4. Download .tar.gz + checksums.txt → 5. Verify SHA-256 →
6. Rename existing → 7. Extract → 8. Clean .old →
9. Register PATH in shell profiles → 10. Print summary
```

- Install location: `$HOME/.local/bin` (user) or `/usr/local/bin` (root)
- Shell-aware PATH: bash (`~/.bashrc`), zsh (`~/.zshrc`), fish (`~/.config/fish/config.fish`)
- SHA-256 fallback: `sha256sum` → `shasum -a 256` (macOS)

### Placeholder Strategy

Both scripts use `VERSION_PLACEHOLDER` and `REPO_PLACEHOLDER` tokens replaced via `sed` after heredoc write during release pipeline.

---

## Code Signing

Windows binary signing via SignPath:

| Aspect | Detail |
|--------|--------|
| Feature flag | `vars.SIGNPATH_SIGNING_ENABLED` repository variable |
| Pipeline placement | After build, before compression/checksums |
| In-place replacement | Signed binaries overwrite unsigned in `dist/` |
| Scope | Only `.exe` files; Linux/macOS not signed |
| Verification | Confirms signing completed without error |

---

## Vulnerability Scanning

| Mode | Trigger | Purpose |
|------|---------|---------|
| In-CI | Every push/PR | Gate code changes |
| Standalone | Weekly schedule + manual | Catch newly disclosed CVEs |

**Classification:**
- Third-party vulnerabilities → **fail** the pipeline
- Stdlib vulnerabilities → **warn** only (logged, not blocking)

---

## Release Body & Changelog

### Changelog Extraction

```bash
awk '/^## \['"${VERSION}"'\]/{flag=1;next}/^## \[/{flag=0}flag' CHANGELOG.md
```

Graceful fallback if version section not found.

### Release Body Template

```markdown
## What's Changed
{changelog content}

## Build Info
| Key | Value |
|-----|-------|
| Version | {version} |
| Commit | {sha} |
| Branch | {branch} |
| Build Date | {date} |
| Go Version | {go_version} |

## Checksums
\`\`\`
{checksums.txt content}
\`\`\`

## Install
**PowerShell:** `irm .../install.ps1 | iex`
**Bash:** `curl -fsSL .../install.sh | bash`

## Assets
| File | OS | Arch |
|------|-----|------|
| binary-windows-amd64.zip | Windows | amd64 |
| ... | ... | ... |
```

---

## Terminal Output Standards

### Formatting Rules

| Rule | Detail |
|------|--------|
| Indentation | 2-space from left margin, never tabs |
| Progress | `[1/4] Step description` |
| Success | `[OK] Check passed` |
| Failure | `[FAIL] Check failed` |
| Warning | `[WARN] Non-fatal issue` |
| Errors | `Error: <message> (<operation>)` to stderr |
| Tables | Fixed-width columns with header separators |

### Status Icons

| Icon | Meaning | Used In |
|------|---------|---------|
| `[OK]` | Success | Doctor checks, verification |
| `[FAIL]` | Failure | Doctor checks |
| `[WARN]` | Warning | Deprecation, stdlib vulns |
| `[+]` | Added | PATH registration |
| `[-]` | Removed | PATH removal |
| `[=]` | No change | Already exists |
| `[SKIP]` | Skipped | Already installed |

**ASCII-only** for scripts and CI output. Unicode (`✓`, `✗`) only in Go-compiled binaries.

### Error Output Format

```
  Error: Failed to open database at /path/to/db.sqlite
    Reason:     file is locked by another process
    Operation:  store.Open
    Suggestion: Close other instances and retry
```

---

## Binary Icon Branding

Windows icon embedding via `go-winres`:

- `winres.json` in project root
- Icon files in `assets/` folder
- Embedded during build via `go generate`
- Distinct icon for updater binary vs main binary

---

## Version & Help System

### Version Display

```
<binary> version
  Version:    1.2.0
  Commit:     abc1234
  Build Date: 2026-04-16
  Go Version: go1.22.0
```

### Help System

- Root `--help` shows command list
- Subcommand `--help` shows usage, flags, examples
- CI verification: `<binary> version` and `<binary> --help` must exit 0

---

## Environment Variable Setup

The `env` command manages persistent PATH and environment:

- `<binary> env` — auto-detect and register
- Windows: User PATH via Registry + PowerShell profile + Git Bash profiles
- Unix: Shell-aware PATH injection (bash/zsh/fish)
- Auto-home directory creation

---

## Key Rules

1. ALL CI/CD content belongs in `spec/13-cicd-pipeline-workflows/`
2. App-specific deployment notes go in `21-app/` instead
3. Shared patterns (version resolution, checksums, releases) are in root-level files
4. Archetype-specific patterns are in their respective subfolders
5. New pipeline types get their own subfolder with `00-overview.md`
6. Client-side update logic lives in `spec/14-self-update-app-update/`
7. Never use `@latest` — pin all versions
8. Never cancel release pipeline jobs
9. Build once — never rebuild after the build stage

---

## File Inventory

| # | File | Description |
|---|------|-------------|
| 01 | `01-shared-conventions.md` | Platform, triggers, concurrency, version resolution |
| 02 | `02-github-release-standard.md` | Release body, pre-release detection, asset matrix |
| 03 | `03-vulnerability-scanning.md` | Standalone and in-CI vuln scanning |
| 04 | `04-install-script-generation.md` | PS1+Bash installer pattern, placeholder strategy |
| 05 | `05-code-signing.md` | SignPath integration, feature-flag gating |
| 06 | `06-self-update-mechanism.md` | CLI self-update: deploy path, rename-first, handoff |
| 07 | `07-release-body-and-changelog.md` | Changelog extraction, release body template |
| 08 | `08-installation-flow.md` | End-to-end install: one-liners, upgrade, uninstall |
| 09 | `09-changelog-integration.md` | Changelog format, CI extraction, release body assembly |
| 10 | `10-version-and-help.md` | Version display, help system, CI verification |
| 11 | `11-environment-variable-setup.md` | `env` command: persistent PATH, auto-home, registry |
| 12 | `12-terminal-output-standards.md` | Output formatting: icons, tables, progress, errors |
| 13 | `13-binary-icon-branding.md` | Windows icon embedding via go-winres |
| — | `01-browser-extension-deploy/` | Browser extension CI + release (3 files) |
| — | `02-go-binary-deploy/` | Go binary CI + release (3 files) |

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Full Source | `../13-cicd-pipeline-workflows/00-overview.md` |
| Self-Update & App Update | `../14-self-update-app-update/00-overview.md` |
| Folder Structure Rules | `../01-spec-authoring-guide/01-folder-structure.md` |

---

*Consolidated CI/CD pipeline workflows — v3.2.0 — 2026-04-16*
