// errtrace_lint_mode_test.go — Slice #143 / Task #7 close-out.
//
// Pins two invariants for the three errtrace lint scripts under
// `linter-scripts/`:
//
//  1. The default `LINT_MODE` is `fail` — i.e. the bash parameter-
//     expansion `LINT_MODE="${LINT_MODE:-fail}"` is present verbatim.
//     This prevents a future PR from silently downgrading the
//     gate to `warn` (which would let new bare-wraps / fmt.Errorf /
//     errors.New sites land without breaking the build).
//
//  2. Each script currently exits 0 when invoked with no env var.
//     That confirms the live tree has zero violations under the
//     enforcing default, so #1 above is meaningful (not vacuous
//     because we happen to be in warn-only mode somewhere).
//
// Skips when bash is unavailable (Windows CI without WSL) or when
// the linter-scripts/ directory is absent (alternate repo layout).
//
// Spec anchors:
//   - .lovable/plan.md → Phase 2 (enforcing error-trace guardrails)
//   - mem://preferences/01-error-stack-traces.md
package specaudit

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// errtraceLintScripts is the canonical set of three Phase-2 guardrails.
// Add new errtrace gates here — the table-driven tests below will pick
// them up automatically.
var errtraceLintScripts = []string{
	"check-no-fmt-errorf.sh",
	"check-no-errors-new.sh",
	"check-no-bare-return-err.sh",
}

// Test_ErrtraceLintScripts_DefaultLintModeIsFail asserts each script
// contains the literal `LINT_MODE="${LINT_MODE:-fail}"` line. We match
// the full expansion (not just the substring "fail") so a sloppy edit
// like `LINT_MODE="fail"` (which ignores the env var) also fails this
// test — Phase 2 demands env-overridable but fail-by-default behaviour.
func Test_ErrtraceLintScripts_DefaultLintModeIsFail(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "linter-scripts")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("linter-scripts/ not present in this repo layout: %v", err)
	}
	const want = `LINT_MODE="${LINT_MODE:-fail}"`
	for _, name := range errtraceLintScripts {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, name)
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read %s: %v", p, err)
			}
			if !strings.Contains(string(b), want) {
				t.Fatalf("%s missing default-mode pin %q — Phase 2 requires fail-by-default", name, want)
			}
		})
	}
}

// Test_ErrtraceLintScripts_PassUnderDefault runs each script with an
// EMPTY environment for LINT_MODE (so the script's own `:-fail`
// default kicks in) and asserts exit 0. If the live tree regrows a
// fmt.Errorf / errors.New / bare `return err` site, this test breaks
// loud and the offender shows up in the script's stdout.
func Test_ErrtraceLintScripts_PassUnderDefault(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not found in PATH (%v) — skipping shell-linter integration", err)
	}
	root := repoRoot(t)
	dir := filepath.Join(root, "linter-scripts")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("linter-scripts/ not present in this repo layout: %v", err)
	}
	for _, name := range errtraceLintScripts {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, name)
			cmd := exec.Command("bash", p)
			cmd.Dir = root
			// Scrub LINT_MODE so the script's own default applies.
			cmd.Env = scrubLintMode(os.Environ())
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			err := cmd.Run()
			if err == nil {
				return
			}
			if ee, ok := err.(*exec.ExitError); ok {
				t.Fatalf("%s exited %d under default LINT_MODE\n\n--- output ---\n%s",
					name, ee.ExitCode(), buf.String())
			}
			t.Fatalf("running %s: %v\noutput:\n%s", name, err, buf.String())
		})
	}
}

// scrubLintMode returns env with any `LINT_MODE=...` entry removed so
// the child process falls through to the script's `:-fail` default.
// Pulled out so the table-driven test stays readable.
func scrubLintMode(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "LINT_MODE=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
