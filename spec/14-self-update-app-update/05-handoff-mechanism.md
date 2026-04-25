# Handoff Mechanism (Windows)

**Version:** 3.1.0  
**Updated:** 2026-04-16

---

## Purpose

Define how a CLI tool replaces its own binary on Windows, where the running process holds a file lock that prevents self-overwrite.

---

## The Core Problem

When a user runs `<binary> update`:

1. The process `<binary>.exe` is running.
2. The update needs to replace `<binary>.exe` with a newer version.
3. Windows locks `<binary>.exe` — it cannot be overwritten or deleted.

Even with rename-first, the build script (`run.ps1`) is launched by the running binary, which may still be alive when deploy happens.

---

## Solution: Copy-and-Handoff

The running binary creates a **temporary copy of itself**, launches the copy as a **worker process**, and exits. The worker is a different file, so it does not conflict with the original binary's lock.

### Flow

```
<binary>.exe update
 │
 ├── 1. Copy self → <binary>-update-<PID>.exe (temp dir or same dir)
 ├── 2. Launch temp copy: <binary>-update-<PID>.exe update-runner
 ├── 3. Parent waits (synchronous — cmd.Run, not cmd.Start)
 │
 └── Worker (<binary>-update-<PID>.exe update-runner):
     ├── 4. Generate temp PowerShell script
     ├── 5. Script calls run.ps1 (pull, build, deploy)
     ├── 6. Deploy uses rename-first on the ORIGINAL binary
     ├── 7. Verify version
     └── 8. Clean up temp files
```

### Why the Parent Waits

The parent process (`<binary>.exe`) calls `cmd.Run()` (blocking), NOT `cmd.Start()` (non-blocking). This keeps the terminal attached so the user sees all build output. The parent holds the lock on its own file, but the worker deploys to the ORIGINAL path — the lock conflict is between parent and deploy, which rename-first resolves.

---

## Step 1 — Create Handoff Copy

```go
func createHandoffCopy() (string, error) {
    selfPath, err := os.Executable()
    if err != nil {
        return "", err
    }

    // Try same directory first, then fall back to temp dir
    selfDir := filepath.Dir(selfPath)
    copyName := fmt.Sprintf("<binary>-update-%d.exe", os.Getpid())
    copyPath := filepath.Join(selfDir, copyName)

    data, err := os.ReadFile(selfPath)
    if err != nil {
        return "", err
    }

    if err := os.WriteFile(copyPath, data, 0o755); err != nil {
        // Fallback to temp directory
        copyPath = filepath.Join(os.TempDir(), copyName)
        if err := os.WriteFile(copyPath, data, 0o755); err != nil {
            return "", err
        }
    }

    return copyPath, nil
}
```

### Same-Dir vs Temp-Dir

| Location | Advantage | Risk |
|----------|-----------|------|
| Same directory as binary | Simpler path resolution | May fail if directory is read-only |
| System temp directory | Always writable | Path may be long, some policies block execution from temp |

Try same-dir first, fall back to temp.

---

## Step 2 — Launch Worker

```go
func launchWorker(copyPath, repoPath string) error {
    args := []string{"update-runner", "--repo-path", repoPath}

    cmd := exec.Command(copyPath, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    // Synchronous — parent blocks until worker completes
    return cmd.Run()
}
```

**Critical**: Use `cmd.Run()` (blocking), not `cmd.Start()`.

---

## Step 3 — Worker Generates Script

The worker (running from the temp copy) generates a temporary PowerShell script that calls the build script:

```go
func buildUpdateScript(repoPath, runPS1 string) string {
    return fmt.Sprintf(`
Set-Location '%s'

# Detect current deploy path
$deployDir = Split-Path (Get-Command <binary>).Source -Parent

# Show version before update
$beforeVersion = & <binary> version 2>&1
Write-Host " Before: $beforeVersion"

# Run the build script with update flag
& '%s' -Update

# Show version after update
$afterVersion = & <binary> version 2>&1
Write-Host " After: $afterVersion"

if ($beforeVersion -eq $afterVersion) {
    Write-Host " !! Version unchanged — source may already be up to date" -ForegroundColor Yellow
}
`, repoPath, runPS1)
}
```

### UTF-8 BOM

Write the script with a UTF-8 BOM so PowerShell correctly handles Unicode characters (checkmarks, arrows, etc.):

```go
bom := []byte{0xEF, 0xBB, 0xBF}
tmpFile.Write(bom)
tmpFile.WriteString(script)
```

---

## Step 4 — Execute Script

```go
func runUpdateScript(scriptPath string) {
    cmd := exec.Command("powershell.exe",
        "-ExecutionPolicy", "Bypass",
        "-NoProfile", "-NoLogo",
        "-File", scriptPath)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    err := cmd.Run()
    // Handle error...
}
```

### PowerShell Flags

| Flag | Purpose |
|------|---------|
| `-ExecutionPolicy Bypass` | Allow running unsigned scripts |
| `-NoProfile` | Skip loading user profile (faster, predictable) |
| `-NoLogo` | Suppress PowerShell banner |
| `-File` | Execute a script file (not a command string) |

---

## Linux/macOS — No Handoff Needed

On Unix systems, the binary can be replaced in-place. The update command simply runs the build script directly:

```go
func executeUpdateUnix(repoPath string) {
    runSH := filepath.Join(repoPath, "run.sh")
    cmd := exec.Command("bash", runSH, "--update")
    cmd.Dir = repoPath
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    err := cmd.Run()
    // Handle error...
}
```

No temporary copy, no worker process, no generated scripts.

---

## Binary-Based Handoff (No Source Repo)

When the source repo is not available, use a standalone updater binary:

```
<binary> update
 │
 ├── Source repo found? YES → source-based handoff (above)
 └── Source repo found? NO  → delegate to <binary>-updater
     │
     ├── Copy <binary>-updater → <binary>-updater-tmp-<PID>.exe
     ├── Launch: <binary>-updater-tmp-<PID>.exe update-worker
     └── Worker:
         ├── Download install.ps1 from release assets
         ├── Execute install.ps1 (handles download + verify + install)
         ├── Verify version
         └── Clean up self
```

---

## Constraints

- The parent MUST wait synchronously (`cmd.Run()`) — never fire and forget.
- The worker runs from a DIFFERENT file than the original binary.
- Temporary scripts and copies are cleaned up after completion.
- The handoff copy name includes the PID to avoid collisions when multiple updates run simultaneously.
- On Unix, skip the handoff entirely — run `bash run.sh --update` directly.
- All output (stdout/stderr) must be piped to the terminal so the user sees progress.

---

## Cross-References

- [Self-Update Overview](./01-self-update-overview.md) — Where handoff fits in the update flow
- [Rename-First Deploy](./03-rename-first-deploy.md) — Used by the worker during deploy
- [Build Scripts](./04-build-scripts.md) — The scripts invoked by the worker
- [Cleanup](./06-cleanup.md) — Removing handoff copies after update

---

*Handoff mechanism — v3.1.0 — 2026-04-10*
