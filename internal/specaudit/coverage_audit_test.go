// coverage_audit_test.go — Slice #119 spec-vs-code coverage guard
// for `AC-*` acceptance-criteria identifiers.
//
// **Problem.** The spec tree under `spec/21-app/` defines 97
// real `AC-*-NN` rows (across the AC-DB / AC-DBP / AC-DS / AC-PROJ /
// AC-SB / AC-SF / AC-SP / AC-SX prefixes). Slice #116d closed the
// `AC-SB-03..10` settings-validation cluster via the `T-SET-*`
// matrix, and other slices have closed scattered AC-DB / AC-PROJ /
// AC-DS rows. But there is no audit that would tell a reviewer
// "this slice you're shipping today re-references AC-XX-12 — is
// the spec → code → test triangle now closed for that row?"
//
// Without that audit, three failure modes go undetected:
//
//  1. **Silent gap growth.** A new spec row gets added (e.g. a
//     reviewer expands the rules feature to claim AC-RU-04) and no
//     test ever cites it. The row is "specced but unenforced".
//  2. **Stale code references.** A slice cites AC-FOO-99 in a test
//     comment, then a later spec rewrite renames or removes that
//     row. The code reference is now lying.
//  3. **Untrackable progress.** Reviewers cannot answer "how many
//     more rows do we need to cover before the matrix is 100%?"
//     without re-running an ad-hoc grep every time.
//
// **Solution.** This test reads the spec tree once, reads every
// `*.go` file outside `spec/` once, computes the symmetric
// difference, and asserts:
//
//   - **No unexpected gap.** Every spec ID either has at least one
//     code reference OR appears in `coverageGapAllowlist` below.
//     The allowlist is the slice-#119 baseline of 77 rows — any
//     *new* spec ID added by a future slice must come with at
//     least one test reference, or the slice's diff must add the
//     row to the allowlist with a one-line comment justifying why
//     coverage is deferred (and to which follow-up slice).
//
//   - **No stale code reference.** Every code-side `AC-*` ID must
//     resolve to a real spec row, OR appear in `staleCodeRefAllowlist`
//     below. The slice-#119 baseline is 7 stale rows, all in
//     `internal/store/` files that pre-date a spec rename.
//
// **Ratchet.** Both allowlists must shrink monotonically. The test
// FAILs loudly if a row is in an allowlist but its underlying
// condition has been resolved (i.e. the gap closed, or the stale
// ref removed) — that's the signal to drop the allowlist row in
// the same diff, locking in the win.
//
// Same proven pattern as `viewLayerGlobalsAllowlist`,
// `focusOrderAllowlistedViewFiles`, and the WCAG `knownDrift`
// flags. The pattern is: bootstrap-PASS today, enforce on every
// future change, ratchet to zero.
package specaudit

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoRoot finds the repository root by walking upward from the
// current working directory until a `go.mod` is found. Required
// because `go test` sets CWD to the test's package directory
// (`internal/specaudit/`) and the audit needs to scan the entire
// tree.
func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found walking up from %s", cwd)
		}
		dir = parent
	}
}

// acIDPattern matches the project's AC ID grammar:
//
//	AC-<PREFIX>-<NUMBER>[-<SUFFIX>]
//
// where PREFIX is one of the 8 known feature codes. The optional
// `-<SUFFIX>` tail catches sub-rows like `AC-DB-D-04`. Prefixes
// are pinned (not `[A-Z]+`) to avoid accidentally matching
// unrelated tokens like `AC-130` or `AC-OK-1`.
//
// The list of known prefixes was derived empirically from the spec
// tree at slice-#119 time (run `rg -oIN 'AC-[A-Z]+-' spec/21-app/
// | awk -F- '{print $2}' | sort -u`). When a future feature
// introduces a new prefix, add it here in the same diff that
// introduces the spec rows — otherwise the audit will silently
// ignore the new family.
var acIDPattern = regexp.MustCompile(`AC-(?:DB|DBP|DS|PROJ|SB|SF|SP|SX)-[A-Z0-9]+(?:-[A-Z0-9]+)?`)

// templatePlaceholder matches the spec's documentation
// placeholders (`AC-XX-NN`, `AC-FOO-XX`) used in templates and
// "how to file an AC" examples. They look like real IDs but are
// not — filtering them out is essential to avoid 4 false-positive
// gaps at slice-#119 baseline.
var templatePlaceholder = regexp.MustCompile(`-(?:NN|XX)$`)

