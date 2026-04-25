#!/bin/bash
# ============================================================
# Spec Internal-Link Linter (AC-PROJ-33)
# ============================================================
# Scans spec/21-app/, spec/23-app-database/, and
# spec/24-app-design-system-and-ui/ for every Markdown link of
# the form [...](./...) or [...](../...) and verifies that the
# target file exists relative to the source file.
#
# Anchors (#section) are stripped before resolution.
# Links to mem:// and http(s):// are skipped.
#
# Usage:    bash linter-scripts/check-internal-links.sh
# Exit 0:   all links resolve
# Exit 1:   one or more broken links found (printed to stderr)
#
# Wired into AC-PROJ-33 in spec/21-app/97-acceptance-criteria.md.
# ============================================================

set -euo pipefail

ROOTS=(
  "spec/21-app"
  "spec/23-app-database"
  "spec/24-app-design-system-and-ui"
)

BROKEN=0
CHECKED=0

# Find every .md file under the configured roots.
mapfile -t MD_FILES < <(find "${ROOTS[@]}" -type f -name "*.md" 2>/dev/null | sort)

if [ "${#MD_FILES[@]}" -eq 0 ]; then
  echo "❌ No Markdown files found under: ${ROOTS[*]}" >&2
  exit 1
fi

echo "🔍 Scanning ${#MD_FILES[@]} Markdown files for broken internal links…"

for src in "${MD_FILES[@]}"; do
  src_dir=$(dirname "$src")

  # Extract every (./... ) or (../... ) link target.
  # POSIX grep -E; trims leading '(' and trailing ')'.
  while IFS= read -r raw; do
    # Strip surrounding parentheses.
    target="${raw#(}"
    target="${target%)}"

    # Strip anchor.
    target="${target%%#*}"

    # Skip empty (pure-anchor) or external links.
    [ -z "$target" ] && continue
    case "$target" in
      http://*|https://*|mem://*|mailto:*) continue ;;
    esac

    CHECKED=$((CHECKED + 1))

    # Resolve relative to the source file's directory.
    resolved="$src_dir/$target"

    # Trailing slash → check directory exists.
    if [[ "$target" == */ ]]; then
      if [ ! -d "$resolved" ]; then
        echo "❌ BROKEN  $src → $target  (directory not found: $resolved)" >&2
        BROKEN=$((BROKEN + 1))
      fi
    else
      if [ ! -e "$resolved" ]; then
        echo "❌ BROKEN  $src → $target  (file not found: $resolved)" >&2
        BROKEN=$((BROKEN + 1))
      fi
    fi
  done < <(grep -oE '\((\.{1,2}/[^)[:space:]]+)\)' "$src" || true)
done

echo
echo "📊 Checked $CHECKED internal links across ${#MD_FILES[@]} files."

if [ "$BROKEN" -gt 0 ]; then
  echo "❌ FAIL: $BROKEN broken internal link(s) found." >&2
  exit 1
fi

echo "✅ PASS: every internal link resolves."
exit 0
