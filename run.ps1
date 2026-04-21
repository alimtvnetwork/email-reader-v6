# run.ps1 — Bootstrap script for email-read CLI
#
# Responsibilities:
#   1. git pull latest changes in this repo
#   2. go build -> ./email-reader-cli/email-read.exe
#   3. Ensure data/ and email/ folders exist next to the EXE
#   4. Add the deploy folder to the user PATH (idempotent)
#   5. Print a success line with the deploy path
#
# Usage:
#   PS> .\run.ps1
#
# Requires: git, go (1.22+), Windows PowerShell 5+ or PowerShell 7+.

[CmdletBinding()]
param(
    [switch]$SkipPull,
    [switch]$SkipPathUpdate
)

$ErrorActionPreference = 'Stop'

function Write-Step($msg) {
    Write-Host "==> $msg" -ForegroundColor Cyan
}

function Write-Ok($msg) {
    Write-Host "    $msg" -ForegroundColor Green
}

function Write-WarnLine($msg) {
    Write-Host "    $msg" -ForegroundColor Yellow
}

# --- Resolve repo root (folder containing this script) ---
$RepoRoot  = Split-Path -Parent $MyInvocation.MyCommand.Definition
$DeployDir = Join-Path $RepoRoot 'email-reader-cli'
$ExePath   = Join-Path $DeployDir 'email-read.exe'
$DataDir   = Join-Path $DeployDir 'data'
$MailDir   = Join-Path $DeployDir 'email'

Set-Location $RepoRoot

# --- 1. git pull ---
if ($SkipPull) {
    Write-Step "Skipping git pull (--SkipPull)"
} else {
    Write-Step "git pull"
    try {
        git pull --ff-only
        Write-Ok "Repo up to date."
    } catch {
        Write-WarnLine "git pull failed: $($_.Exception.Message). Continuing with local code."
    }
}

# --- 2. Verify go is available ---
Write-Step "Checking Go toolchain"
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    throw "Go is not installed or not on PATH. Install Go 1.22+ from https://go.dev/dl/ and re-run."
}
Write-Ok ("Found {0}" -f (& go version))

# --- 3. Build the EXE ---
Write-Step "Building email-read.exe"
if (-not (Test-Path $DeployDir)) {
    New-Item -ItemType Directory -Path $DeployDir | Out-Null
}

$env:GOOS   = 'windows'
$env:GOARCH = 'amd64'

& go build -o $ExePath ./cmd/email-read
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}
Write-Ok "Built: $ExePath"

# --- 4. Ensure runtime folders exist ---
Write-Step "Ensuring data/ and email/ folders"
foreach ($d in @($DataDir, $MailDir)) {
    if (-not (Test-Path $d)) {
        New-Item -ItemType Directory -Path $d | Out-Null
        Write-Ok "Created $d"
    } else {
        Write-Ok "Exists  $d"
    }
}

# --- 5. Idempotent user PATH update ---
if ($SkipPathUpdate) {
    Write-Step "Skipping PATH update (--SkipPathUpdate)"
} else {
    Write-Step "Updating user PATH"
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($null -eq $userPath) { $userPath = '' }

    # Split, normalise, and check for an existing entry (case-insensitive, trailing-slash tolerant)
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
        # Also update current session so the user can use it immediately
        $env:Path = $env:Path.TrimEnd(';') + ';' + $target
        Write-Ok "Added to user PATH: $target"
        Write-WarnLine "Open a new terminal for the PATH change to take effect in other shells."
    }
}

# --- 6. Done ---
Write-Host ""
Write-Host "email-read deployed successfully" -ForegroundColor Green
Write-Host ("  EXE : {0}" -f $ExePath)
Write-Host ("  Data: {0}" -f $DataDir)
Write-Host ("  Mail: {0}" -f $MailDir)
Write-Host ""
Write-Host "Try it out:" -ForegroundColor Cyan
Write-Host "  email-read --version"
Write-Host "  email-read add"
Write-Host "  email-read list"
Write-Host "  email-read <alias>"
