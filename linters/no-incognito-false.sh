#!/bin/bash
# ============================================================
# No `Incognito = false` Literal Linter
# ============================================================
# Enforces spec/21-app/02-features/06-tools/99-consistency-report.md
# Q-11: writing the literal `Incognito: false` (or `Incognito = false`)
# in production code is forbidden — it betrays an attempt to disable
# the privacy default rather than respecting the user's
# `IncognitoArg` setting.
#
# When a code path needs to *opt out* of incognito for a specific
# launch, the correct form is to pass an empty string for the arg
# (`""`) so the launcher omits the flag entirely. The boolean
# `false` literal is reserved for tests that document the
# anti-pattern.
#
# Usage:    bash linters/no-incognito-false.sh
# Exit 0:   no rogue literal found
# Exit 1:   one or more violations (printed to stderr)
#
# Companion: spec/21-app/02-features/06-tools/99-consistency-report.md
#            row "Incognito = false literal forbidden" + Q-11.
#
# Scope:    every *.go file under the repository root except:
#   - **/*_test.go                 (tests document the anti-pattern)
#   - linters/**, linter-scripts/  (these scripts mention it)
#   - spec/**                      (markdown anti-pattern examples)
#   - internal/store/              (DB column `IsIncognito` legitimately
#                                   uses the boolean shape; checked
#                                   separately by store_test.go)
#   - vendor/**, .git/**, node_modules/**
# ============================================================

set -euo pipefail

# Forbidden literal forms. Each is a fixed string (grep -F) so
# regex metacharacters in operator spacing variants don't surprise
# anyone. We list the four common Go-style spacings explicitly to
# avoid a brittle regex.
PATTERNS=(
  'Incognito: false'
  'Incognito:false'
  'Incognito = false'
  'Incognito=false'
)

PRUNE_PATHS=(
  './internal/store'
  './linters'
  './linter-scripts'
  './spec'
  './vendor'
  './.git'
  './node_modules'
)

prune_expr=()
for p in "${PRUNE_PATHS[@]}"; do
  prune_expr+=(-path "$p" -prune -o)
done

VIOLATIONS=0

while IFS= read -r -d '' file; do
  case "$file" in
    *_test.go) continue ;;
  esac
  for pat in "${PATTERNS[@]}"; do
    if grep -nF -- "$pat" "$file" >/tmp/no-incognito-hits.$$ 2>/dev/null; then
      while IFS= read -r line; do
        echo "$file:$line  (literal: $pat)" >&2
        VIOLATIONS=$((VIOLATIONS + 1))
      done < /tmp/no-incognito-hits.$$
    fi
  done
  rm -f /tmp/no-incognito-hits.$$
done < <(find . "${prune_expr[@]}" -type f -name '*.go' -print0)

if [ "$VIOLATIONS" -gt 0 ]; then
  echo "" >&2
  echo "ERROR: $VIOLATIONS Incognito-disabling literal(s) found." >&2
  echo "Fix: pass an empty string for the IncognitoArg in config" >&2
  echo "     instead of writing the boolean literal false (privacy" >&2
  echo "     defaults stay on; per-launch opt-out happens by argless)." >&2
  echo "     See spec/21-app/02-features/06-tools/99-consistency-report.md Q-11." >&2
  exit 1
fi

echo "OK: no Incognito-disabling literals found."
exit 0
