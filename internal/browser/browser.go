// Package browser locates a Chromium-family browser and launches URLs
// in an incognito/private window. Detection is cached for the process
// lifetime; failures never panic — they log and skip.
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/lovable/email-read/internal/config"
)

// Launcher launches URLs in private mode using a detected browser.
type Launcher struct {
	cfg          config.Browser
	once         sync.Once
	resolvedPath string
	resolvedArg  string
	resolveErr   error
}

// New builds a Launcher from browser config (chromePath / incognitoArg overrides).
func New(cfg config.Browser) *Launcher { return &Launcher{cfg: cfg} }

// Path returns the resolved browser executable path (cached).
func (l *Launcher) Path() (string, error) {
	l.once.Do(l.resolve)
	return l.resolvedPath, l.resolveErr
}

// IncognitoArg returns the private-mode flag for the resolved browser.
func (l *Launcher) IncognitoArg() string {
	l.once.Do(l.resolve)
	return l.resolvedArg
}

// Open spawns `<browser> <incognitoArg> --new-window <url>` and returns immediately.
// The returned error is non-nil only when no browser could be found OR the spawn fails.
func (l *Launcher) Open(url string) error {
	path, err := l.Path()
	if err != nil {
		return err
	}
	arg := l.IncognitoArg()
	args := []string{}
	if arg != "" {
		args = append(args, arg)
	}
	// --new-window is a Chromium flag; harmless on Edge/Brave. Skip for Firefox.
	if !strings.Contains(strings.ToLower(filepath.Base(path)), "firefox") {
		args = append(args, "--new-window")
	}
	args = append(args, url)

	cmd := exec.Command(path, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", path, err)
	}
	// Detach: don't wait. Reap in background to avoid zombies on *nix.
	go func() { _ = cmd.Wait() }()
	return nil
}

// --- resolution -----------------------------------------------------------

func (l *Launcher) resolve() {
	// 1. Config override
	if p := strings.TrimSpace(l.cfg.ChromePath); p != "" && fileExists(p) {
		l.resolvedPath = p
		l.resolvedArg = pickIncognitoArg(p, l.cfg.IncognitoArg)
		return
	}
	// 2. Env var
	if p := strings.TrimSpace(os.Getenv("EMAIL_READ_CHROME")); p != "" && fileExists(p) {
		l.resolvedPath = p
		l.resolvedArg = pickIncognitoArg(p, l.cfg.IncognitoArg)
		return
	}
	// 3. OS-specific defaults
	for _, p := range osDefaults() {
		if fileExists(p) {
			l.resolvedPath = p
			l.resolvedArg = pickIncognitoArg(p, l.cfg.IncognitoArg)
			return
		}
	}
	// 4. PATH lookup as last resort
	for _, name := range []string{"google-chrome", "chrome", "chromium", "chromium-browser", "brave-browser", "microsoft-edge"} {
		if p, err := exec.LookPath(name); err == nil {
			l.resolvedPath = p
			l.resolvedArg = pickIncognitoArg(p, l.cfg.IncognitoArg)
			return
		}
	}
	l.resolveErr = fmt.Errorf("no Chrome/Chromium-family browser found; set config.browser.chromePath or EMAIL_READ_CHROME")
}

func osDefaults() []string {
	switch runtime.GOOS {
	case "windows":
		pf := os.Getenv("ProgramFiles")
		pf86 := os.Getenv("ProgramFiles(x86)")
		lad := os.Getenv("LocalAppData")
		return []string{
			filepath.Join(pf, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(pf86, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(lad, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(pf, `Chromium\Application\chrome.exe`),
			filepath.Join(pf86, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(pf, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(pf, `BraveSoftware\Brave-Browser\Application\brave.exe`),
			filepath.Join(pf86, `BraveSoftware\Brave-Browser\Application\brave.exe`),
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		var out []string
		bins := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "brave-browser", "microsoft-edge"}
		dirs := []string{"/usr/bin", "/usr/local/bin", "/snap/bin", "/var/lib/flatpak/exports/bin"}
		for _, d := range dirs {
			for _, b := range bins {
				out = append(out, filepath.Join(d, b))
			}
		}
		return out
	}
	return nil
}

// pickIncognitoArg honors the user override, otherwise picks based on basename.
func pickIncognitoArg(path, override string) string {
	if override != "" {
		return override
	}
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(base, "firefox"):
		return "-private-window"
	default:
		// chrome / chromium / msedge / brave
		return "--incognito"
	}
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
