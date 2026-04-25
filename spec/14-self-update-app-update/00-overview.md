# Self-Update & App Update

**Version:** 3.1.0  
**Updated:** 2026-04-16  
**AI Confidence:** Production-Ready  
**Ambiguity:** None

---

## Purpose

Central location for all CLI self-update and application update specifications. This module defines generic, reusable blueprints that any CLI tool can implement — covering the full lifecycle from detecting the installed binary, through building/downloading a new version, deploying it without file-lock errors, verifying success, and cleaning up artifacts.

Any AI or engineer reading these documents should be able to implement a complete self-update system from scratch without ambiguity.

---

## Core Problem

A running binary **cannot overwrite itself** on Windows. The entire update architecture exists to work around this constraint while maintaining a seamless user experience on all platforms.

---

## Scope

This module covers three complementary areas:

| Area | Description |
|------|-------------|
| **Self-Update** | How a running CLI replaces itself with a newer version |
| **Release Distribution** | How release artifacts are packaged, verified, and distributed to users |
| **Release Pipeline** | How the CI/CD workflow builds, compiles, and publishes releases |

---

## Placement Rules

```
AI INSTRUCTION:

1. ALL self-update and app update content belongs in this folder (spec/14-self-update-app-update/).
2. This is a Core Fundamentals folder (range 01–20) — no app-specific content here.
3. App-specific update behavior goes in 21-app/ instead.
4. CI/CD pipeline specs (GitHub Actions workflows) belong in 12-cicd-pipeline-workflows/.
5. This folder focuses on the CLIENT-SIDE update logic AND generic release pipeline patterns.
6. Each spec file follows the standard {NN}-{kebab-case-name}.md naming convention.
7. Cross-compilation, release pipeline, and install scripts are GENERIC blueprints here.
   App-specific CI workflow files belong in 12-cicd-pipeline-workflows/.
```

---

## Feature Inventory

### Self-Update (Client-Side)

| # | File | Description | Status |
|---|------|-------------|--------|
| 01 | [01-self-update-overview.md](./01-self-update-overview.md) | Problem statement, platform constraints, two update strategies, command flow | ✅ Active |
| 02 | [02-deploy-path-resolution.md](./02-deploy-path-resolution.md) | 3-tier deploy target resolution: CLI flag → PATH lookup → config default | ✅ Active |
| 03 | [03-rename-first-deploy.md](./03-rename-first-deploy.md) | Rename-first file replacement strategy with retry and rollback | ✅ Active |
| 04 | [04-build-scripts.md](./04-build-scripts.md) | Cross-platform build scripts (run.ps1/run.sh): pull → build → deploy | ✅ Active |
| 05 | [05-handoff-mechanism.md](./05-handoff-mechanism.md) | Copy-and-handoff for Windows self-replacement, binary-based fallback | ✅ Active |
| 06 | [06-cleanup.md](./06-cleanup.md) | Post-update artifact removal, .old lifecycle, temp directory hygiene | ✅ Active |

### Release Distribution

| # | File | Description | Status |
|---|------|-------------|--------|
| 07 | [07-release-assets.md](./07-release-assets.md) | Asset naming, compression formats, packaging conventions | ✅ Active |
| 08 | [08-checksums-verification.md](./08-checksums-verification.md) | SHA-256 generation, verification on both platforms, TOCTOU prevention | ✅ Active |
| 09 | [09-release-versioning.md](./09-release-versioning.md) | Version resolution, tagging, changelog extraction, release branch strategy | ✅ Active |

### Release Pipeline (Generic Blueprint)

| # | File | Description | Status |
|---|------|-------------|--------|
| 10 | [10-cross-compilation.md](./10-cross-compilation.md) | 6-target cross-compilation, static linking, build loops, embedded constants | ✅ Active |
| 11 | [11-release-pipeline.md](./11-release-pipeline.md) | End-to-end CI/CD workflow: tag → build → compress → checksum → publish | ✅ Active |
| 12 | [12-install-scripts.md](./12-install-scripts.md) | Cross-platform one-liner installers with checksum verification and PATH setup | ✅ Active |
| 13 | [13-updater-binary.md](./13-updater-binary.md) | Standalone updater binary architecture, CLI interface, GitHub API integration | ✅ Active |
| 14 | [14-network-requirements.md](./14-network-requirements.md) | HTTP client config, retry policies, proxy support, TLS, progress display | ✅ Active |
| 15 | [15-config-file.md](./15-config-file.md) | Config file location, JSON schema, first-time creation, platform defaults | ✅ Active |
| 16 | [16-update-command-workflow.md](./16-update-command-workflow.md) | Step-by-step `update` and `update-cleanup` command workflow with decision tree | ✅ Active |

---

## Placeholders

Throughout these documents, generic placeholders are used:

| Placeholder | Meaning | Example |
|-------------|---------|---------|
| `<binary>` | CLI binary name | `gitmap` |
| `<binary>.exe` | Windows binary with extension | `gitmap.exe` |
| `<deploy-dir>` | Directory where the binary is installed | `$env:LOCALAPPDATA\gitmap` |
| `<repo-root>` | Root of the source repository | `D:\projects\gitmap-v2` |
| `<repo>` | GitHub repository path | `github.com/org/repo` |
| `<version>` | Release version | `v1.2.0` |
| `<module>` | Go module path | `github.com/org/repo` |

---

## Relationship to CI/CD Pipeline Workflows

This module contains **generic, reusable blueprints** for release pipelines. The CI/CD pipeline workflows module (`spec/12-cicd-pipeline-workflows/`) contains **app-specific workflow configurations** and deployment archetypes.

| Concern | Location |
|---------|----------|
| Generic cross-compilation blueprint | `14-self-update-app-update/10-cross-compilation.md` |
| Generic release pipeline blueprint | `14-self-update-app-update/11-release-pipeline.md` |
| Generic install script blueprint | `14-self-update-app-update/12-install-scripts.md` |
| App-specific CI workflows and archetypes | `12-cicd-pipeline-workflows/` |
| How the CLI detects, downloads, and installs updates | `14-self-update-app-update/` (files 01–06) |
| How release artifacts are packaged | `14-self-update-app-update/` (files 07–09) |

---

## Cross-References

| Reference | Location |
|-----------|----------|
| CI/CD Pipeline Workflows | `../13-cicd-pipeline-workflows/00-overview.md` |
| Install Script Generation | `../13-cicd-pipeline-workflows/04-install-script-generation.md` |
| Code Signing | `../13-cicd-pipeline-workflows/05-code-signing.md` |
| Self-Update Mechanism (CI/CD) | `../13-cicd-pipeline-workflows/06-self-update-mechanism.md` |
| Consolidated Summary | `../11-consolidated-guidelines/17-self-update-app-update.md` |
| Folder Structure Rules | `../01-spec-authoring-guide/01-folder-structure.md` |

---

*Overview — updated: 2026-04-13*
