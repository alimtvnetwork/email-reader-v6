#!/usr/bin/env bash
# check-no-fmt-errorf.sh
#
# Phase 1 (warn-only) error-trace guardrail.
#
# Flags every production-code use of `fmt.Errorf(` so the team can migrate
# them to `errtrace.Wrap` / `errtrace.Wrapf` / `errtrace.New`. See
# .lovable/plan.md → Phase 1 and mem://preferences/01-error-stack-traces.md.
#
# Allowed exceptions:
#   - internal/errtrace/**            (the package itself defines fmt.Errorf comments + Errorf adapter)
#   - linter-scripts/**               (this and sibling lint scripts may name the pattern)
#   - **/*_test.go                    (tests may exercise error shapes directly)
#   - cmd/*/main.go                   (only main.go top-level exit messages)
#
# Exit codes:
#   0 — no offending sites OR LINT_MODE=warn (default).
#   1 — offending sites present AND LINT_MODE=fail.
#
# Usage:
#   ./linter-scripts/check-no-fmt-errorf.sh           # warn-only
#   LINT_MODE=fail ./linter-scripts/check-no-fmt-errorf.sh

set -euo pipefail

LINT_MODE="${LINT_MODE:-fail}"

if ! command -v rg >/dev/null 2>&1; then
    echo "check-no-fmt-errorf: ripgrep (rg) not found; skipping." >&2
    exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Collect violations into a tmp file so we can both print and count them.
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

rg -n -g '*.go' \
   -g '!internal/errtrace/**' \
   -g '!linter-scripts/**' \
   -g '!**/*_test.go' \
   'fmt\.Errorf\(' . > "$TMP" || true

COUNT="$(wc -l < "$TMP" | tr -d ' ')"

if [[ "$COUNT" -eq 0 ]]; then
    echo "✓ check-no-fmt-errorf: 0 violations"
    exit 0
fi

echo ""
echo "⚠ check-no-fmt-errorf: $COUNT site(s) still use fmt.Errorf — migrate to errtrace.Wrap/Wrapf/New"
echo "  (see .lovable/plan.md → Phase 2 and mem://preferences/01-error-stack-traces.md)"
echo ""
sed 's/^/  /' "$TMP"
echo ""

if [[ "$LINT_MODE" == "fail" ]]; then
    exit 1
fi
exit 0
