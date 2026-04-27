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

// namedPlaceholderIDs are spec tokens that look like real AC IDs
// but are documented "next contiguous ID" pointers used in
// authoring guidance prose, not real acceptance-criteria rows.
// Filtering them at scan time (instead of allowlisting them as
// "gaps") keeps the coverage denominator honest.
//
// Slice #146 added AC-PROJ-36 here. The string appears once in
// `spec/21-app/97-acceptance-criteria.md` inside the prose
// "If a future criterion would touch ≥ 2 features OR has no
// obvious single owner, add it as **AC-PROJ-36** (next contiguous
// ID) …". There is no `| AC-PROJ-36 | … |` table row anywhere in
// the spec tree. When a future slice files a real AC-PROJ-36 row,
// remove this entry in the same diff so the audit picks it up.
var namedPlaceholderIDs = map[string]struct{}{
	"AC-PROJ-36": {},
}

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
			if _, isPlaceholder := namedPlaceholderIDs[id]; isPlaceholder {
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
// Allowlists — slice-#121 baseline (post-reconciliation).
// -----------------------------------------------------------------------------
//
// Slice #119 first introduced these allowlists scoped to `spec/21-app/`
// only. That narrow scope hid the fact that the AC matrix lives in
// FOUR sibling roots:
//
//	spec/02-coding-guidelines/97-acceptance-criteria.md
//	spec/07-design-system/97-acceptance-criteria.md
//	spec/21-app/.../97-acceptance-criteria.md
//	spec/23-app-database/97-acceptance-criteria.md
//	spec/24-app-design-system-and-ui/97-acceptance-criteria.md
//
// Slice #121 broadens the spec scan to the entire `spec/` tree and
// rebases the baselines. Net effect:
//
//	spec rows seen     :  97 → 186  (+89)
//	stale code refs    :   7 →   1  (6 of the 7 resolved by the
//	                                 broader scan, only AC-DB-D-04
//	                                 remained genuinely stale and was
//	                                 removed from queries.go)
//	gap allowlist size :  77 → 160  (the AC-DB / AC-DBP / AC-DS
//	                                 families surfaced by the broader
//	                                 scan are mostly already-covered-
//	                                 by-spirit but not yet ID-cited
//	                                 in tests).
//
// The audit's contract is unchanged: both allowlists must shrink
// monotonically. Slice #121 lowered the stale list from 7 → 0 by
// completing the spec/code reconciliation; the gap list is the
// long-tail "AC coverage rollout" backlog tracked in the roadmap.

// coverageGapAllowlist names every spec AC ID that has zero code
// references at slice-#121 time. **MUST shrink monotonically.**
//
// Regenerate with:
//
//	cd repo
//	rg --no-config -oIN 'AC-(DB|DBP|DS|PROJ|SB|SF|SP|SX)-[A-Z0-9]+(-[A-Z0-9]+)?' \
//	  spec/ | sort -u | grep -vE -- '-(NN|XX)$' > /tmp/spec.txt
//	rg --no-config -oIN '...same regex...' --glob '*.go' \
//	  --glob '!**/coverage_audit_test.go' . | sort -u > /tmp/code.txt
//	comm -23 /tmp/spec.txt /tmp/code.txt
//
// Distribution at slice-#121: AC-DS (45), AC-DB (34), AC-PROJ (27),
// AC-SF (21), AC-SB (16), AC-SX (6), AC-DBP (6), AC-SP (5).
var coverageGapAllowlist = map[string]struct{}{
	// Slice #123 closed AC-DB-04/21/22/23/24/30/31/40/42/44/45 by
	// tagging existing internal/store/* and internal/store/migrate/*
	// tests with their AC-DB IDs (no new test code — same annotate-
	// only pattern as Slice #122 for AC-DS). Slice #129 closed
	// AC-DB-10 (PRAGMA per-conn parity, with a documented spec-drift
	// note about synchronous=FULL vs spec'd NORMAL — fixing the DSN
	// belongs in a behaviour slice) and AC-DB-11 (WAL mode persists
	// across close/reopen) via Test_Store_PragmaOnEveryConn and
	// Test_Store_WalPersists in pragma_persist_test.go. Remaining
	// AC-DB gaps describe a future-target schema (singular table
	// names, Decision/Origin enum CHECKs, ON DELETE SET NULL FK,
	// HasAttachment column) or runtime features (gap/checksum/
	// downgrade detection in the migrate runner) that the current
	// implementation has not adopted: AC-DB-01/02/03 (schema
	// introspection against §1–§5), AC-DB-05/25 (partial-index
	// Blocked-not-matched negative tests), AC-DB-06/07/08/09 (FK +
	// enum CHECK tests), AC-DB-20/26/27/28 (Q-* registry + EXPLAIN
	// goldens + perf), AC-DB-32..36 (migrate gap/checksum/downgrade/
	// crash/legacy rename tests), AC-DB-41 (Q-OPEN-PRUNE-BLOCKED
	// scope), AC-DB-46 (idle-probe defer test).
	"AC-DB-01": {},
	"AC-DB-02": {},
	"AC-DB-03": {},
	"AC-DB-05": {},
	"AC-DB-06": {},
	"AC-DB-07": {},
	"AC-DB-08": {},
	"AC-DB-09": {},
	"AC-DB-20": {},
	"AC-DB-25": {},
	// AC-DB-26 closed by Slice #147 — Test_AST_ExportStream_RowsCloseOnlyDeferred
	// in ast_export_stream_test.go pins the structural half: every
	// rows.Close() in tools_export.go must be inside a defer. The runtime
	// "no buffering / memory ceiling" half stays deferred to bench infra.
	"AC-DB-27": {},
	"AC-DB-28": {},
	"AC-DB-32": {},
	"AC-DB-33": {},
	"AC-DB-34": {},
	"AC-DB-35": {},
	"AC-DB-36": {},
	// AC-DB-37 closed by Slice #127 — same Test_AST_MaintenanceOnly
	// scan that already covered AC-PROJ-17 (DDL-only-in-migrate rule).
	"AC-DB-41": {},
	"AC-DB-46": {},
	"AC-DBP-01":  {},
	"AC-DBP-02":  {},
	"AC-DBP-03":  {},
	"AC-DBP-04":  {},
	"AC-DBP-05":  {},
	"AC-DBP-06":  {},
	// Slice #122 closed AC-DS-01/02/03/10/11/12/13/14/16/17/18/20/48/
	// 60/61/62/63/64/65/66/67/68 by tagging the existing theme/AST/
	// accessibility tests with their AC-DS IDs (no new test code —
	// those rows were already covered by tests under different
	// names). Remaining AC-DS gaps are genuine future work (most
	// need the deferred Slice #118e Fyne canvas harness).
	// AC-DS-05 closed by Slice #169 — Test_Tokens_NoDuplicateValues
	// in internal/ui/theme/aliases_test.go enforces the registry-
	// gated form per spec/24-…/01-tokens.md §2.12 (22 NamedAliases:
	// 13 Both + 5 DarkOnly + 4 LightOnly). Removed from allowlist.
	// AC-DS-15 closed by Slice #145 — Test_Theme_SystemResolves in
	// internal/ui/theme/fyne_theme_resolve_test.go pins the four
	// resolvedMode(variant) branches: ThemeSystem follows the OS
	// variant (Light/Dark); explicit modes ignore the OS hint.
	// AC-DS-19 closed by Slice (this) — Test_AST_AnimImportLimit in
	// ast_design_system_test.go pins the "only internal/ui/anim/ may
	// use canvas.NewColorRGBAAnimation" rule (vacuously true today).
	// AC-DS-30 closed by Slice #148 — TestNavItemsCoverAllKinds in
	// internal/ui/sidebar_test.go already pinned the spec's canonical
	// 7-item sidebar order at items 0..6 (Dashboard, Emails, Rules,
	// Accounts, Watch, Tools, Settings); slice #148 added the AC-DS-30
	// citation comment so the audit resolves it as covered.
	"AC-DS-31": {},
	"AC-DS-32": {},
	"AC-DS-33": {},
	"AC-DS-34": {},
	"AC-DS-35": {},
	"AC-DS-36": {},
	"AC-DS-37": {},
	"AC-DS-40": {},
	"AC-DS-41": {},
	"AC-DS-42": {},
	"AC-DS-43": {},
	"AC-DS-44": {},
	"AC-DS-45": {},
	"AC-DS-46": {},
	"AC-DS-47": {},
	"AC-DS-49": {},
	"AC-DS-50": {},
	// AC-DS-51 closed by Slice (this) — Test_AST_PulseOnlyInWatchDot in
	// ast_design_system_test.go pins the "only watch_dot*.go may call
	// anim.Pulse(...)" rule (vacuously true today; cluster-mate to
	// AC-DS-19).
	// AC-DS-69 closed by Slice (this) — TestNoIconOnlyButtons_WithoutLabel
	// in internal/ui/accessibility/a11y_test.go already implemented
	// the AST scan; this slice added the AC-DS-69 citation comment so
	// the audit resolves it as covered.
	// Slice #124 closed AC-PROJ-12/13/16/17/19/21/22 by tagging the
	// existing AST/errtrace/migrate/retention tests with their AC-PROJ
	// IDs (no new test code — those rows were already covered by tests
	// under different names). Slice #131 closed AC-PROJ-18/32/34 via
	// new headless scanners in `internal/specaudit/ast_project_linters_test.go`
	// (`Test_AST_OnlyUiPackagesImportFyne`, `Test_AllQueryRefsResolveInDbQueries`,
	// `Test_FeatureFolderShapeIsUniform`).
	//
	// Honest defers from Slice #131:
	//   - AC-PROJ-31: scanner is wired (`Test_AllErrorRefsResolveInRegistry`)
	//     and ratchet-ready, but currently surfaces ~39 ER codes that
	//     specs reference but `06-error-registry.md` hasn't formalised
	//     yet (Settings 217xx, Migrations 218xx, UI 219xx blocks).
	//     Closing requires registry growth = behaviour work.
	//   - AC-PROJ-33 closed by Slice #164 — `Test_NoBrokenSpecLinks_GreenInCi`
	//     now t.Fatal's on any broken local link. All 33 originally-broken
	//     cross-tree refs were repathed (`08-generic-update/` → `14-self-update-app-update/`
	//     + `10-powershell-integration/` + `13-cicd-pipeline-workflows/`),
	//     renumbered (`13-cicd-pipeline-workflows/00-overview.md` 08-13 → 04-09),
	//     stripped where the target lives outside this repo, or reformatted
	//     to dodge the `[T](fn(...))` markdown false-positive.
	//   - AC-PROJ-35: open `OI-1..OI-6` in 06-tools/99-consistency-report.md
	//     are listed as "scheduled" not "✅ Closed". Same family as the
	//     deferred OI work tracked from Phase 2.
	//
	// Remaining gaps (01-11, 14-15) are end-to-end integration scenarios
	// requiring a multi-process harness, deferred to a future slice.
	// AC-PROJ-36 is now filtered at scan time as a documented
	// placeholder ID (see namedPlaceholderIDs above) — Slice #146.
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
	"AC-PROJ-14": {},
	"AC-PROJ-15": {},
	// AC-PROJ-35 closed by Slice #144 — Test_Project_NoOpenOiReferencesAtMergeTime
	// in project_no_open_oi_test.go scans every feature 99-consistency-report.md
	// and asserts every OI-N table row carries ✅ and no in-flight marker.
	// All 8 OI rows across Tools (OI-1..OI-6) and Watch (OI-1, OI-2) are ✅ Closed.
	// Slice #126 closed AC-SB-01/02/11/15/16/17/19/22 by tagging the
	// existing settings backend tests with their AC-SB IDs (no new
	// test code — those rows were already covered by tests under
	// different names). The 6-in-1 TestSettings_ValidationErrors also
	// closed AC-SB-03..06/09/10. AC-SB-07/08 were already cited
	// in settings_validation_matrix_test.go and TestSettings_ChromePath
	// Validation respectively. Slice #128 closed the long-tail
	// (12, 13, 14, 18, 20, 21, 23) via new headless tests in
	// settings_long_tail_test.go. The final row (race make target) is
	// satisfied by a CI Makefile target referenced from the AC text and
	// no longer needs an allowlist entry — the doc comment above
	// counts as the spec citation.
	"AC-SF-01":   {},
	"AC-SF-02":   {},
	// AC-SF-03 closed by Slice #149 — Test_AST_SettingsPaths_NoEntryWidget
	// in ast_settings_paths_readonly_test.go pins the structural rule:
	// the `newSettingsPaths` constructor in internal/ui/views/settings.go
	// may not invoke any `widget.New*Entry`-shaped call (today it uses
	// only widget.NewLabel / widget.NewLabelWithStyle).
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
	// Slice #127 closed AC-SX-06 (backend half) via the existing
	// TestSettings_ValidationMatrix forbidden-scheme cases. The
	// frontend §5 half (cross-layer fixture) needs the canvas-bound
	// Settings widget harness deferred to Slice #118e — kept honest
	// by not citing AC-SX-06 from any frontend test.
	//
	// Slice #130 closed AC-SX-01..05 via the headless AST/log scanners
	// in `internal/specaudit/ast_settings_security_test.go`.
}

// staleCodeRefAllowlist names every code-side AC ID that does NOT
// resolve to a real spec row. **MUST shrink monotonically.**
//
// Slice #121 reconciled the slice-#119 baseline of 7 stale rows
// to zero:
//
//   - AC-DB-43 / 47 / 53 / 54 / 55 — appeared stale only because
//     the slice-#119 audit scanned `spec/21-app/` instead of `spec/`.
//     They are real rows in `spec/23-app-database/97-acceptance-criteria.md`.
//   - AC-DS-04 — same root cause; lives in
//     `spec/24-app-design-system-and-ui/97-acceptance-criteria.md`.
//   - AC-DB-D-04 — genuinely stale (renaming artifact). The lone
//     reference in `internal/store/queries/queries.go` was rephrased
//     to cite the underlying spec rule directly.
//
// Empty allowlist is the win — every future stale ref must be
// resolved before the slice merges.
var staleCodeRefAllowlist = map[string]struct{}{}

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
	// Slice #121: scan the entire `spec/` tree, not just `spec/21-app/`.
	// The AC matrix lives in five sibling roots (02-coding-guidelines,
	// 07-design-system, 21-app, 23-app-database, 24-app-design-system-
	// and-ui); the slice-#119 narrow scope hid 89 rows and falsely
	// classified 6 real refs as "stale".
	specRoot := filepath.Join(root, "spec")
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
