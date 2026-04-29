// reveal.go centralizes "reveal a file in the OS file manager" so the
// `linters/no-other-browser-launch.sh` rule keeps its single-allowlist
// guarantee: every shell-out to a system handler lives in this
// package. Slice #212 (Settings path-panel redesign) added this when
// the Settings "Reveal" button needed cross-platform `open -R` /
// `explorer /select,` / `xdg-open` calls.
//
// Spec linkage:
//   - spec/21-app/02-features/06-tools/99-consistency-report.md row OI-5
//     ("only internal/browser/ may shell out to a system launcher")
//   - spec/21-app/02-features/07-settings/02-frontend.md (Filesystem
//     locations card; Reveal button on the Config row).
package browser

import (
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/lovable/email-read/internal/errtrace"
)

// RevealInFileManager opens the OS file manager with `path` selected.
// Cross-platform behaviour:
//
//   - macOS:   `open -R <path>` (Finder reveals + selects)
//   - Windows: `explorer /select,<path>` (Explorer reveals + selects)
//   - Linux:   `xdg-open <parent dir>` (best-effort; xdg-open has no
//     portable single-file-selector flag, so we land the user in the
//     parent directory and they're one click away from the file).
//
// Returns a wrapped error so the caller (Settings view) can render it
// inline next to the Reveal button instead of opening a popup dialog.
// Pure shell-out; no Fyne dependency, so it stays unit-testable on
// headless CI.
func RevealInFileManager(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return errtrace.Wrap(err, "RevealInFileManager: abs")
	}
	switch runtime.GOOS {
	case "darwin":
		if err := exec.Command("open", "-R", abs).Start(); err != nil {
			return errtrace.Wrap(err, "RevealInFileManager: open -R")
		}
	case "windows":
		if err := exec.Command("explorer", "/select,"+abs).Start(); err != nil {
			return errtrace.Wrap(err, "RevealInFileManager: explorer /select")
		}
	default:
		parent := filepath.Dir(abs)
		if err := exec.Command("xdg-open", parent).Start(); err != nil {
			return errtrace.Wrap(err, "RevealInFileManager: xdg-open")
		}
	}
	return nil
}