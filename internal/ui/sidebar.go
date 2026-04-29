// sidebar.go renders NavItems as a Fyne List + an account picker on top.
// The data lives in nav.go and AppState lives in state.go (both fyne-free)
// so headless CI can test the canonical nav order and observer pattern.
//go:build !nofyne

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/ui/errlog"
)

// SidebarOptions wires the sidebar to the shared AppState plus the live
// account list. OnSelectNav is invoked synchronously when the user picks
// a row so the shell can swap the detail pane.
//
// BadgeFor (Phase 3.4) returns the unread-count badge to render next
// to a nav row's title — used by NavErrorLog to show the
// "(N)" suffix from `errlog.Unread()`. Returning 0 (or leaving the
// field nil) renders the plain title. BadgeSubscribe returns a
// channel that ticks whenever any badge value may have changed; the
// sidebar refreshes the list on each tick. Both fields default to
// the process-wide errlog singleton when nil.
//
// OnErrorLogOpened (Phase 3.5) is invoked when the user selects the
// NavErrorLog row, in addition to OnSelectNav. The shell uses it to
// reset the toast notifier's quiet-period flag so the next batch of
// errors after the user catches up fires a fresh toast.
type SidebarOptions struct {
	State            *AppState
	Aliases          []string
	OnSelectNav      func(NavItem)
	BadgeFor         func(NavKind) int64
	BadgeSubscribe   func() <-chan errlog.Entry
	OnErrorLogOpened func()
}

