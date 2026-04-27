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

# -- OS detection --------------------------------------------------
OS_KIND="unknown"
case "$(uname -s)" in
    Darwin*) OS_KIND="macos" ;;
    Linux*)  OS_KIND="linux" ;;
esac

# Run a command with sudo if available; otherwise run directly.
maybe_sudo() {
    if [[ $EUID -eq 0 ]]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        sudo "$@"
    else
        warn "sudo not available; trying without elevation."
        "$@"
    fi
}

# ---- Auto-install: Xcode Command Line Tools (macOS) ----
ensure_xcode_clt() {
    [[ "$OS_KIND" == "macos" ]] || return 0
    if xcode-select -p >/dev/null 2>&1; then
        ok "Xcode Command Line Tools already installed."
        return 0
    fi
    step "Installing Xcode Command Line Tools (one-time, ~5 min)"
    warn "A system dialog will appear. Click 'Install' and wait for it to finish."
    # Trigger the GUI installer and poll until it completes.
    xcode-select --install >/dev/null 2>&1 || true
    local waited=0
    until xcode-select -p >/dev/null 2>&1; do
        sleep 5
        waited=$((waited + 5))
        if (( waited % 30 == 0 )); then
            warn "Still waiting for Xcode CLT install... (${waited}s)"
        fi
        if (( waited > 1800 )); then
            fail "Xcode CLT install timed out after 30 min. Run 'xcode-select --install' manually."
            exit 1
        fi
    done
    ok "Xcode Command Line Tools installed."
}

# ---- Auto-install: Homebrew (macOS) ----
ensure_homebrew() {
    [[ "$OS_KIND" == "macos" ]] || return 0
    if command -v brew >/dev/null 2>&1; then
        ok "Homebrew already installed."
        return 0
    fi
    step "Installing Homebrew (needed to install Go automatically)"
    NONINTERACTIVE=1 /bin/bash -c \
        "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    # Add brew to PATH for this shell session.
    if [[ -x /opt/homebrew/bin/brew ]]; then
        eval "$(/opt/homebrew/bin/brew shellenv)"
    elif [[ -x /usr/local/bin/brew ]]; then
        eval "$(/usr/local/bin/brew shellenv)"
    fi
    ok "Homebrew installed."
}

# ---- Auto-install: Linux build deps ----
ensure_linux_build_deps() {
    [[ "$OS_KIND" == "linux" ]] || return 0
    # Skip if all key headers/tools already present.
    if command -v gcc >/dev/null 2>&1 && \
       ldconfig -p 2>/dev/null | grep -q libGL.so && \
       [[ -f /usr/include/X11/Xlib.h || -f /usr/include/X11/Xrandr.h ]]; then
        ok "Linux build dependencies already installed."
        return 0
    fi
    step "Installing Linux build dependencies (gcc, OpenGL, X11 headers)"
    if command -v apt-get >/dev/null 2>&1; then
        maybe_sudo apt-get update -y
        maybe_sudo apt-get install -y \
            build-essential pkg-config \
            libgl1-mesa-dev xorg-dev libxkbcommon-dev
    elif command -v dnf >/dev/null 2>&1; then
        maybe_sudo dnf install -y \
            gcc gcc-c++ make pkgconfig \
            mesa-libGL-devel libX11-devel libXcursor-devel \
            libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel
    elif command -v pacman >/dev/null 2>&1; then
        maybe_sudo pacman -Sy --noconfirm \
            base-devel mesa libx11 libxcursor libxrandr libxinerama libxi
    else
        warn "Unknown Linux package manager. Install gcc + OpenGL/X11 headers manually."
    fi
    ok "Linux build dependencies installed."
}

# ---- Auto-install: Go toolchain ----
ensure_go() {
    if command -v go >/dev/null 2>&1; then
        return 0
    fi
    step "Installing Go toolchain automatically"
    case "$OS_KIND" in
        macos)
            ensure_homebrew
            brew install go
            ;;
        linux)
            if command -v apt-get >/dev/null 2>&1; then
                maybe_sudo apt-get update -y
                maybe_sudo apt-get install -y golang-go
            elif command -v dnf >/dev/null 2>&1; then
                maybe_sudo dnf install -y golang
            elif command -v pacman >/dev/null 2>&1; then
                maybe_sudo pacman -Sy --noconfirm go
            else
                fail "Could not auto-install Go: unknown package manager."
                fail "Install Go 1.22+ manually from https://go.dev/dl/ and re-run."
                exit 1
            fi
            ;;
        *)
            fail "Unsupported OS for auto-install. Install Go 1.22+ from https://go.dev/dl/."
            exit 1
            ;;
    esac
    if ! command -v go >/dev/null 2>&1; then
        fail "Go install reported success but 'go' is still not on PATH."
        fail "Open a new terminal and re-run ./run.sh -d"
        exit 1
    fi
    ok "Go installed: $(go version)"
}

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
# Step B: Auto-install OS prerequisites + Go toolchain
# ===================================================================
step "Preparing system prerequisites (auto-install if needed)"
if [[ "$OS_KIND" == "macos" ]]; then
    ensure_xcode_clt
elif [[ "$OS_KIND" == "linux" && "$BUILD_UI" == "true" ]]; then
    ensure_linux_build_deps
fi

step "Checking Go toolchain"
ensure_go
ok "Found $(go version)"

# ===================================================================
# Step C: go mod tidy (shared)
# ===================================================================
step "Resolving Go module dependencies (go mod tidy)"
go mod tidy
ok "Modules resolved."

# ===================================================================
# Step C.1: Error-trace lint guardrails (Phase 1, warn-only)
# These print today's error-handling debt without failing the build.
# Set LINT_MODE=fail in the environment to enforce in CI.
# ===================================================================
step "Error-trace guardrails (warn-only)"
for guard in \
    "$REPO_ROOT/linter-scripts/check-no-fmt-errorf.sh" \
    "$REPO_ROOT/linter-scripts/check-no-bare-return-err.sh" \
    "$REPO_ROOT/linter-scripts/check-no-errors-new.sh"
do
    if [[ -x "$guard" ]]; then
        bash "$guard" || true
    elif [[ -f "$guard" ]]; then
        bash "$guard" || true
    fi
done

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
