// emails.go renders the Emails tab: a left-side list of stored emails for
// the selected account and a right-side detail pane showing the body plus
// extracted URLs as clickable buttons (each opens in incognito via the
// configured browser launcher).
//
// Behind the !nofyne build tag because it imports the Fyne widget set; the
// pure-Go url-extraction helper lives in links.go so it stays testable.
//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// EmailsOptions wires the emails view to app state + side effects. The
// loader/opener seams default to the real core implementations but can be
// overridden in tests (when a Fyne harness is available).
type EmailsOptions struct {
	Alias      string                                                                 // selected account alias; "" ⇒ show empty state
	List       func(ctx context.Context, opts core.ListEmailsOptions) ([]core.EmailSummary, error) // override for tests
	Get        func(ctx context.Context, alias string, uid uint32) (*core.EmailDetail, error)      // override for tests
	OpenURL    func(rawurl string) error                                              // override for tests
	MaxResults int                                                                    // 0 ⇒ default 200
}

// BuildEmails returns the Emails view. When no alias is selected an
// empty-state placeholder is shown so the user knows to pick an account.
func BuildEmails(opts EmailsOptions) fyne.CanvasObject {
	if opts.List == nil {
		opts.List = core.ListEmails
	}
	if opts.Get == nil {
		opts.Get = core.GetEmail
	}
	if opts.OpenURL == nil {
		opts.OpenURL = openExternal
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 200
	}

	heading := widget.NewLabelWithStyle("Emails", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	if opts.Alias == "" {
		hint := widget.NewLabel("Pick an account from the sidebar to browse stored emails.")
		hint.Wrapping = fyne.TextWrapWord
		return container.NewVBox(heading, widget.NewSeparator(), hint)
	}

	status := widget.NewLabel("Loading…")
	status.Wrapping = fyne.TextWrapWord

	// Detail pane state — rebuilt on each list selection.
	detailBox := container.NewVBox(widget.NewLabel("Select an email on the left."))
	detailScroll := container.NewVScroll(detailBox)

	// Initial list load (synchronous; cheap — local sqlite).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := opts.List(ctx, core.ListEmailsOptions{Alias: opts.Alias, Limit: opts.MaxResults})
	if err != nil {
		warn := widget.NewLabel("⚠ Failed to load emails: " + err.Error())
		warn.Wrapping = fyne.TextWrapWord
		return container.NewVBox(heading, widget.NewSeparator(), warn)
	}
	if len(rows) == 0 {
		empty := widget.NewLabel(fmt.Sprintf("No emails stored yet for %q. Run a watch or one-shot fetch first.", opts.Alias))
		empty.Wrapping = fyne.TextWrapWord
		return container.NewVBox(heading, widget.NewSeparator(), empty)
	}

	list := widget.NewList(
		func() int { return len(rows) },
		func() fyne.CanvasObject {
			subj := widget.NewLabelWithStyle("subject", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			meta := widget.NewLabel("from · date")
			return container.NewVBox(subj, meta)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			subj := box.Objects[0].(*widget.Label)
			meta := box.Objects[1].(*widget.Label)
			r := rows[i]
			s := r.Subject
			if s == "" {
				s = "(no subject)"
			}
			subj.SetText(s)
			meta.SetText(fmt.Sprintf("%s · %s", r.From, r.ReceivedAt))
		},
	)

	list.OnSelected = func(i widget.ListItemID) {
		r := rows[i]
		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()
		detail, err := opts.Get(dctx, r.Alias, r.Uid)
		if err != nil {
			detailBox.Objects = []fyne.CanvasObject{
				widget.NewLabel("⚠ Failed to load email: " + err.Error()),
			}
			detailBox.Refresh()
			return
		}
		detailBox.Objects = renderDetail(detail, opts.OpenURL, status)
		detailBox.Refresh()
		detailScroll.ScrollToTop()
	}

	status.SetText(fmt.Sprintf("%d email(s) for %s.", len(rows), opts.Alias))

	split := container.NewHSplit(list, detailScroll)
	split.SetOffset(0.35)

	return container.NewBorder(
		container.NewVBox(heading, widget.NewSeparator()),
		status,
		nil, nil,
		split,
	)
}

// renderDetail builds the right-pane widgets for one email. Returns the
// slice of objects so the caller can swap them into a stack/box in place.
func renderDetail(d *core.EmailDetail, open func(string) error, status *widget.Label) []fyne.CanvasObject {
	subject := d.Subject
	if subject == "" {
		subject = "(no subject)"
	}
	subjectLbl := widget.NewLabelWithStyle(subject, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subjectLbl.Wrapping = fyne.TextWrapWord

	headerForm := widget.NewForm(
		widget.NewFormItem("From", widget.NewLabel(d.From)),
		widget.NewFormItem("To", widget.NewLabel(d.To)),
		widget.NewFormItem("Date", widget.NewLabel(d.ReceivedAt)),
	)

	body := d.BodyText
	if body == "" {
		body = "(no plain-text body — see HTML version)"
	}
	bodyLbl := widget.NewLabel(body)
	bodyLbl.Wrapping = fyne.TextWrapWord

	objs := []fyne.CanvasObject{
		subjectLbl,
		headerForm,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Body", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		bodyLbl,
	}

	// Collect links from both bodies — HTML often has the canonical href.
	urls := ExtractUrls(d.BodyText + "\n" + d.BodyHtml)
	if len(urls) > 0 {
		objs = append(objs, widget.NewSeparator(),
			widget.NewLabelWithStyle(
				fmt.Sprintf("Links (%d) — click to open in incognito", len(urls)),
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
			))
		for _, u := range urls {
			u := u // capture
			btn := widget.NewButton(u, func() {
				if err := open(u); err != nil {
					status.SetText("⚠ Open failed: " + err.Error())
					return
				}
				status.SetText("Opened: " + u)
			})
			btn.Alignment = widget.ButtonAlignLeading
			objs = append(objs, btn)
		}
	}
	return objs
}

// openExternal launches a URL via the OS-default browser. The Emails view
// uses this rather than the rules-engine launcher because it represents a
// direct user click (not a rule match) — same incognito intent applies but
// we keep behaviour predictable: parse, then hand off.
func openExternal(rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	return launchInBrowser(u.String())
}
