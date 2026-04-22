// Package core: diagnose.go runs an IMAP connectivity probe for one account
// and emits structured events so both the CLI and the Fyne UI can render the
// progress in their own style.
package core

import (
	"fmt"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
)

// DiagnoseEventKind classifies a step in the diagnose flow. Consumers switch
// on the kind to render a friendly UI; payload fields are filled accordingly.
type DiagnoseEventKind string

const (
	DiagnoseEventStart       DiagnoseEventKind = "start"        // probe started; Account populated
	DiagnoseEventLoginOK     DiagnoseEventKind = "login_ok"     // IMAP login succeeded
	DiagnoseEventFolders     DiagnoseEventKind = "folders"      // server folder list; Folders populated
	DiagnoseEventFoldersWarn DiagnoseEventKind = "folders_warn" // folder list returned an error/empty; Message populated
	DiagnoseEventInbox       DiagnoseEventKind = "inbox_stats"  // configured mailbox stats; Stats populated
	DiagnoseEventHeaders     DiagnoseEventKind = "headers"      // recent headers; Headers populated (may be empty)
	DiagnoseEventFolderStat  DiagnoseEventKind = "folder_stat"  // per-folder stats during scan; Stats populated
	DiagnoseEventFolderWarn  DiagnoseEventKind = "folder_warn"  // failed to select one folder; Message populated
	DiagnoseEventSummary     DiagnoseEventKind = "summary"      // final human summary; Message populated
)

// DiagnoseEvent is one observable step from the diagnose flow.
type DiagnoseEvent struct {
	Kind    DiagnoseEventKind
	Account *config.Account            // populated on Start
	Folders []mailclient.MailboxName   // populated on Folders
	Stats   *mailclient.MailboxStats   // populated on Inbox / FolderStat
	Headers []mailclient.HeaderSummary // populated on Headers
	Message string                     // human-readable text for warn/summary
}

// Diagnose runs the IMAP probe for the given alias. When alias is empty the
// first configured account is used. Each step is reported via emit so the
// caller (CLI / UI) can render incrementally. emit may be nil.
func Diagnose(alias string, emit func(DiagnoseEvent)) error {
	if emit == nil {
		emit = func(DiagnoseEvent) {}
	}
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if len(cfg.Accounts) == 0 {
		return errtrace.New("no accounts configured. Run `email-read add` first")
	}

	var acct config.Account
	if alias == "" {
		acct = cfg.Accounts[0]
	} else {
		p := cfg.FindAccount(alias)
		if p == nil {
			return errtrace.New(fmt.Sprintf("no account with alias %q", alias))
		}
		acct = *p
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventStart, Account: &acct})

	mc, err := mailclient.Dial(acct)
	if err != nil {
		return errtrace.Wrap(err, "dial imap")
	}
	defer mc.Close()
	emit(DiagnoseEvent{Kind: DiagnoseEventLoginOK})

	folders, err := mc.ListMailboxes()
	switch {
	case err != nil:
		emit(DiagnoseEvent{Kind: DiagnoseEventFoldersWarn, Message: err.Error()})
	case len(folders) == 0:
		emit(DiagnoseEvent{Kind: DiagnoseEventFoldersWarn, Message: "server returned no folders"})
	default:
		emit(DiagnoseEvent{Kind: DiagnoseEventFolders, Folders: folders})
	}

	stats, err := mc.SelectInbox()
	if err != nil {
		return errtrace.Wrap(err, "select inbox")
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventInbox, Stats: &stats})

	headers, err := mc.FetchRecentHeaders(stats, 10)
	if err != nil {
		return errtrace.Wrap(err, "fetch recent headers")
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventHeaders, Headers: headers})

	foundElsewhere := false
	for _, f := range folders {
		fs, err := mc.SelectMailbox(f.Name)
		if err != nil {
			emit(DiagnoseEvent{Kind: DiagnoseEventFolderWarn, Message: fmt.Sprintf("%s: %v", f.Name, err)})
			continue
		}
		emit(DiagnoseEvent{Kind: DiagnoseEventFolderStat, Stats: &fs})
		if fs.Name != stats.Name && fs.Messages > 0 {
			foundElsewhere = true
		}
	}

	emit(DiagnoseEvent{Kind: DiagnoseEventSummary, Message: summarize(stats, foundElsewhere)})
	return nil
}

// summarize produces the same human-readable diagnosis the CLI used to print.
func summarize(stats mailclient.MailboxStats, foundElsewhere bool) string {
	if stats.Messages <= 1 && stats.UidNext <= 2 {
		base := "The IMAP server is still exposing only the baseline message in the configured mailbox."
		if foundElsewhere {
			return base + " Other folders contain messages; the new email may be in Spam/Junk/Sent/All Mail instead of INBOX."
		}
		return base + " No other listed folder showed obvious new mail either; this points to mail routing/delivery before IMAP."
	}
	return "The configured mailbox has more mail than the watcher baseline; run `email-read watch <alias>` to process UID > LastUid."
}
