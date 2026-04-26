// tools_openurl.go renders the OpenUrl sub-tool tab inside Tools. It
// invokes core.Tools.OpenUrl — the only authorised browser-launch path —
// so the same validation, redaction, and dedup rules apply as for
// rule-driven launches. Manual launches use EmailId=0 (no DB audit) so
// the form works with a no-op store stub.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (OpenUrl sub-tool).
//go:build !nofyne

package views

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/store"
)

// noopOpenedUrlStore satisfies core's openedUrlRecorder for manual
// launches (EmailId=0 means HasOpenedUrl is never called and the
// in-memory dedup index alone protects against rapid double-clicks).
type noopOpenedUrlStore struct{}

func (noopOpenedUrlStore) HasOpenedUrl(_ context.Context, _ int64, _ string) (bool, error) {
	return false, nil
}
func (noopOpenedUrlStore) RecordOpenedUrl(_ context.Context, _ int64, _, _ string) (bool, error) {
	return true, nil
}
func (noopOpenedUrlStore) RecordOpenedUrlExt(_ context.Context, _ store.OpenedUrlInsert) (bool, error) {
	return true, nil
}

// BuildOpenUrlTab returns the manual URL launcher body: an URL input,
// a Launch button, and a status line that reflects the OpenUrlReport
// (including Deduped, redacted canonical form, and resolved binary).
func BuildOpenUrlTab() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Open URL — incognito launcher",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Validates scheme, strips userinfo + secret query keys, dedups within 60 s, then opens in a private browser window.")
	subtitle.Wrapping = fyne.TextWrapWord

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/...")
	status := widget.NewLabel("Paste a URL and click Launch.")
	status.Wrapping = fyne.TextWrapWord

	launchBtn := widget.NewButton("Launch", func() { runOpenUrlIntoUI(urlEntry.Text, status) })
	launchBtn.Importance = widget.HighImportance

	header := container.NewVBox(heading, subtitle,
		container.NewBorder(nil, nil, widget.NewLabel("URL:"), launchBtn, urlEntry),
		widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewPadded(status))
}

// runOpenUrlIntoUI fires core.Tools.OpenUrl and reflects the outcome.
func runOpenUrlIntoUI(rawurl string, status *widget.Label) {
	tools, err := buildOpenUrlTools()
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

// buildOpenUrlTools constructs a fresh Tools each launch so live config
// edits (browser path, dedup window) take effect without restart.
func buildOpenUrlTools() (*core.Tools, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	r := core.NewTools(browser.New(cfg.Browser), noopOpenedUrlStore{}, core.DefaultToolsConfig())
	if r.HasError() {
		return nil, r.Error()
	}
	return r.Value(), nil
}
