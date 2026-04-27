# AC coverage rollout — slice template (Slices #117+)

This file documents the **methodology** every AC-coverage slice has
followed since Slice #117. Read this first when starting Slice #132 or
later — it captures pattern, anti-patterns, and the honest-scope
principle established in Slices #129/#131.

## Goal

Close residual rows in `coverageGapAllowlist` (in
`internal/specaudit/coverage_audit_test.go`). The allowlist shrinks
**monotonically** — `Test_AC_CoverageAudit/gap_no_stale_allow` fails the
build if any allowlisted row gains a citing test (forces removal).

## Slice anatomy

1. **Pick a target family** (AC-SB, AC-DB, AC-SX, AC-PROJ, AC-DS, …)
   from `mem://workflow/progress-tracker.md`.
2. **List uncovered rows** — `grep "AC-XX-" internal/specaudit/coverage_audit_test.go`.
3. **Read the spec rows** — `grep "AC-XX-NN " spec/...97-acceptance-criteria.md`.
4. **Classify each row**:
   - **Closeable now** (headless, no infra blockers) — write the test.
   - **Wired-deferrable** (test can be written, but currently fails on
     pre-existing spec/registry debt) — write the test, use `t.Logf` +
     `t.Skip` so it ratchets when underlying defect is fixed.
   - **Honestly deferred** (needs canvas/bench/E2E harness, schema-evolution
     behaviour work) — leave in allowlist with a comment block explaining
     the blocker.
5. **Implement closeable tests** — follow the existing patterns
   (see "Pattern templates" below).
6. **Remove closed rows from allowlist** — and only those rows.
7. **Verify** — `nix run nixpkgs#go -- test -tags nofyne ./internal/specaudit/`
   must be green AND `Test_AC_CoverageAudit` must report fewer allowlist
   rows than before.
8. **Update memory** — bump `mem://workflow/progress-tracker.md` Last-updated
   line, family table, allowlist count.

## Pattern templates (real examples from this session)

### AST scan over production .go files (template: AC-SX-01, AC-PROJ-18)

```go
func Test_AST_X(t *testing.T) {
    root := repoRootForSXGuard(t)         // shared helper
    var violations []string
    walk := func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if d.IsDir() { return skipUninterestingDirSX(d.Name()) }
        rel, ok := candidateProductionGo(root, path)
        if !ok { return nil }
        // ...parse, scan, append to violations...
        return nil
    }
    if err := filepath.WalkDir(root, walk); err != nil {
        t.Fatalf("walk repo: %v", err)
    }
    if len(violations) > 0 {
        t.Fatalf("AC-XX violation: ...:\n  %s", strings.Join(violations, "\n  "))
    }
}
```

Shared helpers live in `internal/specaudit/ast_settings_security_test.go`
(`repoRootForSXGuard`, `skipUninterestingDirSX`, `candidateProductionGo`,
`candidateTestGo`). **Do not duplicate** — extend them in place if needed.

### Spec-text linter (template: AC-PROJ-31/32/33/34)

```go
func Test_SpecLinter_X(t *testing.T) {
    root := repoRootForSXGuard(t)
    walkSpecMarkdown(t, root, func(abs, rel, body string) {
        clean := stripCodeFences(body)   // strips ``` AND `inline` spans
        for _, m := range myRegex.FindAllStringSubmatch(clean, -1) {
            if isPlaceholderToken(m[0]) { continue }   // ignores XXX/NNNNN
            // ...check, append violations...
        }
    })
    // ...fail or skip with t.Logf...
}
```

### Log scan for value leaks (template: AC-SX-04/05)

Line-based scan with redactor recognition:

```go
if isTestingHelperCall(line) { continue }        // t.Errorf, t.Fatalf
if isRedactedReference(line, needle) { continue } // redactX(field) wraps
```

The `isRedactedReference` helper recognises any `redact*(...)` wrapper —
when a real production leak is found, fix it by introducing a
`redact<Name>(value) string` helper returning a constant marker like
`<set>`/`<none>`. See `internal/cli/read.go` and `internal/watcher/watcher.go`
for the canonical pattern.

## Honest-scope principle (Slice #129/#131 establishment)

If a scanner you write surfaces **real spec/registry debt** that can't
be cleaned up in the same slice without ballooning the diff:

1. ✅ Keep the scanner wired and ratchet-ready.
2. ✅ Use `t.Logf` to record what was found (count + examples).
3. ✅ Use `t.Skip` so the test stays green-bar.
4. ✅ Document the defer in `coverage_audit_test.go` allowlist comment.
5. ❌ Do NOT cite the AC ID in the scanner file (the audit's stale-ref
   guard would treat the citation as coverage — false positive).
6. ❌ Do NOT add the row back to the allowlist with a citing test
   present (the audit will correctly fail).

**Why**: keeps the AC count honest. A skipped test isn't coverage; it's
a tripwire. Honest scope > inflated metrics.

## Anti-patterns to avoid

- ❌ **Citing an AC in a comment without a real test** — the audit treats
  that as coverage and silently inflates the % done.
- ❌ **Adding rows to the allowlist** — only removal is allowed; the
  audit enforces monotonic shrink.
- ❌ **Duplicating `repoRootForSXGuard`** — there's one canonical
  implementation; reuse it.
- ❌ **Mass-fixing spec text inside an AC-coverage slice** — that's
  documentation work; keep the slices small and behaviour-bounded.
- ❌ **Inline-code spans in regex linters** — markdown like
  `[code, msg](err)` looks like a link but isn't. Always strip inline
  code before regex extraction.

## Related files

- `internal/specaudit/coverage_audit_test.go` — the audit + allowlist.
- `internal/specaudit/ast_settings_security_test.go` — AC-SX scanners (Slice #130).
- `internal/specaudit/ast_project_linters_test.go` — AC-PROJ linters (Slice #131).
- `internal/store/ast_*_test.go` — older AST guards (Slices #34-#41).
- `mem://workflow/progress-tracker.md` — running coverage scoreboard.
