// linters_smoke_test.go — Slice #120 wiring: invokes the project's
// shell linters as part of `go test ./...` so they fail CI exactly
// as if they were a Go test.
//
// **Why here?** The two linters under audit (`linters/no-incognito-false.sh`
// and `linters/no-other-browser-launch.sh`) are policy enforcers
// that span the whole repo. Their natural home is the same neutral
// host as the spec coverage audit (slice #119) — `internal/specaudit/`
// — for the same reasons documented there: cross-tree scope,
// no production import, allowlist-style ratchet semantics.
//
// **Skip behavior.** Tests skip when:
//   - `bash` is not in PATH (Windows CI without WSL).
//   - The project's `linters/` directory is missing (alternate repo
//     layout — the script-level docstrings document Option A vs.
//     Option B per spec/01-spec-authoring-guide §10).
//
// On a normal sandbox / CI run both linters execute and their exit
// code becomes the test result. A non-zero exit produces a failed
// Go test with the linter's stderr captured in the failure message.
//
// Spec anchors:
//   - linters/no-other-browser-launch.sh  → spec OI-5 / A-01
//   - linters/no-incognito-false.sh       → spec Q-11 (Incognito = false forbidden)
package specaudit

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// scriptPath resolves a linter script under `<repoRoot>/linters/`.
// Returns "" if the file does not exist (caller should `t.Skip`).
func scriptPath(t *testing.T, name string) string {
	t.Helper()
	root := repoRoot(t)
	p := filepath.Join(root, "linters", name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// runLinter shells out to `bash <path>` from the repo root and
// returns the combined stderr/stdout. The repo root is essential
// because the scripts use `find .` relative to the working dir.
func runLinter(t *testing.T, scriptRel string) (exitCode int, output string) {
	t.Helper()
	root := repoRoot(t)
	full := scriptPath(t, scriptRel)
	if full == "" {
		t.Skipf("linters/%s not present in this repo layout — skipping", scriptRel)
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not found in PATH (%v) — skipping shell-linter integration", err)
	}
	cmd := exec.Command("bash", full)
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err == nil {
		return 0, buf.String()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), buf.String()
	}
	t.Fatalf("running %s: %v\noutput:\n%s", scriptRel, err, buf.String())
	return -1, ""
}

// Test_Linter_NoOtherBrowserLaunch wires the OI-5 / A-01 enforcer
// into the Go test suite. PASSes when the script exits 0 (no rogue
// browser-launch sites outside the allowlisted directories).
func Test_Linter_NoOtherBrowserLaunch(t *testing.T) {
	code, out := runLinter(t, "no-other-browser-launch.sh")
	if code != 0 {
		t.Fatalf("linters/no-other-browser-launch.sh exited %d\n\n--- output ---\n%s", code, out)
	}
}

// Test_Linter_NoIncognitoFalse wires the Q-11 enforcer into the
// Go test suite. PASSes when the script exits 0 (no
// `Incognito = false` literal in production code).
func Test_Linter_NoIncognitoFalse(t *testing.T) {
	code, out := runLinter(t, "no-incognito-false.sh")
	if code != 0 {
		t.Fatalf("linters/no-incognito-false.sh exited %d\n\n--- output ---\n%s", code, out)
	}
}
