# CI/CD Pipeline Workflows

**Version:** 3.1.0  
**Updated:** 2026-04-16  
**AI Confidence:** Production-Ready  
**Ambiguity:** None

---

## Purpose

Central location for all CI/CD pipeline specifications, deployment automation, and related infrastructure-as-code documentation. All pipeline-related content — build pipelines, deployment workflows, environment promotion strategies, and CI/CD tooling configurations — MUST be documented in this folder.

---

## Scope

This module covers two distinct pipeline archetypes, shared conventions, and cross-cutting concerns:

| Archetype | Subfolder | Description |
|-----------|-----------|-------------|
| Browser Extension Deploy | `01-browser-extension-deploy/` | Node.js/pnpm multi-component builds, zip packaging, Chrome Web Store |
| Go Binary Deploy | `02-go-binary-deploy/` | Cross-compiled Go binaries, tar.gz/zip, install scripts, code signing |
| Shared | Root files | Common patterns used across all pipeline types |

---

## Placement Rules

```
AI INSTRUCTION:

1. ALL CI/CD and pipeline content belongs in this folder (spec/12-cicd-pipeline-workflows/).
2. This is a Core Fundamentals folder (range 01–20) — no app-specific content here.
3. App-specific deployment notes go in 21-app/ instead.
4. Each pipeline spec file follows the standard {NN}-{kebab-case-name}.md naming convention.
5. Add new files to the Feature Inventory below and update 99-consistency-report.md.
6. Shared patterns (version resolution, checksums, release creation) go in root-level files.
7. Archetype-specific patterns go in the appropriate subfolder.
```

---

## Feature Inventory

### Root (Shared Conventions)

| # | File | Description | Status |
|---|------|-------------|--------|
| 01 | [01-shared-conventions.md](./01-shared-conventions.md) | Platform, triggers, concurrency, version resolution, checksums | ✅ Active |
| 02 | [02-github-release-standard.md](./02-github-release-standard.md) | Release body assembly, pre-release detection, asset matrix | ✅ Active |
| 03 | [03-vulnerability-scanning.md](./03-vulnerability-scanning.md) | Standalone and in-CI vulnerability scanning patterns | ✅ Active |
| 04 | [04-install-script-generation.md](./04-install-script-generation.md) | Reusable PS1+Bash installer pattern, placeholder strategy, checksum verification | ✅ Active |
| 05 | [05-code-signing.md](./05-code-signing.md) | SignPath integration, feature-flag gating, signature verification | ✅ Active |
| 06 | [06-self-update-mechanism.md](./06-self-update-mechanism.md) | Generic CLI self-update blueprint: deploy path, rename-first, handoff, cleanup | ✅ Active |
| 07 | [07-release-body-and-changelog.md](./07-release-body-and-changelog.md) | Changelog extraction, release body template, asset matrix assembly | ✅ Active |
| 08 | [04-installation-flow.md](./04-installation-flow.md) | End-to-end install: one-liners, terminal output, upgrade, uninstall | ✅ Active |
| 09 | [05-changelog-integration.md](./05-changelog-integration.md) | Changelog format, CI extraction, release body assembly, terminal display | ✅ Active |
| 10 | [06-version-and-help.md](./06-version-and-help.md) | Version display, help system, command-level docs, CI verification | ✅ Active |
| 11 | [07-environment-variable-setup.md](./07-environment-variable-setup.md) | `env` command: persistent variables, PATH registration, auto-home | ✅ Active |
| 12 | [08-terminal-output-standards.md](./08-terminal-output-standards.md) | Output formatting: icons, tables, progress, errors, CI summaries | ✅ Active |
| 13 | [09-binary-icon-branding.md](./09-binary-icon-branding.md) | Windows binary icon embedding via `go-winres`: icon, manifest, version info | ✅ Active |

### Subfolder: Browser Extension Deploy

| # | File | Description | Status |
|---|------|-------------|--------|
| 00 | [00-overview.md](./01-browser-extension-deploy/00-overview.md) | Overview of browser extension pipeline | ✅ Active |
| 01 | [01-ci-pipeline.md](./01-browser-extension-deploy/01-ci-pipeline.md) | CI: lint, test, dependency-graph builds, extension assembly | ✅ Active |
| 02 | [02-release-pipeline.md](./01-browser-extension-deploy/02-release-pipeline.md) | Release: version, build, package, source map removal, GitHub Release | ✅ Active |

### Subfolder: Go Binary Deploy

| # | File | Description | Status |
|---|------|-------------|--------|
| 00 | [00-overview.md](./02-go-binary-deploy/00-overview.md) | Overview of Go binary pipeline | ✅ Active |
| 01 | [01-ci-pipeline.md](./02-go-binary-deploy/01-ci-pipeline.md) | CI: SHA dedup, lint, vulncheck, test matrix, cross-compile | ✅ Active |
| 02 | [02-release-pipeline.md](./02-go-binary-deploy/02-release-pipeline.md) | Release: binary build, icon embedding, code signing, install scripts, GitHub Release | ✅ Active |

---

## Migration History

| Date | Change |
|------|--------|
| 2026-04-10 | v3.0.0 — Added 04-install-script-generation, 05-code-signing, 06-self-update-mechanism, 07-release-body-and-changelog; updated Go release pipeline with multi-module, icon embedding, LDFLAGS variables |
| 2026-04-09 | v2.0.0 — Initial creation with shared conventions, two archetypes, vulnerability scanning |

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Folder Structure Rules | `../01-spec-authoring-guide/01-folder-structure.md` |
| Coding Guidelines | `../02-coding-guidelines/00-overview.md` |
| PowerShell Automation | `../09-powershell-integration/00-overview.md` |
| Consolidated Summary | `../11-consolidated-guidelines/15-cicd-pipeline-workflows.md` |

---

*Overview — updated: 2026-04-10*
