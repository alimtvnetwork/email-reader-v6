//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/ui/theme"
)

// AccountsOptions wires the Accounts table to its data + side effects.
// All fields are optional with sensible defaults so existing callers
// (app.go's viewFor) keep working.
//
// `LoadHealth` is the per-row health badge feed (Slice #112). Returns
// a per-alias map so the row builder can do an O(1) lookup. When nil,
// the Accounts table renders the column with "— Unknown" placeholders
// rather than skipping it — keeps column alignment stable across the
// "no runtime / store unavailable" degraded path.
type AccountsOptions struct {
	List              func() errtrace.Result[[]config.Account]
	WatchState        func(ctx context.Context, alias string) (store.WatchState, error)
	Remove            func(alias string) errtrace.Result[struct{}]
	LoadHealth        func(ctx context.Context) map[string]core.HealthLevel
	OnAccountsChanged func() // fired after a successful Edit / Delete
}

func BuildAccounts(opts AccountsOptions) fyne.CanvasObject {
	if opts.List == nil {
		opts.List = core.ListAccounts
	}
	if opts.WatchState == nil {
		opts.WatchState = loadWatchState
	}
	if opts.Remove == nil {
		opts.Remove = core.RemoveAccount
	}

	heading := widget.NewLabelWithStyle("Accounts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Edit or delete an account inline. Add via Tools → Add account.")
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	body := container.NewVBox()

	var reload func()
	reload = func() {
		r := opts.List()
		if r.HasError() {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("⚠ Failed to load accounts: " + r.Error().Error()),
			}
			body.Refresh()
			return
		}
		accts := r.Value()
		if len(accts) == 0 {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("No accounts configured. Add one from Tools → Add account."),
			}
			body.Refresh()
			status.SetText("0 accounts.")
			return
		}

		// 6 evenly-distributed data columns + Actions pinned right at
		// intrinsic width via container.NewBorder. Previously the row
		// used GridWithColumns(7) which gave Actions a full 1/7 of the
		// window — on a 1280-wide canvas that meant ~180 px of empty
		// space inside the Actions cell, making Edit / Delete look
		// disproportionately huge (user feedback 2026-04-27).
		dataHeader := container.NewGridWithColumns(6,
			widget.NewLabelWithStyle("Alias", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Email", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Server", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Mailbox", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Last UID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Health", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)
		actionsHeader := widget.NewLabelWithStyle("Actions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		// Reserve a fixed-width slot for the Actions header so it lines
		// up with the per-row buttons (~200 px = pencil-Edit + trash-Delete
		// + inter-button padding at default density).
		actionsHeaderCell := container.New(&fixedWidthLayout{width: actionsColumnWidth}, actionsHeader)
		header := container.NewBorder(nil, nil, nil, actionsHeaderCell, dataHeader)
		rows := []fyne.CanvasObject{header, widget.NewSeparator()}


		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Per-row health is loaded once per reload (not per row) so a
		// 50-account user pays one query, not 50. Nil-safe: empty map
		// means every row gets "— Unknown".
		var health map[string]core.HealthLevel
		if opts.LoadHealth != nil {
			health = opts.LoadHealth(ctx)
		}

		for _, a := range accts {
			ws, _ := opts.WatchState(ctx, a.Alias)
			rows = append(rows, accountRow(a, ws, health[a.Alias], opts, status, reload))
		}
		body.Objects = rows
		body.Refresh()
		status.SetText(fmt.Sprintf("%d account(s).", len(accts)))
	}
	reload()

	refreshBtn := widget.NewButton("Refresh", reload)
	scroll := container.NewVScroll(body)
	// Border layout: refresh button pinned left, status label fills the
	// remaining horizontal space. Using NewHBox here would give `status`
	// zero width and its TextWrapWord setting would break each character
	// onto its own line ("0\na\nc\nc\no\nu\nn\nt\ns").
	footer := container.NewBorder(nil, nil, refreshBtn, nil, status)
	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), footer),
		nil, nil,
		scroll,
	)
}

func accountRow(a config.Account, ws store.WatchState, health core.HealthLevel, opts AccountsOptions, status *widget.Label, reload func()) fyne.CanvasObject {
	mailbox, lastUid := accountRowMeta(a, ws)
	dataRow := accountRowDataGrid(a, mailbox, lastUid, health)
	actions := accountRowActions(a, opts, status, reload)
	return container.NewBorder(nil, nil, nil, actions, dataRow)
}

// accountRowMeta normalises the mailbox label (defaulting to INBOX) and
// composes the two-line "last UID + last received timestamp" string.
// Extracted from accountRow to keep that function under the 15-statement
// linter budget (AC-PROJ-20).
func accountRowMeta(a config.Account, ws store.WatchState) (string, string) {
	mailbox := a.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	lastUid := LastSeenLabel(ws.LastUid)
	if !ws.LastReceivedAt.IsZero() {
		lastUid += "\n" + ws.LastReceivedAt.UTC().Format("2006-01-02 15:04")
	}
	return mailbox, lastUid
}

