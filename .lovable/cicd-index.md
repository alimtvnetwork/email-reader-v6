# CI/CD Issues — Index

Tracks every CI/CD-related issue encountered. One file per issue under
`.lovable/cicd-issues/`. Filename convention: `XX-short-description.md`.

## Active

| # | Issue | Status | File |
|---|-------|--------|------|
| 01 | Sandbox lacks Go toolchain | 🔄 Workaround in place | [01-sandbox-no-go-toolchain.md](./cicd-issues/01-sandbox-no-go-toolchain.md) |
| 02 | Workspace files revert between sessions | 🔄 Workaround in place | [02-workspace-revert-on-resume.md](./cicd-issues/02-workspace-revert-on-resume.md) |
| 03 | Fyne UI requires cgo + display server | 🚫 Blocked (use `-tags nofyne`) | [03-fyne-canvas-needs-cgo.md](./cicd-issues/03-fyne-canvas-needs-cgo.md) |
| 04 | No `-race` gate in PR pipeline | ⏳ Pending external CI runner | [04-no-race-gate-in-ci.md](./cicd-issues/04-no-race-gate-in-ci.md) |
| 05 | No bench/perf infra (blocks AC-DBP/AC-SP) | 🚫 Blocked on bench infra | [05-no-bench-infra.md](./cicd-issues/05-no-bench-infra.md) |

## Conventions

- Status markers: ✅ Resolved · 🔄 Workaround · ⏳ Pending · 🚫 Blocked.
- Once resolved, leave the file in place; flip status to ✅ and add a
  `## Resolution` section. Do not delete.
- New issues: pick the next sequence number; update this index in the
  same operation.