// scanIDs walks `root` recursively, opens every file matching
// `accept(path)`, applies `acIDPattern` to the bytes, and returns
// the deduplicated set of matches. Template placeholders are
// dropped at the source.
func scanIDs(t *testing.T, root string, accept func(path string, info os.FileInfo) bool) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			// Universal skips: vendor/, testdata/, .git/, node_modules/.
			name := info.Name()
			if name == "vendor" || name == "testdata" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !accept(path, info) {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		for _, m := range acIDPattern.FindAll(raw, -1) {
			id := string(m)
			if templatePlaceholder.MatchString(id) {
				continue
			}
			out[id] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// -----------------------------------------------------------------------------
// Allowlists — slice-#119 baseline.
// -----------------------------------------------------------------------------

// coverageGapAllowlist names every spec AC ID that has zero code
// references at slice-#119 time. **MUST shrink monotonically.**
//
// Generated 2026-04-27 via:
//
//	cd repo && rg --no-config -oIN 'AC-(DB|DBP|DS|PROJ|SB|SF|SP|SX)-[A-Z0-9]+(-[A-Z0-9]+)?' \
//	  spec/21-app/ | sort -u | grep -vE -- '-(NN|XX)$' > /tmp/spec.txt
//	rg --no-config -oIN '...' . --glob '*.go' --glob '!**/spec/**' | sort -u > /tmp/code.txt
//	comm -23 /tmp/spec.txt /tmp/code.txt
//
// Distribution: AC-PROJ (27), AC-SF (21), AC-SB (16), AC-SX (6),
// AC-SP (5), AC-DB (2). The AC-DB cluster is small because Slice
// #116d's T-SET matrix and the DB hardening slices already cover
// most rows.
var coverageGapAllowlist = map[string]struct{}{
	"AC-DB-30":   {},
	"AC-DB-37":   {},
	"AC-PROJ-01": {},
	"AC-PROJ-02": {},
	"AC-PROJ-03": {},
	"AC-PROJ-04": {},
	"AC-PROJ-05": {},
	"AC-PROJ-06": {},
	"AC-PROJ-07": {},
	"AC-PROJ-08": {},
	"AC-PROJ-09": {},
	"AC-PROJ-10": {},
	"AC-PROJ-11": {},
	"AC-PROJ-12": {},
	"AC-PROJ-13": {},
	"AC-PROJ-14": {},
	"AC-PROJ-15": {},
	"AC-PROJ-16": {},
	"AC-PROJ-17": {},
	"AC-PROJ-18": {},
	"AC-PROJ-19": {},
	"AC-PROJ-21": {},
	"AC-PROJ-22": {},
	"AC-PROJ-31": {},
	"AC-PROJ-32": {},
	"AC-PROJ-33": {},
	"AC-PROJ-34": {},
	"AC-PROJ-35": {},
	"AC-PROJ-36": {},
	"AC-SB-01":   {},
	"AC-SB-02":   {},
	"AC-SB-11":   {},
	"AC-SB-12":   {},
	"AC-SB-13":   {},
	"AC-SB-14":   {},
	"AC-SB-15":   {},
	"AC-SB-16":   {},
	"AC-SB-17":   {},
	"AC-SB-18":   {},
	"AC-SB-19":   {},
	"AC-SB-20":   {},
	"AC-SB-21":   {},
	"AC-SB-22":   {},
	"AC-SB-23":   {},
	"AC-SB-24":   {},
	"AC-SF-01":   {},
	"AC-SF-02":   {},
	"AC-SF-03":   {},
	"AC-SF-04":   {},
	"AC-SF-05":   {},
	"AC-SF-06":   {},
	"AC-SF-07":   {},
	"AC-SF-08":   {},
	"AC-SF-09":   {},
	"AC-SF-10":   {},
	"AC-SF-11":   {},
	"AC-SF-12":   {},
	"AC-SF-13":   {},
	"AC-SF-14":   {},
	"AC-SF-15":   {},
	"AC-SF-16":   {},
	"AC-SF-17":   {},
	"AC-SF-18":   {},
	"AC-SF-19":   {},
	"AC-SF-20":   {},
	"AC-SF-21":   {},
	"AC-SP-01":   {},
	"AC-SP-02":   {},
	"AC-SP-03":   {},
	"AC-SP-04":   {},
	"AC-SP-05":   {},
	"AC-SX-01":   {},
	"AC-SX-02":   {},
	"AC-SX-03":   {},
	"AC-SX-04":   {},
	"AC-SX-05":   {},
	"AC-SX-06":   {},
}

// staleCodeRefAllowlist names every code-side AC ID that does NOT
// resolve to a real spec row at slice-#119 time. **MUST shrink
// monotonically.** Each row is either (a) a renamed/removed spec ID
// the code still cites, or (b) a code-internal ID that escaped the
// `T-*` / `ER-*` namespaces and reached the AC-* prefix by mistake.
//
// Slice-#119 baseline distribution: 5 in `internal/store/` files
// referencing AC-DB-43/47/53/54/55 (which the spec renamed during
// Phase 6 reconciliation), plus AC-DB-D-04 (a parser-artifact ID
// from spec/23-app-database) and AC-DS-04 (cited in store shims
// before AC-DS was renamed to AC-DBP). Slice #121 (final
// reconciliation) is the natural home for resolving these — it's
// the spec/code reconciliation slice anyway.
var staleCodeRefAllowlist = map[string]struct{}{
	"AC-DB-43":   {},
	"AC-DB-47":   {},
	"AC-DB-53":   {},
	"AC-DB-54":   {},
	"AC-DB-55":   {},
	"AC-DB-D-04": {},
	"AC-DS-04":   {},
}

// -----------------------------------------------------------------------------
// The audit.
// -----------------------------------------------------------------------------

// Test_AC_CoverageAudit is the slice-#119 main guard. See package
// doc above for the contract. Three sub-tests so failures localize:
//
//   - `gap_no_unexpected`   — every gap must be in the allowlist.
//   - `gap_no_stale_allow`  — every allowlisted gap must still be a real gap.
//   - `code_no_stale_ref`   — every code ref must resolve to a real spec ID
//                             (or be allowlisted as known-stale).
//   - `code_no_stale_allow` — every allowlisted stale ref must still be stale.
//
// Sub-test names use `_` separators so a CI dashboard with a
// `failed: ` prefix grep can filter cleanly.
func Test_AC_CoverageAudit(t *testing.T) {
	root := repoRoot(t)
	specRoot := filepath.Join(root, "spec", "21-app")

	specIDs := scanIDs(t, specRoot, func(path string, info os.FileInfo) bool {
		return strings.HasSuffix(path, ".md")
	})
	codeIDs := scanIDs(t, root, func(path string, info os.FileInfo) bool {
		// Only Go source files outside the spec tree.
		if !strings.HasSuffix(path, ".go") {
			return false
		}
		// Skip generated / vendored / spec-mirror trees.
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return false
		}
		if strings.HasPrefix(rel, "spec"+string(filepath.Separator)) {
			return false
		}
		// Don't scan THIS file — it contains every gap ID as a map
		// key, which would falsely "cover" 77 rows.
		if strings.HasSuffix(path, "coverage_audit_test.go") {
			return false
		}
		return true
	})

	// Gap = spec ID with no code reference.
	var unexpectedGap []string
	var staleGapAllow []string
	for id := range specIDs {
		_, hasCode := codeIDs[id]
		_, allowed := coverageGapAllowlist[id]
		switch {
		case !hasCode && !allowed:
			unexpectedGap = append(unexpectedGap, id)
		case hasCode && allowed:
			staleGapAllow = append(staleGapAllow, id)
		}
	}

	// Stale code ref = code ID with no spec row.
	var unexpectedStale []string
	var resolvedStaleAllow []string
	for id := range codeIDs {
		_, hasSpec := specIDs[id]
		_, allowed := staleCodeRefAllowlist[id]
		switch {
		case !hasSpec && !allowed:
			unexpectedStale = append(unexpectedStale, id)
		case hasSpec && allowed:
			resolvedStaleAllow = append(resolvedStaleAllow, id)
		}
	}

	t.Run("gap_no_unexpected", func(t *testing.T) {
		if len(unexpectedGap) == 0 {
			return
		}
		sort.Strings(unexpectedGap)
		t.Fatalf("slice-#119 audit — %d new spec AC ID(s) have zero code references and are not in coverageGapAllowlist:\n  %s\n\nFix: either (a) add at least one test that names the ID in a comment, or (b) add the ID to coverageGapAllowlist in this file with a one-line `// deferred to slice #N` justification.",
			len(unexpectedGap), strings.Join(unexpectedGap, "\n  "))
	})

	t.Run("gap_no_stale_allow", func(t *testing.T) {
		if len(staleGapAllow) == 0 {
			return
		}
		sort.Strings(staleGapAllow)
		t.Fatalf("slice-#119 audit — %d allowlisted gap row(s) now have code coverage and must be removed from coverageGapAllowlist (allowlist shrinks monotonically):\n  %s",
			len(staleGapAllow), strings.Join(staleGapAllow, "\n  "))
	})

	t.Run("code_no_stale_ref", func(t *testing.T) {
		if len(unexpectedStale) == 0 {
			return
		}
		sort.Strings(unexpectedStale)
		t.Fatalf("slice-#119 audit — %d code-side AC ID(s) do not match any spec row and are not in staleCodeRefAllowlist:\n  %s\n\nFix: either (a) update the spec to reintroduce the ID, (b) update the code to cite the new ID, or (c) add the ID to staleCodeRefAllowlist with a one-line justification.",
			len(unexpectedStale), strings.Join(unexpectedStale, "\n  "))
	})

	t.Run("code_no_stale_allow", func(t *testing.T) {
		if len(resolvedStaleAllow) == 0 {
			return
		}
		sort.Strings(resolvedStaleAllow)
		t.Fatalf("slice-#119 audit — %d allowlisted stale-ref(s) now resolve to real spec rows and must be removed from staleCodeRefAllowlist (allowlist shrinks monotonically):\n  %s",
			len(resolvedStaleAllow), strings.Join(resolvedStaleAllow, "\n  "))
	})

	// Coverage telemetry — `t.Logf` so a `go test -v` run shows
	// progress without polluting the PASS line.
	covered := len(specIDs) - len(coverageGapAllowlist)
	if covered < 0 {
		covered = 0
	}
	pct := float64(covered) * 100 / float64(len(specIDs))
	t.Logf("AC coverage: %d/%d real spec rows referenced in code (%.1f%%); %d gap(s) allowlisted; %d stale code ref(s) allowlisted",
		covered, len(specIDs), pct, len(coverageGapAllowlist), len(staleCodeRefAllowlist))
}
