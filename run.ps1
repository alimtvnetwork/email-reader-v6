# run.ps1 — Bootstrap script for email-read
#
# Modes:
#   .\run.ps1          Show this help and exit (no side effects)
#   .\run.ps1 -i       INSTALL: git pull + go mod tidy. No build, no deploy.
#   .\run.ps1 -d       DEPLOY : git pull + go mod tidy +
#                              build CLI (email-read) +
#                              build UI  (email-read-ui) +
#                              ensure data/email folders +
#                              add to user PATH +
#                              launch the desktop UI.
#
# Optional modifiers (apply to -d):
#   -SkipPull          Skip the git pull step.
#   -SkipPathUpdate    Skip the user PATH update.
#   -NoUI              Skip building the desktop UI binary.
#   -NoLaunch          Build everything but do not launch the UI.
#   -CliOnly           Shorthand for -NoUI -NoLaunch.
#
# Examples:
#   .\run.ps1                       # show help
#   .\run.ps1 -i                    # just refresh source + Go modules
#   .\run.ps1 -d                    # full build + deploy + launch UI
#   .\run.ps1 -d -NoLaunch          # build CLI + UI, don't launch
#   .\run.ps1 -d -CliOnly           # build only CLI (legacy behaviour)
#
# Requires: git, go (1.22+), Windows PowerShell 5+ or PowerShell 7+.
# UI build needs: cgo + a working C toolchain.
#   On Windows: install TDM-GCC or MSYS2 mingw-w64 and ensure gcc is on PATH.

[CmdletBinding()]
param(
    [Alias('i')]
    [switch]$Install,
    [Alias('d')]
    [switch]$Deploy,
    [switch]$SkipPull,
    [switch]$SkipPathUpdate,
    [switch]$NoUI,
    [switch]$NoLaunch,
    [switch]$CliOnly
)

$ErrorActionPreference = 'Stop'

function Write-Step($msg)     { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)       { Write-Host "    $msg" -ForegroundColor Green }
function Write-WarnLine($msg) { Write-Host "    $msg" -ForegroundColor Yellow }
function Write-Fail($msg)     { Write-Host "ERROR: $msg" -ForegroundColor Red }

function Show-Usage {
    Write-Host ""
    Write-Host "email-read bootstrap" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage:" -ForegroundColor White
    Write-Host "  .\run.ps1 -i       Install deps only (git pull + go mod tidy)"
    Write-Host "  .\run.ps1 -d       Deploy app (pull + tidy + build CLI + build UI + PATH + launch UI)"
    Write-Host ""
    Write-Host "Modifiers (apply to -d):" -ForegroundColor White
    Write-Host "  -SkipPull          Skip git pull"
    Write-Host "  -SkipPathUpdate    Skip user PATH update"
    Write-Host "  -NoUI              Don't build the desktop UI"
    Write-Host "  -NoLaunch          Build everything, but don't launch the UI"
    Write-Host "  -CliOnly           Shorthand for -NoUI -NoLaunch"
    Write-Host ""
    Write-Host "Run with no flags to see this help." -ForegroundColor DarkGray
    Write-Host ""
}

# --- Mode validation ---
if ($Install -and $Deploy) {
    Write-Fail "-i and -d are mutually exclusive. Pick one."
    Show-Usage
    exit 2
}
if (-not $Install -and -not $Deploy) {
    Show-Usage
    exit 0
}

# Resolve combined flags
if ($CliOnly) { $NoUI = $true; $NoLaunch = $true }

# --- Resolve paths ---
$RepoRoot  = Split-Path -Parent $MyInvocation.MyCommand.Definition
$DeployDir = Join-Path $RepoRoot 'email-reader-cli'

# Detect host OS so this script works on Windows, macOS, and Linux.
$IsWindowsHost = $true
if ($PSVersionTable.PSEdition -eq 'Core') {
    $IsWindowsHost = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::Windows)
}

$ExeName  = if ($IsWindowsHost) { 'email-read.exe'    } else { 'email-read'    }
$UIName   = if ($IsWindowsHost) { 'email-read-ui.exe' } else { 'email-read-ui' }
$ExePath  = Join-Path $DeployDir $ExeName
$UIPath   = Join-Path $DeployDir $UIName
$DataDir  = Join-Path $DeployDir 'data'
$MailDir  = Join-Path $DeployDir 'email'

