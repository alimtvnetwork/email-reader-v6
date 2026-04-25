<#
.SYNOPSIS
    Pull latest changes and run the coding guidelines validator (Go edition).

.DESCRIPTION
    This script:
    1. Performs a `git pull` to fetch the latest changes.
    2. Runs the Go-based coding guidelines validator against the specified path.
    Use -d to skip validation and only pull.

.PARAMETER Path
    Directory to scan (default: "src").

.PARAMETER Json
    Output results as JSON.

.PARAMETER MaxLines
    Maximum allowed function body lines (default: 15).

.PARAMETER d
    Skip validation — only perform git pull.

.EXAMPLE
    .\scripts\run.ps1
    .\scripts\run.ps1 -d
    .\scripts\run.ps1 -Path "cmd" -MaxLines 20
#>

param(
    [string]$Path = "src",
    [switch]$Json,
    [int]$MaxLines = 15,
    [switch]$d
)

$ErrorActionPreference = "Stop"

# ── Colors ──────────────────────────────────────────────────────────
function Write-Header { param([string]$Text) Write-Host "`n═══ $Text ═══" -ForegroundColor Cyan }

# ── Step 1: Git Pull ───────────────────────────────────────────────
Write-Header "Step 1 — git pull"
try {
    git pull
    if ($LASTEXITCODE -ne 0) {
        Write-Host "⚠️  git pull returned exit code $LASTEXITCODE" -ForegroundColor Yellow
    } else {
        Write-Host "✅ Repository up to date." -ForegroundColor Green
    }
} catch {
    Write-Host "⚠️  git pull failed: $_" -ForegroundColor Yellow
    Write-Host "   Continuing with local files..." -ForegroundColor Gray
}

# ── Skip validation if -d flag ─────────────────────────────────────
if ($d) {
    Write-Host "`n⏭️  Skipping validation (-d flag)." -ForegroundColor Yellow
    exit 0
}

# ── Step 2: Run Go Validator ───────────────────────────────────────
Write-Header "Step 2 — Running coding guidelines validator"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$goFile    = Join-Path $scriptDir "validate-guidelines.go"

if (-not (Test-Path $goFile)) {
    Write-Host "❌ Cannot find $goFile" -ForegroundColor Red
    exit 1
}

# Check Go is installed
try {
    $goVersion = & go version 2>&1
    Write-Host "Using $goVersion" -ForegroundColor Gray
} catch {
    Write-Host "❌ Go is not installed or not in PATH." -ForegroundColor Red
    Write-Host "   Install from https://go.dev/dl/" -ForegroundColor Gray
    exit 1
}

# Build args
$goArgs = @("run", $goFile, "--path", $Path, "--max-lines", $MaxLines)
if ($Json) {
    $goArgs += "--json"
}

Write-Host "Scanning: $Path (max $MaxLines lines/function)`n" -ForegroundColor Gray

# Run
& go @goArgs
$exitCode = $LASTEXITCODE

# ── Summary ────────────────────────────────────────────────────────
Write-Host ""
if ($exitCode -eq 0) {
    Write-Host "✅ Validation passed!" -ForegroundColor Green
} else {
    Write-Host "❌ Validation failed with CODE RED violations." -ForegroundColor Red
}

exit $exitCode
