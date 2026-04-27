//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/ui/errlog"
)

// EmailsOptions wires the emails view to data + actions.
//
// **Phase 2.5 migration.** The old shape defaulted `List`/`Get` to
// the deprecated package-level `core.ListEmails` / `core.GetEmail`.
// The new shape requires a typed `*core.EmailsService` (constructed
// once at app boot via `core.NewEmailsService`). `List` / `Get`
// survive as optional overrides used exclusively by tests to inject
// deterministic rows without standing up a real service. When the
// override is nil we delegate to the service. When both Service and
// the override are nil we render a degraded view rather than
// panicking — keeps headless / partial-bootstrap previews safe.
type EmailsOptions struct {
	Alias      string
	Service    *core.EmailsService // production seam — constructed in app bootstrap
	List       func(ctx context.Context, opts core.ListEmailsOptions) errtrace.Result[[]core.EmailSummary]
	Get        func(ctx context.Context, alias string, uid uint32) errtrace.Result[*core.EmailDetail]
	OpenURL    func(rawurl string) error
	MaxResults int
}

func BuildEmails(opts EmailsOptions) fyne.CanvasObject {
	opts = applyEmailsDefaults(opts)
	heading := widget.NewLabelWithStyle("Emails", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	if opts.Alias == "" {
		return emailsEmptyAlias(heading)
	}
	if opts.List == nil || opts.Get == nil {
		// Degraded path: bootstrap didn't wire a *EmailsService and no
		// test overrides were supplied. Surface the wiring gap instead
		// of panicking on the first list call.
		return emailsErrorView(heading,
			fmt.Errorf("emails service not wired (no Service or List/Get overrides injected)"))
	}

	// Body is a swappable container so the Refresh button can
	// re-render the list/detail tree in place without rebuilding the
	// outer layout (which would also drop the toolbar focus).
	body := container.NewStack()
	render := func() {
		body.Objects = []fyne.CanvasObject{buildEmailsBody(opts)}
		body.Refresh()
	}
	render()

	toolbar := buildEmailsToolbar(opts, render)
	if toolbar == nil {
		return container.NewBorder(
			container.NewVBox(heading, widget.NewSeparator()),
			nil, nil, nil, body,
		)
	}
	return container.NewBorder(
		container.NewVBox(heading, toolbar, widget.NewSeparator()),
		nil, nil, nil, body,
	)
}

// buildEmailsBody runs the per-render data fetch and returns the
// rows-or-error widget. Extracted from BuildEmails so the Refresh
// button (buildEmailsToolbar) can swap it in place without touching
// the surrounding chrome (heading + toolbar). Body variants pass
// `nil` for the inner heading because the outer Border already
// renders it — passing a real heading here would double-stack it
// after every Refresh click.
func buildEmailsBody(opts EmailsOptions) fyne.CanvasObject {
	rows, err := loadEmailRows(opts)
	if err != nil {
		return emailsErrorBody(err)
	}
	if len(rows) == 0 {
		return emailsEmptyRowsBody(opts.Alias)
	}
	return buildEmailsBrowserBody(opts, rows)
}

// buildEmailsToolbar returns the "🔄 Refresh" action row — but only
// when the wired EmailsService can actually do the work (a Refresher
// was injected via WithRefresher at bootstrap; see app.go NavEmails
// arm). Returns nil in degraded modes so the toolbar simply doesn't
// render rather than displaying a button that always errors.
//
// The button uses a 30s timeout: a single IMAP poll cycle (connect,
// SELECT, fetch new UIDs, persist, evaluate rules) typically
// finishes in <2s but can spike on cold connections or large new-
// message batches. 30s matches the `Run` loop's per-cycle budget.
func buildEmailsToolbar(opts EmailsOptions, onRefresh func()) fyne.CanvasObject {
	if opts.Service == nil {
		return nil
	}
	status := widget.NewLabel("")
	btn := widget.NewButton("🔄 Refresh", func() {
		status.SetText("Refreshing…")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if res := opts.Service.Refresh(ctx, opts.Alias); res.HasError() {
			status.SetText("⚠ Refresh failed: " + res.Error().Error())
			return
		}
		status.SetText("")
		onRefresh()
	})
	return container.NewHBox(btn, status)
}

// applyEmailsDefaults fills test-override seams from the injected
// service when present, then applies the standard fallbacks for the
// non-data dependencies (OpenURL, MaxResults). Test overrides take
// precedence over the service so existing test fixtures continue to
// work unchanged.
// applyEmailsDefaults fills test-override seams from the injected
// service when present. The non-data dependencies (`OpenURL`,
// `MaxResults`) get standard fallbacks too: `MaxResults` defaults
// to 200, and `OpenURL` defaults to a typed-error stub when the
// shell did not inject one (Slice #117 / Phase 6.5 removed the
// inline `openExternal` → `launchInBrowser` → `config.Load()`
// path; production callers now receive the launcher via
// `services.OpenURL`). Test fixtures that need to assert link
// behaviour can still pass their own `OpenURL` closure.
func applyEmailsDefaults(opts EmailsOptions) EmailsOptions {
	if opts.Service != nil {
		if opts.List == nil {
			opts.List = opts.Service.List
		}
		if opts.Get == nil {
			opts.Get = opts.Service.Get
		}
	}
	if opts.OpenURL == nil {
		opts.OpenURL = openURLUnwired
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 200
	}
	return opts
}

// openURLUnwired is the documented degraded-path callback used when
// neither the shell nor a test fixture injected an `OpenURL`
// closure. Returns a typed error the link-click handler converts
// into a status banner. Replaces the pre-Slice-#117
// `openExternal` → `launchInBrowser` chain that called
// `config.Load()` directly from the view layer.
func openURLUnwired(string) error {
	return fmt.Errorf("browser launcher not wired (no OpenURL injected)")
}

// loadEmailRows fetches the email summary list with a 5s timeout.
func loadEmailRows(opts EmailsOptions) ([]core.EmailSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res := opts.List(ctx, core.ListEmailsOptions{Alias: opts.Alias, Limit: opts.MaxResults})
	if res.HasError() {
		return nil, res.Error()
	}
	return res.Value(), nil
}

// emailsEmptyAlias renders the "pick an account" hint.
func emailsEmptyAlias(heading fyne.CanvasObject) fyne.CanvasObject {
	hint := widget.NewLabel("Pick an account from the sidebar to browse stored emails.")
	hint.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), hint)
}

