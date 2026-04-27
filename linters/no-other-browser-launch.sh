#!/bin/bash
# ============================================================
# No Other Browser-Launch Sites Linter
# ============================================================
# Enforces spec/21-app/02-features/06-tools/99-consistency-report.md
# row OI-5 / Acceptance A-01:
#
#   "Only `internal/browser/` may shell out to a system browser
#    launcher (`xdg-open`, `open`, `start`, `cmd /c start`,
#    `gnome-open`, `kde-open`, `wslview`)."
#
# All other call sites must route through the typed
# `*browser.Launcher` injected via `Services.OpenURL` (see Slice
# #117 — `BrowserFactory` lift).
#
# Usage:    bash linters/no-other-browser-launch.sh
# Exit 0:   only allowlisted files reference a system launcher
# Exit 1:   one or more rogue references found (printed to stderr)
#
# Companion test:
#   internal/ui/views/cf_r2_ast_guard_test.go ::
#       Tools_NoOtherFile_ShellsOutToBrowser
#
# Scope:    every *.go file under the repository root except:
#   - internal/browser/**         (the canonical launcher)
#   - **/*_test.go                (tests document anti-patterns)
#   - linters/**                  (this script's own banner)
#   - linter-scripts/**           (banner mentions the launchers)
#   - spec/**                     (markdown + spec listings)
#   - vendor/**, .git/**, node_modules/**
# ============================================================

set -euo pipefail

# Patterns we forbid. Quoted to survive any shell expansion that
# would otherwise glob the asterisk in `cmd /c start`.
# Patterns we forbid. Each must appear inside an `exec.Command(`
# call to count as a violation — this prevents false positives
# from unrelated Go enum literals like `DiagnoseEventStart = "start"`.
#
# We model the rule as two greps composed: line must contain
# `exec.Command(` AND any of the launcher names. Implemented below
# as a single grep -E with the launchers union'd.
LAUNCHER_RE='xdg-open|gnome-open|kde-open|wslview|"open"|"start"|cmd /c start'

# Locations exempt from the rule. Each is a path-prefix the find
# command will -prune. Order matters only for readability.
PRUNE_PATHS=(
  './internal/browser'
  './linters'
  './linter-scripts'
  './spec'
  './vendor'
  './.git'
  './node_modules'
)

# Build the find prune expression dynamically so adding a new
# allowlisted directory is a one-line edit.
prune_expr=()
for p in "${PRUNE_PATHS[@]}"; do
  prune_expr+=(-path "$p" -prune -o)
done

VIOLATIONS=0

# Walk every *.go file outside the prune list, skip *_test.go (test
# files document anti-patterns), grep for forbidden patterns.
while IFS= read -r -d '' file; do
  case "$file" in
    *_test.go) continue ;;
  esac
  for pat in "${PATTERNS[@]}"; do
    if grep -nF -- "$pat" "$file" >/tmp/no-other-browser-hits.$$ 2>/dev/null; then
      while IFS= read -r line; do
        echo "$file:$line  (pattern: $pat)" >&2
        VIOLATIONS=$((VIOLATIONS + 1))
      done < /tmp/no-other-browser-hits.$$
    fi
  done
  rm -f /tmp/no-other-browser-hits.$$
done < <(find . "${prune_expr[@]}" -type f -name '*.go' -print0)

if [ "$VIOLATIONS" -gt 0 ]; then
  echo "" >&2
  echo "ERROR: $VIOLATIONS rogue browser-launch reference(s) found." >&2
  echo "Fix: route through the canonical browser.Launcher injected via" >&2
  echo "     Services.OpenURL (see internal/browser/browser.go and" >&2
  echo "     spec/21-app/02-features/06-tools/99-consistency-report.md row OI-5)." >&2
  exit 1
fi

echo "OK: no rogue browser-launch references found."
exit 0
