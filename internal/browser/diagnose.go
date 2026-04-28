// Package browser — diagnostics.
//
// Diagnose() inspects every input the resolver consults (config override,
// EMAIL_READ_CHROME env var, OS-default candidate list, PATH lookup) and
// returns a structured report. Used by `email-read doctor browser` to
// turn opaque "could not open URL" failures into a concrete, copy-pasteable
// trace of WHY the launcher picked (or failed to pick) a binary, and what
// would happen if Open() were called now.
//
// Diagnose has zero side-effects: it does NOT spawn a process. The
// optional Probe(url) helper does spawn (used only when the user asks
// for a live launch test via `--probe <url>`).
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lovable/email-read/internal/config"
)

// DiagnoseSource enumerates which resolver step a candidate came from.
type DiagnoseSource string

const (
	SourceConfigOverride DiagnoseSource = "config.browser.chromePath"
	SourceEnvVar         DiagnoseSource = "env EMAIL_READ_CHROME"
	SourceOSDefault      DiagnoseSource = "os-default"
	SourcePathLookup     DiagnoseSource = "PATH lookup"
)

// DiagnoseCandidate is one path the resolver considered.
type DiagnoseCandidate struct {
	Source DiagnoseSource
	Path   string // empty when Source==EnvVar/ConfigOverride and the value was unset
	Exists bool   // result of fileExists()
	Picked bool   // true for the candidate the resolver would actually choose
}

// DiagnoseReport is the full structured snapshot of resolver inputs.
type DiagnoseReport struct {
	GOOS         string
	ConfigPath   string // resolved config file (for "where to edit")
	ConfigBrowser config.Browser
	EnvChrome    string // raw value of EMAIL_READ_CHROME
	Candidates   []DiagnoseCandidate
	ResolvedPath string // empty if nothing matched
	IncognitoArg string
	WouldSpawn   string // exact argv that Open() would use, or "" on failure
	Error        string // resolver error, if any
}

// Diagnose builds a DiagnoseReport from the given config without spawning.
// Pure inspection — safe to call from any context.
func Diagnose(cfg config.Browser) DiagnoseReport {
	cfgPath, _ := config.Path()
	rep := DiagnoseReport{
		GOOS:          runtime.GOOS,
		ConfigPath:    cfgPath,
		ConfigBrowser: cfg,
		EnvChrome:     os.Getenv("EMAIL_READ_CHROME"),
	}

	// Build the candidate list in the SAME order resolve() walks.
	var candidates []DiagnoseCandidate

	// 1. Config override
	if p := strings.TrimSpace(cfg.ChromePath); p != "" {
		candidates = append(candidates, DiagnoseCandidate{
			Source: SourceConfigOverride, Path: p, Exists: fileExists(p),
		})
	} else {
		candidates = append(candidates, DiagnoseCandidate{
			Source: SourceConfigOverride, Path: "", Exists: false,
		})
	}

	// 2. Env var
	if p := strings.TrimSpace(rep.EnvChrome); p != "" {
		candidates = append(candidates, DiagnoseCandidate{
			Source: SourceEnvVar, Path: p, Exists: fileExists(p),
		})
	} else {
		candidates = append(candidates, DiagnoseCandidate{
			Source: SourceEnvVar, Path: "", Exists: false,
		})
	}

	// 3. OS-specific defaults
	for _, p := range osDefaults() {
		candidates = append(candidates, DiagnoseCandidate{
			Source: SourceOSDefault, Path: p, Exists: fileExists(p),
		})
	}

	// 4. PATH lookup
	for _, name := range []string{"google-chrome", "chrome", "chromium", "chromium-browser", "brave-browser", "microsoft-edge"} {
		p, err := exec.LookPath(name)
		if err != nil {
			candidates = append(candidates, DiagnoseCandidate{
				Source: SourcePathLookup, Path: name + " (not on PATH)", Exists: false,
			})
			continue
		}
		candidates = append(candidates, DiagnoseCandidate{
			Source: SourcePathLookup, Path: p, Exists: true,
		})
	}

	// Mark first existing candidate (with non-empty Path) as picked.
	for i := range candidates {
		c := candidates[i]
		if c.Exists && c.Path != "" {
			candidates[i].Picked = true
			rep.ResolvedPath = c.Path
			rep.IncognitoArg = pickIncognitoArg(c.Path, cfg.IncognitoArg)
			break
		}
	}

	if rep.ResolvedPath == "" {
		rep.Error = "no Chrome/Chromium-family browser found; set config.browser.chromePath or EMAIL_READ_CHROME"
	} else {
		// Show the exact argv Open() would build.
		args := []string{rep.ResolvedPath}
		if rep.IncognitoArg != "" {
			args = append(args, rep.IncognitoArg)
		}
		if !strings.Contains(strings.ToLower(filepath.Base(rep.ResolvedPath)), "firefox") {
			args = append(args, "--new-window")
		}
		args = append(args, "<url>")
		rep.WouldSpawn = strings.Join(args, " ")
	}

	rep.Candidates = candidates
	return rep
}

// Probe spawns the resolved browser against a URL and returns the spawn
// error (or nil). This DOES launch a real window — call only when the
// user explicitly opts in (e.g. `doctor browser --probe https://…`).
func (l *Launcher) Probe(url string) error {
	return l.Open(url)
}

// FormatReport renders a DiagnoseReport as a step-numbered text block
// in the same style as `email-read doctor` and `email-read diagnose`.
// Kept in this package so callers (CLI + future UI Tools card) share format.
func FormatReport(rep DiagnoseReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Browser launch diagnostic\n")
	fmt.Fprintf(&b, "GOOS:    %s\n", rep.GOOS)
	fmt.Fprintf(&b, "Config:  %s\n", rep.ConfigPath)
	fmt.Fprintf(&b, "  config.browser.chromePath   = %q\n", rep.ConfigBrowser.ChromePath)
	fmt.Fprintf(&b, "  config.browser.incognitoArg = %q\n", rep.ConfigBrowser.IncognitoArg)
	fmt.Fprintf(&b, "  env EMAIL_READ_CHROME       = %q\n", rep.EnvChrome)
	fmt.Fprintf(&b, "\nCandidates (in resolver order):\n")
	for i, c := range rep.Candidates {
		mark := " "
		if c.Picked {
			mark = "✓"
		} else if c.Path == "" {
			mark = "·"
		} else if !c.Exists {
			mark = "✗"
		}
		fmt.Fprintf(&b, "  %s [%2d] %-32s %s\n", mark, i+1, c.Source, c.Path)
	}
	fmt.Fprintln(&b)
	if rep.Error != "" {
		fmt.Fprintf(&b, "Result: ✗ %s\n", rep.Error)
		fmt.Fprintln(&b, "Fix:    set config.browser.chromePath in your config file,")
		fmt.Fprintln(&b, "        OR `export EMAIL_READ_CHROME=/path/to/chrome`,")
		fmt.Fprintln(&b, "        OR install Chrome/Chromium/Brave/Edge so it appears on PATH.")
	} else {
		fmt.Fprintf(&b, "Result: ✓ resolved %s\n", rep.ResolvedPath)
		fmt.Fprintf(&b, "Spawn:  %s\n", rep.WouldSpawn)
	}
	return b.String()
}
