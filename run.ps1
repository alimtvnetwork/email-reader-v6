# run.ps1 — Bootstrap script for email-read CLI
#
# Modes:
#   .\run.ps1          Show this help and exit (no side effects)
#   .\run.ps1 -i       INSTALL: git pull + go mod tidy. No build, no deploy.
#   .\run.ps1 -d       DEPLOY : git pull + go mod tidy + go build +
#                              ensure data/email folders + add to user PATH.
#
# Optional modifiers (apply to -d):
#   -SkipPull          Skip the git pull step.
#   -SkipPathUpdate    Skip the user PATH update.
#
# Examples:
#   .\run.ps1                       # show help
#   .\run.ps1 -i                    # just refresh source + Go modules
#   .\run.ps1 -d                    # full build + deploy
#   .\run.ps1 -d -SkipPull          # build + deploy without pulling
#
# Requires: git, go (1.22+), Windows PowerShell 5+ or PowerShell 7+.

[CmdletBinding()]
param(
    [Alias('i')]
    [switch]$Install,
    [Alias('d')]
    [switch]$Deploy,
    [switch]$SkipPull,
    [switch]$SkipPathUpdate
)

$ErrorActionPreference = 'Stop'

function Write-Step($msg)     { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)       { Write-Host "    $msg" -ForegroundColor Green }
function Write-WarnLine($msg) { Write-Host "    $msg" -ForegroundColor Yellow }

function Show-Usage {
    Write-Host ""
    Write-Host "email-read bootstrap" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage:" -ForegroundColor White
    Write-Host "  .\run.ps1 -i       Install deps only (git pull + go mod tidy)"
    Write-Host "  .\run.ps1 -d       Deploy app (pull + tidy + build + PATH)"
    Write-Host ""
    Write-Host "Modifiers (apply to -d):" -ForegroundColor White
    Write-Host "  -SkipPull          Skip git pull"
    Write-Host "  -SkipPathUpdate    Skip user PATH update"
    Write-Host ""
    Write-Host "Run with no flags to see this help." -ForegroundColor DarkGray
    Write-Host ""
}

# --- Mode validation ---
if ($Install -and $Deploy) {
    Write-Host "ERROR: -i and -d are mutually exclusive. Pick one." -ForegroundColor Red
    Show-Usage
    exit 2
}
if (-not $Install -and -not $Deploy) {
    Show-Usage
    exit 0
}

# --- Resolve paths ---
$RepoRoot  = Split-Path -Parent $MyInvocation.MyCommand.Definition
$DeployDir = Join-Path $RepoRoot 'email-reader-cli'

# Detect host OS so this script works on Windows, macOS, and Linux.
$IsWindowsHost = $true
if ($PSVersionTable.PSEdition -eq 'Core') {
    $IsWindowsHost = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::Windows)
}

$ExeName = if ($IsWindowsHost) { 'email-read.exe' } else { 'email-read' }
$ExePath = Join-Path $DeployDir $ExeName
$DataDir = Join-Path $DeployDir 'data'
$MailDir = Join-Path $DeployDir 'email'

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
# Step B: Verify Go toolchain (shared)
# =====================================================================
Write-Step "Checking Go toolchain"
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    throw "Go is not installed or not on PATH. Install Go 1.22+ from https://go.dev/dl/ and re-run."
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
    Write-Host "Run '.\run.ps1 -d' when you want to build and deploy." -ForegroundColor DarkGray
    Write-Host ""
    exit 0
}

# =====================================================================
# DEPLOY MODE — build + deploy + PATH
# =====================================================================
Write-Step "Building $ExeName"
if (-not (Test-Path $DeployDir)) {
    New-Item -ItemType Directory -Path $DeployDir | Out-Null
}

if ($IsWindowsHost) {
    $env:GOOS   = 'windows'
    $env:GOARCH = 'amd64'
}

& go build -o $ExePath ./cmd/email-read
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}
Write-Ok "Built: $ExePath"

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
    Write-WarnLine "Run the binary directly: $ExePath"
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

# --- Done ---
Write-Host ""
Write-Host "email-read deployed successfully" -ForegroundColor Green
Write-Host ("  EXE : {0}" -f $ExePath)
Write-Host ("  Data: {0}" -f $DataDir)
Write-Host ("  Mail: {0}" -f $MailDir)
Write-Host ""
Write-Host "Try it out:" -ForegroundColor Cyan
Write-Host "  email-read --help       # Show all commands"
Write-Host "  email-read --version"
Write-Host "  email-read add"
Write-Host "  email-read list"
Write-Host "  email-read <alias>"
