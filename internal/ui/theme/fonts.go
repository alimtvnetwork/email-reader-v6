// fonts.go embeds the Inter + JetBrains Mono variable fonts and exposes
// them as Fyne resources for the AppTheme.Font(...) router.
//
// Spec: spec/24-app-design-system-and-ui/01-tokens.md §3 (Typography).
//
// Embed strategy: we declare the embed.FS with a glob pattern that
// matches optional .ttf payloads under fonts/. When the binary assets
// are absent (e.g. fresh checkout, pre-asset CI), embed.FS is empty and
// TextFont/TextMonospaceFont return nil — the AppTheme.Font router falls
// back to Fyne's bundled default font. When the assets ship, the same
// code paths return the embedded resource without any further wiring.
//
// The `all:fonts` directive is intentional: it includes any file under
// fonts/ that matches *.ttf or *.otf, but tolerates an empty directory
// (embed only fails on a glob with zero matches, not on an empty all:
// directory). We keep a `.gitkeep` in fonts/ so the directory exists.
package theme

import (
	"embed"
	"io/fs"

	"fyne.io/fyne/v2"
)

//go:embed all:fonts
var fontFS embed.FS

// fontPath is the canonical filename pair from §3 of the tokens spec.
// Keeping these as constants (instead of inline strings) makes asset
// rename refactors a one-liner and lets tests assert the wiring.
const (
	fontPathInter  = "fonts/Inter-Variable.ttf"
	fontPathJBMono = "fonts/JetBrainsMono-Variable.ttf"
)

// TextFont returns the embedded Inter Variable resource, or nil if the
// asset has not been added to the binary yet. AppTheme.Font(...) treats
// nil as "fall back to Fyne default" so the UI never breaks on a fresh
// checkout — only the typeface differs.
func TextFont() fyne.Resource {
	return loadFontResource(fontPathInter)
}

// TextMonospaceFont returns the embedded JetBrains Mono Variable
// resource, or nil if the asset has not been added yet. Same fallback
// rules as TextFont.
func TextMonospaceFont() fyne.Resource {
	return loadFontResource(fontPathJBMono)
}

// loadFontResource is the shared embed-or-nil helper. Errors (missing
// file, embed.FS empty) collapse to nil — callers MUST treat that as
// "use Fyne default" rather than a hard failure.
func loadFontResource(path string) fyne.Resource {
	data, err := fs.ReadFile(fontFS, path)
	if err != nil || len(data) == 0 {
		return nil
	}
	return fyne.NewStaticResource(filenameOf(path), data)
}

// filenameOf strips the embed-relative path down to the last segment
// for the Fyne resource Name() field — diagnostic only.
func filenameOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