Set-Location $RepoRoot

# =====================================================================
# Step A: git pull (shared by -i and -d, unless -SkipPull)
# =====================================================================
if ($SkipPull) {
    Write-Step "Skipping git pull (-SkipPull)"
} else {
    Write-Step "git pull"
    try {
        git pull --ff-only
        Write-Ok "Repo up to date."
    } catch {
        Write-WarnLine "git pull failed: $($_.Exception.Message). Continuing with local code."
    }
}

# =====================================================================
# Step B: Auto-install Go toolchain + C build deps (Windows)
# =====================================================================
function Install-WithWinget($id, $name) {
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if (-not $winget) { return $false }
    Write-Step "Installing $name via winget"
    winget install --id $id --silent --accept-source-agreements --accept-package-agreements
    return ($LASTEXITCODE -eq 0)
}

function Install-WithChoco($pkg, $name) {
    $choco = Get-Command choco -ErrorAction SilentlyContinue
    if (-not $choco) { return $false }
    Write-Step "Installing $name via Chocolatey"
    choco install $pkg -y --no-progress
    return ($LASTEXITCODE -eq 0)
}

function Refresh-Path {
    $machine = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $user    = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:Path = ($machine, $user -join ';')
}

function Ensure-Go {
    if (Get-Command go -ErrorAction SilentlyContinue) { return }
    Write-Step "Go not found - installing automatically"
    if (-not $IsWindowsHost) {
        throw "Auto-install on non-Windows hosts: please run ./run.sh -d instead."
    }
    $ok = Install-WithWinget 'GoLang.Go' 'Go'
    if (-not $ok) { $ok = Install-WithChoco 'golang' 'Go' }
    if (-not $ok) {
        throw "Could not auto-install Go. Install winget or Chocolatey, or grab Go from https://go.dev/dl/."
    }
    Refresh-Path
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go installed but not on PATH yet. Open a NEW PowerShell window and re-run .\run.ps1 -d"
    }
    Write-Ok ("Installed: {0}" -f (& go version))
}

function Ensure-WindowsCC {
    if (-not $IsWindowsHost) { return }
    if ($NoUI) { return }
    if (Get-Command gcc -ErrorAction SilentlyContinue) {
        Write-Ok "gcc already available."
        return
    }
    Write-Step "C compiler (gcc) not found - installing MinGW for cgo/UI build"
    $ok = Install-WithWinget 'BrechtSanders.WinLibs.POSIX.UCRT' 'MinGW (WinLibs)'
    if (-not $ok) { $ok = Install-WithChoco 'mingw' 'MinGW' }
    if (-not $ok) {
        Write-WarnLine "Could not auto-install MinGW. Install TDM-GCC or MSYS2 manually,"
        Write-WarnLine "or re-run with -NoUI to skip the desktop UI."
        return
    }
    Refresh-Path
}

Write-Step "Preparing system prerequisites (auto-install if needed)"
Ensure-Go
Ensure-WindowsCC

Write-Step "Checking Go toolchain"
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    throw "Go still not on PATH after install attempt. Open a new terminal and re-run."
}
Write-Ok ("Found {0}" -f (& go version))

# =====================================================================
# Step C: go mod tidy (shared)
# =====================================================================
Write-Step "Resolving Go module dependencies (go mod tidy)"
& go mod tidy
if ($LASTEXITCODE -ne 0) {
    throw "go mod tidy failed with exit code $LASTEXITCODE"
}
Write-Ok "Modules resolved."

# =====================================================================
# INSTALL MODE — stop here.
# =====================================================================
if ($Install) {
    Write-Host ""
    Write-Host "Install complete (-i): source pulled and Go modules resolved." -ForegroundColor Green
    Write-Host "Run '.\run.ps1 -d' when you want to build, deploy and launch the UI." -ForegroundColor DarkGray
    Write-Host ""
    exit 0
}

