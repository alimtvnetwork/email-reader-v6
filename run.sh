#!/usr/bin/env bash
# run.sh — Bootstrap script for email-read CLI (macOS / Linux)
#
# Modes:
#   ./run.sh           Show help and exit (no side effects)
#   ./run.sh -i        INSTALL: git pull + go mod tidy. No build, no deploy.
#   ./run.sh -d        DEPLOY : git pull + go mod tidy + go build +
#                              ensure data/email folders.
#
# Optional modifiers (apply to -d):
#   --skip-pull        Skip the git pull step.
#
# Examples:
#   ./run.sh                  # show help
#   ./run.sh -i               # just refresh source + Go modules
#   ./run.sh -d               # full build + deploy
#   ./run.sh -d --skip-pull   # build + deploy without pulling
#
# Requires: git, go (1.22+), bash 4+.

set -euo pipefail

# -- Defaults --------------------------------------------------------
INSTALL=false
DEPLOY=false
SKIP_PULL=false

# -- Colors ----------------------------------------------------------
if [[ -t 1 ]]; then
    C_CYAN=$'\033[36m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'
    C_RED=$'\033[31m';  C_GRAY=$'\033[90m';  C_RESET=$'\033[0m'
else
    C_CYAN=""; C_GREEN=""; C_YELLOW=""; C_RED=""; C_GRAY=""; C_RESET=""
fi

step()  { echo "${C_CYAN}==> $*${C_RESET}"; }
ok()    { echo "    ${C_GREEN}$*${C_RESET}"; }
warn()  { echo "    ${C_YELLOW}$*${C_RESET}"; }
fail()  { echo "${C_RED}ERROR: $*${C_RESET}" >&2; }

show_usage() {
    cat <<EOF

${C_CYAN}email-read bootstrap${C_RESET}

Usage:
  ./run.sh -i       Install deps only (git pull + go mod tidy)
  ./run.sh -d       Deploy app (pull + tidy + build)

Modifiers (apply to -d):
  --skip-pull       Skip git pull

${C_GRAY}Run with no flags to see this help.${C_RESET}

EOF
}

# -- Parse arguments -------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        -i|--install)   INSTALL=true;   shift ;;
        -d|--deploy)    DEPLOY=true;    shift ;;
        --skip-pull)    SKIP_PULL=true; shift ;;
        -h|--help)      show_usage; exit 0 ;;
        *)
            fail "Unknown option: $1"
            show_usage
            exit 2
            ;;
    esac
done

# -- Mode validation -------------------------------------------------
if $INSTALL && $DEPLOY; then
    fail "-i and -d are mutually exclusive. Pick one."
    show_usage
    exit 2
fi
if ! $INSTALL && ! $DEPLOY; then
    show_usage
    exit 0
fi

# -- Resolve paths ---------------------------------------------------
REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$REPO_ROOT/email-reader-cli"
EXE_PATH="$DEPLOY_DIR/email-read"
DATA_DIR="$DEPLOY_DIR/data"
MAIL_DIR="$DEPLOY_DIR/email"

cd "$REPO_ROOT"

# ===================================================================
# Step A: git pull (shared)
# ===================================================================
if $SKIP_PULL; then
    step "Skipping git pull (--skip-pull)"
else
    step "git pull"
    if git pull --ff-only; then
        ok "Repo up to date."
    else
        warn "git pull failed. Continuing with local code."
    fi
fi

# ===================================================================
# Step B: Verify Go toolchain (shared)
# ===================================================================
step "Checking Go toolchain"
if ! command -v go >/dev/null 2>&1; then
    fail "Go is not installed or not on PATH. Install Go 1.22+ from https://go.dev/dl/"
    exit 1
fi
ok "Found $(go version)"

# ===================================================================
# Step C: go mod tidy (shared)
# ===================================================================
step "Resolving Go module dependencies (go mod tidy)"
go mod tidy
ok "Modules resolved."

# ===================================================================
# INSTALL MODE — stop here.
# ===================================================================
if $INSTALL; then
    echo ""
    echo "${C_GREEN}Install complete (-i): source pulled and Go modules resolved.${C_RESET}"
    echo "${C_GRAY}Run './run.sh -d' when you want to build and deploy.${C_RESET}"
    echo ""
    exit 0
fi

# ===================================================================
# DEPLOY MODE — build + deploy
# ===================================================================
step "Building email-read"
mkdir -p "$DEPLOY_DIR"

go build -o "$EXE_PATH" ./cmd/email-read
ok "Built: $EXE_PATH"

# --- Ensure runtime folders exist ---
step "Ensuring data/ and email/ folders"
for d in "$DATA_DIR" "$MAIL_DIR"; do
    if [[ -d "$d" ]]; then
        ok "Exists  $d"
    else
        mkdir -p "$d"
        ok "Created $d"
    fi
done

# --- Done ---
echo ""
echo "${C_GREEN}email-read deployed successfully${C_RESET}"
echo "  EXE : $EXE_PATH"
echo "  Data: $DATA_DIR"
echo "  Mail: $MAIL_DIR"
echo ""
echo "${C_CYAN}Try it out:${C_RESET}"
echo "  $EXE_PATH --help"
echo "  $EXE_PATH --version"
echo "  $EXE_PATH add"
echo "  $EXE_PATH list"
