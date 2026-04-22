//go:build !nofyne

package views

import (
	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
)

// launchInBrowser resolves the configured browser and launches the URL in
// incognito. Errors propagate to the caller so the UI can surface them.
func launchInBrowser(rawurl string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	l := browser.New(cfg.Browser)
	return l.Open(rawurl)
}
