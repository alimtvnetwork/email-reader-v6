//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
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

		header := container.NewGridWithColumns(7,
			widget.NewLabelWithStyle("Alias", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Email", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Server", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Mailbox", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Last UID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Health", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Actions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)
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
	mailbox := a.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	lastUid := LastSeenLabel(ws.LastUid)
	if !ws.LastReceivedAt.IsZero() {
		lastUid += "\n" + ws.LastReceivedAt.UTC().Format("2006-01-02 15:04")
	}

	server := widget.NewLabel(AccountServer(a))
	server.Wrapping = fyne.TextWrapBreak
	email := widget.NewLabel(a.Email)
	email.Wrapping = fyne.TextWrapBreak

	// Per-row health badge — uses pure-Go formatter so the glyph set
	// stays in sync with the dashboard rollup. Importance hint nudges
	// the Fyne theme into rendering Error rows in red without
	// hard-coding a colour (theme-respecting; survives dark/light).
	healthLabel := widget.NewLabel(formatAccountHealthBadge(health))

	// Icon-led action buttons keep the column compact and visually
	// consistent (Edit = pencil, Delete = trash). Importance hints
	// recolor the buttons via the active theme so they stay legible
	// in light + dark modes without hard-coding HSL.
	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(),
		func() { openEditAccountDialog(a, opts, status, reload) })
	editBtn.Importance = widget.MediumImportance
	delBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(),
		func() { confirmDeleteAccount(a, opts, status, reload) })
	delBtn.Importance = widget.DangerImportance
	// GridWithColumns(2) gives both buttons identical width so the
	// Actions column reads as one tidy pair instead of the uneven
	// HBox the user saw in the screenshot.
	actions := container.NewGridWithColumns(2, editBtn, delBtn)

	return container.NewGridWithColumns(7,
		widget.NewLabel(a.Alias),
		email,
		server,
		widget.NewLabel(mailbox),
		widget.NewLabel(lastUid),
		healthLabel,
		actions,
	)
}

// openEditAccountDialog shows the Add Account form in edit mode inside a
// modal dialog. On successful Update the dialog closes and the table
// reloads via OnAccountsChanged + the local reload.
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
			if opts.OnAccountsChanged != nil {
				opts.OnAccountsChanged()
			}
			reload()
		},
	})
	d = dialog.NewCustom("Edit account: "+a.Alias, "Close", form, parent)
	d.Resize(fyne.NewSize(560, 480))
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
