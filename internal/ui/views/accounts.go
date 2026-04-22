//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/store"
)

type AccountsOptions struct {
	List       func() ([]config.Account, error)
	WatchState func(ctx context.Context, alias string) (store.WatchState, error)
}

func BuildAccounts(opts AccountsOptions) fyne.CanvasObject {
	if opts.List == nil {
		opts.List = core.ListAccounts
	}
	if opts.WatchState == nil {
		opts.WatchState = loadWatchState
	}

	heading := widget.NewLabelWithStyle("Accounts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Read-only summary. Add or remove via the Tools view.")
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	body := container.NewVBox()

	reload := func() {
		accts, err := opts.List()
		if err != nil {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("⚠ Failed to load accounts: " + err.Error()),
			}
			body.Refresh()
			return
		}
		if len(accts) == 0 {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("No accounts configured. Add one from Tools → Add account."),
			}
			body.Refresh()
			status.SetText("0 accounts.")
			return
		}

		header := container.NewGridWithColumns(5,
			widget.NewLabelWithStyle("Alias", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Email", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Server", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Mailbox", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Last UID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)
		rows := []fyne.CanvasObject{header, widget.NewSeparator()}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, a := range accts {
			ws, _ := opts.WatchState(ctx, a.Alias)
			rows = append(rows, accountRow(a, ws))
		}
		body.Objects = rows
		body.Refresh()
		status.SetText(fmt.Sprintf("%d account(s).", len(accts)))
	}
	reload()

	refreshBtn := widget.NewButton("Refresh", reload)
	scroll := container.NewVScroll(body)
	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), container.NewHBox(refreshBtn, status)),
		nil, nil,
		scroll,
	)
}

func accountRow(a config.Account, ws store.WatchState) fyne.CanvasObject {
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

	return container.NewGridWithColumns(5,
		widget.NewLabel(a.Alias),
		email,
		server,
		widget.NewLabel(mailbox),
		widget.NewLabel(lastUid),
	)
}

func loadWatchState(ctx context.Context, alias string) (store.WatchState, error) {
	st, err := store.Open()
	if err != nil {
		return store.WatchState{Alias: alias}, err
	}
	defer st.Close()
	return st.GetWatchState(ctx, alias)
}
