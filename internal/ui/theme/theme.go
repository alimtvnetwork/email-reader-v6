package theme

import (
	"image/color"
	"log"
	"sync"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// state holds the live theme mode. Reads are RLock-cheap so per-frame
// Color(...) lookups don't serialise. Apply takes the write lock.
//
// Default mode is ThemeDark — matches DefaultSettingsInput().Theme.
var state = struct {
	mu   sync.RWMutex
	mode core.ThemeMode
}{mode: core.ThemeDark}

// Apply switches the active palette. Per spec/24-…/02-theme-implementation
// §3 it is safe to call from any goroutine; Fyne refresh marshalling lives
// in the (build-tagged) fyne_theme.go adapter — this pure-Go core only
// updates the mode pointer.
//
// Unknown / zero ThemeMode → ER-SET-21772 surface (we re-use the existing
// settings code so a single registry entry covers both producers).
func Apply(mode core.ThemeMode) errtrace.Result[struct{}] {
	if !validMode(mode) {
		return errtrace.Err[struct{}](errtrace.WrapCode(
			errtrace.ErrSettingsTheme, "theme.Apply: invalid mode"))
	}
	state.mu.Lock()
	state.mode = mode
	state.mu.Unlock()
	return errtrace.Ok(struct{}{})
}

// Active returns the currently applied mode. Cheap RLock read.
func Active() core.ThemeMode {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.mode
}

// Color resolves a token for the active mode. Unknown name → logs
// ER-UI-21900 (once per process per token, see warnedTokens) and returns
// ColorForeground from the active palette as a safe visible fallback.
// MUST never panic — UI rendering is on the hot path.
func Color(name ColorName) color.Color {
	pal := paletteFor(Active())
	if v, ok := pal[name]; ok {
		return v
	}
	warnUnknown(name)
	return pal[ColorForeground]
}

// paletteFor returns the map for the given mode. ThemeSystem currently
// resolves to ThemeDark — OS detection is tracked under OI-DS-2 and lands
// with the Settings UI consumer (Delta #4 follow-up).
func paletteFor(mode core.ThemeMode) map[ColorName]color.NRGBA {
	if mode == core.ThemeLight {
		return paletteLight
	}
	return paletteDark
}

// validMode mirrors core.ParseThemeMode's accepted set.
func validMode(m core.ThemeMode) bool {
	return m == core.ThemeDark || m == core.ThemeLight || m == core.ThemeSystem
}

// warnedTokens dedupes ER-UI-21900 logs so a buggy view referencing an
// unknown token doesn't spam the log every frame.
var warnedTokens = struct {
	mu  sync.Mutex
	set map[ColorName]struct{}
}{set: map[ColorName]struct{}{}}

func warnUnknown(name ColorName) {
	warnedTokens.mu.Lock()
	defer warnedTokens.mu.Unlock()
	if _, seen := warnedTokens.set[name]; seen {
		return
	}
	warnedTokens.set[name] = struct{}{}
	log.Printf("%s theme: unknown color token %q (falling back to ColorForeground)",
		errtrace.ErrUiThemeUnknownToken, string(name))
}

// resetForTest clears the dedup cache. Test-only; not exported.
func resetForTest() {
	warnedTokens.mu.Lock()
	warnedTokens.set = map[ColorName]struct{}{}
	warnedTokens.mu.Unlock()
	state.mu.Lock()
	state.mode = core.ThemeDark
	state.mu.Unlock()
}