// accountRowDataGrid builds the 6-column data grid (alias / email / server
// / mailbox / last-seen / health). Server + email labels wrap on word so
// long values stay legible without forcing a wider column.
func accountRowDataGrid(a config.Account, mailbox, lastUid string, health core.HealthLevel) fyne.CanvasObject {
	server := widget.NewLabel(AccountServer(a))
	server.Wrapping = fyne.TextWrapBreak
	email := widget.NewLabel(a.Email)
	email.Wrapping = fyne.TextWrapBreak
	// Per-row health badge — pure-Go formatter keeps the glyph set in sync
	// with the dashboard rollup.
	healthLabel := widget.NewLabel(formatAccountHealthBadge(health))
	return container.NewGridWithColumns(6,
		widget.NewLabel(a.Alias),
		email,
		server,
		widget.NewLabel(mailbox),
		widget.NewLabel(lastUid),
		healthLabel,
	)
}

// accountRowActions builds the icon-led Edit/Delete button pair, wrapped
// in a fixedWidthLayout so per-row alignment with the header column stays
// stable regardless of how Fyne measures the icons.
func accountRowActions(a config.Account, opts AccountsOptions, status *widget.Label, reload func()) fyne.CanvasObject {
	editBtn := widget.NewButtonWithIcon("Edit", theme.IconEdit(),
		func() { openEditAccountDialog(a, opts, status, reload) })
	editBtn.Importance = widget.MediumImportance
	delBtn := widget.NewButtonWithIcon("Delete", theme.IconDelete(),
		func() { confirmDeleteAccount(a, opts, status, reload) })
	delBtn.Importance = widget.DangerImportance
	return container.New(&fixedWidthLayout{width: actionsColumnWidth},
		container.NewHBox(editBtn, delBtn))
}

// openEditAccountDialog shows the Add Account form in edit mode inside a
// modal dialog. On successful Update the dialog closes, the page-level
// status label is updated with a success banner (so the user gets
// visible confirmation that Update did something — the dialog
// disappearing on its own was being read as "nothing happened"), and
// the table reloads via OnAccountsChanged + the local reload.
//
// Dialog size 640×560 (was 560×480) so the inline status banner inside
// the form — used for validation errors like "⚠ email is required" —
// stays visible without scrolling. The form has 9 rows + a separator +
// the actions row + status, and the previous height was clipping the
// status into the dialog chrome.
func openEditAccountDialog(a config.Account, opts AccountsOptions, status *widget.Label, reload func()) {
	parent := currentParentWindow()
	if parent == nil {
		status.SetText("⚠ Cannot open Edit dialog: no parent window.")
		return
	}
	var d dialog.Dialog
	form := BuildAddAccountForm(AddAccountFormOptions{
		Initial: &a,
		OnSaved: func() {
			if d != nil {
				d.Hide()
			}
			// Surface the result on the page-level status label so the
			// user sees confirmation after the modal closes. Without
			// this the only feedback was the in-dialog "✓ Saved …"
			// banner, which the user never read because the dialog
			// auto-hid the same tick.
			status.SetText("✓ Updated account " + a.Alias)
			if opts.OnAccountsChanged != nil {
				opts.OnAccountsChanged()
			}
			reload()
		},
	})
	d = dialog.NewCustom("Edit account: "+a.Alias, "Close", form, parent)
	d.Resize(fyne.NewSize(640, 560))
	d.Show()
}

// confirmDeleteAccount shows a yes/no confirm before calling RemoveAccount.
// On success the table reloads via OnAccountsChanged + the local reload.
func confirmDeleteAccount(a config.Account, opts AccountsOptions, status *widget.Label, reload func()) {
	parent := currentParentWindow()
	if parent == nil {
		status.SetText("⚠ Cannot open Delete confirm: no parent window.")
		return
	}
	msg := fmt.Sprintf("Permanently remove account %q (%s)? This cannot be undone.", a.Alias, a.Email)
	dialog.ShowConfirm("Delete account", msg, func(yes bool) {
		if !yes {
			return
		}
		r := opts.Remove(a.Alias)
		if r.HasError() {
			status.SetText("⚠ Delete failed: " + r.Error().Error())
			return
		}
		status.SetText("✓ Removed account " + a.Alias)
		if opts.OnAccountsChanged != nil {
			opts.OnAccountsChanged()
		}
		reload()
	}, parent)
}

// currentParentWindow finds an open Fyne window to parent dialogs to.
// Returns nil when called outside a running app (e.g. headless tests).
func currentParentWindow() fyne.Window {
	app := fyne.CurrentApp()
	if app == nil {
		return nil
	}
	wins := app.Driver().AllWindows()
	if len(wins) == 0 {
		return nil
	}
	return wins[0]
}

func loadWatchState(ctx context.Context, alias string) (store.WatchState, error) {
	st, err := store.Open()
	if err != nil {
		return store.WatchState{Alias: alias}, err
	}
	defer st.Close()
	return st.GetWatchState(ctx, alias)
}