// NewSidebar builds the sidebar: header, account picker, nav list.
func NewSidebar(opts SidebarOptions) fyne.CanvasObject {
	if opts.BadgeFor == nil {
		opts.BadgeFor = defaultBadgeFor
	}
	if opts.BadgeSubscribe == nil {
		opts.BadgeSubscribe = errlog.Subscribe
	}

	header := widget.NewLabelWithStyle("email-read", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	var picker *widget.Select
	if len(opts.Aliases) == 0 {
		picker = widget.NewSelect([]string{"No accounts — add one"}, nil)
		picker.Disable()
		// CF-A4 (Slice #198): clear any stale alias inherited from a
		// previous shell render. Without this the detail pane (Watch,
		// Emails, …) keeps reading `state.Alias()` and wires Start to
		// an account that no longer exists, producing the "0 accounts
		// but watcher logs `[Admin] poll error`" regression.
		if opts.State != nil && opts.State.Alias() != "" {
			opts.State.SetAlias("")
		}
	} else {
		picker = widget.NewSelect(opts.Aliases, func(a string) {
			if opts.State != nil {
				opts.State.SetAlias(a)
			}
		})
		// If the previously-selected alias is no longer in the list
		// (account was removed since last render), fall back to the
		// first available alias rather than leaving stale state.
		prev := ""
		if opts.State != nil {
			prev = opts.State.Alias()
		}
		if prev != "" && containsAlias(opts.Aliases, prev) {
			picker.SetSelected(prev)
		} else {
			picker.SetSelected(opts.Aliases[0])
			if opts.State != nil {
				opts.State.SetAlias(opts.Aliases[0])
			}
		}
	}

	// The list renders one row per NavItem, plus an italic group header
	// row whenever the Group changes. We model this by computing a
	// flat row slice up front: each row is either a header (groupRow)
	// or a real nav row pointing at NavItems[i].
	rows := buildSidebarRows(NavItems)

	// Track the currently-selected row so the binder can render an
	// explicit "▸" leading indicator. Fyne's `List.OnSelected` fires
	// after the row is highlighted, but the highlight is subtle on
	// dark themes — the leading caret makes the active row
	// unambiguous and helps users avoid the "Diagnose opens when I
	// click Error Log" misclick by giving them visible click-target
	// feedback before they release the mouse. Slice #213.
	activeRow := -1

	// Row template: a horizontally-padded label inside a container
	// whose MinSize is taller than the bare label. This is the core
	// hit-area fix for Slice #213 — the old template
	// (`widget.NewLabel("template")`) reported a MinSize of ~14px
	// at default density, so adjacent rows shared a 28px painted
	// box but only the top 14px registered the click visually.
	// Padding the label adds ~theme.Padding()*2 vertical breathing
	// room and Fyne's list propagates that to every row uniformly,
	// so users now get a 24-30px-tall click target per row.
	mkTemplate := func() fyne.CanvasObject {
		lbl := widget.NewLabel("template")
		lbl.Truncation = fyne.TextTruncateEllipsis
		return container.New(layout.NewPaddedLayout(), lbl)
	}
	list := widget.NewList(
		func() int { return len(rows) },
		mkTemplate,
		func(i widget.ListItemID, o fyne.CanvasObject) {
			renderSidebarRow(o, rows, i, opts.BadgeFor, i == activeRow)
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(rows) {
			return
		}
		r := rows[i]
		if r.header != "" {
			// Headers aren't selectable destinations — bounce
			// back to the first selectable row that comes after
			// it so the UI never lands on an empty pane.
			for j := i + 1; j < len(rows); j++ {
				if rows[j].header == "" {
					list.Select(j)
					return
				}
			}
			return
		}
		activeRow = i
		list.Refresh() // repaint so the leading "▸" caret moves
		item := NavItems[r.navIdx]
		if opts.State != nil {
			opts.State.SetNav(item.Kind)
		}
		if opts.OnSelectNav != nil {
			opts.OnSelectNav(item)
		}
		// Opening NavErrorLog calls errlog.MarkRead inside
		// BuildErrorLog (synchronously, before this callback
		// returns). MarkRead doesn't fan out on the Subscribe
		// channel, so refresh the list explicitly here so the
		// "(N)" suffix vanishes the instant the view opens.
		// Also reset the toast notifier's quiet-period flag so
		// the next batch of errors fires a fresh toast.
		if item.Kind == NavErrorLog {
			list.Refresh()
			if opts.OnErrorLogOpened != nil {
				opts.OnErrorLogOpened()
			}
		}
	}
	// Pre-select the row matching state.Nav() (or the first nav row).
	preIdx := firstNavRow(rows)
	if opts.State != nil {
		for i, r := range rows {
			if r.header == "" && NavItems[r.navIdx].Kind == opts.State.Nav() {
				preIdx = i
				break
			}
		}
	}
	activeRow = preIdx
	list.Select(preIdx)

	// Live badge refresh: every errlog append (and MarkRead via the
	// view) triggers a list.Refresh so the "(N)" suffix tracks reality
	// without polling. The goroutine exits when the singleton drops
	// the channel — see error_log.go for the same pattern.
	go func() {
		for range opts.BadgeSubscribe() {
			list.Refresh()
		}
	}()

	top := container.NewVBox(
		header,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Account", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		picker,
		widget.NewSeparator(),
	)
	return container.NewBorder(top, nil, nil, nil, list)
}

// defaultBadgeFor is the production BadgeFor — wires NavErrorLog to
// the process-wide `errlog.Unread()` counter and returns 0 for every
// other nav kind.
func defaultBadgeFor(k NavKind) int64 {
	if k == NavErrorLog {
		return errlog.Unread()
	}
	return 0
}

// renderSidebarRow paints one list row. Pulled out so the binder
// closure stays readable and the rendering rules are unit-testable
// via SidebarRowText (sidebar_render.go).
//
// Header rows: italic + bold, prefixed with a thin space + bullet
// so they read as a section divider rather than a missing/empty
// nav entry. Nav rows: plain text, with a leading "▸ " caret on
// the active row to give the user unambiguous click-target
// feedback (Slice #213 misclick fix).
func renderSidebarRow(
	o fyne.CanvasObject,
	rows []sidebarRow,
	i int,
	badgeFor func(NavKind) int64,
	active bool,
) {
	if i < 0 || i >= len(rows) {
		return
	}
	// The template is a *fyne.Container wrapping the label; reach
	// in to pull the label out for SetText / style edits.
	box, ok := o.(*fyne.Container)
	if !ok || len(box.Objects) == 0 {
		return
	}
	lbl, ok := box.Objects[0].(*widget.Label)
	if !ok {
		return
	}
	r := rows[i]
	if r.header != "" {
		lbl.SetText(SidebarRowText(r, NavItems, badgeFor, false))
		lbl.TextStyle = fyne.TextStyle{Italic: true, Bold: true}
	} else {
		lbl.SetText(SidebarRowText(r, NavItems, badgeFor, active))
		lbl.TextStyle = fyne.TextStyle{Bold: active}
	}
	lbl.Refresh()
}

// containsAlias reports whether `a` is present in `aliases`. Local
// helper kept tiny so we avoid importing slices just for this lookup.
// Slice #198 — supports the "previously-selected alias still valid?"
// guard added in NewSidebar's account-picker setup.
func containsAlias(aliases []string, a string) bool {
	for _, x := range aliases {
		if x == a {
			return true
		}
	}
	return false
}
