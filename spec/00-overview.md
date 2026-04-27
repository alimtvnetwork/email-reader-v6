# Specification Root

**Version:** 3.1.0  
**Updated:** 2026-04-16  
**AI Confidence:** Production-Ready  
**Ambiguity:** None

---

## Purpose

Root index for the entire specification tree. Each top-level folder contains a domain-specific specification module with its own overview, acceptance criteria, and consistency report.

---

## Module Inventory

### Core Fundamentals (01–20)

| # | Module | Description |
|---|--------|-------------|
| 01 | [Spec Authoring Guide](./01-spec-authoring-guide/00-overview.md) | Rules for writing and maintaining spec documents |
| 02 | [Coding Guidelines](./02-coding-guidelines/00-overview.md) | Cross-language coding standards (Go, TS, PHP, Rust, C#) |
| 03 | [Error Management](./03-error-manage/00-overview.md) | Error capture, modal UI, and resolution workflows |
| 04 | [Database Conventions](./04-database-conventions/00-overview.md) | Naming, schema design, ORM, REST API format |
| 05 | [Split DB Architecture](./05-split-db-architecture/00-overview.md) | SQLite partitioning and migration patterns |
| 06 | [Seedable Config (CW Config)](./06-seedable-config-architecture/00-overview.md) | Configuration seeding and feature management |
| 07 | [Design System](./07-design-system/00-overview.md) | Theme variables, typography, spacing, and component patterns |
| 08 | Docs Viewer UI _(retired — module removed from this repo; viewer lives in the docs-platform tree)_ | Specification viewer React application |
| 09 | [Code Block System](./09-code-block-system/00-overview.md) | Rich code block rendering — syntax highlighting, interactions, styling |
| 10 | [PowerShell Integration](./10-powershell-integration/00-overview.md) | PowerShell scripting conventions, cross-platform automation |
| 11 | [Research](./11-research/00-overview.md) | Comparative studies, technology evaluations, exploratory notes |
| 12 | [Consolidated Guidelines](./12-consolidated-guidelines/00-overview.md) | AI-readable summaries of every major spec module |
| 13 | [CI/CD Pipeline Workflows](./13-cicd-pipeline-workflows/00-overview.md) | CI/CD pipeline specs, deployment workflows, automation |
| 14 | [Self-Update & App Update](./14-self-update-app-update/00-overview.md) | Rename-first deployment, release pipeline, cross-compilation |

### App-Specific (21+)

| # | Module | Description |
|---|--------|-------------|
| 21 | [App](./21-app/00-overview.md) | App-specific specs: features, workflows, architecture |
| 22 | [App Issues](./22-app-issues/00-overview.md) | App bug analysis, root cause analysis, fix documentation |
| 23 | [App Database](./23-app-database/00-overview.md) | App-specific data model, table designs, migration strategies |
| 24 | [App Design System & UI](./24-app-design-system-and-ui/00-overview.md) | App-specific design system, theming, component patterns |

---

## Supporting Files

| File | Purpose |
|------|---------|
| [folder-structure-root.md](./folder-structure-root.md) | Redirect to canonical folder structure spec |
| [spec-index.md](./spec-index.md) | Flat index of all spec files |
| [health-dashboard.md](./health-dashboard.md) | Spec tree health metrics and broken link report |
| [dashboard-data.json](./dashboard-data.json) | Machine-readable health data |

---

## Cross-References

| Reference | Location |
|-----------|----------|
| Folder Structure (canonical) | `./01-spec-authoring-guide/01-folder-structure.md` |
| Spec Authoring Guide | `./01-spec-authoring-guide/00-overview.md` |
| Coding Guidelines | `./02-coding-guidelines/00-overview.md` |
| Error Management | `./03-error-manage/00-overview.md` |
| Design System | `./06-design-system/00-overview.md` |
