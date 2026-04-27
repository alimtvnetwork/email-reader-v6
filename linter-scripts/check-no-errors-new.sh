#!/usr/bin/env bash
# check-no-errors-new.sh
#
# Phase 1 (warn-only) error-trace guardrail.
#
# Flags `errors.New(` calls that should be `errtrace.New(` so the sentinel
# itself carries a frame.
# See .lovable/plan.md → Phase 1 and mem://preferences/01-error-stack-traces.md.
#
# Allowed exceptions:
#   - internal/errtrace/**            (defines the wrap helpers themselves)
#   - linter-scripts/**
#   - **/*_test.go                    (tests construct sentinels for assertions)
#   - **/codegen/**
#
# Exit codes:
#   0 — no offending sites OR LINT_MODE=warn (default).
#   1 — offending sites present AND LINT_MODE=fail.

set -euo pipefail

LINT_MODE="${LINT_MODE:-warn}"

if ! command -v rg >/dev/null 2>&1; then
    echo "check-no-errors-new: ripgrep (rg) not found; skipping." >&2
    exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

rg -n -g '*.go' \
   -g '!internal/errtrace/**' \
   -g '!linter-scripts/**' \
   -g '!**/*_test.go' \
   -g '!**/codegen/**' \
   'errors\.New\(' . > "$TMP" || true

COUNT="$(wc -l < "$TMP" | tr -d ' ')"

if [[ "$COUNT" -eq 0 ]]; then
    echo "✓ check-no-errors-new: 0 violations"
    exit 0
fi

echo ""
echo "⚠ check-no-errors-new: $COUNT errors.New site(s) — replace with errtrace.New for frame capture"
echo "  (see .lovable/plan.md → Phase 2 and mem://preferences/01-error-stack-traces.md)"
echo ""
sed 's/^/  /' "$TMP"
echo ""

if [[ "$LINT_MODE" == "fail" ]]; then
    exit 1
fi
exit 0
