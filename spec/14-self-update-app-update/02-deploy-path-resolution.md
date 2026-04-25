# Deploy Path Resolution

**Version:** 3.1.0  
**Updated:** 2026-04-16

---

## Purpose

Define how the build/deploy script determines **where to install** the new binary. The resolution must handle first-time installs, existing installs, and explicit overrides.

---

## 3-Tier Resolution Strategy

The deploy target is resolved with this priority:

| Priority | Source | When Used |
|----------|--------|-----------|
| 1 | **CLI flag** (`--deploy-path`) | User explicitly specifies a path |
| 2 | **Global PATH lookup** | Binary is already installed and on PATH |
| 3 | **Config file default** | First-time install or binary not on PATH |

### Tier 1 — CLI Flag Override

If the user passes `--deploy-path <path>`, use it unconditionally:

```powershell
# PowerShell
if ($DeployPath.Length -gt 0) {
    return $DeployPath
}
```

```bash
# Bash
if [[ -n "$DEPLOY_PATH" ]]; then
    echo "$DEPLOY_PATH"
    return
fi
```

### Tier 2 — Global PATH Lookup

If the binary is already installed and accessible via `PATH`, detect its current location and deploy there:

```powershell
# PowerShell
$activeCmd = Get-Command <binary> -ErrorAction SilentlyContinue
if ($activeCmd) {
    $activePath = $activeCmd.Source
    $resolvedPath = (Resolve-Path $activePath).Path
    $activeDir = Split-Path $resolvedPath -Parent
    $dirName = Split-Path $activeDir -Leaf

    # Binary lives in <name>/<name>.exe
    if ($dirName -eq "<binary>") {
        return Split-Path $activeDir -Parent
    }

    # Binary is directly in a folder
    return Split-Path $activeDir -Parent
}
```

```bash
# Bash
active_cmd=$(command -v <binary> 2>/dev/null || true)
if [[ -n "$active_cmd" ]] && [[ -f "$active_cmd" ]]; then
    resolved=$(readlink -f "$active_cmd" 2>/dev/null || echo "$active_cmd")
    active_dir=$(dirname "$resolved")
    dir_name=$(basename "$active_dir")

    # Binary lives in <name>/<name>
    if [[ "$dir_name" == "<binary>" ]]; then
        echo "$(dirname "$active_dir")"
        return
    fi

    echo "$(dirname "$active_dir")"
    return
fi
```

#### Symlink Resolution

On Linux/macOS, the PATH binary may be a symlink. Always resolve symlinks before extracting the directory:

```bash
# readlink -f follows all symlinks to the final target
resolved=$(readlink -f "$active_cmd")
```

On macOS, `readlink -f` may not be available. Fall back:

```bash
resolved=$(python3 -c "import os; print(os.path.realpath('$active_cmd'))" 2>/dev/null || echo "$active_cmd")
```

#### Nested Directory Detection

Most CLI deploy structures use a nested folder:

```
<deploy-dir>/
└── <binary>/
    ├── <binary> (or <binary>.exe)
    └── data/
```

When detecting the PATH location, check if the binary's parent directory matches the binary name. If so, the deploy target is the **grandparent** directory.

### Tier 3 — Config File Default

If the binary is not on PATH (first-time install), read the deploy path from the user configuration file.

See [Configuration File](./15-config-file.md) for the full schema and file location.

| Platform | Config Location | Default `deployPath` |
|----------|----------------|---------------------|
| Windows | `%APPDATA%\<binary>\config.json` | `%LOCALAPPDATA%\<binary>` |
| Linux | `~/.config/<binary>/config.json` | `$HOME/.local/bin` |
| macOS | `~/Library/Application Support/<binary>/config.json` | `$HOME/.local/bin` |

```go
cfg, err := LoadConfig()
if err == nil && cfg.DeployPath != "" {
    return cfg.DeployPath
}
// Final fallback if no config exists
return defaultDeployPath() // platform-specific default
```

For PowerShell project-level configs (e.g., `powershell.json`):
```json
{
    "deployPath": "E:\\bin-run"
}
```

---

## Complete Resolution Function

### PowerShell

```powershell
function Resolve-DeployTarget {
    param($Config, $OverridePath)

    # 1) CLI override
    if ($OverridePath.Length -gt 0) {
        Write-Info "Deploy target: CLI override -> $OverridePath"
        return $OverridePath
    }

    # 2) PATH detection
    $activeCmd = Get-Command <binary> -ErrorAction SilentlyContinue
    if ($activeCmd) {
        $activePath = (Resolve-Path $activeCmd.Source).Path
        $activeDir = Split-Path $activePath -Parent
        $dirName = Split-Path $activeDir -Leaf

        if ($dirName -eq "<binary>") {
            $target = Split-Path $activeDir -Parent
        } else {
            $target = Split-Path $activeDir -Parent
        }

        Write-Info "Deploy target: detected from PATH -> $target"
        return $target
    }

    # 3) Config default
    Write-Info "Deploy target: config default -> $($Config.deployPath)"
    return $Config.deployPath
}
```

### Bash

```bash
resolve_deploy_target() {
    # 1) CLI override
    if [[ -n "$DEPLOY_PATH" ]]; then
        echo "$DEPLOY_PATH"
        return
    fi

    # 2) PATH detection
    local active_cmd
    active_cmd=$(command -v <binary> 2>/dev/null || true)
    if [[ -n "$active_cmd" ]] && [[ -f "$active_cmd" ]]; then
        local resolved
        resolved=$(readlink -f "$active_cmd" 2>/dev/null || echo "$active_cmd")
        local active_dir
        active_dir=$(dirname "$resolved")
        local dir_name
        dir_name=$(basename "$active_dir")

        if [[ "$dir_name" == "<binary>" ]]; then
            echo "$(dirname "$active_dir")"
        else
            echo "$(dirname "$active_dir")"
        fi
        return
    fi

    # 3) Config default
    echo "$DEPLOY_TARGET"
}
```

---

## Installed Directory Command

Provide a utility command for users to check where the binary is installed:

```
<binary> installed-dir
```

Output:

```
 📂 Installed directory

    Binary:    /home/user/.local/bin/<binary>
    Directory: /home/user/.local/bin/<binary>/
```

Implementation:

```go
func runInstalledDir() {
    selfPath, err := os.Executable()
    if err != nil {
        fmt.Fprintf(os.Stderr, " ✗ Could not resolve executable path: %v\n", err)
        os.Exit(1)
    }

    resolved, err := filepath.EvalSymlinks(selfPath)
    if err != nil {
        resolved = selfPath
    }

    absPath, _ := filepath.Abs(resolved)
    dir := filepath.Dir(absPath)

    fmt.Printf("\n 📂 Installed directory\n\n")
    fmt.Printf("    Binary:    %s\n", absPath)
    fmt.Printf("    Directory: %s\n\n", dir)
}
```

---

## Constraints

- CLI flag always takes highest priority — no exceptions.
- PATH detection must resolve symlinks before extracting directories.
- The config file default is only used when the binary is not found on PATH (first-time installs).
- Never prompt the user for a deploy path during the build script — resolution must be fully automatic.
- Log which tier was used so the user can see where the binary will be deployed.

---

## Cross-References

- [Self-Update Overview](./01-self-update-overview.md) — Overall update flow
- [Rename-First Deploy](./03-rename-first-deploy.md) — File replacement after path resolution
- [Build Scripts](./04-build-scripts.md) — Scripts that use deploy path resolution
- [Configuration File](./15-config-file.md) — Tier 3 config file schema and location

---

*Deploy path resolution — v3.1.0 — 2026-04-13*
