// tools_openurl.go renders the OpenUrl sub-tool tab inside Tools. It
// invokes core.Tools.OpenUrl — the only authorised browser-launch path —
// so the same validation, redaction, and dedup rules apply as for
// rule-driven launches. Manual launches use EmailId=0 (no DB audit) so
// the form works with a no-op store stub.
//
// **Slice #116c (Phase 6.3) refactor.** The per-call `*core.Tools`
// constructor used to live here as `buildOpenUrlTools`, calling
// `config.Load()` directly. That violated the Phase-2 view-layer
// purity contract (only allowlisted because the AST guard predates
// the Tools refactor). The factory is now injected via the
// `ToolsFactory` parameter on `BuildOpenUrlTab`, sourced from the
// shell's `*Services` bundle (`services.Tools`). The `noopOpenedUrlStore`
// type and helper move with the factory into `internal/ui/services.go`.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (OpenUrl sub-tool).
//go:build !nofyne

package views

import (
	"context"
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// BuildOpenUrlTab returns the manual URL launcher body: an URL input,
// a Launch button, and a status line that reflects the OpenUrlReport
// (including Deduped, redacted canonical form, and resolved binary).
//
// `factory` is the injected per-call `*core.Tools` builder (see
// `internal/ui/services.go::ToolsFactory`). When nil, the Launch
// button renders the documented degraded-path status instead of
// panicking — matches how Dashboard / Emails handle a missing
// service after a soft-failed bootstrap.
func BuildOpenUrlTab(factory ToolsFactory) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Open URL — incognito launcher",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Validates scheme, strips userinfo + secret query keys, dedups within 60 s, then opens in a private browser window.")
	subtitle.Wrapping = fyne.TextWrapWord

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/...")
	status := widget.NewLabel("Paste a URL and click Launch.")
	status.Wrapping = fyne.TextWrapWord

	launchBtn := widget.NewButton("Launch", func() { runOpenUrlIntoUI(factory, urlEntry.Text, status) })
	launchBtn.Importance = widget.HighImportance

	header := container.NewVBox(heading, subtitle,
		container.NewBorder(nil, nil, widget.NewLabel("URL:"), launchBtn, urlEntry),
		widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewPadded(status))
}

// runOpenUrlIntoUI fires core.Tools.OpenUrl and reflects the outcome.
func runOpenUrlIntoUI(factory ToolsFactory, rawurl string, status *widget.Label) {
	tools, err := buildToolsFromFactory(factory)
	if err != nil {
		status.SetText("⚠ setup: " + err.Error())
		return
	}
	r := tools.OpenUrl(context.Background(),
		core.OpenUrlSpec{Url: rawurl, Origin: core.OriginManual})
	if r.HasError() {
		status.SetText("⚠ " + r.Error().Error())
		return
	}
	rep := r.Value()
	if rep.Deduped {
		status.SetText(fmt.Sprintf("● already opened recently — skipped (canonical: %s)", rep.Url))
		return
	}
	status.SetText(fmt.Sprintf("✓ launched in incognito\nbinary: %s\nurl: %s", rep.BrowserBinary, rep.Url))
}

// buildToolsFromFactory is the shared sub-tab guard around the
// injected `ToolsFactory`. Returns a typed sentinel error when the
// factory is nil so each sub-tab surfaces the same degraded-path
// status text without coupling on a string match.
//
// Slice #116c hoist: replaces the per-file `buildOpenUrlTools` /
// `buildReadTools` constructors that each called `config.Load()`
// inline.
func buildToolsFromFactory(factory ToolsFactory) (*core.Tools, error) {
	if factory == nil {
		return nil, errToolsFactoryUnavailable
	}
	return factory()
}

// errToolsFactoryUnavailable is the typed sentinel surfaced when the
// shell's `*Services.Tools` field is nil (e.g. boot-time wiring
// failure). Kept package-private — sub-tabs render its message via
// the standard "⚠ setup:" status prefix.
var errToolsFactoryUnavailable = errors.New("tools factory unavailable (shell bootstrap may have failed)")
