#!/usr/bin/env bash
# run.sh — Bootstrap script for email-read (macOS / Linux)
#
# Modes:
#   ./run.sh           Show help and exit (no side effects)
#   ./run.sh -i        INSTALL: git pull + go mod tidy. No build, no deploy.
#   ./run.sh -d        DEPLOY : git pull + go mod tidy +
#                              build CLI (email-read) +
#                              build UI  (email-read-ui) +
#                              ensure data/email folders +
#                              launch the desktop UI.
#
# Optional modifiers (apply to -d):
#   --skip-pull        Skip the git pull step.
#   --no-ui            Skip building the desktop UI binary.
#   --no-launch        Build everything but do not launch the UI.
#   --cli-only         Shorthand for --no-ui --no-launch.
#
# Examples:
#   ./run.sh                       # show help
#   ./run.sh -i                    # just refresh source + Go modules
#   ./run.sh -d                    # full build + deploy + launch UI
#   ./run.sh -d --no-launch        # build CLI + UI, don't launch
#   ./run.sh -d --cli-only         # build only CLI (legacy behaviour)
#   ./run.sh -d --skip-pull        # build + deploy without pulling
#
# Requires: git, go (1.22+), bash 4+.
# UI build needs: cgo + a working C toolchain (Xcode CLT on macOS,
#                 build-essential + libgl1-mesa-dev + xorg-dev on Linux).

set -euo pipefail

# -- Defaults --------------------------------------------------------
INSTALL=false
DEPLOY=false
SKIP_PULL=false
BUILD_UI=true
LAUNCH_UI=true

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
  ./run.sh -d       Deploy app (pull + tidy + build CLI + build UI + launch UI)

Modifiers (apply to -d):
  --skip-pull       Skip git pull
  --no-ui           Don't build the desktop UI
  --no-launch       Build everything, but don't launch the UI
  --cli-only        Shorthand for --no-ui --no-launch

${C_GRAY}Run with no flags to see this help.${C_RESET}

EOF
}

# -- Parse arguments -------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        -i|--install)   INSTALL=true;        shift ;;
        -d|--deploy)    DEPLOY=true;         shift ;;
        --skip-pull)    SKIP_PULL=true;      shift ;;
        --no-ui)        BUILD_UI=false;      shift ;;
        --no-launch)    LAUNCH_UI=false;     shift ;;
        --cli-only)     BUILD_UI=false; LAUNCH_UI=false; shift ;;
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
UI_PATH="$DEPLOY_DIR/email-read-ui"
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
    if git pull --ff-only 2>/dev/null; then
        ok "Repo up to date."
    else
        warn "git pull failed or no upstream. Continuing with local code."
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
    echo "${C_GRAY}Run './run.sh -d' when you want to build, deploy and launch the UI.${C_RESET}"
    echo ""
    exit 0
fi

# ===================================================================
# DEPLOY MODE — build CLI + (optional) UI
# ===================================================================
mkdir -p "$DEPLOY_DIR"

# --- Build CLI ---
step "Building email-read (CLI)"
go build -o "$EXE_PATH" ./cmd/email-read
ok "Built: $EXE_PATH"

# --- Build UI ---
if $BUILD_UI; then
    step "Building email-read-ui (desktop)"
    # The "ld: warning: ignoring duplicate libraries: '-lobjc'" line on
    # macOS is a harmless linker notice from Apple's new linker — the
    # binary is produced successfully. We let it through but mark it OK.
    if go build -o "$UI_PATH" ./cmd/email-read-ui 2> >(grep -v 'ignoring duplicate libraries' >&2); then
        ok "Built: $UI_PATH"
    else
        fail "UI build failed. On macOS install Xcode CLT: xcode-select --install"
        fail "On Linux install: build-essential libgl1-mesa-dev xorg-dev"
        exit 1
    fi
else
    step "Skipping UI build (--no-ui / --cli-only)"
fi

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

# --- Done summary ---
echo ""
echo "${C_GREEN}email-read deployed successfully${C_RESET}"
echo "  CLI : $EXE_PATH"
if $BUILD_UI; then
    echo "  UI  : $UI_PATH"
fi
echo "  Data: $DATA_DIR"
echo "  Mail: $MAIL_DIR"
echo ""
echo "${C_CYAN}Try the CLI:${C_RESET}"
echo "  $EXE_PATH --help"
echo "  $EXE_PATH --version"
echo "  $EXE_PATH add"
echo "  $EXE_PATH list"

# --- Launch UI ---
if $BUILD_UI && $LAUNCH_UI; then
    echo ""
    step "Launching desktop UI"
    # Run detached so the script returns immediately.
    # stdout/stderr go to a log file inside the deploy dir for inspection.
    UI_LOG="$DEPLOY_DIR/ui.log"
    nohup "$UI_PATH" >"$UI_LOG" 2>&1 &
    UI_PID=$!
    disown "$UI_PID" 2>/dev/null || true
    ok "Started: $UI_PATH (pid $UI_PID)"
    ok "Log    : $UI_LOG"
elif $BUILD_UI && ! $LAUNCH_UI; then
    echo ""
    echo "${C_GRAY}UI built but not launched (--no-launch). Run it manually:${C_RESET}"
    echo "  $UI_PATH"
fi
