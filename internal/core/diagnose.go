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
//
// Returns errtrace.Result[struct{}] per Delta #2 — the value side is empty
// because Diagnose communicates its results through the streamed `emit`
// callback rather than a single return value.
func Diagnose(alias string, emit func(DiagnoseEvent)) errtrace.Result[struct{}] {
	_, acct, err := loadConfigAndAccount(alias, emit)
	if err != nil {
		return errtrace.Err[struct{}](err)
	}

	mc, err := probeConnect(acct, emit)
	if err != nil {
		return errtrace.Err[struct{}](err)
	}
	defer mc.Close()

	folders, err := listFolders(mc, emit)
	if err != nil {
		return errtrace.Err[struct{}](err)
	}

	stats, err := fetchInbox(mc, emit)
	if err != nil {
		return errtrace.Err[struct{}](err)
	}

	if err := fetchHeaders(mc, stats, emit); err != nil {
		return errtrace.Err[struct{}](err)
	}

	runFolderScanAndSummarize(mc, folders, stats, emit)
	return errtrace.Ok(struct{}{})
}

// loadConfigAndAccount loads config, resolves the account, and initializes emit.
// Errors are returned as *Coded so the surrounding Diagnose Result carries
// a stable error code (ErrConfigOpen / ErrConfigAccountMissing).
func loadConfigAndAccount(alias string, emit func(DiagnoseEvent)) (*config.Config, config.Account, error) {
	if emit == nil {
		emit = func(DiagnoseEvent) {}
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, config.Account{}, errtrace.WrapCode(err, errtrace.ErrConfigOpen, "core.Diagnose")
	}
	acct, err := resolveAccount(cfg, alias)
	if err != nil {
		return nil, config.Account{}, err
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventStart, Account: &acct})
	return cfg, acct, nil
}

// runFolderScanAndSummarize scans all folders and emits the summary event.
func runFolderScanAndSummarize(mc *mailclient.Client, folders []mailclient.MailboxName, stats mailclient.MailboxStats, emit func(DiagnoseEvent)) {
	foundElsewhere := scanFolders(mc, folders, stats, emit)
	emit(DiagnoseEvent{Kind: DiagnoseEventSummary, Message: summarize(stats, foundElsewhere)})
}

// resolveAccount picks the target account from config by alias (or first if empty).
func resolveAccount(cfg *config.Config, alias string) (config.Account, error) {
	if len(cfg.Accounts) == 0 {
		return config.Account{}, errtrace.WrapCode(
			errtrace.New("no accounts configured. Run `email-read add` first"),
			errtrace.ErrConfigAccountMissing, "core.Diagnose")
	}
	if alias == "" {
		return cfg.Accounts[0], nil
	}
	p := cfg.FindAccount(alias)
	if p == nil {
		return config.Account{}, errtrace.WrapCode(
			errtrace.New(fmt.Sprintf("no account with alias %q", alias)),
			errtrace.ErrConfigAccountMissing, "core.Diagnose").WithContext("alias", alias)
	}
	return *p, nil
}

// probeConnect dials IMAP and emits login_ok on success.
func probeConnect(acct config.Account, emit func(DiagnoseEvent)) (*mailclient.Client, error) {
	mc, err := mailclient.Dial(acct)
	if err != nil {
		return nil, errtrace.WrapCode(err, errtrace.ErrMailDial, "core.Diagnose").
			WithContext("alias", acct.Alias)
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventLoginOK})
	return mc, nil
}

// listFolders fetches mailboxes and emits the appropriate event.
func listFolders(mc *mailclient.Client, emit func(DiagnoseEvent)) ([]mailclient.MailboxName, error) {
	folders, err := mc.ListMailboxes()
	switch {
	case err != nil:
		emit(DiagnoseEvent{Kind: DiagnoseEventFoldersWarn, Message: err.Error()})
		return nil, errtrace.WrapCode(err, errtrace.ErrMailFetchUid, "core.Diagnose.listFolders")
	case len(folders) == 0:
		emit(DiagnoseEvent{Kind: DiagnoseEventFoldersWarn, Message: "server returned no folders"})
		return nil, errtrace.WrapCode(errtrace.New("no folders"),
			errtrace.ErrMailFetchUid, "core.Diagnose.listFolders")
	default:
		emit(DiagnoseEvent{Kind: DiagnoseEventFolders, Folders: folders})
		return folders, nil
	}
}

// fetchInbox selects the inbox and emits its stats.
func fetchInbox(mc *mailclient.Client, emit func(DiagnoseEvent)) (mailclient.MailboxStats, error) {
	stats, err := mc.SelectInbox()
	if err != nil {
		return mailclient.MailboxStats{}, errtrace.WrapCode(err,
			errtrace.ErrMailFetchUid, "core.Diagnose.fetchInbox")
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventInbox, Stats: &stats})
	return stats, nil
}

// fetchHeaders retrieves recent headers and emits them.
func fetchHeaders(mc *mailclient.Client, stats mailclient.MailboxStats, emit func(DiagnoseEvent)) error {
	headers, err := mc.FetchRecentHeaders(stats, 10)
	if err != nil {
		return errtrace.WrapCode(err, errtrace.ErrMailFetchUid, "core.Diagnose.fetchHeaders")
	}
	emit(DiagnoseEvent{Kind: DiagnoseEventHeaders, Headers: headers})
	return nil
}

// scanFolders iterates over all folders, emitting per-folder stats and tracking if mail exists elsewhere.
func scanFolders(mc *mailclient.Client, folders []mailclient.MailboxName, inboxStats mailclient.MailboxStats, emit func(DiagnoseEvent)) bool {
	foundElsewhere := false
	for _, f := range folders {
		fs, err := mc.SelectMailbox(f.Name)
		if err != nil {
			emit(DiagnoseEvent{Kind: DiagnoseEventFolderWarn, Message: fmt.Sprintf("%s: %v", f.Name, err)})
			continue
		}
		emit(DiagnoseEvent{Kind: DiagnoseEventFolderStat, Stats: &fs})
		if fs.Name != inboxStats.Name && fs.Messages > 0 {
			foundElsewhere = true
		}
	}
	return foundElsewhere
}

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
