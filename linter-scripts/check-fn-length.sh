#!/bin/bash
# ============================================================
# Go Function-Length Linter (AC-PROJ-20)
# ============================================================
# Enforces the 15-statement function-length rule from
# spec/12-consolidated-guidelines/02-coding-guidelines.md §3.
#
# Counts top-level statements inside every Go function body
# under internal/ and cmd/. A "statement" is any non-blank,
# non-comment, non-brace line at brace-depth 1 inside the
# function body — i.e. siblings of the function's outer block,
# nested blocks count as one statement.
#
# Defaults:
#   - MAX_STATEMENTS  = 15
#   - SCAN_ROOTS      = internal cmd
#   - EXCLUDE_GLOBS   = */mock_*.go *_gen.go *_test.go
#
# Override via env: MAX_STATEMENTS=20 bash linter-scripts/check-fn-length.sh
#
# Usage:    bash linter-scripts/check-fn-length.sh
# Exit 0:   every function ≤ MAX_STATEMENTS
# Exit 1:   one or more violations (printed to stderr)
#
# Wired into AC-PROJ-20 in spec/21-app/97-acceptance-criteria.md.
# ============================================================

set -euo pipefail

MAX_STATEMENTS="${MAX_STATEMENTS:-15}"
SCAN_ROOTS=("internal" "cmd")
EXCLUDE_REGEX='(_test\.go|_gen\.go|/mock_[^/]+\.go)$'

# Collect candidate Go files. If neither root exists, the linter
# is a no-op and exits 0 (spec is ahead of code — see project
# 99-consistency-report §6 delta #2).
GO_FILES=()
for root in "${SCAN_ROOTS[@]}"; do
  if [ -d "$root" ]; then
    while IFS= read -r f; do
      if ! [[ "$f" =~ $EXCLUDE_REGEX ]]; then
        GO_FILES+=("$f")
      fi
    done < <(find "$root" -type f -name "*.go" 2>/dev/null | sort)
  fi
done

if [ "${#GO_FILES[@]}" -eq 0 ]; then
  echo "ℹ️  No Go files under ${SCAN_ROOTS[*]} — nothing to check (spec ahead of code)."
  exit 0
fi

echo "🔍 Scanning ${#GO_FILES[@]} Go files (max statements per function: $MAX_STATEMENTS)…"

VIOLATIONS=0

# AWK pass per file: detect "func ... {" headers, then walk the
# brace structure counting statements at depth 1 inside the body.
# Strings, runes, line comments, and block comments are stripped
# from each line before counting so braces / semicolons inside
# them never affect depth.
for f in "${GO_FILES[@]}"; do
  awk -v file="$f" -v max="$MAX_STATEMENTS" '
    function strip(line,    out, i, c, n, in_str, in_rune, in_lc, in_bc) {
      # Strip trailing line comment + string/rune literals so braces
      # inside them are ignored.
      n = length(line); out = ""; i = 1
      in_str = 0; in_rune = 0; in_bc = 0
      while (i <= n) {
        c = substr(line, i, 1)
        if (in_bc) {
          if (c == "*" && substr(line, i, 2) == "*/") { in_bc = 0; i += 2; continue }
          i++; continue
        }
        if (in_str) {
          if (c == "\\") { i += 2; continue }
          if (c == "\"") in_str = 0
          i++; continue
        }
        if (in_rune) {
          if (c == "\\") { i += 2; continue }
          if (c == "\x27") in_rune = 0
          i++; continue
        }
        if (c == "/" && substr(line, i, 2) == "//") { break }
        if (c == "/" && substr(line, i, 2) == "/*") { in_bc = 1; i += 2; continue }
        if (c == "\"") { in_str = 1; i++; continue }
        if (c == "\x27") { in_rune = 1; i++; continue }
        out = out c
        i++
      }
      return out
    }

    BEGIN { depth = 0; in_func = 0; fn_name = ""; fn_line = 0; stmt_count = 0 }

    {
      raw = $0
      stripped = strip(raw)

      # Trim whitespace for emptiness checks.
      trimmed = stripped
      sub(/^[ \t]+/, "", trimmed)
      sub(/[ \t]+$/, "", trimmed)

      # Detect a function header (the simple, common single-line case).
      # We start counting after we see the opening brace at depth 0.
      if (!in_func && match(stripped, /^[[:space:]]*func[[:space:]][^{]*\{[[:space:]]*$/)) {
        # Extract name (best-effort, allows methods and generics).
        name = stripped
        sub(/^[[:space:]]*func[[:space:]]+/, "", name)
        sub(/[[:space:]]*\(.*$/, "", name)  # cut before first arg paren
        sub(/\[.*$/, "", name)               # cut generics
        if (name == "") name = "<anon>"
        fn_name = name
        fn_line = NR
        in_func = 1
        depth = 1
        stmt_count = 0
        next
      }

      if (!in_func) next

      # Update brace depth using stripped line.
      # Count braces explicitly so multiple per line are honored.
      # NOTE: avoid the reserved awk function name "close".
      n_open  = gsub(/\{/, "{", stripped)
      n_close = gsub(/\}/, "}", stripped)

      # A "statement" is a non-empty, non-pure-brace line at depth == 1
      # (siblings of the function body), counted BEFORE depth changes
      # caused by this line take effect.
      if (depth == 1 && trimmed != "" && trimmed !~ /^[\{\}[:space:]]+$/) {
        stmt_count++
      }

      depth += n_open - n_close

      if (depth <= 0) {
        if (stmt_count > max) {
          printf("%s:%d  %s  has %d statements (max %d)\n",
                 file, fn_line, fn_name, stmt_count, max) > "/dev/stderr"
          # Use a non-zero exit signal via printing a marker to stdout.
          print "__VIOLATION__"
        }
        in_func = 0
        depth = 0
        fn_name = ""
        stmt_count = 0
      }
    }
  ' "$f" >> /tmp/_fnlen_markers || true
done

# Count violation markers (grep returns 1 when no matches; tolerate it).
if [ -s /tmp/_fnlen_markers ]; then
  VIOLATIONS=$(grep -c "^__VIOLATION__$" /tmp/_fnlen_markers || true)
fi
rm -f /tmp/_fnlen_markers

echo
if [ "$VIOLATIONS" -gt 0 ]; then
  echo "❌ FAIL: $VIOLATIONS function(s) exceed $MAX_STATEMENTS statements." >&2
  exit 1
fi

echo "✅ PASS: every function ≤ $MAX_STATEMENTS statements."
exit 0
