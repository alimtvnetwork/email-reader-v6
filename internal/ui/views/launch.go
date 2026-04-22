// launch.go — small wrapper around internal/browser used by the Emails
// view to honour the user's configured browser/incognito flag. Behind the
// !nofyne tag only because it's only consumed by fyne-tagged code; the
// underlying internal/browser package is plain Go.
//go:build !nofyne

package views

import (
	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
)

// launchInBrowser resolves the configured browser once per process and
// launches the given URL in incognito. Errors propagate to the caller so
// the UI status line can surface them.
func launchInBrowser(rawurl string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	l := browser.New(cfg.Browser)
	return l.Open(rawurl)
}
