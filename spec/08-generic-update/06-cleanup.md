# 06 — Cleanup

## Purpose

Define how temporary artifacts from the update process are identified
and removed after a successful update.


## Flow Diagram

See [`images/cleanup-flow.mmd`](images/cleanup-flow.mmd)

---

## Artifacts That Need Cleanup

| Artifact | Created By | Location |
|----------|------------|----------|
| `<binary>-update-<pid>.exe` | Handoff copy (Step 1) | Same dir as binary or temp dir |
| `<binary>-update-*.ps1` | Generated PowerShell script | System temp dir |
| `<binary>.exe.old` | Rename-first deploy | Deploy directory |
| `<binary>-updater-tmp-<pid>.exe` | Standalone updater handoff | Same dir as updater |

---

## Cleanup Command

Provide an explicit cleanup command:

```
<binary> update-cleanup
```

This command scans the binary's directory and temp directory for
leftover artifacts and removes them.

### Implementation

```go
func runUpdateCleanup() {
    // 1. Clean up .old backups in the binary's directory
    selfDir := resolveInstalledDir()
    cleanGlob(selfDir, "*.old")

    // 2. Clean up handoff copies
    cleanGlob(selfDir, "<binary>-update-*")
    cleanGlob(os.TempDir(), "<binary>-update-*")

    // 3. Clean up generated scripts
    cleanGlob(os.TempDir(), "<binary>-update-*.ps1")
}

func cleanGlob(dir, pattern string) {
    matches, err := filepath.Glob(filepath.Join(dir, pattern))
    if err != nil {
        return
    }

    for _, match := range matches {
        if err := os.Remove(match); err != nil {
            fmt.Printf("  !! Could not remove %s: %v\n", filepath.Base(match), err)
            continue
        }
        fmt.Printf("  OK Removed %s\n", filepath.Base(match))
    }
}
```

---

## Automatic Cleanup

The update process should attempt automatic cleanup after a successful
update. However, some files may be locked (e.g., the handoff copy is
still running). In that case:

1. **Try to clean up immediately** after the update completes.
2. **If any files are locked**, skip them silently — they will be
   cleaned up on the next `update-cleanup` or next update.
3. **Never fail the update** because cleanup failed.

```go
func attemptAutoCleanup() {
    // Best-effort — ignore errors
    matches, _ := filepath.Glob(filepath.Join(selfDir, "<binary>-update-*"))
    for _, m := range matches {
        os.Remove(m) // Ignore error — may be locked
    }
}
```

---

## Build Script Cleanup

The build script (`run.ps1` / `run.sh`) should call `update-cleanup`
at the end of an update run:

### PowerShell

```powershell
if ($Update) {
    Write-Info "Running update cleanup"
    & $binaryPath update-cleanup
}
```

### Bash

```bash
if [[ "$UPDATE" == "true" ]]; then
    echo "  -> Running update cleanup"
    "$BINARY_PATH" update-cleanup || true
fi
```

---

## `.old` File Lifecycle

The `.old` backup files from rename-first deploy follow this lifecycle:

```
Deploy:
  <binary>.exe → <binary>.exe.old    (renamed)
  new binary   → <binary>.exe        (copied)

After successful update:
  <binary>.exe.old → deleted          (by update-cleanup)

After failed update:
  <binary>.exe.old → <binary>.exe    (rollback — rename back)
```

**Important**: Never delete `.old` files during deploy. They are the
rollback safety net. Only delete them after the update is confirmed
successful (version check passes).

---

## Temp Directory Hygiene

The system temp directory can accumulate artifacts if updates are
interrupted. The cleanup command should scan for patterns:

| Pattern | Artifact |
|---------|----------|
| `<binary>-update-*.exe` | Handoff copies |
| `<binary>-update-*.ps1` | Generated scripts |
| `<binary>-install-*.ps1` | Downloaded install scripts |

Use conservative matching — only delete files that match the exact
prefix pattern for your tool.

---

## When to Run Cleanup

| Trigger | Automatic | Manual |
|---------|-----------|--------|
| After successful update | ✅ Best-effort | |
| User runs `update-cleanup` | | ✅ Full scan |
| Before starting a new update | ✅ Clean old copies | |
| On `doctor` command | ✅ Warn about stale files | |

---

## Constraints

- Cleanup must never fail the parent operation (update, build, etc.).
- Use `os.Remove()`, not recursive deletion — only target specific files.
- Match patterns conservatively to avoid deleting unrelated files.
- Never delete the active binary or its data directory.
- Log every file removed so the user has visibility.
- On Windows, some files may be locked — skip them without error.

## Application-Specific References

| App Spec | Covers |
|----------|--------|
| [PowerShell Integration — Overview](../10-powershell-integration/00-overview.md) | `.old` artifact lifecycle during deploy |
| [Self-Update Mechanism (CI/CD)](../13-cicd-pipeline-workflows/06-self-update-mechanism.md) | Post-update cleanup of handoff and `.old` files |

## Contributors

- [**Md. Alim Ul Karim**](https://www.linkedin.com/in/alimkarim) — Creator & Lead Architect. System architect with 20+ years of professional software engineering experience across enterprise, fintech, and distributed systems. Recognized as one of the top software architects globally. Alim's architectural philosophy — consistency over cleverness, convention over configuration — is the driving force behind every design decision in this framework.
  - [Google Profile](https://www.google.com/search?q=Alim+Ul+Karim)
- [Riseup Asia LLC (Top Leading Software Company in WY)](https://riseup-asia.com) (2026)
  - [Facebook](https://www.facebook.com/riseupasia.talent/)
  - [LinkedIn](https://www.linkedin.com/company/105304484/)
  - [YouTube](https://www.youtube.com/@riseup-asia)
