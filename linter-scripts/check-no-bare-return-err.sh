#!/usr/bin/env bash
# check-no-bare-return-err.sh
#
# Phase 1 (warn-only) error-trace guardrail.
#
# Flags `return err` lines that have no surrounding `errtrace.Wrap` context,
# so every package boundary contributes a frame to the trace.
# See .lovable/plan.md → Phase 1 and mem://preferences/01-error-stack-traces.md.
#
# Allowed exceptions:
#   - internal/errtrace/**            (defines the wrap helpers themselves)
#   - linter-scripts/**               (this script + siblings)
#   - **/*_test.go                    (tests may propagate err verbatim)
#   - **/codegen/**                   (generators)
#
# Exit codes:
#   0 — no offending sites OR LINT_MODE=warn (default).
#   1 — offending sites present AND LINT_MODE=fail.

set -euo pipefail

LINT_MODE="${LINT_MODE:-fail}"

if ! command -v rg >/dev/null 2>&1; then
    echo "check-no-bare-return-err: ripgrep (rg) not found; skipping." >&2
    exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

# `^\s+return\s+err\s*$` — bare propagation, no wrap, no formatting.
rg -nP -g '*.go' \
   -g '!internal/errtrace/**' \
   -g '!linter-scripts/**' \
   -g '!**/*_test.go' \
   -g '!**/codegen/**' \
   '^\s+return\s+err\s*$' . > "$TMP" || true

COUNT="$(wc -l < "$TMP" | tr -d ' ')"

if [[ "$COUNT" -eq 0 ]]; then
    echo "✓ check-no-bare-return-err: 0 violations"
    exit 0
fi

echo ""
echo "⚠ check-no-bare-return-err: $COUNT bare 'return err' site(s) — wrap with errtrace.Wrap(err, \"context\")"
echo "  (see .lovable/plan.md → Phase 2 and mem://preferences/01-error-stack-traces.md)"
echo ""
sed 's/^/  /' "$TMP"
echo ""

if [[ "$LINT_MODE" == "fail" ]]; then
    exit 1
fi
exit 0