# =====================================================================
# DEPLOY MODE — build CLI + (optional) UI + PATH + launch
# =====================================================================
if (-not (Test-Path $DeployDir)) {
    New-Item -ItemType Directory -Path $DeployDir | Out-Null
}

if ($IsWindowsHost) {
    $env:GOOS   = 'windows'
    $env:GOARCH = 'amd64'
}

# --- Build CLI ---
Write-Step "Building $ExeName (CLI)"
& go build -o $ExePath ./cmd/email-read
if ($LASTEXITCODE -ne 0) {
    throw "go build (CLI) failed with exit code $LASTEXITCODE"
}
Write-Ok "Built: $ExePath"

# --- Build UI ---
if (-not $NoUI) {
    Write-Step "Building $UIName (desktop)"
    & go build -o $UIPath ./cmd/email-read-ui
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "UI build failed."
        if ($IsWindowsHost) {
            Write-Fail "On Windows install TDM-GCC or MSYS2 mingw-w64 and add gcc to PATH,"
            Write-Fail "or re-run with -NoUI to skip the UI."
        } else {
            Write-Fail "On macOS install Xcode CLT (xcode-select --install)."
            Write-Fail "On Linux install build-essential libgl1-mesa-dev xorg-dev."
        }
        throw "go build (UI) failed with exit code $LASTEXITCODE"
    }
    Write-Ok "Built: $UIPath"
} else {
    Write-Step "Skipping UI build (-NoUI / -CliOnly)"
}

# --- Ensure runtime folders exist ---
Write-Step "Ensuring data/ and email/ folders"
foreach ($d in @($DataDir, $MailDir)) {
    if (-not (Test-Path $d)) {
        New-Item -ItemType Directory -Path $d | Out-Null
        Write-Ok "Created $d"
    } else {
        Write-Ok "Exists  $d"
    }
}

# --- Idempotent user PATH update (Windows-only) ---
if (-not $IsWindowsHost) {
    Write-Step "Skipping PATH update (non-Windows host)"
    Write-WarnLine "Run the binaries directly: $ExePath / $UIPath"
} elseif ($SkipPathUpdate) {
    Write-Step "Skipping PATH update (-SkipPathUpdate)"
} else {
    Write-Step "Updating user PATH"
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($null -eq $userPath) { $userPath = '' }

    $existing = $userPath.Split(';') |
        Where-Object { $_ -ne '' } |
        ForEach-Object { $_.TrimEnd('\') }

    $target = $DeployDir.TrimEnd('\')

    if ($existing -contains $target) {
        Write-Ok "PATH already contains: $target"
    } else {
        $newPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $target
        } else {
            ($userPath.TrimEnd(';') + ';' + $target)
        }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        $env:Path = $env:Path.TrimEnd(';') + ';' + $target
        Write-Ok "Added to user PATH: $target"
        Write-WarnLine "Open a new terminal for the PATH change to take effect in other shells."
    }
}

# --- Done summary ---
Write-Host ""
Write-Host "email-read deployed successfully" -ForegroundColor Green
Write-Host ("  CLI : {0}" -f $ExePath)
if (-not $NoUI) {
    Write-Host ("  UI  : {0}" -f $UIPath)
}
Write-Host ("  Data: {0}" -f $DataDir)
Write-Host ("  Mail: {0}" -f $MailDir)
Write-Host ""
Write-Host "Try the CLI:" -ForegroundColor Cyan
Write-Host "  email-read --help       # Show all commands"
Write-Host "  email-read --version"
Write-Host "  email-read add"
Write-Host "  email-read list"

# --- Launch UI ---
if ((-not $NoUI) -and (-not $NoLaunch)) {
    Write-Host ""
    Write-Step "Launching desktop UI"
    try {
        Start-Process -FilePath $UIPath -WorkingDirectory $DeployDir | Out-Null
        Write-Ok "Started: $UIPath"
    } catch {
        Write-Fail "Failed to launch UI: $($_.Exception.Message)"
        Write-WarnLine "Run it manually: $UIPath"
    }
} elseif ((-not $NoUI) -and $NoLaunch) {
    Write-Host ""
    Write-Host "UI built but not launched (-NoLaunch). Run it manually:" -ForegroundColor DarkGray
    Write-Host "  $UIPath"
}
