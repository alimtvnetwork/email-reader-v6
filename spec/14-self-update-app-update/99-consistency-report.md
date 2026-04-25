# Consistency Report — Self-Update & App Update

**Version:** 3.1.0  
**Updated:** 2026-04-16

---

## File Inventory

| # | File | Status | Version |
|---|------|--------|---------|
| 1 | `00-overview.md` | ✅ Present | 3.1.0 |
| 2 | `01-self-update-overview.md` | ✅ Present | 1.0.0 |
| 3 | `02-deploy-path-resolution.md` | ✅ Present | 1.0.0 |
| 4 | `03-rename-first-deploy.md` | ✅ Present | 1.0.0 |
| 5 | `04-build-scripts.md` | ✅ Present | 1.0.0 |
| 6 | `05-handoff-mechanism.md` | ✅ Present | 1.0.0 |
| 7 | `06-cleanup.md` | ✅ Present | 1.0.0 |
| 8 | `07-release-assets.md` | ✅ Present | 1.0.0 |
| 9 | `08-checksums-verification.md` | ✅ Present | 1.0.0 |
| 10 | `09-release-versioning.md` | ✅ Present | 2.0.0 |
| 11 | `10-cross-compilation.md` | ✅ Present | 1.0.0 |
| 12 | `11-release-pipeline.md` | ✅ Present | 1.0.0 |
| 13 | `12-install-scripts.md` | ✅ Present | 1.0.0 |
| 14 | `13-updater-binary.md` | ✅ Present | 1.0.0 |
| 15 | `14-network-requirements.md` | ✅ Present | 1.0.0 |
| 16 | `15-config-file.md` | ✅ Present | 1.0.0 |
| 17 | `16-update-command-workflow.md` | ✅ Present | 1.0.0 |
| 18 | `diagrams/00-overview.md` | ✅ Present | 1.0.0 |
| 19 | `diagrams/01-self-update-workflow.mmd` | ✅ Present | 1.0.0 |
| 20 | `diagrams/02-update-cleanup-workflow.mmd` | ✅ Present | 1.0.0 |

**Total:** 20 files (excluding this report)

---

## Source Mapping

| Spec File | gitmap-v2 Source |
|-----------|-----------------|
| 01–06 | `spec/generic-update/01–06` |
| 07 | `spec/generic-release/05-release-assets.md` |
| 08 | `spec/generic-release/04-checksums-verification.md` |
| 09 | `spec/generic-release/06-release-metadata.md` |
| 10 | `spec/generic-release/01-cross-compilation.md` |
| 11 | `spec/generic-release/02-release-pipeline.md` |
| 12 | `spec/generic-release/03-install-scripts.md` |
| 13 | Gap analysis — updater binary architecture |
| 14 | Gap analysis — network requirements |
| 15 | Gap analysis — config file specification |
| 16 | `gitmap/cmd/update.go`, `gitmap/cmd/updatecleanup.go` — step-by-step workflow |
| diagrams/* | Mermaid visualizations of files 16 workflows |

---

## Cross-Reference Integrity

- [x] All overview inventory entries point to existing files
- [x] All internal cross-references resolve (verified 2026-04-15)
- [x] Bidirectional cross-refs with `12-cicd-pipeline-workflows/`
- [x] Consolidated summary exists at `../11-consolidated-guidelines/17-self-update-app-update.md`
- [x] Files 13–16 cross-reference each other correctly
- [x] Diagrams subfolder has its own `00-overview.md` index

---

## v3.2.0 Changes

Added diagrams subfolder inventory:

- **diagrams/00-overview.md**: Index of all Mermaid diagrams with rendering instructions and cross-references
- **diagrams/01-self-update-workflow.mmd**: Renamed from `self-update-workflow.mmd` with numeric prefix
- **diagrams/02-update-cleanup-workflow.mmd**: Renamed from `update-cleanup-workflow.mmd` with numeric prefix

---

## Summary

- **Errors:** 0
- **Health Score:** 100/100 (A+)

---

## Validation History

| Date | Version | Action |
|------|---------|--------|
| 2026-04-10 | 1.0.0 | Initial consistency report with 9 files |
| 2026-04-11 | 2.0.0 | Added 10-cross-compilation, 11-release-pipeline, 12-install-scripts from gitmap-v2 generic-release |
| 2026-04-13 | 3.0.0 | Added 13-updater-binary, 14-network-requirements, 15-config-file from gap analysis; verified all cross-references |
| 2026-04-13 | 3.1.0 | Added 16-update-command-workflow from gitmap-v2 source code analysis |
| 2026-04-15 | 3.2.0 | Added diagrams subfolder (00-overview.md, 01/02 .mmd files) to inventory; renamed diagrams with numeric prefixes |

---

*Consistency Report — updated: 2026-04-15*
