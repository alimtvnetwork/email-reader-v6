# Consolidated Guidelines

**Version:** 3.1.0  
**Updated:** 2026-04-16  
**Status:** Active  
**AI Confidence:** Production-Ready  
**Ambiguity:** None

---

## Purpose

This folder provides **standalone, self-contained AI-readable references** for every major spec module. Each file consolidates the essential rules, patterns, code examples, and decisions from its source module into a single comprehensive document. **No file in this folder requires reading the source specs** — every consolidated file contains enough detail for an AI agent to enforce rules, implement patterns, or reproduce systems independently.

---

## Keywords

`consolidated` · `standalone` · `self-contained` · `quick-reference` · `ai-onboarding` · `spec-digest`

---

## Scoring

| Metric | Value |
|--------|-------|
| AI Confidence | Production-Ready |
| Ambiguity | None |
| Health Score | 100/100 (A+) |

---

## File Inventory

| # | File | Summarizes (Source Module) |
|---|------|---------------------------|
| 01 | [01-spec-authoring.md](./01-spec-authoring.md) | `01-spec-authoring-guide/` — Spec authoring conventions, folder rules, naming, scoring, templates |
| 02 | [02-coding-guidelines.md](./02-coding-guidelines.md) | `02-coding-guidelines/` — Cross-language coding standards, boolean principles, code style, typing, naming |
| 03 | [03-error-management.md](./03-error-management.md) | `03-error-manage/` — 3-tier error architecture, apperror, response envelope, error codes, TypedQuery |
| 04 | [04-enum-standards.md](./04-enum-standards.md) | `02-coding-guidelines/` — Enum patterns for Go, TypeScript, PHP, Rust with full examples |
| 05 | [05-split-db-architecture.md](./05-split-db-architecture.md) | `05-split-db-architecture/` — Hierarchical SQLite pattern, schemas, WAL, RBAC, reset API, user isolation |
| 06 | [06-seedable-config.md](./06-seedable-config.md) | `06-seedable-config-architecture/` — Version-controlled config seeding, merge strategy, schemas, Go implementation |
| 07 | [07-design-system.md](./07-design-system.md) | `07-design-system/` — CSS variable-driven design system, tokens, typography, motion, re-theming |
| 08 | [08-docs-viewer-ui.md](./08-docs-viewer-ui.md) | `08-docs-viewer-ui/` — Documentation viewer: typography, keyboard nav, fullscreen, search, rendering |
| 09 | [09-code-block-system.md](./09-code-block-system.md) | `09-code-block-system/` — Code block pipeline, interactions, styling, line selection, checklist blocks |
| 10 | [10-powershell-integration.md](./10-powershell-integration.md) | `10-powershell-integration/` — PowerShell runner, pnpm PnP, pipeline steps, config, cross-project automation |
| 11 | [11-research.md](./11-research.md) | `02-coding-guidelines/` — Coding guidelines research placement rules |
| 12 | [12-root-research.md](./12-root-research.md) | `11-research/` — Root-level research placement rules |
| 13 | [13-app.md](./13-app.md) | `21-app/` — App-specific spec placement and decision guide |
| 14 | [14-app-issues.md](./14-app-issues.md) | `22-app-issues/` — App bug analysis, issue file template, placement rules |
| 15 | [15-cicd-pipeline-workflows.md](./15-cicd-pipeline-workflows.md) | `13-cicd-pipeline-workflows/` — CI/CD pipeline specs, deployment workflows, install scripts, code signing |
| 16 | [16-app-design-system-and-ui.md](./16-app-design-system-and-ui.md) | `24-app-design-system-and-ui/` — App-specific design system extending core tokens |
| 17 | [17-self-update-app-update.md](./17-self-update-app-update.md) | `14-self-update-app-update/` — CLI self-update, rename-first deploy, handoff, release pipeline, install scripts |
| 18 | [18-database-conventions.md](./18-database-conventions.md) | `04-database-conventions/` — Database naming, PK/FK patterns, singular tables, booleans, views, ORM, schema design |
| 19 | [19-gap-analysis.md](./19-gap-analysis.md) | Gap analysis — coverage matrix, implementability scores, priority recommendations |
| 20 | [20-wp-plugin-conventions.md](./20-wp-plugin-conventions.md) | `15-wp-plugin-how-to/` — WordPress plugin Gold Standard architecture, traits, enums, REST API |
| 21 | [21-lovable-folder-structure.md](./21-lovable-folder-structure.md) | `.lovable/` folder structure — memory, tasks, suggestions, strictly-avoid, AI reading order |
| 22 | [22-app-database.md](./22-app-database.md) | `23-app-database/` — App-specific schema design, migration strategy, query patterns, ORM integration |
| 23 | [23-generic-cli.md](./23-generic-cli.md) | `16-generic-cli/` — CLI creation blueprint: project structure, subcommands, flags, config, output, errors, help, database, build, testing, shell completion |

---

## How to Use This Folder

1. **AI onboarding** — Read all files in this folder to understand the full system
2. **Each file is standalone** — no need to follow cross-references to source specs
3. **Updates** — When a source module changes significantly, update the corresponding summary here

---

*Overview — updated: 2026-04-16*