// emailsErrorView renders a load-failure warning. The full trace is
// also routed through errlog so the Diagnostics → Error Log view shows
// the file:line chain even though the inline label is one line.
func emailsErrorView(heading fyne.CanvasObject, err error) fyne.CanvasObject {
	errlog.ReportError("emails", err)
	warn := widget.NewLabel("⚠ Failed to load emails: " + err.Error() + " — see Diagnostics → Error Log")
	warn.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), warn)
}

// emailsEmptyRows renders the empty-state for accounts with no stored emails.
func emailsEmptyRows(heading fyne.CanvasObject, alias string) fyne.CanvasObject {
	empty := widget.NewLabel(fmt.Sprintf("No emails stored yet for %q. Run a watch or one-shot fetch first.", alias))
	empty.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), empty)
}

// buildEmailsBrowser composes the split-pane list + detail browser.
func buildEmailsBrowser(heading fyne.CanvasObject, opts EmailsOptions, rows []core.EmailSummary) fyne.CanvasObject {
	status := widget.NewLabel("Loading…")
	status.Wrapping = fyne.TextWrapWord
	detailBox := container.NewVBox(widget.NewLabel("Select an email on the left."))
	detailScroll := container.NewVScroll(detailBox)

	list := newEmailList(rows)
	list.OnSelected = makeEmailSelectHandler(opts, rows, detailBox, detailScroll, status)

	status.SetText(fmt.Sprintf("%d email(s) for %s.", len(rows), opts.Alias))
	split := container.NewHSplit(list, detailScroll)
	split.SetOffset(0.35)
	return container.NewBorder(
		container.NewVBox(heading, widget.NewSeparator()),
		status, nil, nil, split,
	)
}

// emailsErrorBody renders the load-failure warning WITHOUT a heading
// row — used by the per-render `buildEmailsBody` swap path where the
// heading is owned by the outer Border.
func emailsErrorBody(err error) fyne.CanvasObject {
	errlog.ReportError("emails", err)
	warn := widget.NewLabel("⚠ Failed to load emails: " + err.Error() + " — see Diagnostics → Error Log")
	warn.Wrapping = fyne.TextWrapWord
	return container.NewVBox(warn)
}

// emailsEmptyRowsBody renders the empty-state WITHOUT a heading row.
// Mirrors `emailsEmptyRows` minus the heading + separator since the
// outer Border already provides that chrome.
func emailsEmptyRowsBody(alias string) fyne.CanvasObject {
	empty := widget.NewLabel(fmt.Sprintf("No emails stored yet for %q. Run a watch or one-shot fetch first.", alias))
	empty.Wrapping = fyne.TextWrapWord
	return container.NewVBox(empty)
}

// buildEmailsBrowserBody composes the split-pane list + detail
// browser WITHOUT a leading heading row. Used by the per-render swap
// path. Identical to `buildEmailsBrowser` minus the heading-bearing
// Border `top` argument.
func buildEmailsBrowserBody(opts EmailsOptions, rows []core.EmailSummary) fyne.CanvasObject {
	status := widget.NewLabel("Loading…")
	status.Wrapping = fyne.TextWrapWord
	detailBox := container.NewVBox(widget.NewLabel("Select an email on the left."))
	detailScroll := container.NewVScroll(detailBox)

	list := newEmailList(rows)
	list.OnSelected = makeEmailSelectHandler(opts, rows, detailBox, detailScroll, status)

	status.SetText(fmt.Sprintf("%d email(s) for %s.", len(rows), opts.Alias))
	split := container.NewHSplit(list, detailScroll)
	split.SetOffset(0.35)
	return container.NewBorder(nil, status, nil, nil, split)
}

// newEmailList builds the email summary list widget.
func newEmailList(rows []core.EmailSummary) *widget.List {
	return widget.NewList(
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
}

// makeEmailSelectHandler returns the row-selection callback that loads detail.
func makeEmailSelectHandler(opts EmailsOptions, rows []core.EmailSummary, detailBox *fyne.Container, detailScroll *container.Scroll, status *widget.Label) func(widget.ListItemID) {
	return func(i widget.ListItemID) {
		r := rows[i]
		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()
		res := opts.Get(dctx, r.Alias, r.Uid)
		if res.HasError() {
			detailBox.Objects = []fyne.CanvasObject{
				widget.NewLabel("⚠ Failed to load email: " + res.Error().Error()),
			}
			detailBox.Refresh()
			return
		}
		detailBox.Objects = renderDetail(res.Value(), opts.OpenURL, status)
		detailBox.Refresh()
		detailScroll.ScrollToTop()
	}
}

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

	urls := ExtractUrls(d.BodyText + "\n" + d.BodyHtml)
	if len(urls) > 0 {
		objs = append(objs, widget.NewSeparator(),
			widget.NewLabelWithStyle(
				fmt.Sprintf("Links (%d) — click to open in incognito", len(urls)),
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
			))
		for _, u := range urls {
			u := u
			btn := widget.NewButton(u, func() {
				if err := open(u); err != nil {
					errlog.ReportError("emails.open_url", err)
					status.SetText("⚠ Open failed: " + err.Error() + " — see Diagnostics → Error Log")
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
